package serving

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	apiv1connect "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1/apiv1connect"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// bearerTransport injects the session token into every request, the same
// way the browser page's interceptor does.
type bearerTransport struct {
	inner http.RoundTripper
	token string
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+b.token)
	return b.inner.RoundTrip(r2)
}

// newWebStack boots the serving layer under httptest and returns a
// gRPC-Web client for it.
func newWebStack(t *testing.T) (*Server, apiv1connect.CoreServiceClient, *httptest.Server) {
	t.Helper()

	srv, err := New("", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(srv.Close)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	client := apiv1connect.NewCoreServiceClient(ts.Client(), ts.URL, connect.WithGRPCWeb())
	return srv, client, ts
}

func signUpAndLogin(t *testing.T, ctx context.Context, client apiv1connect.CoreServiceClient) string {
	t.Helper()

	const password = "correct horse battery staple"
	if _, err := client.SignUp(ctx, connect.NewRequest(&apiv1.SignUpRequest{
		Username:   "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{Password: &apiv1.PasswordSignUp{Password: []byte(password)}},
	})); err != nil {
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
	return loginRes.Msg.GetToken()
}

func authedClient(baseURL, token string) apiv1connect.CoreServiceClient {
	hc := &http.Client{Transport: bearerTransport{inner: http.DefaultTransport, token: token}}
	return apiv1connect.NewCoreServiceClient(hc, baseURL, connect.WithGRPCWeb())
}

// TestWebClient_FullFlow drives the served mux exactly as the browser page
// does — over the gRPC-Web protocol: sign up, log in, receive a live
// job-status event through Subscribe, and read the job back with GetJob.
func TestWebClient_FullFlow(t *testing.T) {
	srv, client, ts := newWebStack(t)
	ctx := t.Context()
	baseURL := ts.URL

	token := signUpAndLogin(t, ctx, client)
	authed := authedClient(baseURL, token)

	// The connect client's Subscribe call does not return until the server
	// sends its first bytes, and the ephemeral bus registers subscribers
	// asynchronously — so a background enqueuer keeps creating probe jobs
	// until the stream is live and delivers one of their events.
	stop := make(chan struct{})
	enqueuerDone := make(chan struct{})
	go func() {
		defer close(enqueuerDone)
		for i := 1; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			id := fmt.Sprintf("web-job-%d", i)
			err := srv.stack.EnqueueJob(ctx, &corev1.Job{
				Id:          id,
				Kind:        corev1.JobKind_JOB_KIND_REFRESH,
				Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
				OwnerUserId: "user:alice",
			})
			if err != nil {
				t.Errorf("EnqueueJob: %v", err)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	stream, err := authed.Subscribe(ctx, connect.NewRequest(&apiv1.SubscribeRequest{}))
	if err != nil {
		close(stop)
		t.Fatalf("Subscribe over gRPC-Web: %v", err)
	}

	type recvResult struct {
		msg *apiv1.SubscribeResponse
		err error
	}
	recvCh := make(chan recvResult, 1)
	go func() {
		if !stream.Receive() {
			recvCh <- recvResult{msg: nil, err: stream.Err()}
			return
		}
		recvCh <- recvResult{msg: stream.Msg(), err: nil}
	}()

	var event *corev1.EventEnvelope
	deadline := time.After(5 * time.Second)
	select {
	case r := <-recvCh:
		if r.err != nil {
			t.Fatalf("stream Receive: %v", r.err)
		}
		event = r.msg.GetEvent()
	case <-deadline:
		close(stop)
		t.Fatal("timed out waiting for job-status event over gRPC-Web")
	}
	close(stop)
	<-enqueuerDone

	if event.GetType() != corev1.EventType_EVENT_TYPE_JOB_STATUS {
		t.Fatalf("event type = %v, want JOB_STATUS", event.GetType())
	}
	jobID := event.GetJobStatus().GetJobId()
	if !strings.HasPrefix(jobID, "web-job-") {
		t.Fatalf("event job_id = %q, want a job created by this test", jobID)
	}
	getRes, err := authed.GetJob(ctx, connect.NewRequest(&apiv1.GetJobRequest{JobId: jobID}))
	if err != nil {
		t.Fatalf("GetJob over gRPC-Web: %v", err)
	}
	if got := getRes.Msg.GetJob().GetId(); got != jobID {
		t.Fatalf("GetJob id = %q, want %q", got, jobID)
	}
}

// TestDebugJob_Unauthorized proves the debug endpoint enforces the same
// authentication rule as the protected RPC methods: missing and unknown
// bearer tokens are rejected.
func TestDebugJob_Unauthorized(t *testing.T) {
	_, _, ts := newWebStack(t)
	ctx := t.Context()
	baseURL := ts.URL

	for _, tc := range []struct {
		name   string
		token  string
		status int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"unknown", "definitely-not-a-valid-token", http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/debug/job", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			res, err := ts.Client().Do(req)
			if err != nil {
				t.Fatalf("POST /debug/job: %v", err)
			}
			_ = res.Body.Close()
			if res.StatusCode != tc.status {
				t.Fatalf("POST /debug/job status = %d, want %d", res.StatusCode, tc.status)
			}
		})
	}
}

// TestDebugCapabilities reports admitted capabilities to a logged-in session
// and rejects unauthenticated callers, mirroring the debug-job rules.
func TestDebugCapabilities(t *testing.T) {
	_, client, ts := newWebStack(t)
	ctx := t.Context()
	token := signUpAndLogin(t, ctx, client)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/debug/capabilities", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /debug/capabilities: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /debug/capabilities status = %d, want 200", res.StatusCode)
	}
	var caps []struct {
		Slot    string `json:"Slot"`
		Name    string `json:"Name"`
		Version uint32 `json:"Version"`
	}
	if err := json.NewDecoder(res.Body).Decode(&caps); err != nil {
		t.Fatalf("decode capabilities: %v", err)
	}
	found := false
	for _, c := range caps {
		if c.Slot == "builtin" && c.Name == "meta" && c.Version == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("builtin meta v1 missing from %v", caps)
	}

	// Unauthenticated callers are rejected like every protected method.
	bare, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/debug/capabilities", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	res2, err := ts.Client().Do(bare)
	if err != nil {
		t.Fatalf("GET /debug/capabilities unauthenticated: %v", err)
	}
	_ = res2.Body.Close()
	if res2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", res2.StatusCode)
	}
}

