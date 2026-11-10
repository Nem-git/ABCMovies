package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
)

func TestLibraryEndpointsRequireSession(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{}, &fakeSourceClient{}, &fakeLibraryClient{})
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/library/source/src_x/sync", `{"credentials":{"api_key":"k"}}`},
		{http.MethodGet, "/api/library", ""},
		{http.MethodGet, "/api/library/search?q=matrix", ""},
		{http.MethodGet, "/api/library/title/ttl_x", ""},
	}
	for _, tc := range tests {
		rr := doJSON(srv.Handler(), tc.method, tc.path, tc.body, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", tc.method, tc.path, rr.Code)
		}
	}
}

func TestLibraryEndpointsRejectInvalidSession(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{
		validateSession: func(_ context.Context, _ *identityv1.ValidateSessionRequest) (*identityv1.ValidateSessionResponse, error) {
			return &identityv1.ValidateSessionResponse{Valid: false}, nil
		},
	}, &fakeSourceClient{}, &fakeLibraryClient{})
	rr := doJSON(srv.Handler(), http.MethodGet, "/api/library", "", sessionCookieHeader("bad"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestLibrarySyncHappyPath(t *testing.T) {
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		syncSource: func(_ context.Context, in *libraryv1.SyncSourceRequest, _ ...grpc.CallOption) (*libraryv1.SyncSourceResponse, error) {
			if in.GetActorUserId() != "usr_admin" {
				t.Errorf("actor = %q, want usr_admin", in.GetActorUserId())
			}
			if in.GetSourceId() != "src_1" {
				t.Errorf("source_id = %q", in.GetSourceId())
			}
			if pw := in.GetPassword(); pw != nil {
				if pw.GetUsername() != "admin" || pw.GetPassword() != "correct horse" {
					t.Errorf("password credentials not forwarded: %+v", pw)
				}
			} else if apiKey := in.GetApiKey(); apiKey != "" {
				if strings.TrimSpace(apiKey) == "" {
					t.Errorf("api key missing")
				}
			} else {
				t.Errorf("no credentials forwarded: %+v", in)
			}
			return &libraryv1.SyncSourceResponse{
				SourceId: "src_1",
				Outcome:  libraryv1.SyncOutcome_SYNC_OUTCOME_SUCCESS,
				State: &libraryv1.SourceSyncState{
					SourceId:          "src_1",
					SourceDisplayName: "Media Server",
					LastOutcome:       libraryv1.SyncOutcome_SYNC_OUTCOME_SUCCESS,
					Detail:            "",
					TitleCount:        2,
				},
			}, nil
		},
	})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/library/source/src_1/sync",
		`{"credentials":{"username":"admin","password":"correct horse"}}`,
		sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body)
	}
	var out syncResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Outcome != "success" || out.State.LastSyncedAt != "" && out.State.TitleCount != 2 {
		t.Fatalf("sync response = %+v", out)
	}
	if strings.Contains(rr.Body.String(), "correct horse") {
		t.Fatal("password echoed in sync response")
	}

	// API-key sync should not be rejected at the edge.
	rr = doJSON(srv.Handler(), http.MethodPost, "/api/library/source/src_1/sync",
		`{"credentials":{"api_key":"very-secret-key"}}`,
		sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("api-key status = %d, want 200, body=%s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), "very-secret-key") {
		t.Fatal("API key echoed in sync response")
	}
}

func TestLibrarySyncPartialAndFailed(t *testing.T) {
	// Partial sync: outcome propagates, credentials still input-only.
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		syncSource: func(_ context.Context, in *libraryv1.SyncSourceRequest, _ ...grpc.CallOption) (*libraryv1.SyncSourceResponse, error) {
			return &libraryv1.SyncSourceResponse{
				SourceId: in.GetSourceId(),
				Outcome:  libraryv1.SyncOutcome_SYNC_OUTCOME_PARTIAL,
				State: &libraryv1.SourceSyncState{
					SourceId:          in.GetSourceId(),
					SourceDisplayName: "Media Server",
					LastOutcome:       libraryv1.SyncOutcome_SYNC_OUTCOME_PARTIAL,
					Detail:            "some items could not be read; partial results",
					TitleCount:        1,
				},
			}, nil
		},
	})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/library/source/src_1/sync",
		`{"credentials":{"api_key":"very-secret-key"}}`, sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rr.Code, rr.Body)
	}
	var out syncResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Outcome != "partial" || out.State.TitleCount != 1 {
		t.Fatalf("partial response = %+v", out)
	}
	if strings.Contains(rr.Body.String(), "very-secret-key") {
		t.Fatal("API key echoed in partial response")
	}

	// Failed sync from core: mapped to 503 with no credentials leaked.
	srvFail := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		syncSource: func(_ context.Context, _ *libraryv1.SyncSourceRequest, _ ...grpc.CallOption) (*libraryv1.SyncSourceResponse, error) {
			return nil, status.Error(codes.Unavailable, "the server could not be reached")
		},
	})
	rr = doJSON(srvFail.Handler(), http.MethodPost, "/api/library/source/src_1/sync",
		`{"credentials":{"api_key":"very-secret-key"}}`, sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body=%s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), "very-secret-key") {
		t.Fatal("API key echoed in failure response")
	}
}

