package retention

import (
	"context"
	"log"
	"time"

	"github.com/ArianAr/Gantry/pkg/db"
)

// Runner periodically purges old terminal job runs.
type Runner struct {
	DB           *db.DB
	Retention    time.Duration // 0 disables
	TickInterval time.Duration // default 1h
}

// Start launches a background loop until ctx is cancelled.
func (r *Runner) Start(ctx context.Context) {
	if r.Retention <= 0 {
		return
	}
	interval := r.TickInterval
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		// initial purge shortly after start
		r.purgeOnce()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				r.purgeOnce()
			}
		}
	}()
}

func (r *Runner) purgeOnce() {
	cutoff := time.Now().UTC().Add(-r.Retention)
	n, err := r.DB.PurgeOldJobs(cutoff)
	if err != nil {
		log.Printf("retention: purge failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("retention: purged %d job run(s) older than %s", n, r.Retention)
	}
}
