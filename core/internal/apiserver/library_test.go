package apiserver_test

import (
	"context"
	"strconv"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

func TestGetLibrary_ReturnsFilteredPagedLibrary(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)

	entries := make([]*corev1.LibraryEntry, 0, 105)
	meta := map[string]*corev1.TitleMetadata{}
	for i := 0; i < 105; i++ {
		id := "entry-" + strconv.Itoa(i)
		entries = append(entries, &corev1.LibraryEntry{
			Id:                 id,
			ExternalIdentities: []*corev1.ExternalIdentity{{Namespace: "imdb", Value: "tt" + strconv.Itoa(1000+i)}},
			MetadataRef:        "ref-" + strconv.Itoa(i),
		})
		title := "Film " + strconv.Itoa(i)
		if i%5 == 0 {
			title = "The Matrix " + strconv.Itoa(i)
		}
		meta["ref-"+strconv.Itoa(i)] = &corev1.TitleMetadata{Title: title}
	}

	stores := testStores(t)
	lib := &stubLibrary{entries: entries, meta: meta}
	srv := apiserver.NewServer(bus, stores, authenticator, session)
	srv.SetLibrary(lib)
	ctx := ctxAs(session, "user-1")

	// Page 1: the first libraryPageSize entries, and a continuation token.
	page1, err := srv.GetLibrary(ctx, &apiv1.GetLibraryRequest{})
	if err != nil {
		t.Fatalf("GetLibrary page1: %v", err)
	}
	if len(page1.GetItems()) != 100 {
		t.Fatalf("page1 items = %d, want 100", len(page1.GetItems()))
	}
	if page1.GetNextPageToken() != "100" {
		t.Fatalf("next_page_token = %q, want 100", page1.GetNextPageToken())
	}
	if page1.GetItems()[0].GetEntry().GetId() != "entry-0" {
		t.Fatalf("first entry = %q", page1.GetItems()[0].GetEntry().GetId())
	}

	// Page 2 drains the tail and terminates the pagination.
	page2, err := srv.GetLibrary(ctx, &apiv1.GetLibraryRequest{PageToken: "100"})
	if err != nil {
		t.Fatalf("GetLibrary page2: %v", err)
	}
	if len(page2.GetItems()) != 5 {
		t.Fatalf("page2 items = %d, want 5", len(page2.GetItems()))
	}
	if page2.GetNextPageToken() != "" {
		t.Fatalf("page2 next_page_token = %q, want empty", page2.GetNextPageToken())
	}

	// A query narrows to matching titles (the display surface).
	q, err := srv.GetLibrary(ctx, &apiv1.GetLibraryRequest{Query: "matrix 100"})
	if err != nil {
		t.Fatalf("GetLibrary query: %v", err)
	}
	if len(q.GetItems()) != 1 || q.GetItems()[0].GetEntry().GetId() != "entry-100" {
		t.Fatalf("query items = %d, want [entry-100]", len(q.GetItems()))
	}
	// Identity values and ids also match.
	if e, err := srv.GetLibrary(ctx, &apiv1.GetLibraryRequest{Query: "tt1001"}); err != nil || len(e.GetItems()) != 1 {
		t.Fatalf("identity query: items=%d err=%v", len(e.GetItems()), err)
	}
	if e, err := srv.GetLibrary(ctx, &apiv1.GetLibraryRequest{Query: "entry-104"}); err != nil || len(e.GetItems()) != 1 {
		t.Fatalf("id query: items=%d err=%v", len(e.GetItems()), err)
	}

	// No match is an empty page, not an error.
	if e, err := srv.GetLibrary(ctx, &apiv1.GetLibraryRequest{Query: "zzz"}); err != nil || len(e.GetItems()) != 0 {
		t.Fatalf("no-match query: items=%d err=%v", len(e.GetItems()), err)
	}
}

func TestGetLibrary_UnconfiguredAndBadToken(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)

	if _, err := apiserver.NewServer(bus, testStores(t), authenticator, session).
		GetLibrary(context.Background(), &apiv1.GetLibraryRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("unarmed library code = %v, want Unavailable", status.Code(err))
	}

	lib := &stubLibrary{entries: []*corev1.LibraryEntry{{Id: "entry-1"}}}
	armed := apiserver.NewServer(bus, testStores(t), authenticator, session)
	armed.SetLibrary(lib)
	if _, err := armed.GetLibrary(context.Background(), &apiv1.GetLibraryRequest{PageToken: "not-a-number"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("bad page token code = %v, want InvalidArgument", status.Code(err))
	}
}
