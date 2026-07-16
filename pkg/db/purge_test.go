package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPurgeOldJobs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	old := time.Now().UTC().Add(-48 * time.Hour)
	recent := time.Now().UTC().Add(-1 * time.Hour)
	activeStart := time.Now().UTC()

	_ = d.CreateJobRun(&JobRun{SyncRuleID: "r1", Status: JobStatusCompleted, CompletedAt: &old})
	_ = d.CreateJobRun(&JobRun{SyncRuleID: "r1", Status: JobStatusFailed, CompletedAt: &old})
	_ = d.CreateJobRun(&JobRun{SyncRuleID: "r1", Status: JobStatusCompleted, CompletedAt: &recent})
	_ = d.CreateJobRun(&JobRun{SyncRuleID: "r1", Status: JobStatusActive, StartedAt: &activeStart})

	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	n, err := d.PurgeOldJobs(cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("purged %d want 2", n)
	}
	list, err := d.ListJobRuns(50)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("remaining %d want 2", len(list))
	}
	for _, j := range list {
		if j.Status == JobStatusCompleted && j.CompletedAt != nil && j.CompletedAt.Before(cutoff) {
			t.Fatalf("old completed job still present: %+v", j)
		}
	}
}

func TestPing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Ping(); err != nil {
		t.Fatal(err)
	}
}
