package apiserver_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

func TestAuthInterceptor_ValidToken(t *testing.T) {
	interceptor := apiserver.AuthUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) {
		uid, ok := apiserver.UserIDFromContext(ctx)
		if !ok {
			t.Fatal("user ID not in context")
		}
		if uid != "user:1" {
			t.Fatalf("got user ID %q, want %q", uid, "user:1")
		}
		return "ok", nil
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer test-token"))
	resp, err := interceptor(ctx, nil, nil, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestAuthInterceptor_MissingToken(t *testing.T) {
	interceptor := apiserver.AuthUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())
	_, err := interceptor(ctx, nil, nil, handler)
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestAuthInterceptor_MalformedToken(t *testing.T) {
	interceptor := apiserver.AuthUnaryInterceptor()
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic abc"))
	_, err := interceptor(ctx, nil, nil, handler)
	if got := status.Code(err); got != codes.Unauthenticated {
		t.Fatalf("got code %v, want %v", got, codes.Unauthenticated)
	}
}

func TestBus_PublishSubscribe(t *testing.T) {
	bus := apiserver.NewBus()
	defer bus.Close()

	ch := bus.Subscribe("test")
	event := &corev1.EventEnvelope{
		Id:       "evt-1",
		Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
		UserId:   "user:1",
	}
	bus.Publish(event)

	select {
	case got := <-ch:
		if got.GetId() != "evt-1" {
			t.Fatalf("got event id %q, want %q", got.GetId(), "evt-1")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := apiserver.NewBus()
	defer bus.Close()

	ch := bus.Subscribe("test")
	bus.Unsubscribe("test")

	// Channel should be closed.
	_, ok := <-ch
	if ok {
		t.Fatal("channel should be closed after unsubscribe")
	}

	// Unsubscribing again should be a no-op.
	bus.Unsubscribe("test")
}

func TestBus_PublishSlow(t *testing.T) {
	bus := apiserver.NewBus()
	defer bus.Close()

	// Subscribe with a full buffer channel (capacity 64).
	ch := bus.Subscribe("slow")

	// Publish 65 events: the 65th should not block (dropped silently).
	for i := range 70 {
		bus.Publish(&corev1.EventEnvelope{
			Id:       "evt",
			Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
			Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
			UserId:   "user:1",
		})
		_ = i
	}

	// Should still have events in the channel (buffered), but publishing
	// did not block.
	got := len(ch)
	if got == 0 || got > 64 {
		t.Fatalf("expected buffered events, got %d", got)
	}
}

func TestServer_GetJob_NotFound(t *testing.T) {
	bus := apiserver.NewBus()
	defer bus.Close()
	srv := apiserver.NewServer(bus)

	_, err := srv.GetJob(context.Background(), &apiv1.GetJobRequest{JobId: "job-1"})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("got code %v, want %v", got, codes.NotFound)
	}
}

func TestServer_GetJob_EmptyID(t *testing.T) {
	bus := apiserver.NewBus()
	defer bus.Close()
	srv := apiserver.NewServer(bus)

	_, err := srv.GetJob(context.Background(), &apiv1.GetJobRequest{})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Fatalf("got code %v, want %v", got, codes.InvalidArgument)
	}
}
