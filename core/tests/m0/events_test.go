package m0_test

import (
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

func TestEvents_JobCreation_PublishesEvent(t *testing.T) {
	stack := newFullStack(t)

	ch := stack.bus.Subscribe("test-sub", "user:alice")
	defer stack.bus.Unsubscribe("test-sub")

	job := &corev1.Job{
		Id:          "job-evt-1",
		Kind:        corev1.JobKind_JOB_KIND_REFRESH,
		Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
		OwnerUserId: "user:alice",
	}
	if err := stack.server.CreateJob(t.Context(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	select {
	case event := <-ch:
		if event.GetType() != corev1.EventType_EVENT_TYPE_JOB_STATUS {
			t.Fatalf("event type = %v, want %v", event.GetType(), corev1.EventType_EVENT_TYPE_JOB_STATUS)
		}
		if event.GetUserId() != "user:alice" {
			t.Fatalf("event user_id = %q, want %q", event.GetUserId(), "user:alice")
		}
		payload := event.GetJobStatus()
		if payload == nil {
			t.Fatal("event payload should be JobStatus")
		}
		if payload.GetJobId() != "job-evt-1" {
			t.Fatalf("payload job_id = %q, want %q", payload.GetJobId(), "job-evt-1")
		}
		if payload.GetStatus() != corev1.JobStatus_JOB_STATUS_QUEUED {
			t.Fatalf("payload status = %v, want %v", payload.GetStatus(), corev1.JobStatus_JOB_STATUS_QUEUED)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job-status event")
	}
}

func TestEvents_NoEvent_BeforeJobCreation(t *testing.T) {
	stack := newFullStack(t)

	ch := stack.bus.Subscribe("test-sub", "user:alice")
	defer stack.bus.Unsubscribe("test-sub")

	// No job created — channel should be empty within a short window.
	select {
	case event := <-ch:
		t.Fatalf("unexpected event: %v", event)
	case <-time.After(50 * time.Millisecond):
		// Expected: no event.
	}
}

// TestEvents_MultipleSubscribers proves a user's several concurrent
// connections each receive their events (fan-out within one identity).
func TestEvents_MultipleSubscribers(t *testing.T) {
	stack := newFullStack(t)

	ch1 := stack.bus.Subscribe("sub-1", "user:alice")
	ch2 := stack.bus.Subscribe("sub-2", "user:alice")
	defer stack.bus.Unsubscribe("sub-1")
	defer stack.bus.Unsubscribe("sub-2")

	job := &corev1.Job{
		Id:          "job-multi-1",
		Kind:        corev1.JobKind_JOB_KIND_REFRESH,
		Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
		OwnerUserId: "user:alice",
	}
	if err := stack.server.CreateJob(t.Context(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	for _, ch := range []<-chan *corev1.EventEnvelope{ch1, ch2} {
		select {
		case event := <-ch:
			if event.GetType() != corev1.EventType_EVENT_TYPE_JOB_STATUS {
				t.Fatalf("event type = %v, want %v", event.GetType(), corev1.EventType_EVENT_TYPE_JOB_STATUS)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event on subscriber")
		}
	}
}

// TestEvents_JobWithoutOwnerRejected proves a job without an owner is
// rejected: a job's event audience is its owner (PLAN.md §9.2), so an
// owner-less job has no audience and must never enter the system.
func TestEvents_JobWithoutOwnerRejected(t *testing.T) {
	stack := newFullStack(t)

	ch := stack.bus.Subscribe("test-sub", "user:alice")
	defer stack.bus.Unsubscribe("test-sub")

	job := &corev1.Job{
		Id:     "job-no-owner",
		Kind:   corev1.JobKind_JOB_KIND_REFRESH,
		Status: corev1.JobStatus_JOB_STATUS_QUEUED,
	}
	err := stack.server.CreateJob(t.Context(), job)
	if err == nil {
		t.Fatal("expected error for job without owner_user_id")
	}

	select {
	case event := <-ch:
		t.Fatalf("rejected job must not produce an event, got %v", event)
	case <-time.After(50 * time.Millisecond):
		// Expected: nothing delivered.
	}
}

// TestEvents_Isolation proves the bus's tenancy boundary (PLAN.md §9.2):
// Bob's subscription never receives Alice's job-status events.
func TestEvents_Isolation(t *testing.T) {
	stack := newFullStack(t)

	aliceCh := stack.bus.Subscribe("alice-sub", "user:alice")
	bobCh := stack.bus.Subscribe("bob-sub", "user:bob")
	defer stack.bus.Unsubscribe("alice-sub")
	defer stack.bus.Unsubscribe("bob-sub")

	job := &corev1.Job{
		Id:          "job-isolation-1",
		Kind:        corev1.JobKind_JOB_KIND_REFRESH,
		Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
		OwnerUserId: "user:alice",
	}
	if err := stack.server.CreateJob(t.Context(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Alice receives her event.
	select {
	case event := <-aliceCh:
		if event.GetUserId() != "user:alice" {
			t.Fatalf("event user_id = %q, want %q", event.GetUserId(), "user:alice")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for alice's job-status event")
	}

	// Bob does not.
	select {
	case event := <-bobCh:
		t.Fatalf("bob received alice's event: %+v", event)
	case <-time.After(50 * time.Millisecond):
		// Expected: no cross-user delivery.
	}
}
