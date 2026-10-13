package delivery

import (
	"context"
	"errors"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// ErrQueued carries a queued job back to the API layer, which returns it as a
// successful StartDeliveryResponse (PLAN.md §6.5: a queued start is a success,
// not an error).
type ErrQueued struct {
	job *corev1.Job
}

func (e *ErrQueued) Error() string { return "delivery queued" }

// Job returns the queued job for the API layer to return as a success.
func (e *ErrQueued) Job() *corev1.Job { return e.job }

// parkLocked adds a download to the member's queue and returns its queued
// job. The caller must hold e.mu. The job id is stable: it becomes the
// session id when the request is promoted, so a client polling GetJob sees
// the same job id from queue to completion.
func (e *Engine) parkLocked(req StartRequest) *corev1.Job {
	qr := &queuedRequest{
		id:             e.newSessionID(),
		req:            req,
		idempotencyKey: req.IdempotencyKey,
		createdAt:      e.now(),
	}
	if req.IdempotencyKey != "" {
		e.idempotency[req.IdempotencyKey] = idempotencyEntry{jobID: qr.id, createdAt: e.now()}
	}
	e.queue[req.MemberUserID] = append(e.queue[req.MemberUserID], qr)
	return &corev1.Job{
		Id:          qr.id,
		Kind:        corev1.JobKind_JOB_KIND_DELIVERY,
		OwnerUserId: req.MemberUserID,
		Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
		Delivery: &corev1.DeliveryContext{
			Provider:     req.Provider,
			AccountId:    req.AccountID,
			MemberUserId: req.MemberUserID,
			Sink:         req.Sink,
			Goal:         string(req.Goal),
			NativeId:     req.NativeID,
		},
		Queued: &corev1.Queued{Position: uint32(len(e.queue[req.MemberUserID]))},
	}
}

// queuedJobLocked renders a queued request as its job. The caller must hold
// e.mu.
func (e *Engine) queuedJobLocked(qr *queuedRequest) *corev1.Job {
	pos := uint32(1)
	for _, other := range e.queue[qr.req.MemberUserID] {
		if other.id == qr.id {
			break
		}
		pos++
	}
	return &corev1.Job{
		Id:          qr.id,
		Kind:        corev1.JobKind_JOB_KIND_DELIVERY,
		OwnerUserId: qr.req.MemberUserID,
		Status:      corev1.JobStatus_JOB_STATUS_QUEUED,
		Delivery: &corev1.DeliveryContext{
			Provider:     qr.req.Provider,
			AccountId:    qr.req.AccountID,
			MemberUserId: qr.req.MemberUserID,
			Sink:         qr.req.Sink,
			Goal:         string(qr.req.Goal),
			NativeId:     qr.req.NativeID,
		},
		Queued: &corev1.Queued{Position: pos},
	}
}

// promoteNextLocked starts the next queued download for a member, if any.
// It is called when a seat frees (a session completed, was revoked, or was
// evicted). If the promotion fails for a reason other than re-queueing, the
// job is marked failed.
func (e *Engine) promoteNextLocked(ctx context.Context, memberUserID string) {
	e.mu.Lock()
	q := e.queue[memberUserID]
	if len(q) == 0 {
		e.mu.Unlock()
		return
	}
	qr := q[0]
	e.queue[memberUserID] = q[1:]
	e.mu.Unlock()

	// Use the queued id as the session id so the job id is stable from
	// queue to completion (PLAN.md §9.1).
	qr.req.sessionID = qr.id
	sess, err := e.start(ctx, qr.req)
	if err != nil {
		var qerr *ErrQueued
		if errors.As(err, &qerr) {
			// Re-queued (a concurrent start took the seat). No action.
			return
		}
		// The promotion failed. Mark the job as failed.
		e.recordJob(&corev1.Job{
			Id:          qr.id,
			Kind:        corev1.JobKind_JOB_KIND_DELIVERY,
			OwnerUserId: memberUserID,
			Status:      corev1.JobStatus_JOB_STATUS_FAILED,
			Error:       err.Error(),
		})
		return
	}

	// A promoted download begins pushing its bytes now that it holds a
	// seat — never before (PLAN.md §6.5: a queued download waits).
	if sess.Goal == GoalDownload && sess.Sink != nil {
		go e.DeliverDownload(context.Background(), sess)
	}
}

// Cancel removes a queued download or ends a running one (PLAN.md §6.5).
// Owner-only; the caller must own the job. A queued job is removed from
// the queue and marked cancelled. A running session is aborted and marked
// cancelled. A job that is already done, failed, or cancelled is a no-op.
func (e *Engine) Cancel(jobID string) (*Session, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check the queue first.
	for memberUserID, q := range e.queue {
		for i, qr := range q {
			if qr.id == jobID {
				e.queue[memberUserID] = append(q[:i], q[i+1:]...)
				e.recordJob(&corev1.Job{
					Id:          qr.id,
					Kind:        corev1.JobKind_JOB_KIND_DELIVERY,
					OwnerUserId: memberUserID,
					Status:      corev1.JobStatus_JOB_STATUS_CANCELLED,
				})
				return nil, nil
			}
		}
	}

	// Then check live sessions.
	if s, ok := e.sessions[jobID]; ok {
		s.mu.Lock()
		s.Status = StatusCancelled
		if s.Sink != nil {
			s.Sink.Abort(context.Background(), s)
		}
		job := s.toJobLocked()
		s.mu.Unlock()
		e.recordJob(job)
		return s, nil
	}

	return nil, errNotFound("session %q not found", jobID)
}

// FailQueued marks a queued job as failed. It is used at startup to fail
// orphaned queued jobs (those with no live session after a restart).
func (e *Engine) FailQueued(job *corev1.Job) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.recordJob(job)
}

// checkQueueTTL fails queued downloads that have waited longer than the
// queue TTL. The caller must hold e.mu.
func (e *Engine) checkQueueTTL() {
	now := e.now()
	for memberUserID, q := range e.queue {
		kept := q[:0]
		for _, qr := range q {
			if now.Sub(qr.createdAt) > e.queueTTL {
				e.recordJob(&corev1.Job{
					Id:          qr.id,
					Kind:        corev1.JobKind_JOB_KIND_DELIVERY,
					OwnerUserId: memberUserID,
					Status:      corev1.JobStatus_JOB_STATUS_FAILED,
					Error:       "queued too long",
				})
				continue
			}
			kept = append(kept, qr)
		}
		e.queue[memberUserID] = kept
	}
}

// cleanupIdempotency drops idempotency keys older than the retention window.
// The caller must hold e.mu.
func (e *Engine) cleanupIdempotency() {
	now := e.now()
	for key, entry := range e.idempotency {
		if now.Sub(entry.createdAt) > e.idempotencyTTL {
			delete(e.idempotency, key)
		}
	}
}
