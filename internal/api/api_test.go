package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
	sourcev1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/source/v1"
)

type fakeIdentityClient struct {
	getStatus       func(context.Context, *identityv1.GetStatusRequest) (*identityv1.GetStatusResponse, error)
	setupAdmin      func(context.Context, *identityv1.SetupAdminRequest) (*identityv1.SetupAdminResponse, error)
	getUser         func(context.Context, *identityv1.GetUserRequest) (*identityv1.GetUserResponse, error)
	login           func(context.Context, *identityv1.LoginRequest) (*identityv1.LoginResponse, error)
	validateSession func(context.Context, *identityv1.ValidateSessionRequest) (*identityv1.ValidateSessionResponse, error)
	logout          func(context.Context, *identityv1.LogoutRequest) (*identityv1.LogoutResponse, error)
}

func (f *fakeIdentityClient) GetStatus(ctx context.Context, in *identityv1.GetStatusRequest, _ ...grpc.CallOption) (*identityv1.GetStatusResponse, error) {
	if f.getStatus != nil {
		return f.getStatus(ctx, in)
	}
	return &identityv1.GetStatusResponse{}, nil
}

func (f *fakeIdentityClient) SetupAdmin(ctx context.Context, in *identityv1.SetupAdminRequest, _ ...grpc.CallOption) (*identityv1.SetupAdminResponse, error) {
	if f.setupAdmin != nil {
		return f.setupAdmin(ctx, in)
	}
	return &identityv1.SetupAdminResponse{}, nil
}

func (f *fakeIdentityClient) GetUser(ctx context.Context, in *identityv1.GetUserRequest, _ ...grpc.CallOption) (*identityv1.GetUserResponse, error) {
	if f.getUser != nil {
		return f.getUser(ctx, in)
	}
	return &identityv1.GetUserResponse{}, nil
}

func (f *fakeIdentityClient) Login(ctx context.Context, in *identityv1.LoginRequest, _ ...grpc.CallOption) (*identityv1.LoginResponse, error) {
	if f.login != nil {
		return f.login(ctx, in)
	}
	return &identityv1.LoginResponse{}, nil
}

func (f *fakeIdentityClient) ValidateSession(ctx context.Context, in *identityv1.ValidateSessionRequest, _ ...grpc.CallOption) (*identityv1.ValidateSessionResponse, error) {
	if f.validateSession != nil {
		return f.validateSession(ctx, in)
	}
	return &identityv1.ValidateSessionResponse{}, nil
}

func (f *fakeIdentityClient) Logout(ctx context.Context, in *identityv1.LogoutRequest, _ ...grpc.CallOption) (*identityv1.LogoutResponse, error) {
	if f.logout != nil {
		return f.logout(ctx, in)
	}
	return &identityv1.LogoutResponse{}, nil
}

type fakeSourceClient struct {
	connect     func(context.Context, *sourcev1.ConnectSourceRequest, ...grpc.CallOption) (*sourcev1.ConnectSourceResponse, error)
	update      func(context.Context, *sourcev1.UpdateSourceRequest, ...grpc.CallOption) (*sourcev1.UpdateSourceResponse, error)
	listSources func(context.Context, *sourcev1.ListSourcesRequest, ...grpc.CallOption) (*sourcev1.ListSourcesResponse, error)
	getSource   func(context.Context, *sourcev1.GetSourceRequest, ...grpc.CallOption) (*sourcev1.GetSourceResponse, error)
}

func (f *fakeSourceClient) ConnectSource(ctx context.Context, in *sourcev1.ConnectSourceRequest, opts ...grpc.CallOption) (*sourcev1.ConnectSourceResponse, error) {
	if f.connect != nil {
		return f.connect(ctx, in, opts...)
	}
	return &sourcev1.ConnectSourceResponse{Outcome: sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_CONNECTED,
		Source: &sourcev1.Source{SourceId: "src_test", ConnectorId: in.GetConnectorId(), VerifiedExternalId: "jf-test-id", Endpoint: "https://example.test", DisplayName: "Test", Status: sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE}}, nil
}
func (f *fakeSourceClient) UpdateSource(ctx context.Context, in *sourcev1.UpdateSourceRequest, opts ...grpc.CallOption) (*sourcev1.UpdateSourceResponse, error) {
	if f.update != nil {
		return f.update(ctx, in, opts...)
	}
	return &sourcev1.UpdateSourceResponse{Source: &sourcev1.Source{SourceId: in.GetSourceId(), Status: sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE}}, nil
}
func (f *fakeSourceClient) ListSources(ctx context.Context, in *sourcev1.ListSourcesRequest, opts ...grpc.CallOption) (*sourcev1.ListSourcesResponse, error) {
	if f.listSources != nil {
		return f.listSources(ctx, in, opts...)
	}
	return &sourcev1.ListSourcesResponse{}, nil
}
func (f *fakeSourceClient) GetSource(ctx context.Context, in *sourcev1.GetSourceRequest, opts ...grpc.CallOption) (*sourcev1.GetSourceResponse, error) {
	if f.getSource != nil {
		return f.getSource(ctx, in, opts...)
	}
	return &sourcev1.GetSourceResponse{Source: &sourcev1.Source{SourceId: in.GetSourceId(), Status: sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE}}, nil
}

