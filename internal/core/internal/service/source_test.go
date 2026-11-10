package service

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sourcev1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/source/v1"
	"github.com/Nem-git/ABCMovies/internal/core/internal/audit"
	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

// fakeVerifier is a deterministic stand-in for the jellyfin connector in
// core tests.
type fakeVerifier struct {
	mu        sync.Mutex
	result    *jellyfin.ServerInfo
	vErr      *jellyfin.Error
	calls     int
	endpoints []string
	usernames []string
	apiKeys   []string
}

func (f *fakeVerifier) Verify(_ context.Context, endpoint string, creds jellyfin.Credentials) (*jellyfin.ServerInfo, *jellyfin.Error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.endpoints = append(f.endpoints, endpoint)
	if creds.Password != nil {
		f.usernames = append(f.usernames, creds.Password.Username)
	}
	f.apiKeys = append(f.apiKeys, creds.APIKey)
	if f.vErr != nil {
		return nil, f.vErr
	}
	if f.result != nil {
		return f.result, nil
	}
	// Default: echo a deterministic server identity.
	base := strings.TrimSuffix(endpoint, "/")
	base = strings.TrimPrefix(base, "https://")
	base = strings.TrimPrefix(base, "http://")
	return &jellyfin.ServerInfo{
		Endpoint:    strings.TrimRight(endpoint, "/"),
		VerifiedID:  "jf-test-" + strings.ReplaceAll(base, ".", "_"),
		DisplayName: "Test Server " + base,
		Version:     "10.0.0",
	}, nil
}

func newSourceTestService(t *testing.T, v sourceVerifier) (*SourceService, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	aud, err := audit.Open(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatalf("audit.Open: %v", err)
	}
	t.Cleanup(func() { _ = aud.Close() })
	return NewSourceServiceWithVerifier(st, v, NewSessionState(), aud), st
}

func connectPassword(t *testing.T, svc *SourceService, ctx context.Context, endpoint, displayName string) *sourcev1.ConnectSourceResponse {
	t.Helper()
	resp, err := svc.ConnectSource(ctx, &sourcev1.ConnectSourceRequest{
		ActorUserId: "usr_admin",
		ConnectorId: "jellyfin",
		Endpoint:    endpoint,
		DisplayName: displayName,
		Credentials: &sourcev1.ConnectSourceRequest_Password{
			Password: &sourcev1.PasswordCredentials{Username: "admin", Password: "correct horse"},
		},
	})
	if err != nil {
		t.Fatalf("ConnectSource: %v", err)
	}
	return resp
}

func TestConnectSourcePasswordConnected(t *testing.T) {
	svc, st := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()

	resp := connectPassword(t, svc, ctx, "https://media.example.test", "")
	if resp.GetOutcome() != sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_CONNECTED {
		t.Fatalf("outcome = %v, want CONNECTED", resp.GetOutcome())
	}
	s := resp.GetSource()
	if s.GetSourceId() == "" || !strings.HasPrefix(s.GetSourceId(), "src_") {
		t.Fatalf("source_id = %q", s.GetSourceId())
	}
	if s.GetVerifiedExternalId() == "" {
		t.Fatal("verified_external_id is empty")
	}
	if s.GetEndpoint() != "https://media.example.test" {
		t.Fatalf("endpoint = %q, want normalized", s.GetEndpoint())
	}
	if s.GetDisplayName() == "" {
		t.Fatal("display_name is empty (expected server-provided name)")
	}
	if s.GetStatus() != sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE {
		t.Fatalf("status = %v, want ACTIVE", s.GetStatus())
	}
	// Persisted and retrievable by identity.
	row, err := st.GetSourceByVerifiedIdentity(ctx, "jellyfin", s.GetVerifiedExternalId())
	if err != nil {
		t.Fatal(err)
	}
	if row.SourceID != s.GetSourceId() || row.CreatedBy != "usr_admin" {
		t.Fatalf("store row = %+v", row)
	}
	// No credential material persisted.
	if strings.Contains(row.DisplayName, "correct horse") || row.Endpoint == "" {
		t.Fatalf("unexpected persisted data: %+v", row)
	}
}

