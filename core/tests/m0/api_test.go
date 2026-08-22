package m0_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// startWireServer serves the real CoreService behind the production auth
// interceptors on an in-memory connection, wired exactly as main.go wires it.
func startWireServer(t *testing.T, stack *fullStack) *grpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer(
		grpc.UnaryInterceptor(apiserver.AuthUnaryInterceptor(stack.session)),
		grpc.StreamInterceptor(apiserver.AuthStreamInterceptor(stack.session)),
	)
	apiv1.RegisterCoreServiceServer(gs, stack.server)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// authedCtx returns ctx carrying the bearer token, as a frontend would send it.
func authedCtx(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

// TestWireAPI_SyncCallAndEvent proves M0's inbound-API criterion over the
// wire (IMPLEMENTATION.md §2): one synchronous call (GetJob) and one event
// (Subscribe) cross the real gRPC API boundary with bearer-token auth.
func TestWireAPI_SyncCallAndEvent(t *testing.T) {
	stack := newFullStack(t)
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	ctx := t.Context()

	// Sign up and log in over the wire — no token needed for these.
	if _, err := client.SignUp(ctx, &apiv1.SignUpRequest{
		Username: "alice",
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte("password123")},
		},
	}); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	loginResp, err := client.Login(ctx, &apiv1.LoginRequest{
		Username: "alice",
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte("password123")},
		},
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	authed := authedCtx(ctx, loginResp.GetToken())

	// Subscribe before creating the job — the bus is ephemeral, no replay.
	//
	// The server registers the subscriber asynchronously with respect to the
	// client call returning, and the bus keeps no history, so an event
	// published before registration is gone. To stay deterministic the test
	// probes: it creates jobs until one of their events arrives, then asserts
	// the delivered event belongs to a job it created.
	stream, err := client.Subscribe(authed, &apiv1.SubscribeRequest{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	type recvResult struct {
		resp *apiv1.SubscribeResponse
		err  error
	}
	recvCh := make(chan recvResult, 1)
	go func() {
		resp, err := stream.Recv()
		recvCh <- recvResult{resp: resp, err: err}
	}()

	createJob := func(id string) {
		t.Helper()
		job := &corev1.Job{
			Id:          id,
			Kind:        corev1.JobKind_JOB_KIND_REFRESH,
			Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
			OwnerUserId: "user:alice",
		}
		if err := stack.server.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob: %v", err)
		}
	}

	var event *corev1.EventEnvelope
	created := map[string]bool{}
	probe := 0
	deadline := time.After(5 * time.Second)
	for event == nil {
		probe++
		id := fmt.Sprintf("wire-job-%d", probe)
		createJob(id)
		created[id] = true

		select {
		case r := <-recvCh:
			if r.err != nil {
				t.Fatalf("stream Recv: %v", r.err)
			}
			event = r.resp.GetEvent()
		case <-time.After(50 * time.Millisecond):
			// Subscriber not registered yet — probe again.
		case <-deadline:
			t.Fatal("timed out waiting for job-status event over the wire")
		}
	}

	if event.GetType() != corev1.EventType_EVENT_TYPE_JOB_STATUS {
		t.Fatalf("event type = %v, want %v", event.GetType(), corev1.EventType_EVENT_TYPE_JOB_STATUS)
	}
	if !created[event.GetJobStatus().GetJobId()] {
		t.Fatalf("event job_id = %q, want one of the jobs created by this test", event.GetJobStatus().GetJobId())
	}

	// The synchronous call: GetJob over the wire, for the first probe job.
	getResp, err := client.GetJob(authed, &apiv1.GetJobRequest{JobId: "wire-job-1"})
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got := getResp.GetJob().GetId(); got != "wire-job-1" {
		t.Fatalf("GetJob id = %q, want %q", got, "wire-job-1")
	}
}

// TestWireAPI_UnauthenticatedRejected proves the API boundary rejects calls
// without a valid token (negative fixtures for anything that accepts input).
func TestWireAPI_UnauthenticatedRejected(t *testing.T) {
	stack := newFullStack(t)
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	ctx := t.Context()

	if _, err := client.GetJob(ctx, &apiv1.GetJobRequest{JobId: "x"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetJob without token: got %v, want Unauthenticated", status.Code(err))
	}

	stream, subErr := client.Subscribe(ctx, &apiv1.SubscribeRequest{})
	var streamErr error
	if subErr == nil {
		_, streamErr = stream.Recv()
	}
	if got := status.Code(subErr); got != codes.Unauthenticated {
		if got := status.Code(streamErr); got != codes.Unauthenticated {
			t.Fatalf("Subscribe without token: call err %v, recv err %v, want Unauthenticated", subErr, streamErr)
		}
	}
}

// TestWireAPI_BadTokenRejected proves a malformed or invalid token is
// rejected, not just an absent one.
func TestWireAPI_BadTokenRejected(t *testing.T) {
	stack := newFullStack(t)
	client := apiv1.NewCoreServiceClient(startWireServer(t, stack))
	ctx := t.Context()

	for name, token := range map[string]string{
		"malformed":  "Basic abc",
		"not-bearer": "random-string",
		"unknown":    "Bearer no-such-token",
	} {
		if _, err := client.GetJob(authedCtx(ctx, token), &apiv1.GetJobRequest{JobId: "x"}); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("%s token: got %v, want Unauthenticated", name, status.Code(err))
		}
	}
}
