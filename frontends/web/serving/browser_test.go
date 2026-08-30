package serving

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	apiv1connect "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1/apiv1connect"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// newWebStackConfig boots the serving layer over the YAML at configPath,
// exactly like the release root does from a real config file.
func newWebStackConfig(t *testing.T, configPath string) (*Server, apiv1connect.CoreServiceClient, *httptest.Server) {
	t.Helper()

	srv, err := New(configPath, nil)
	if err != nil {
		t.Fatalf("New(%s): %v", configPath, err)
	}
	t.Cleanup(srv.Close)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	client := apiv1connect.NewCoreServiceClient(ts.Client(), ts.URL, connect.WithGRPCWeb())
	return srv, client, ts
}

// signUpAndLoginID is signUpAndLogin plus the member user id, which the
// play flow must carry (StartDeliveryRequest.member_user_id).
func signUpAndLoginID(t *testing.T, ctx context.Context, client apiv1connect.CoreServiceClient) (token, userID string) {
	t.Helper()

	const password = "correct horse battery staple"
	signUpRes, err := client.SignUp(ctx, connect.NewRequest(&apiv1.SignUpRequest{
		Username:   "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{Password: &apiv1.PasswordSignUp{Password: []byte(password)}},
	}))
	if err != nil {
		t.Fatalf("SignUp over gRPC-Web: %v", err)
	}
	loginRes, err := client.Login(ctx, connect.NewRequest(&apiv1.LoginRequest{
		Username:   "alice",
		AuthMethod: &apiv1.LoginRequest_Password{Password: &apiv1.PasswordLogin{Password: []byte(password)}},
	}))
	if err != nil {
		t.Fatalf("Login over gRPC-Web: %v", err)
	}
	if loginRes.Msg.GetToken() == "" {
		t.Fatal("Login returned no token")
	}
	return loginRes.Msg.GetToken(), signUpRes.Msg.GetUserId()
}

