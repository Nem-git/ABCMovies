package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Nem-git/ABCMovies/internal/core/internal/connector/source/jellyfin"
	"github.com/Nem-git/ABCMovies/internal/core/internal/store"
)

func TestSyncSourceRecordsJobSucceeded(t *testing.T) {
	svc, st := newTestLibraryService(t)
	req := mustConnectForSync(t, st)
	svc.fetcher = fakeFetcher{
		snapshot: &jellyfin.LibrarySnapshot{Items: []jellyfin.LibraryItem{
			{ExternalID: "jf-1", Name: "Blade Runner", Kind: "movie", Year: 1982},
		}},
	}
	ctx := context.Background()
	resp, err := svc.SyncSource(ctx, req)
	if err != nil {
		t.Fatalf("SyncSource: %v", err)
	}
	if resp.GetState().GetInFlight() {
		t.Fatal("response in_flight = true after completed sync")
	}
	job, err := st.LatestSyncJob(ctx, req.GetSourceId())
	if err != nil {
		t.Fatal(err)
	}
	if job.State != store.JobStateSucceeded {
		t.Fatalf("job state = %d, want succeeded", job.State)
	}
	if job.UserID != req.GetActorUserId() {
		t.Fatalf("job user = %q, want %q", job.UserID, req.GetActorUserId())
	}
}

func TestSyncSourceRecordsJobFailed(t *testing.T) {
	svc, st := newTestLibraryService(t)
	req := mustConnectForSync(t, st)
	svc.fetcher = fakeFetcher{err: &jellyfin.Error{Message: "the source rejected the credentials (401)"}}
	ctx := context.Background()
	_, err := svc.SyncSource(ctx, req)
	if err == nil {
		t.Fatal("expected sync to fail")
	}
	job, jerr := st.LatestSyncJob(ctx, req.GetSourceId())
	if jerr != nil {
		t.Fatal(jerr)
	}
	if job.State != store.JobStateFailed {
		t.Fatalf("job state = %d, want failed", job.State)
	}
	if !strings.Contains(job.Detail, "credentials") {
		t.Fatalf("job detail = %q, want the safe fetch-error message", job.Detail)
	}
}
