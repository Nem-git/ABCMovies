package m0_test

import (
	"testing"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

func TestEvents_JobCreation_PublishesEvent(t *testing.T) {
	stack := newFullStack(t)

	ch := stack.bus.Subscribe("test-sub")
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

	ch := stack.bus.Subscribe("test-sub")
	defer stack.bus.Unsubscribe("test-sub")

	// No job created — channel should be empty within a short window.
	select {
	case event := <-ch:
		t.Fatalf("unexpected event: %v", event)
	case <-time.After(50 * time.Millisecond):
		// Expected: no event.
	}
}

func TestEvents_MultipleSubscribers(t *testing.T) {
	stack := newFullStack(t)

	ch1 := stack.bus.Subscribe("sub-1")
	ch2 := stack.bus.Subscribe("sub-2")
	defer stack.bus.Unsubscribe("sub-1")
	defer stack.bus.Unsubscribe("sub-2")

	job := &corev1.Job{
		Id:     "job-multi-1",
		Kind:   corev1.JobKind_JOB_KIND_REFRESH,
		Status: corev1.JobStatus_JOB_STATUS_QUEUED,
	}
	if err := stack.server.CreateJob(t.Context(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	// Both subscribers should receive the event.
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

func TestEvents_JobWithoutOwner(t *testing.T) {
	stack := newFullStack(t)

	ch := stack.bus.Subscribe("test-sub")
	defer stack.bus.Unsubscribe("test-sub")

	job := &corev1.Job{
		Id:     "job-no-owner",
		Kind:   corev1.JobKind_JOB_KIND_REFRESH,
		Status: corev1.JobStatus_JOB_STATUS_QUEUED,
	}
	if err := stack.server.CreateJob(t.Context(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	select {
	case event := <-ch:
		if event.GetUserId() != "" {
			t.Fatalf("event user_id = %q, want empty", event.GetUserId())
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}