// TestDebugJob_CreatesJobAndEvents drives the manual-testing path end to
// end: a logged-in browser session subscribes, triggers the debug probe
// endpoint, observes live job-status events over gRPC-Web, and reads the
// final job state back through GetJob.
func TestDebugJob_CreatesJobAndEvents(t *testing.T) {
	_, client, ts := newWebStack(t)
	ctx := t.Context()
	baseURL := ts.URL

	token := signUpAndLogin(t, ctx, client)
	authed := authedClient(baseURL, token)

	type received struct {
		msg *apiv1.SubscribeResponse
		err error
	}
	eventCh := make(chan received, 64)

	// The connect client's Subscribe call does not return until the server
	// sends its first bytes, and the ephemeral bus registers subscribers
	// asynchronously — so the prober runs first, keeping probe events
	// flowing until the subscription is live and delivers one.
	stop := make(chan struct{})
	proberDone := make(chan struct{})
	go func() {
		defer close(proberDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/debug/job", nil)
			if err != nil {
				t.Errorf("new request: %v", err)
				return
			}
			req.Header.Set("Authorization", "Bearer "+token)
			res, err := ts.Client().Do(req)
			if err != nil {
				t.Errorf("POST /debug/job: %v", err)
				return
			}
			_ = res.Body.Close()
			if res.StatusCode != http.StatusOK {
				t.Errorf("POST /debug/job status = %d, want 200", res.StatusCode)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	stream, err := authed.Subscribe(ctx, connect.NewRequest(&apiv1.SubscribeRequest{}))
	if err != nil {
		close(stop)
		t.Fatalf("Subscribe over gRPC-Web: %v", err)
	}
	go func() {
		for stream.Receive() {
			eventCh <- received{msg: stream.Msg()}
		}
		eventCh <- received{err: stream.Err()}
	}()

	var doneEvent *corev1.JobStatusEvent
	deadline := time.After(5 * time.Second)

loop:
	for {
		select {
		case r := <-eventCh:
			if r.err != nil {
				close(stop)
				t.Fatalf("stream Receive: %v", r.err)
			}
			js := r.msg.GetEvent().GetJobStatus()
			if js == nil || !strings.HasPrefix(js.GetJobId(), "probe-") {
				continue
			}
			if js.GetStatus() == corev1.JobStatus_JOB_STATUS_DONE {
				doneEvent = js
				break loop
			}
		case <-deadline:
			t.Fatal("timed out waiting for probe job-status events over gRPC-Web")
		}
	}
	close(stop)
	<-proberDone

	jobID := doneEvent.GetJobId()
	getRes, err := authed.GetJob(ctx, connect.NewRequest(&apiv1.GetJobRequest{JobId: jobID}))
	if err != nil {
		t.Fatalf("GetJob over gRPC-Web: %v", err)
	}
	if got := getRes.Msg.GetJob(); got.GetId() != jobID || got.GetStatus() != corev1.JobStatus_JOB_STATUS_DONE {
		t.Fatalf("GetJob = {id: %q, status: %v}, want {%q, DONE}", got.GetId(), got.GetStatus(), jobID)
	}
}

// TestWebClient_AuthEnforced proves the connect termination enforces the
// same authentication rules as the gRPC one: protected methods reject
// missing, malformed, and unknown tokens; a valid token reaches business
// logic.
func TestWebClient_AuthEnforced(t *testing.T) {
	_, client, ts := newWebStack(t)
	ctx := t.Context()
	baseURL := ts.URL

	token := signUpAndLogin(t, ctx, client)

	for _, tc := range []struct {
		name    string
		authTok string
	}{
		{"missing", ""},
		{"malformed", "garbage"},
		{"unknown", "definitely-not-a-valid-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := client
			if tc.authTok != "" {
				c = authedClient(baseURL, tc.authTok)
			}
			_, err := c.GetJob(ctx, connect.NewRequest(&apiv1.GetJobRequest{JobId: "any"}))
			if connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Fatalf("GetJob error = %v, want Unauthenticated", err)
			}
		})
	}

	// A valid token passes authentication: an unknown job yields NotFound,
	// not Unauthenticated.
	_, err := authedClient(baseURL, token).GetJob(ctx, connect.NewRequest(&apiv1.GetJobRequest{JobId: "no-such-job"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetJob with valid token error = %v, want NotFound", err)
	}
}
