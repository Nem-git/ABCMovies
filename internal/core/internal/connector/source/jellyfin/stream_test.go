package jellyfin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthorizeAPIKeyIsVerifiedAgainstServer(t *testing.T) {
	// The mock knows one good key; anything else is unauthorized, like a
	// real server would treat a revoked/invalid key.
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /System/Info", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if !strings.Contains(gotAuth, `Token="good-key"`) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"Message":"Invalid API Key"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ServerName":"Mock","Id":"mock-sys-id","Version":"10.10.7"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient()

	// Good key: verified, header handed out, no AuthenticateByName involved.
	header, aerr := client.Authorize(context.Background(), srv.URL, Credentials{APIKey: "good-key"})
	if aerr != nil {
		t.Fatalf("Authorize with good key: %+v", aerr)
	}
	if !strings.Contains(header, `Token="good-key"`) {
		t.Fatalf("header = %q, want the API key as Token", header)
	}
	if !strings.Contains(gotAuth, `Token="good-key"`) {
		t.Fatalf("verify call did not carry the key, got %q", gotAuth)
	}

	// Bad key: refused with an Unauthorized-classified error.
	_, aerr = client.Authorize(context.Background(), srv.URL, Credentials{APIKey: "bad-key"})
	if aerr == nil {
		t.Fatal("Authorize with a bad key succeeded; want an error")
	}
	if aerr.Kind != ErrKindUnauthorized {
		t.Fatalf("error kind = %v, want %v", aerr.Kind, ErrKindUnauthorized)
	}
}
