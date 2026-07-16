package schedule

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ArianAr/Gantry/pkg/db"
)

type fakeEngine struct {
	starts []string
}

func (f *fakeEngine) StartJob(_ context.Context, ruleID string) (*db.JobRun, error) {
	f.starts = append(f.starts, ruleID)
	return &db.JobRun{ID: "job-1", SyncRuleID: ruleID, Status: db.JobStatusActive}, nil
}

func TestValidateAndNextRun(t *testing.T) {
	s := New(nil, &fakeEngine{})
	if err := s.ValidateCron("not a cron"); err == nil {
		t.Fatal("expected invalid cron error")
	}
	if err := s.ValidateCron("*/5 * * * *"); err != nil {
		t.Fatalf("valid cron: %v", err)
	}
	from := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	next, err := s.NextRun("*/5 * * * *", from)
	if err != nil || next == nil {
		t.Fatalf("next: %v %v", next, err)
	}
	// at exactly 12:00, next minute-based */5 may be 12:05
	if next.Before(from) || next.Equal(from) {
		// robfig Next is strictly after from
		t.Fatalf("next %v should be after %v", next, from)
	}
}

func TestTickStartsDueRule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	src := &db.Provider{Name: "s", ProviderType: "aws", Region: "us-east-1", AccessKeyID: "a", SecretAccessKey: "b"}
	dst := &db.Provider{Name: "d", ProviderType: "aws", Region: "us-east-1", AccessKeyID: "c", SecretAccessKey: "d"}
	_ = database.CreateProvider(src)
	_ = database.CreateProvider(dst)

	past := time.Now().UTC().Add(-time.Minute)
	rule := &db.SyncRule{
		Name:             "scheduled",
		SourceProviderID: src.ID,
		SourceBucket:     "in",
		TargetProviderID: dst.ID,
		TargetBucket:     "out",
		ScheduleCron:     "*/1 * * * *",
		ScheduleEnabled:  true,
		NextRunAt:        &past,
		ConcurrencyLimit: 2,
	}
	if err := database.CreateOrUpdateRule(rule); err != nil {
		t.Fatal(err)
	}

	eng := &fakeEngine{}
	s := New(database, eng)
	s.tick(context.Background(), time.Now().UTC())

	if len(eng.starts) != 1 || eng.starts[0] != rule.ID {
		t.Fatalf("starts=%v want [%s]", eng.starts, rule.ID)
	}
	got, _ := database.GetRule(rule.ID)
	if got.LastScheduledAt == nil {
		t.Fatal("expected LastScheduledAt")
	}
	if got.NextRunAt == nil || !got.NextRunAt.After(time.Now().UTC().Add(-time.Minute)) {
		t.Fatalf("expected future NextRunAt, got %v", got.NextRunAt)
	}
}

func TestTickSkipsActiveJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	src := &db.Provider{Name: "s", ProviderType: "aws", Region: "us-east-1", AccessKeyID: "a", SecretAccessKey: "b"}
	dst := &db.Provider{Name: "d", ProviderType: "aws", Region: "us-east-1", AccessKeyID: "c", SecretAccessKey: "d"}
	_ = database.CreateProvider(src)
	_ = database.CreateProvider(dst)

	past := time.Now().UTC().Add(-time.Minute)
	rule := &db.SyncRule{
		Name: "busy", SourceProviderID: src.ID, SourceBucket: "in",
		TargetProviderID: dst.ID, TargetBucket: "out",
		ScheduleCron: "*/1 * * * *", ScheduleEnabled: true, NextRunAt: &past,
	}
	_ = database.CreateOrUpdateRule(rule)
	now := time.Now().UTC()
	_ = database.CreateJobRun(&db.JobRun{
		SyncRuleID: rule.ID, Status: db.JobStatusActive, StartedAt: &now,
	})

	eng := &fakeEngine{}
	s := New(database, eng)
	s.tick(context.Background(), time.Now().UTC())
	if len(eng.starts) != 0 {
		t.Fatalf("expected skip, starts=%v", eng.starts)
	}
}
