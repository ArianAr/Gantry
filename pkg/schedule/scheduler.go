package schedule

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ArianAr/Gantry/pkg/db"
	"github.com/ArianAr/Gantry/pkg/s3"
	"github.com/robfig/cron/v3"
)

// EngineStarter starts a sync job for a rule (implemented by *s3.Engine).
type EngineStarter interface {
	StartJob(ctx context.Context, ruleID string) (*db.JobRun, error)
}

// Scheduler evaluates enabled cron schedules and starts jobs.
type Scheduler struct {
	DB     *db.DB
	Engine EngineStarter
	// TickInterval is how often to poll rules (default 30s).
	TickInterval time.Duration
	// parser uses standard 5-field cron (minute-first).
	parser cron.Parser

	mu     sync.Mutex
	cancel context.CancelFunc
}

// New creates a scheduler.
func New(database *db.DB, engine EngineStarter) *Scheduler {
	return &Scheduler{
		DB:           database,
		Engine:       engine,
		TickInterval: 30 * time.Second,
		parser:       cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
	}
}

// ValidateCron returns an error if expr is non-empty and invalid.
func (s *Scheduler) ValidateCron(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	_, err := s.parser.Parse(expr)
	return err
}

// NextRun returns the next fire time after from for a valid cron expression.
func (s *Scheduler) NextRun(expr string, from time.Time) (*time.Time, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}
	sched, err := s.parser.Parse(expr)
	if err != nil {
		return nil, err
	}
	n := sched.Next(from)
	return &n, nil
}

// Start begins the background loop until ctx is cancelled or Stop is called.
func (s *Scheduler) Start(parent context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.mu.Unlock()

	interval := s.TickInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go s.loop(ctx, interval)
}

// Stop cancels the background loop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *Scheduler) loop(ctx context.Context, interval time.Duration) {
	// Refresh next_run_at for all enabled rules on start.
	s.refreshAllNextRuns()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.tick(ctx, now.UTC())
		}
	}
}

func (s *Scheduler) refreshAllNextRuns() {
	rules, err := s.DB.ListRules()
	if err != nil {
		log.Printf("schedule: list rules: %v", err)
		return
	}
	now := time.Now().UTC()
	for i := range rules {
		r := &rules[i]
		if !r.ScheduleEnabled || strings.TrimSpace(r.ScheduleCron) == "" {
			if r.NextRunAt != nil {
				r.NextRunAt = nil
				_ = s.DB.CreateOrUpdateRule(r)
			}
			continue
		}
		next, err := s.NextRun(r.ScheduleCron, now)
		if err != nil {
			log.Printf("schedule: rule %s invalid cron %q: %v", r.ID, r.ScheduleCron, err)
			continue
		}
		r.NextRunAt = next
		_ = s.DB.CreateOrUpdateRule(r)
	}
}

func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	rules, err := s.DB.ListRules()
	if err != nil {
		log.Printf("schedule: list rules: %v", err)
		return
	}
	for i := range rules {
		r := &rules[i]
		if !r.ScheduleEnabled || strings.TrimSpace(r.ScheduleCron) == "" {
			continue
		}
		// Ensure NextRunAt is set
		if r.NextRunAt == nil {
			next, err := s.NextRun(r.ScheduleCron, now.Add(-time.Second))
			if err != nil || next == nil {
				continue
			}
			r.NextRunAt = next
			_ = s.DB.CreateOrUpdateRule(r)
		}
		if r.NextRunAt.After(now) {
			continue
		}
		// Due: skip if already active for this rule
		if s.ruleHasActiveJob(r.ID) {
			log.Printf("schedule: skip rule %s (%s): job already active", r.ID, r.Name)
			// push next run forward so we don't spin
			next, err := s.NextRun(r.ScheduleCron, now)
			if err == nil && next != nil {
				r.NextRunAt = next
				_ = s.DB.CreateOrUpdateRule(r)
			}
			continue
		}
		log.Printf("schedule: starting rule %s (%s)", r.ID, r.Name)
		if _, err := s.Engine.StartJob(ctx, r.ID); err != nil {
			log.Printf("schedule: start rule %s: %v", r.ID, err)
			// still advance next to avoid tight retry loop on permanent errors
		}
		ts := now
		r.LastScheduledAt = &ts
		next, err := s.NextRun(r.ScheduleCron, now)
		if err == nil {
			r.NextRunAt = next
		}
		_ = s.DB.CreateOrUpdateRule(r)
	}
}

func (s *Scheduler) ruleHasActiveJob(ruleID string) bool {
	jobs, err := s.DB.ListActiveJobs()
	if err != nil {
		return false
	}
	for _, j := range jobs {
		if j.SyncRuleID == ruleID {
			return true
		}
	}
	return false
}

// Ensure s3.Engine implements EngineStarter at compile time when imported by main.
var _ EngineStarter = (*s3.Engine)(nil)
