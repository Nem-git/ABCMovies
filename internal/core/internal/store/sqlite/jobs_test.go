package sqlite

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCreateAndCompleteJobLifecycle(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	src := mustCreateSource(t, s, "jf-jobs-1")

	job, err := s.CreateJob(ctx, JobKindSync, "usr_admin", src.SourceID)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if !strings.HasPrefix(job.JobID, "job_") {
		t.Fatalf("job_id = %q, want job_ prefix", job.JobID)
	}
	if job.State != JobStateRunning || job.Kind != JobKindSync || job.SourceID != src.SourceID {
		t.Fatalf("job = %+v", job)
	}

	// Running job reports in_flight in the freshness snapshot.
	list, err := s.ListSyncStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !list[0].InFlight {
		t.Fatalf("ListSyncStates in_flight = %v, want [] with in_flight=true", list)
	}

	long := strings.Repeat("a", 300)
	if err := s.CompleteJob(ctx, job.JobID, JobStateSucceeded, long); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}
	got, err := s.LatestSyncJob(ctx, src.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != JobStateSucceeded {
		t.Fatalf("job state = %d, want succeeded", got.State)
	}
	if len(got.Detail) != jobDetailMaxLength {
		t.Fatalf("detail length = %d, want truncated to %d", len(got.Detail), jobDetailMaxLength)
	}
	ss, err := s.getSourceSyncState(ctx, src.SourceID)

	if err != nil {
		// Source never synced: no freshness row exists, but that is the honest
		// state — ListSyncStates already confirmed in_flight=false.
		if err != ErrNotFound {
			t.Fatalf("getSourceSyncState: %v", err)
		}
	} else if ss.InFlight {
		t.Fatal("in_flight = true after job completed")
	}

	// Invalid terminal state is rejected.
	if err := s.CompleteJob(ctx, job.JobID, JobStateRunning, ""); err == nil {
		t.Fatal("CompleteJob with running state must be rejected")
	}
	// Unknown job id fails loudly.
	if err := s.CompleteJob(ctx, "job_notexists", JobStateSucceeded, ""); err != ErrNotFound {
		t.Fatalf("CompleteJob missing id = %v, want ErrNotFound", err)
	}
}

func TestInterruptOrphanedJobsMarksRunningInterrupted(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	src := mustCreateSource(t, s, "jf-jobs-2")

	// Prior visible snapshot (success) — must survive the sweep atomically with
	// only outcome/detail flipped to interrupted.
	syncedAt := time.Now().UTC().Add(-time.Hour)
	if _, err := s.ReplaceSourceSnapshot(ctx, src.SourceID, LibraryOutcomeSuccess, &syncedAt, "", []LibraryTitleInput{
		{ExternalID: "jf-1", Name: "Blade Runner", Kind: LibraryTitleKindMovie, Year: 1982},
		{ExternalID: "jf-2", Name: "Severance", Kind: LibraryTitleKindSeries},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateJob(ctx, JobKindSync, "usr_admin", src.SourceID); err != nil {
		t.Fatal(err)
	}

	n, err := s.InterruptOrphanedJobs(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("interrupted = %d, want 1", n)
	}
	got, err := s.LatestSyncJob(ctx, src.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != JobStateInterrupted {
		t.Fatalf("job state = %d, want interrupted", got.State)
	}
	// Freshness must not pretend the sync completed.
	list, err := s.ListSyncStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 sync state, got %d", len(list))
	}
	st := list[0]
	if st.LastOutcome != LibraryOutcomeInterrupted {
		t.Fatalf("last_outcome = %d, want LibraryOutcomeInterrupted", st.LastOutcome)
	}
	if st.TitleCount != 2 {
		t.Fatalf("title_count = %d, want 2 (preserved)", st.TitleCount)
	}
	if st.LastSyncedAt == nil || !st.LastSyncedAt.Equal(syncedAt) {
		t.Fatalf("last_synced_at = %v, want preserved %v", st.LastSyncedAt, syncedAt)
	}
	if st.Detail == "" {
		t.Fatal("interrupted detail must not be empty")
	}
	if st.InFlight {
		t.Fatal("in_flight must be false after sweep")
	}

	// Sweep is idempotent: nothing left running.
	n2, err := s.InterruptOrphanedJobs(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second sweep = %d, want 0", n2)
	}
}

func TestInterruptOrphanedJobsNeverSyncedSourceGetsMarker(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	src := mustCreateSource(t, s, "jf-jobs-3")

	if _, err := s.CreateJob(ctx, JobKindSync, "usr_admin", src.SourceID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InterruptOrphanedJobs(ctx, ""); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSyncStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 sync state, got %d", len(list))
	}
	st := list[0]
	if st.LastOutcome != LibraryOutcomeInterrupted || st.TitleCount != 0 {
		t.Fatalf("state = %+v, want interrupted with 0 titles", st)
	}
	if st.LastSyncedAt != nil {
		t.Fatalf("last_synced_at = %v, want nil for never-synced source", st.LastSyncedAt)
	}
}

func TestInterruptOrphanedJobsLeavesSourcesAlone(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	good := mustCreateSource(t, s, "jf-jobs-4a")
	orphaned := mustCreateSource(t, s, "jf-jobs-4b")

	// 'good' completes its job; 'orphaned' keeps a running one.
	gj, err := s.CreateJob(ctx, JobKindSync, "usr_admin", good.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteJob(ctx, gj.JobID, JobStateSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateJob(ctx, JobKindSync, "usr_admin", orphaned.SourceID); err != nil {
		t.Fatal(err)
	}

	if _, err := s.InterruptOrphanedJobs(ctx, ""); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSyncStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]int32{}
	for _, st := range list {
		outcomes[st.SourceID] = st.LastOutcome
	}
	if outcomes[good.SourceID] != 0 {
		t.Fatalf("good source outcome = %d, want 0 (never synced, but also not swept — its job finished)", outcomes[good.SourceID])
	}
	if outcomes[orphaned.SourceID] != LibraryOutcomeInterrupted {
		t.Fatalf("orphaned source outcome = %d, want interrupted", outcomes[orphaned.SourceID])
	}
}