// writeJellyfinConfig writes a release-shaped config: one Jellyfin provider
// slot declared by the operator with the test fake as its server, plus the
// built-in device sink the play flow delivers into.
func writeJellyfinConfig(t *testing.T, fakeURL string) string {
	t.Helper()
	cfg := `
core:
  api:
    bind: "127.0.0.1:0"
slots:
  providers:
    - adapter: jellyfin
      id: jf
      enabled: true
      sync-cadence: "5m"
      accounts:
        - id: jf-op
          url: ` + fakeURL + `
          username: alice.homeserver.user
          password-env: JF_TEST_PASSWORD
  sinks:
    - adapter: device
      id: device
      enabled: true
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("JF_TEST_PASSWORD", "operator-password-not-real")
	return path
}

// TestWebBrowser_AccountsLibraryPlay drives the served mux as the browser
// page does — sign up, log in, then: the operator-declared account is
// visible, the derived library browses over it, an account linked over the
// wire lands in the account list and is removable, and a play delivery
// stages its menu (announced by the delivered event), answers GetPlayInfo,
// and serves the provider's bytes through the relay.
func TestWebBrowser_AccountsLibraryPlay(t *testing.T) {
	fake := startFakeJellyfin(t)
	configPath := writeJellyfinConfig(t, fake.URL())
	srv, client, ts := newWebStackConfig(t, configPath)
	ctx := t.Context()
	baseURL := ts.URL

	token, userID := signUpAndLoginID(t, ctx, client)
	authed := authedClient(baseURL, token)

	// Operator-declared account is visible to the caller (PLAN.md §5.1:
	// operator accounts are PUBLIC and read-only through the API).
	accts := listAccounts(t, ctx, authed)
	if len(accts.GetAccounts()) != 1 {
		t.Fatalf("expected 1 operator account, got %d", len(accts.GetAccounts()))
	}
	op := accts.GetAccounts()[0]
	if op.GetAccountId() != "jf-op" || op.GetProvider() != "jf" {
		t.Errorf("unexpected operator account: id=%q provider=%q", op.GetAccountId(), op.GetProvider())
	}
	if op.GetCallerLinked() {
		t.Error("operator account must not be caller-linked")
	}
	if op.GetStatus() != apiv1.AccountStatus_ACCOUNT_STATUS_LINKED {
		t.Errorf("operator status = %v, want LINKED", op.GetStatus())
	}
	if op.GetVisibility() != apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PUBLIC {
		t.Errorf("operator visibility = %v, want PUBLIC", op.GetVisibility())
	}

	// The derived library browses from the operator account's source cache.
	lib := browse(t, ctx, authed, "")
	if got := len(lib.GetItems()); got != 3 {
		t.Fatalf("browse library: got %d items, want 3", got)
	}
	gondwana := findItem(t, lib.GetItems(), func(i *apiv1.LibraryItem) bool {
		return i.GetEntry().GetCoverage()["jf:movie-gondwana"] != nil
	})
	if gondwana == nil {
		t.Fatalf("gondwana item missing; coverage keys: %v", coverageKeys(lib))
	}
	row := gondwana.GetEntry().GetCoverage()["jf:movie-gondwana"]
	if !row.GetPresent() {
		t.Errorf("gondwana coverage row: present=false")
	}
	foundIMDB := false
	for _, id := range gondwana.GetEntry().GetExternalIdentities() {
		if id.GetNamespace() == "imdb" && id.GetValue() == "tt-gondwana" {
			foundIMDB = true
		}
	}
	if !foundIMDB {
		t.Errorf("gondwana external identities lack imdb:tt-gondwana: %v", gondwana.GetEntry().GetExternalIdentities())
	}
	// Query narrows the derived library; identity values are in the search
	// surface as the core fixtures prove (query "tt-gondwana" -> 1).
	if q := browse(t, ctx, authed, "tt-gondwana"); len(q.GetItems()) != 1 {
		t.Errorf("query tt-gondwana: got %d items, want 1", len(q.GetItems()))
	}

	// An account linked over the wire lands in the list for its owner and is
	// removable again (PLAN.md §7.5).
	linkRes, err := authed.LinkAccount(ctx, connect.NewRequest(&apiv1.LinkAccountRequest{
		Provider: "jellyfin",
		BaseUrl:  fake.URL(),
		AuthMethod: &apiv1.LinkAccountRequest_Password{
			Password: &apiv1.AccountPassword{
				Username: "alice.homeserver.linked",
				Password: []byte("linked-password"),
			},
		},
		Visibility: apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE,
	}))
	if err != nil {
		t.Fatalf("LinkAccount over gRPC-Web: %v", err)
	}
	afterLink := listAccounts(t, ctx, authed)
	if got := len(afterLink.GetAccounts()); got != 2 {
		t.Fatalf("after link: got %d accounts, want 2", got)
	}
	var linked *apiv1.Account
	for _, a := range afterLink.GetAccounts() {
		if a.GetAccountId() == linkRes.Msg.GetAccountId() {
			linked = a
		}
	}
	if linked == nil {
		t.Fatalf("linked account %q missing from list", linkRes.Msg.GetAccountId())
	}
	if !linked.GetCallerLinked() {
		t.Errorf("linked account must be caller-linked")
	}
	if _, err := authed.RemoveAccount(ctx, connect.NewRequest(&apiv1.RemoveAccountRequest{
		AccountId: linkRes.Msg.GetAccountId(),
	})); err != nil {
		t.Fatalf("RemoveAccount over gRPC-Web: %v", err)
	}
	if afterRemove := listAccounts(t, ctx, authed); len(afterRemove.GetAccounts()) != 1 {
		t.Errorf("after remove: got %d accounts, want 1", len(afterRemove.GetAccounts()))
	}

	// Play: subscribe first, then start a delivery. The connect Subscribe call
	// only returns once the server has sent its first bytes, and the ephemeral
	// bus registers the subscriber asynchronously — so a background probe
	// enqueuer keeps the stream live (the same readiness trick the core
	// serving tests use). Once a probe event is actually delivered, the
	// subscription is attached and StartDelivery's events reach it.
	stopWarm := make(chan struct{})
	warmDone := make(chan struct{})
	go func() {
		defer close(warmDone)
		for i := 0; ; i++ {
			select {
			case <-stopWarm:
				return
			default:
			}
			if err := srv.stack.EnqueueJob(ctx, &corev1.Job{
				Id:          fmt.Sprintf("web-warm-%d", i),
				Kind:        corev1.JobKind_JOB_KIND_REFRESH,
				Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
				OwnerUserId: "user:alice",
			}); err != nil {
				t.Errorf("warm EnqueueJob: %v", err)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	stream, err := authed.Subscribe(ctx, connect.NewRequest(&apiv1.SubscribeRequest{}))
	if err != nil {
		close(stopWarm)
		t.Fatalf("Subscribe over gRPC-Web: %v", err)
	}
	for {
		if ev := recvEvent(ctx, t, stream, 5*time.Second); ev.GetType() == corev1.EventType_EVENT_TYPE_JOB_STATUS && strings.HasPrefix(ev.GetJobStatus().GetJobId(), "web-warm-") {
			break
		}
	}
	close(stopWarm)
	<-warmDone

	// The delivery-play-menu-ready event must arrive (published on the same
	// bus Subscribe reads) and announce the session before the job-status does
	// — the order the engine fixes: menu staged, then the job recorded.
	startRes, err := authed.StartDelivery(ctx, connect.NewRequest(&apiv1.StartDeliveryRequest{
		Goal:         apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY,
		Provider:     "jf",
		AccountId:    "jf-op",
		MemberUserId: userID,
		NativeId:     "movie-gondwana",
		Sink:         "device",
	}))
	if err != nil {
		t.Fatalf("StartDelivery over gRPC-Web: %v", err)
	}
	sessionID := startRes.Msg.GetJob().GetId()
	if sessionID == "" {
		t.Fatal("StartDelivery returned an empty session id")
	}

	sawMenuReady := false
deadline:
	for {
		ev := recvEvent(ctx, t, stream, 5*time.Second)
		switch {
		case ev.GetType() == corev1.EventType_EVENT_TYPE_DELIVERY_PLAY_MENU_READY && ev.GetPlayMenuReady().GetJobId() == sessionID:
			sawMenuReady = true
		case ev.GetType() == corev1.EventType_EVENT_TYPE_JOB_STATUS && ev.GetJobStatus().GetJobId() == sessionID:
			if !sawMenuReady {
				t.Errorf("job-status arrived before delivery-play-menu-ready")
			}
			break deadline
		}
	}
	if !sawMenuReady {
		t.Fatalf("delivery-play-menu-ready for %s never arrived", sessionID)
	}

	// The staged menu answers GetPlayInfo with the video track + relay URL,
	// and the relay serves the provider's bytes without a second credential.
	info, err := authed.GetPlayInfo(ctx, connect.NewRequest(&apiv1.GetPlayInfoRequest{SessionId: sessionID}))
	if err != nil {
		t.Fatalf("GetPlayInfo over gRPC-Web: %v", err)
	}
	var video *apiv1.PlayTrack
	for _, tr := range info.Msg.GetTracks() {
		if tr.GetVideo() != nil {
			video = tr
		}
	}
	if video == nil {
		t.Fatalf("no video track in menu: %d tracks", len(info.Msg.GetTracks()))
	}
	if !strings.HasPrefix(video.GetRelayUrl(), "/media/relay/") {
		t.Fatalf("relay url %q not under /media/relay/", video.GetRelayUrl())
	}
	got := pull(t, baseURL+video.GetRelayUrl())
	if string(got) != "P5-gondwana-mkv-bytes" {
		t.Errorf("relay body = %q, want provider stream bytes", got)
	}
	if hits := fake.Hits("movie-gondwana"); hits != 1 {
		t.Errorf("provider stream hit count = %d, want 1", hits)
	}

	if srv.BindAddress() == "" {
		t.Error("bind address not propagated")
	}
}

func listAccounts(t *testing.T, ctx context.Context, client apiv1connect.CoreServiceClient) *apiv1.ListAccountsResponse {
	t.Helper()
	res, err := client.ListAccounts(ctx, connect.NewRequest(&apiv1.ListAccountsRequest{}))
	if err != nil {
		t.Fatalf("ListAccounts over gRPC-Web: %v", err)
	}
	return res.Msg
}

func browse(t *testing.T, ctx context.Context, client apiv1connect.CoreServiceClient, query string) *apiv1.GetLibraryResponse {
	t.Helper()
	res, err := client.GetLibrary(ctx, connect.NewRequest(&apiv1.GetLibraryRequest{Query: query}))
	if err != nil {
		t.Fatalf("GetLibrary over gRPC-Web: %v", err)
	}
	return res.Msg
}

func findItem(t *testing.T, items []*apiv1.LibraryItem, match func(*apiv1.LibraryItem) bool) *apiv1.LibraryItem {
	t.Helper()
	for _, it := range items {
		if match(it) {
			return it
		}
	}
	return nil
}

func coverageKeys(lib *apiv1.GetLibraryResponse) []string {
	var out []string
	for _, it := range lib.GetItems() {
		for k := range it.GetEntry().GetCoverage() {
			out = append(out, k)
		}
	}
	return out
}

func pull(t *testing.T, url string) []byte {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("relay pull %s: %v", url, err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	if res.StatusCode != http.StatusOK {
		t.Fatalf("relay pull %s: HTTP %d", url, res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("relay pull %s: %v", url, err)
	}
	return body
}

// recvEvent waits for one event on a gRPC-Web Subscribe stream, timing out if
// nothing arrives. The connect client's Receive blocks indefinitely, so the
// wait runs on a goroutine selected against a deadline.
func recvEvent(ctx context.Context, t *testing.T, stream *connect.ServerStreamForClient[apiv1.SubscribeResponse], timeout time.Duration) *corev1.EventEnvelope {
	t.Helper()
	type result struct {
		ev  *corev1.EventEnvelope
		err error
	}
	recvCh := make(chan result, 1)
	go func() {
		if !stream.Receive() {
			recvCh <- result{err: stream.Err()}
			return
		}
		recvCh <- result{ev: stream.Msg().GetEvent()}
	}()
	select {
	case r := <-recvCh:
		if r.err != nil {
			t.Fatalf("subscribe stream receive: %v", r.err)
		}
		return r.ev
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for a subscribe event")
		return nil
	case <-ctx.Done():
		t.Fatalf("subscribe context done: %v", ctx.Err())
		return nil
	}
}