func TestLibraryListAndSearch(t *testing.T) {
	syncState := &libraryv1.SourceSyncState{
		SourceId:          "src_1",
		SourceDisplayName: "Media Server",
		LastOutcome:       libraryv1.SyncOutcome_SYNC_OUTCOME_SUCCESS,
		TitleCount:        1,
	}
	listResp := &libraryv1.ListTitlesResponse{
		Titles: []*libraryv1.Title{
			{TitleId: "ttl_1", SourceId: "src_1", SourceDisplayName: "Media Server", ExternalId: "jf-1", Name: "Blade Runner", Kind: libraryv1.TitleKind_TITLE_KIND_MOVIE, Year: 1982, Availability: libraryv1.AvailabilityState_AVAILABILITY_STATE_AVAILABLE},
		},
		SyncStates: []*libraryv1.SourceSyncState{syncState},
		TotalCount: 1,
	}
	searchResp := &libraryv1.SearchTitlesResponse{
		Titles:     listResp.Titles,
		SyncStates: []*libraryv1.SourceSyncState{syncState},
		TotalCount: 1,
	}
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		listTitles: func(_ context.Context, _ *libraryv1.ListTitlesRequest, _ ...grpc.CallOption) (*libraryv1.ListTitlesResponse, error) {
			return listResp, nil
		},
		searchTitles: func(_ context.Context, _ *libraryv1.SearchTitlesRequest, _ ...grpc.CallOption) (*libraryv1.SearchTitlesResponse, error) {
			return searchResp, nil
		},
	})

	rr := doJSON(srv.Handler(), http.MethodGet, "/api/library", "", sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", rr.Code, rr.Body)
	}
	var out listTitlesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Titles) != 1 || out.Titles[0].TitleID != "ttl_1" {
		t.Fatalf("titles = %+v", out.Titles)
	}
	if len(out.SyncStates) == 0 || out.SyncStates[0].TitleCount != 1 {
		t.Fatalf("sync_states = %+v", out.SyncStates)
	}

	rr = doJSON(srv.Handler(), http.MethodGet, "/api/library/search?q=blade+runner", "", sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("search status = %d", rr.Code)
	}
	var outSearch listTitlesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &outSearch); err != nil {
		t.Fatal(err)
	}
	if len(outSearch.Titles) != 1 {
		t.Fatalf("search titles = %+v", outSearch.Titles)
	}
}

func TestLibraryGetTitleNotFound(t *testing.T) {
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		getTitle: func(_ context.Context, _ *libraryv1.GetTitleRequest, _ ...grpc.CallOption) (*libraryv1.GetTitleResponse, error) {
			return nil, status.Error(codes.NotFound, "title not found")
		},
	})
	rr := doJSON(srv.Handler(), http.MethodGet, "/api/library/title/ttl_nope", "", sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestLibrarySyncValidationAndErrors(t *testing.T) {
	// Empty credentials object: forwarded to the core with no oneof set ("reuse
	// the session cache"). The core decides; the API does not require creds.
	var got *libraryv1.SyncSourceRequest
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		syncSource: func(_ context.Context, in *libraryv1.SyncSourceRequest, _ ...grpc.CallOption) (*libraryv1.SyncSourceResponse, error) {
			got = in
			return &libraryv1.SyncSourceResponse{SourceId: in.GetSourceId(), Outcome: libraryv1.SyncOutcome_SYNC_OUTCOME_SUCCESS}, nil
		},
	})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/library/source/src_1/sync", `{"credentials":{}}`, sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("empty-credentials sync status = %d, want 200; body=%s", rr.Code, rr.Body)
	}
	if got == nil {
		t.Fatal("core never called")
	}
	if got.GetPassword() != nil || got.GetApiKey() != "" {
		t.Fatalf("expected no credentials oneof set (session cache path), got password=%v api_key=%v", got.GetPassword() != nil, got.GetApiKey())
	}
	// Half-filled credentials: 400, not a silent session-cache fallback (the
	// play endpoint already rejects these the same way).
	for _, tc := range []struct {
		name string
		body string
	}{
		{"username only", `{"credentials":{"username":"admin"}}`},
		{"password only", `{"credentials":{"password":"hunter2"}}`},
		{"username with api_key", `{"credentials":{"username":"admin","api_key":"some-key"}}`},
	} {
		got = nil // reset the capture: the core must not be called
		rr = doJSON(srv.Handler(), http.MethodPost, "/api/library/source/src_1/sync", tc.body, sessionCookieHeader("sess_1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400; body=%s", tc.name, rr.Code, rr.Body)
		}
		if got != nil {
			t.Fatalf("%s: core was called; want a local 400 rejection", tc.name)
		}
	}
	// Bad content type -> 415.
	rr = doJSON(srv.Handler(), http.MethodPost, "/api/library/source/src_1/sync", `{}`, map[string]string{"Content-Type": "text/plain", "Cookie": "abcmovies_session=sess_1"})
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("bad content-type status = %d, want 415", rr.Code)
	}
	// Core auth failure -> 401 (not leaking the error text).
	srvAuth := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		syncSource: func(_ context.Context, _ *libraryv1.SyncSourceRequest, _ ...grpc.CallOption) (*libraryv1.SyncSourceResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid")
		},
	})
	rr = doJSON(srvAuth.Handler(), http.MethodPost, "/api/library/source/src_1/sync",
		`{"credentials":{"username":"admin","password":"wrong"}}`, sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", rr.Code)
	}
}

func TestLibraryLimitClamps(t *testing.T) {
	bigLimit := &libraryv1.ListTitlesResponse{}
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{}, &fakeLibraryClient{
		listTitles: func(_ context.Context, in *libraryv1.ListTitlesRequest, _ ...grpc.CallOption) (*libraryv1.ListTitlesResponse, error) {
			if in.GetLimit() != 200 {
				t.Errorf("limit = %d, want 200 (clamped)", in.GetLimit())
			}
			if in.GetOffset() != 0 {
				t.Errorf("offset = %d, want 0", in.GetOffset())
			}
			return bigLimit, nil
		},
	})
	rr := doJSON(srv.Handler(), http.MethodGet, "/api/library?limit=999&offset=-1", "", sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}
