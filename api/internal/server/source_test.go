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
	sourcev1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/source/v1"
)

func sessionCookieHeader(id string) map[string]string {
	return map[string]string{"Cookie": SessionCookieName + "=" + id}
}

func validSessionIdentity() *fakeIdentityClient {
	return &fakeIdentityClient{
		validateSession: func(_ context.Context, _ *identityv1.ValidateSessionRequest) (*identityv1.ValidateSessionResponse, error) {
			return &identityv1.ValidateSessionResponse{Valid: true, UserId: "usr_admin"}, nil
		},
	}
}

func TestSourceEndpointsRequireSession(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{}, &fakeSourceClient{})
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/source/connect", `{"connector_id":"jellyfin","endpoint":"https://m","credentials":{"api_key":"k"}}`},
		{http.MethodGet, "/api/source", ""},
		{http.MethodGet, "/api/source/src_x", ""},
		{http.MethodPost, "/api/source/src_x/update", `{"display_name":"x"}`},
	}
	for _, tc := range tests {
		rr := doJSON(srv.Handler(), tc.method, tc.path, tc.body, nil)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", tc.method, tc.path, rr.Code)
		}
	}
}

func TestSourceEndpointsRejectInvalidSession(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{
		validateSession: func(_ context.Context, _ *identityv1.ValidateSessionRequest) (*identityv1.ValidateSessionResponse, error) {
			return &identityv1.ValidateSessionResponse{Valid: false}, nil
		},
	}, &fakeSourceClient{})
	rr := doJSON(srv.Handler(), http.MethodGet, "/api/source", "", sessionCookieHeader("bad"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestSourceConnectHappyPath(t *testing.T) {
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{
		connect: func(_ context.Context, in *sourcev1.ConnectSourceRequest, _ ...grpc.CallOption) (*sourcev1.ConnectSourceResponse, error) {
			if in.GetActorUserId() != "usr_admin" {
				t.Errorf("actor = %q, want usr_admin", in.GetActorUserId())
			}
			if in.GetConnectorId() != "jellyfin" {
				t.Errorf("connector = %q", in.GetConnectorId())
			}
			pw := in.GetPassword()
			if pw == nil || pw.GetUsername() != "admin" || pw.GetPassword() != "correct horse" {
				t.Errorf("password credentials not forwarded: %+v", pw)
			}
			return &sourcev1.ConnectSourceResponse{
				Outcome: sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_CONNECTED,
				Source: &sourcev1.Source{
					SourceId: "src_1", ConnectorId: "jellyfin", VerifiedExternalId: "jf-1",
					Endpoint: "https://media.example.test", DisplayName: "Living Room",
					Status: sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE,
				},
			}, nil
		},
	})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/source/connect",
		`{"connector_id":"jellyfin","endpoint":"https://media.example.test","credentials":{"username":"admin","password":"correct horse"}}`,
		sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rr.Code, rr.Body)
	}
	var out connectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Outcome != "connected" {
		t.Fatalf("outcome = %q", out.Outcome)
	}
	if out.Source.SourceID != "src_1" || out.Source.DisplayName != "Living Room" || out.Source.Status != "active" {
		t.Fatalf("source = %+v", out.Source)
	}
	// The password must not be echoed in the response.
	if strings.Contains(rr.Body.String(), "correct horse") {
		t.Fatal("password echoed in response body")
	}
}

func TestSourceConnectDuplicate(t *testing.T) {
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{
		connect: func(_ context.Context, _ *sourcev1.ConnectSourceRequest, _ ...grpc.CallOption) (*sourcev1.ConnectSourceResponse, error) {
			return &sourcev1.ConnectSourceResponse{
				Outcome: sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_DUPLICATE,
				Source: &sourcev1.Source{
					SourceId: "src_existing", ConnectorId: "jellyfin", VerifiedExternalId: "jf-1",
					Endpoint: "https://media.example.test", DisplayName: "Existing",
					Status: sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE,
				},
			}, nil
		},
	})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/source/connect",
		`{"connector_id":"jellyfin","endpoint":"https://media.example.test","credentials":{"api_key":"k"}}`,
		sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for duplicate", rr.Code)
	}
	var out connectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Outcome != "duplicate" || out.Source.SourceID != "src_existing" {
		t.Fatalf("duplicate response = %+v", out)
	}
}

