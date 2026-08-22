package serving

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nem-git/abcmovies/core/app"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// The debug job endpoint is a manual-testing affordance, not part of the core
// API contract (TECHNICAL-DECISIONS.md §1.2: the core API speaks gRPC only).
// It exists so a browser session can observe the live job/event flow — M0
// exposes no public CreateJob RPC and nothing in a running instance publishes
// events on its own. It drives the documented embedder seam (app.Stack.
// EnqueueJob), requires the same bearer-token authentication as every other
// protected method, and is owned entirely by this frontend's serving layer.

// debugJobHandler serves POST /debug/job: it authenticates the caller,
// creates one probe job owned by them, and steps it queued → running → done
// so subscribers see real status transitions on the event bus.
type debugJobHandler struct {
	stack *app.Stack
}

func (h debugJobHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := bearerUser(r, h.stack.Auth())
	if !ok {
		http.Error(w, "missing or invalid bearer token", http.StatusUnauthorized)
		return
	}

	jobID := fmt.Sprintf("probe-%d", time.Now().UnixNano())
	steps := []struct {
		status  corev1.JobStatus
		message string
	}{
		{corev1.JobStatus_JOB_STATUS_QUEUED, ""},
		{corev1.JobStatus_JOB_STATUS_RUNNING, "probe running"},
		{corev1.JobStatus_JOB_STATUS_DONE, ""},
	}
	for i, step := range steps {
		job := &corev1.Job{
			Id:          jobID,
			Kind:        corev1.JobKind_JOB_KIND_REFRESH,
			Status:      step.status,
			OwnerUserId: userID,
		}
		if step.message != "" {
			job.Progress = &corev1.JobProgress{Percent: uint32(i) * 50, Message: step.message}
		}
		if err := h.stack.EnqueueJob(r.Context(), job); err != nil {
			http.Error(w, fmt.Sprintf("enqueue job: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"job_id": jobID}); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
	}
}

// bearerUser resolves the request's Authorization header to an authenticated
// user ID, mirroring the connect interceptor's rule for protected methods.
func bearerUser(r *http.Request, session app.Session) (string, bool) {
	const prefix = "Bearer "
	raw := r.Header.Get("Authorization")
	if !strings.HasPrefix(raw, prefix) {
		return "", false
	}
	uid, err := session.Validate(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return "", false
	}
	return uid, true
}
