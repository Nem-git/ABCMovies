package serving

import (
	"context"
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
func newWebStack(t *testing.T) (*Server, apiv1connect.CoreServiceClient, string) {
	t.Helper()

	srv, err := New("", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(srv.Close)

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	client := apiv1connect.NewCoreServiceClient(ts.Client(), ts.URL, connect.WithGRPCWeb())
	return srv, client, ts.URL
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
	srv, client, baseURL := newWebStack(t)
	ctx := t.Context()

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

// TestWebClient_AuthEnforced proves the connect termination enforces the
// same authentication rules as the gRPC one: protected methods reject
// missing, malformed, and unknown tokens; a valid token reaches business
// logic.
func TestWebClient_AuthEnforced(t *testing.T) {
	_, client, baseURL := newWebStack(t)
	ctx := t.Context()

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
