package s3

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ArianAr/Gantry/pkg/db"
)

func TestEnqueuePriorityOrder(t *testing.T) {
	e := NewEngine(nil, nil)
	e.mu.Lock()
	e.enqueueLocked(queuedStart{job: &db.JobRun{ID: "low", Priority: 1}})
	e.enqueueLocked(queuedStart{job: &db.JobRun{ID: "high", Priority: 10}})
	e.enqueueLocked(queuedStart{job: &db.JobRun{ID: "mid", Priority: 5}})
	if len(e.queue) != 3 {
		t.Fatalf("len=%d", len(e.queue))
	}
	if e.queue[0].job.ID != "high" || e.queue[1].job.ID != "mid" || e.queue[2].job.ID != "low" {
		t.Fatalf("order: %s %s %s", e.queue[0].job.ID, e.queue[1].job.ID, e.queue[2].job.ID)
	}
	e.mu.Unlock()
}

func TestMaxConcurrentJobsQueues(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "q.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	src := &db.Provider{Name: "s", ProviderType: "minio", Region: "us-east-1", AccessKeyID: "a", SecretAccessKey: "b", Endpoint: "http://127.0.0.1:1"}
	dst := &db.Provider{Name: "d", ProviderType: "minio", Region: "us-east-1", AccessKeyID: "c", SecretAccessKey: "d", Endpoint: "http://127.0.0.1:1"}
	_ = d.CreateProvider(src)
	_ = d.CreateProvider(dst)

	// Block runJob path by never connecting — but StartJob only enqueues + dispatches.
	// Use max=1 and cancel immediately to validate queue depth without real S3.
	eng := NewEngine(d, nopEmitter{})
	eng.SetMaxConcurrentJobs(1)

	// Inject a hang by replacing jobCtx with a cancelled-never context and
	// using rules that fail DryRun quickly after start.
	// Better approach: max=1, start three jobs with priorities; first will fail
	// list (bad endpoint) but still occupies a slot briefly.
	// Use a custom wait: set MaxConcurrentJobs=1, manually fill running via lock.

	eng.mu.Lock()
	eng.running = 1 // simulate full capacity
	eng.mu.Unlock()

	ruleHi := &db.SyncRule{
		Name: "hi", SourceProviderID: src.ID, SourceBucket: "in",
		TargetProviderID: dst.ID, TargetBucket: "out", Priority: 50, ConcurrencyLimit: 1,
	}
	ruleLo := &db.SyncRule{
		Name: "lo", SourceProviderID: src.ID, SourceBucket: "in",
		TargetProviderID: dst.ID, TargetBucket: "out", Priority: 1, ConcurrencyLimit: 1,
	}
	if err := d.CreateOrUpdateRule(ruleHi); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateOrUpdateRule(ruleLo); err != nil {
		t.Fatal(err)
	}

	j1, err := eng.StartJob(context.Background(), ruleLo.ID)
	if err != nil {
		t.Fatal(err)
	}
	j2, err := eng.StartJob(context.Background(), ruleHi.ID)
	if err != nil {
		t.Fatal(err)
	}
	if j1.Status != db.JobStatusQueued || j2.Status != db.JobStatusQueued {
		t.Fatalf("expected queued statuses")
	}

	queued, running := eng.QueueDepth()
	if running != 1 {
		t.Fatalf("running=%d want 1 (simulated)", running)
	}
	if queued != 2 {
		t.Fatalf("queued=%d want 2", queued)
	}
	// High priority should be first in the wait list.
	eng.mu.Lock()
	if eng.queue[0].job.ID != j2.ID {
		eng.mu.Unlock()
		t.Fatalf("expected high priority job first, got %s then %s", eng.queue[0].job.ID, eng.queue[1].job.ID)
	}
	// Free the simulated slot and dispatch.
	eng.running = 0
	eng.dispatchLocked()
	// High priority should have started (running=1, queue=1).
	if eng.running != 1 || len(eng.queue) != 1 {
		eng.mu.Unlock()
		t.Fatalf("after dispatch running=%d queue=%d", eng.running, len(eng.queue))
	}
	if eng.queue[0].job.ID != j1.ID {
		eng.mu.Unlock()
		t.Fatalf("low priority should remain queued")
	}
	lowID := eng.queue[0].job.ID
	var runIDs []string
	for id, c := range eng.cancels {
		runIDs = append(runIDs, id)
		if c != nil {
			c()
		}
	}
	eng.mu.Unlock()

	if !eng.CancelJob(lowID) {
		t.Fatal("expected cancel of queued low-priority job")
	}
	// Wait briefly for failed dry-runs / cancel to finish.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		q, r := eng.QueueDepth()
		if q == 0 && r == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	q, r := eng.QueueDepth()
	if q != 0 || r != 0 {
		t.Logf("queue not fully drained q=%d r=%d runIDs=%v", q, r, runIDs)
	}
}