func TestConnectSourceAPIKey(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	resp, err := svc.ConnectSource(ctx, &sourcev1.ConnectSourceRequest{
		ActorUserId: "usr_admin",
		ConnectorId: "jellyfin",
		Endpoint:    "https://media.example.test",
		Credentials: &sourcev1.ConnectSourceRequest_ApiKey{ApiKey: "secret-api-key"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetOutcome() != sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_CONNECTED {
		t.Fatalf("outcome = %v", resp.GetOutcome())
	}
}

func TestConnectSourceDisplayNameOverride(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	resp := connectPassword(t, svc, ctx, "https://media.example.test", "Living Room Jellyfin")
	if resp.GetSource().GetDisplayName() != "Living Room Jellyfin" {
		t.Fatalf("display_name = %q, want override", resp.GetSource().GetDisplayName())
	}
}

func TestConnectSourceDuplicate(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()

	first := connectPassword(t, svc, ctx, "https://media.example.test", "")
	second, err := svc.ConnectSource(ctx, &sourcev1.ConnectSourceRequest{
		ActorUserId: "usr_admin",
		ConnectorId: "jellyfin",
		Endpoint:    "https://media.example.test/", // same server, different URL form
		Credentials: &sourcev1.ConnectSourceRequest_Password{
			Password: &sourcev1.PasswordCredentials{Username: "other", Password: "irrelevant"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.GetOutcome() != sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_DUPLICATE {
		t.Fatalf("outcome = %v, want DUPLICATE", second.GetOutcome())
	}
	if second.GetSource().GetSourceId() != first.GetSource().GetSourceId() {
		t.Fatalf("duplicate returned different source: %q vs %q", second.GetSource().GetSourceId(), first.GetSource().GetSourceId())
	}
}

func TestConnectSourceDuplicateBySeededRow(t *testing.T) {
	v := &fakeVerifier{
		result: &jellyfin.ServerInfo{
			Endpoint: "https://seeded.example.test", VerifiedID: "jf-seeded-id", DisplayName: "Seeded", Version: "10.0.0",
		},
	}
	svc, st := newSourceTestService(t, v)
	ctx := context.Background()

	row, err := st.CreateSource(ctx, store.Source{
		ConnectorID: "jellyfin", VerifiedExternalID: "jf-seeded-id",
		Endpoint: "https://seeded.example.test", DisplayName: "Original", Status: 1, CreatedBy: "usr_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := svc.ConnectSource(ctx, &sourcev1.ConnectSourceRequest{
		ActorUserId: "usr_admin",
		ConnectorId: "jellyfin",
		Endpoint:    "https://seeded.example.test",
		Credentials: &sourcev1.ConnectSourceRequest_Password{
			Password: &sourcev1.PasswordCredentials{Username: "admin", Password: "correct horse"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetOutcome() != sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_DUPLICATE {
		t.Fatalf("outcome = %v, want DUPLICATE", resp.GetOutcome())
	}
	if resp.GetSource().GetSourceId() != row.SourceID {
		t.Fatalf("duplicate returned %q, want seeded source %q", resp.GetSource().GetSourceId(), row.SourceID)
	}
}

func TestConnectSourceSameServerTwiceReturnsExistingSource(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()

	first := connectPassword(t, svc, ctx, "https://media.example.test", "")
	second, err := svc.ConnectSource(ctx, &sourcev1.ConnectSourceRequest{
		ActorUserId: "usr_admin",
		ConnectorId: "jellyfin",
		Endpoint:    "https://media.example.test",
		Credentials: &sourcev1.ConnectSourceRequest_ApiKey{ApiKey: "another-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.GetOutcome() != sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_DUPLICATE {
		t.Fatalf("outcome = %v, want DUPLICATE", second.GetOutcome())
	}
	if second.GetSource().GetSourceId() != first.GetSource().GetSourceId() {
		t.Fatalf("duplicate returned different source: %q vs %q", second.GetSource().GetSourceId(), first.GetSource().GetSourceId())
	}
}

func TestConnectSourceUnsupportedConnector(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	_, err := svc.ConnectSource(ctx, &sourcev1.ConnectSourceRequest{
		ActorUserId: "usr_admin",
		ConnectorId: "plex",
		Endpoint:    "https://plex.example.test",
		Credentials: &sourcev1.ConnectSourceRequest_ApiKey{ApiKey: "k"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestConnectSourceValidation(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	cases := []struct {
		name string
		req  *sourcev1.ConnectSourceRequest
		want codes.Code
	}{
		{"missing actor", &sourcev1.ConnectSourceRequest{ConnectorId: "jellyfin", Endpoint: "https://x", Credentials: &sourcev1.ConnectSourceRequest_ApiKey{ApiKey: "k"}}, codes.InvalidArgument},
		{"missing endpoint", &sourcev1.ConnectSourceRequest{ActorUserId: "u", ConnectorId: "jellyfin", Credentials: &sourcev1.ConnectSourceRequest_ApiKey{ApiKey: "k"}}, codes.InvalidArgument},
		{"missing credentials", &sourcev1.ConnectSourceRequest{ActorUserId: "u", ConnectorId: "jellyfin", Endpoint: "https://x"}, codes.InvalidArgument},
		{"long display name", &sourcev1.ConnectSourceRequest{ActorUserId: "u", ConnectorId: "jellyfin", Endpoint: "https://x", DisplayName: strings.Repeat("x", maxDisplayNameLength+1), Credentials: &sourcev1.ConnectSourceRequest_ApiKey{ApiKey: "k"}}, codes.InvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ConnectSource(ctx, tc.req)
			if code := status.Code(err); code != tc.want {
				t.Fatalf("code = %v, want %v", code, tc.want)
			}
		})
	}
}

func TestConnectSourceVerifierErrors(t *testing.T) {
	cases := []struct {
		name string
		ver  *fakeVerifier
		want codes.Code
	}{
		{"unreachable", &fakeVerifier{vErr: &jellyfin.Error{Kind: jellyfin.ErrKindUnreachable, Message: "down", Retryable: true}}, codes.Unavailable},
		{"unauthorized", &fakeVerifier{vErr: &jellyfin.Error{Kind: jellyfin.ErrKindUnauthorized, Message: "bad creds"}}, codes.Unauthenticated},
		{"misconfig", &fakeVerifier{vErr: &jellyfin.Error{Kind: jellyfin.ErrKindMisconfig, Message: "weird"}}, codes.FailedPrecondition},
		{"notfound", &fakeVerifier{vErr: &jellyfin.Error{Kind: jellyfin.ErrKindNotFound, Message: "missing"}}, codes.NotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newSourceTestService(t, tc.ver)
			_, err := svc.ConnectSource(context.Background(), &sourcev1.ConnectSourceRequest{
				ActorUserId: "u", ConnectorId: "jellyfin", Endpoint: "https://media.example.test",
				Credentials: &sourcev1.ConnectSourceRequest_ApiKey{ApiKey: "k"},
			})
			if code := status.Code(err); code != tc.want {
				t.Fatalf("code = %v, want %v", code, tc.want)
			}
		})
	}
}

func TestListAndGetSources(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	connectPassword(t, svc, ctx, "https://a.example.test", "A")
	connectPassword(t, svc, ctx, "https://b.example.test", "B")

	list, err := svc.ListSources(ctx, &sourcev1.ListSourcesRequest{ActorUserId: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetSources()) != 2 {
		t.Fatalf("got %d sources, want 2", len(list.GetSources()))
	}
	for _, s := range list.GetSources() {
		if s.GetSourceId() == "" || s.GetVerifiedExternalId() == "" || s.GetStatus() != sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE {
			t.Fatalf("bad source in list: %+v", s)
		}
	}

	first := list.GetSources()[0]
	got, err := svc.GetSource(ctx, &sourcev1.GetSourceRequest{ActorUserId: "u", SourceId: first.GetSourceId()})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetSource().GetSourceId() != first.GetSourceId() {
		t.Fatalf("GetSource returned %q, want %q", got.GetSource().GetSourceId(), first.GetSourceId())
	}
	if _, err := svc.GetSource(ctx, &sourcev1.GetSourceRequest{ActorUserId: "u", SourceId: "src_missing"}); status.Code(err) != codes.NotFound {
		t.Fatalf("missing code = %v, want NotFound", status.Code(err))
	}
}

func TestUpdateSourceDisplayNameOnly(t *testing.T) {
	svc, st := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	conn := connectPassword(t, svc, ctx, "https://media.example.test", "Old Name")
	srcID := conn.GetSource().GetSourceId()

	updated, err := svc.UpdateSource(ctx, &sourcev1.UpdateSourceRequest{
		ActorUserId: "u", SourceId: srcID, DisplayName: "New Name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := updated.GetSource().GetDisplayName(); got != "New Name" {
		t.Fatalf("display_name = %q", got)
	}
	row, _ := st.GetSource(ctx, srcID)
	if row.Endpoint != conn.GetSource().GetEndpoint() {
		t.Fatalf("endpoint changed unexpectedly: %q", row.Endpoint)
	}
}

func TestUpdateSourceEndpointChangeRequiresCredentials(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	conn := connectPassword(t, svc, ctx, "https://a.example.test", "")
	_, err := svc.UpdateSource(ctx, &sourcev1.UpdateSourceRequest{
		ActorUserId: "u", SourceId: conn.GetSource().GetSourceId(), Endpoint: "https://b.example.test",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestUpdateSourceEndpointChangeReverifiesSameIdentity(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	conn := connectPassword(t, svc, ctx, "https://a.example.test", "My server")
	id := conn.GetSource().GetSourceId()

	updated, err := svc.UpdateSource(ctx, &sourcev1.UpdateSourceRequest{
		ActorUserId: "u",
		SourceId:    id,
		Endpoint:    "https://a.example.test/", // normalized to same
		Credentials: &sourcev1.UpdateSourceRequest_Password{
			Password: &sourcev1.PasswordCredentials{Username: "admin", Password: "correct horse"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetSource().GetEndpoint() != conn.GetSource().GetEndpoint() {
		t.Fatalf("endpoint = %q, want unchanged after normalization", updated.GetSource().GetEndpoint())
	}
}

func TestUpdateSourceReassociationRejected(t *testing.T) {
	v := &fakeVerifier{}
	svc, st := newSourceTestService(t, v)
	ctx := context.Background()
	conn := connectPassword(t, svc, ctx, "https://a.example.test", "My server")
	id := conn.GetSource().GetSourceId()
	v.result = &jellyfin.ServerInfo{
		Endpoint: "https://other.example.test", VerifiedID: "jf-different-server", DisplayName: "Other", Version: "10.0.0",
	}
	_, err := svc.UpdateSource(ctx, &sourcev1.UpdateSourceRequest{
		ActorUserId: "u",
		SourceId:    id,
		Endpoint:    "https://other.example.test",
		Credentials: &sourcev1.UpdateSourceRequest_Password{
			Password: &sourcev1.PasswordCredentials{Username: "admin", Password: "correct horse"},
		},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", status.Code(err))
	}
	row, _ := st.GetSource(ctx, id)
	if row.VerifiedExternalID != conn.GetSource().GetVerifiedExternalId() {
		t.Fatal("verified_external_id changed despite rejected update")
	}
	if row.Endpoint != conn.GetSource().GetEndpoint() {
		t.Fatal("endpoint changed despite rejected update")
	}
}

func TestUpdateSourceNotFound(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	_, err := svc.UpdateSource(ctx, &sourcev1.UpdateSourceRequest{ActorUserId: "u", SourceId: "src_missing", DisplayName: "x"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound", status.Code(err))
	}
}

func TestUpdateSourceDisplayNameTooLong(t *testing.T) {
	svc, _ := newSourceTestService(t, &fakeVerifier{})
	ctx := context.Background()
	conn := connectPassword(t, svc, ctx, "https://a.example.test", "")
	_, err := svc.UpdateSource(ctx, &sourcev1.UpdateSourceRequest{
		ActorUserId: "u", SourceId: conn.GetSource().GetSourceId(),
		DisplayName: strings.Repeat("x", maxDisplayNameLength+1),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}
