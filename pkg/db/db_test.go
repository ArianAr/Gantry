package db

import (
	"path/filepath"
	"testing"
	"time"
)

func tempDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestProviderCRUD(t *testing.T) {
	d := tempDB(t)
	p := &Provider{
		Name:            "MinIO Local",
		ProviderType:    "minio",
		Endpoint:        "http://127.0.0.1:9000",
		Region:          "us-east-1",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
	}
	if err := d.CreateProvider(p); err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected ID")
	}
	list, err := d.ListProviders()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListProviders: %v len=%d", err, len(list))
	}
	got, err := d.GetProvider(p.ID)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if got.SecretAccessKey != "minioadmin" {
		t.Fatalf("secret not stored")
	}
	redacted := got.Redacted()
	if redacted.SecretAccessKey != "********" {
		t.Fatalf("expected redacted secret, got %q", redacted.SecretAccessKey)
	}
	if err := d.DeleteProvider(p.ID); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}
}

func TestProviderDeleteBlockedByRule(t *testing.T) {
	d := tempDB(t)
	src := &Provider{Name: "src", ProviderType: "aws", Region: "us-east-1", AccessKeyID: "a", SecretAccessKey: "b"}
	dst := &Provider{Name: "dst", ProviderType: "aws", Region: "us-east-1", AccessKeyID: "c", SecretAccessKey: "d"}
	_ = d.CreateProvider(src)
	_ = d.CreateProvider(dst)
	rule := &SyncRule{
		Name:             "r1",
		SourceProviderID: src.ID,
		SourceBucket:     "b1",
		TargetProviderID: dst.ID,
		TargetBucket:     "b2",
		ConcurrencyLimit: 8,
	}
	if err := d.CreateOrUpdateRule(rule); err != nil {
		t.Fatalf("CreateOrUpdateRule: %v", err)
	}
	if err := d.DeleteProvider(src.ID); err == nil {
		t.Fatal("expected delete blocked")
	}
}

func TestSyncRuleClampAndUpdate(t *testing.T) {
	d := tempDB(t)
	src := &Provider{Name: "src", ProviderType: "aws", Region: "us-east-1", AccessKeyID: "a", SecretAccessKey: "b"}
	dst := &Provider{Name: "dst", ProviderType: "aws", Region: "us-east-1", AccessKeyID: "c", SecretAccessKey: "d"}
	_ = d.CreateProvider(src)
	_ = d.CreateProvider(dst)

	rule := &SyncRule{
		Name:               "pipe",
		SourceProviderID:   src.ID,
		SourceBucket:       "in",
		TargetProviderID:   dst.ID,
		TargetBucket:       "out",
		ConcurrencyLimit:   100,
		BandwidthLimitKbps: 1024,
	}
	if err := d.CreateOrUpdateRule(rule); err != nil {
		t.Fatalf("create: %v", err)
	}
	if rule.ConcurrencyLimit != 32 {
		t.Fatalf("expected clamp to 32, got %d", rule.ConcurrencyLimit)
	}
	rule.Name = "pipe-renamed"
	rule.ConcurrencyLimit = 0
	if err := d.CreateOrUpdateRule(rule); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := d.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.Name != "pipe-renamed" || got.ConcurrencyLimit != 1 {
		t.Fatalf("unexpected rule: %+v", got)
	}
	parts := ParsePatterns(".png; .jpg ;; .gif")
	if len(parts) != 3 {
		t.Fatalf("ParsePatterns: %v", parts)
	}
}

func TestJobRunLifecycle(t *testing.T) {
	d := tempDB(t)
	now := time.Now().UTC()
	j := &JobRun{
		SyncRuleID: "rule-1",
		Status:     JobStatusQueued,
		IsDryRun:   false,
		StartedAt:  &now,
	}
	if err := d.CreateJobRun(j); err != nil {
		t.Fatalf("CreateJobRun: %v", err)
	}
	j.Status = JobStatusActive
	j.FilesTransferred = 3
	j.BytesTransferred = 1024
	if err := d.UpdateJobRun(j); err != nil {
		t.Fatalf("UpdateJobRun: %v", err)
	}
	got, err := d.GetJobRun(j.ID)
	if err != nil {
		t.Fatalf("GetJobRun: %v", err)
	}
	if got.Status != JobStatusActive || got.FilesTransferred != 3 {
		t.Fatalf("unexpected job: %+v", got)
	}
	active, err := d.ListActiveJobs()
	if err != nil || len(active) != 1 {
		t.Fatalf("ListActiveJobs: %v len=%d", err, len(active))
	}
	done := time.Now().UTC()
	j.Status = JobStatusCompleted
	j.CompletedAt = &done
	_ = d.UpdateJobRun(j)
	list, err := d.ListJobRuns(10)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListJobRuns: %v len=%d", err, len(list))
	}
}

func TestParseExtraTargets(t *testing.T) {
	got := ParseExtraTargets("backup;archive:cold/;  ;:skip")
	if len(got) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(got), got)
	}
	if got[0].Bucket != "backup" || got[0].Prefix != "" {
		t.Fatalf("got[0]=%+v", got[0])
	}
	if got[1].Bucket != "archive" || got[1].Prefix != "cold/" {
		t.Fatalf("got[1]=%+v", got[1])
	}
	if ParseExtraTargets("") != nil {
		t.Fatal("empty should be nil")
	}
}