func TestSourceConnectValidationAndErrors(t *testing.T) {
	// Missing credentials -> 400 (from applyConnectCredentials).
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/source/connect",
		`{"connector_id":"jellyfin","endpoint":"https://media.example.test"}`, sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing credentials status = %d, want 400", rr.Code)
	}

	// Bad content type -> 415.
	rr = doJSON(srv.Handler(), http.MethodPost, "/api/source/connect", `{}`, map[string]string{"Content-Type": "text/plain", "Cookie": "abcmovies_session=sess_1"})
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("bad content-type status = %d, want 415", rr.Code)
	}

	// Core unreachable -> 503.
	srv2 := NewServer(validSessionIdentity(), &fakeSourceClient{
		connect: func(_ context.Context, _ *sourcev1.ConnectSourceRequest, _ ...grpc.CallOption) (*sourcev1.ConnectSourceResponse, error) {
			return nil, status.Error(codes.Unavailable, "down")
		},
	})
	rr = doJSON(srv2.Handler(), http.MethodPost, "/api/source/connect",
		`{"connector_id":"jellyfin","endpoint":"https://media.example.test","credentials":{"api_key":"k"}}`, sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d, want 503", rr.Code)
	}

	// Bad credentials from core -> 401.
	srv3 := NewServer(validSessionIdentity(), &fakeSourceClient{
		connect: func(_ context.Context, _ *sourcev1.ConnectSourceRequest, _ ...grpc.CallOption) (*sourcev1.ConnectSourceResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid")
		},
	})
	rr = doJSON(srv3.Handler(), http.MethodPost, "/api/source/connect",
		`{"connector_id":"jellyfin","endpoint":"https://media.example.test","credentials":{"password":"wrong","username":"admin"}}`, sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", rr.Code)
	}
}

func TestSourceListAndGet(t *testing.T) {
	listResp := &sourcev1.ListSourcesResponse{Sources: []*sourcev1.Source{
		{SourceId: "src_1", ConnectorId: "jellyfin", VerifiedExternalId: "jf-1", Endpoint: "https://a", DisplayName: "A", Status: sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE},
	}}
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{
		listSources: func(_ context.Context, in *sourcev1.ListSourcesRequest, _ ...grpc.CallOption) (*sourcev1.ListSourcesResponse, error) {
			if in.GetActorUserId() != "usr_admin" {
				t.Errorf("actor = %q", in.GetActorUserId())
			}
			return listResp, nil
		},
		getSource: func(_ context.Context, in *sourcev1.GetSourceRequest, _ ...grpc.CallOption) (*sourcev1.GetSourceResponse, error) {
			return &sourcev1.GetSourceResponse{Source: listResp.Sources[0]}, nil
		},
	})
	rr := doJSON(srv.Handler(), http.MethodGet, "/api/source", "", sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d", rr.Code)
	}
	var out listResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Sources) != 1 || out.Sources[0].SourceID != "src_1" {
		t.Fatalf("sources = %+v", out.Sources)
	}

	rr = doJSON(srv.Handler(), http.MethodGet, "/api/source/src_1", "", sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("get status = %d", rr.Code)
	}

	// Not found mapping from core.
	srv2 := NewServer(validSessionIdentity(), &fakeSourceClient{
		getSource: func(_ context.Context, _ *sourcev1.GetSourceRequest, _ ...grpc.CallOption) (*sourcev1.GetSourceResponse, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	})
	rr = doJSON(srv2.Handler(), http.MethodGet, "/api/source/src_nope", "", sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get missing status = %d, want 404", rr.Code)
	}
}

func TestSourceUpdateFailedPreconditionMapsTo422(t *testing.T) {
	srv := NewServer(validSessionIdentity(), &fakeSourceClient{
		update: func(_ context.Context, _ *sourcev1.UpdateSourceRequest, _ ...grpc.CallOption) (*sourcev1.UpdateSourceResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "endpoint resolves to a different upstream server")
		},
	})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/source/src_1/update",
		`{"endpoint":"https://other","credentials":{"api_key":"k"}}`, sessionCookieHeader("sess_1"))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", rr.Code, rr.Body)
	}
}