func doJSON(handler http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func cookieValue(rr *httptest.ResponseRecorder, name string) (string, bool) {
	for _, c := range rr.Result().Cookies() {
		if c.Name == name {
			return c.Value, true
		}
	}
	return "", false
}

func TestStatus(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{
		getStatus: func(_ context.Context, _ *identityv1.GetStatusRequest) (*identityv1.GetStatusResponse, error) {
			return &identityv1.GetStatusResponse{Initialized: true}, nil
		},
	}, &fakeSourceClient{})
	rr := doJSON(srv.Handler(), http.MethodGet, "/api/status", "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var out statusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Initialized {
		t.Fatal("initialized = false, want true")
	}
}

func TestSetupConflictAndValidation(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{
		setupAdmin: func(_ context.Context, _ *identityv1.SetupAdminRequest) (*identityv1.SetupAdminResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "instance already initialized")
		},
	}, &fakeSourceClient{})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/setup", `{"username":"a","password":"testpass1234"}`, nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}

	rr = doJSON(srv.Handler(), http.MethodPost, "/api/setup", `{"username":"a"}`, map[string]string{"Content-Type": "text/plain"})
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("bad content-type status = %d, want 415", rr.Code)
	}

	rr = doJSON(srv.Handler(), http.MethodPost, "/api/setup", `{`, nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d, want 400", rr.Code)
	}
}

func TestLoginSetsSessionCookie(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{
		login: func(_ context.Context, _ *identityv1.LoginRequest) (*identityv1.LoginResponse, error) {
			return &identityv1.LoginResponse{SessionId: "sess_test", UserId: "usr_test"}, nil
		},
		getUser: func(_ context.Context, _ *identityv1.GetUserRequest) (*identityv1.GetUserResponse, error) {
			return &identityv1.GetUserResponse{UserId: "usr_test", Username: "admin"}, nil
		},
	}, &fakeSourceClient{})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/login", `{"username":"admin","password":"testpass1234"}`, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	v, ok := cookieValue(rr, SessionCookieName)
	if !ok || v != "sess_test" {
		t.Fatalf("session cookie = (%q,%v), want sess_test", v, ok)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == SessionCookieName {
			if !c.HttpOnly {
				t.Fatal("session cookie not HttpOnly")
			}
			if c.Secure {
				t.Fatal("session cookie unexpectedly Secure over plain HTTP")
			}
		}
	}
}

func TestLoginSetsSecureCookieWithForwardedProto(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{
		login: func(_ context.Context, _ *identityv1.LoginRequest) (*identityv1.LoginResponse, error) {
			return &identityv1.LoginResponse{SessionId: "sess_test", UserId: "usr_test"}, nil
		},
		getUser: func(_ context.Context, _ *identityv1.GetUserRequest) (*identityv1.GetUserResponse, error) {
			return &identityv1.GetUserResponse{UserId: "usr_test", Username: "admin"}, nil
		},
	}, &fakeSourceClient{})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/login", `{"username":"admin","password":"testpass1234"}`, map[string]string{"X-Forwarded-Proto": "https"})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == SessionCookieName && !c.Secure {
			t.Fatal("session cookie not Secure with X-Forwarded-Proto: https")
		}
	}
}

func TestMeUnauthorized(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{}, &fakeSourceClient{})
	rr := doJSON(srv.Handler(), http.MethodGet, "/api/me", "", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}

	srv = NewServer(&fakeIdentityClient{
		validateSession: func(_ context.Context, _ *identityv1.ValidateSessionRequest) (*identityv1.ValidateSessionResponse, error) {
			return &identityv1.ValidateSessionResponse{Valid: false}, nil
		},
	}, &fakeSourceClient{})
	rr = doJSON(srv.Handler(), http.MethodGet, "/api/me", "", map[string]string{"Cookie": SessionCookieName + "=bad"})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("invalid session status = %d, want 401", rr.Code)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	srv := NewServer(&fakeIdentityClient{}, &fakeSourceClient{})
	rr := doJSON(srv.Handler(), http.MethodPost, "/api/logout", "{}", map[string]string{"Cookie": SessionCookieName + "=sess_test"})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == SessionCookieName && c.MaxAge >= 0 {
			t.Fatalf("logout cookie MaxAge = %d, want < 0", c.MaxAge)
		}
	}
}
