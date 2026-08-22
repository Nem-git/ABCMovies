package m0_test

import (
	"context"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/store"
)

type fullStack struct {
	stores  config.Stores
	auth    *auth.CompositeAuthenticator
	session auth.Session
	bus     *apiserver.InMemoryBus
	server  *apiserver.Server
}

func newFullStack(t *testing.T) *fullStack {
	t.Helper()

	c := config.Default()
	stores, err := config.BuildStores(t.Context(), c, nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	stores.WatchHistory = store.NewUserBlobStore(stores.WatchHistory)

	users, tokens, deks := config.BuildAuth(stores.Users, stores.Sessions)
	composite, err := config.BuildAuthenticator([]string{"password"}, users)
	if err != nil {
		t.Fatalf("BuildAuthenticator: %v", err)
	}
	session := config.BuildSession(tokens, deks, time.Hour)

	bus := apiserver.NewInMemoryBus()
	t.Cleanup(bus.Close)

	srv := apiserver.NewServer(bus, stores, composite, session)

	return &fullStack{
		stores:  stores,
		auth:    composite,
		session: session,
		bus:     bus,
		server:  srv,
	}
}

// signUp creates a user via the server and returns the response.
func signUp(t *testing.T, srv *apiserver.Server, username, password string) *auth.SignUpResult {
	t.Helper()
	resp, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: username,
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte(password)},
		},
	})
	if err != nil {
		t.Fatalf("SignUp(%s): %v", username, err)
	}
	return &auth.SignUpResult{UserID: resp.GetUserId(), RecoveryKey: resp.GetRecoveryKey()}
}

// login authenticates a user and returns the session token.
func login(t *testing.T, srv *apiserver.Server, username, password string) string {
	t.Helper()
	resp, err := srv.Login(context.Background(), &apiv1.LoginRequest{
		Username: username,
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte(password)},
		},
	})
	if err != nil {
		t.Fatalf("Login(%s): %v", username, err)
	}
	return resp.GetToken()
}
