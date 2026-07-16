package s3

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// EngineStats holds thread-safe transfer counters for active jobs.
type EngineStats struct {
	BytesRead     atomic.Int64
	BytesWritten  atomic.Int64 // alias of transferred payload
	FilesDone     atomic.Int64
	FilesFailed   atomic.Int64
	FilesSkipped  atomic.Int64
	TotalFiles    atomic.Int64
	TotalBytes    atomic.Int64
	ActiveWorkers atomic.Int64
	JobID         string
	RuleID        string
	StartedAt     time.Time
	mu            sync.RWMutex
	workers       map[int]*WorkerStatus
	lastBytes     int64
	lastSample    time.Time
	rollingBps    float64
}

// WorkerStatus describes what a single worker is transferring.
type WorkerStatus struct {
	WorkerID  int     `json:"worker_id"`
	Key       string  `json:"key"`
	Source    string  `json:"source"`
	Target    string  `json:"target"`
	SizeBytes int64   `json:"size_bytes"`
	BytesDone int64   `json:"bytes_done"`
	Percent   float64 `json:"percent"`
	Active    bool    `json:"active"`
}

// NewEngineStats creates stats for a job.
func NewEngineStats(jobID, ruleID string) *EngineStats {
	return &EngineStats{
		JobID:      jobID,
		RuleID:     ruleID,
		StartedAt:  time.Now().UTC(),
		workers:    make(map[int]*WorkerStatus),
		lastSample: time.Now(),
	}
}

// SetWorker updates a worker slot status (merges non-empty fields).
func (s *EngineStats) SetWorker(w WorkerStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.workers[w.WorkerID]
	if !ok || cur == nil {
		cp := w
		s.workers[w.WorkerID] = &cp
		return
	}
	if w.Key != "" {
		cur.Key = w.Key
	}
	if w.Source != "" {
		cur.Source = w.Source
	}
	if w.Target != "" {
		cur.Target = w.Target
	}
	if w.SizeBytes > 0 {
		cur.SizeBytes = w.SizeBytes
	}
	cur.BytesDone = w.BytesDone
	cur.Percent = w.Percent
	cur.Active = w.Active
	cur.WorkerID = w.WorkerID
}

// ClearWorker marks a worker idle.
func (s *EngineStats) ClearWorker(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.workers[id]; ok {
		w.Active = false
		w.Key = ""
		w.Percent = 0
		w.BytesDone = 0
	}
}

// SnapshotWorkers returns a copy of worker statuses.
func (s *EngineStats) SnapshotWorkers() []WorkerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]WorkerStatus, 0, len(s.workers))
	for _, w := range s.workers {
		if w != nil {
			out = append(out, *w)
		}
	}
	return out
}

// SampleSpeed updates rolling bytes/sec and returns current estimate.
func (s *EngineStats) SampleSpeed() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cur := s.BytesRead.Load()
	elapsed := now.Sub(s.lastSample).Seconds()
	if elapsed >= 0.2 {
		delta := float64(cur - s.lastBytes)
		inst := delta / elapsed
		if s.rollingBps <= 0 {
			s.rollingBps = inst
		} else {
			s.rollingBps = s.rollingBps*0.7 + inst*0.3
		}
		s.lastBytes = cur
		s.lastSample = now
	}
	return s.rollingBps
}

// ProgressReader wraps an io.Reader, counting bytes and optionally rate-limiting.
type ProgressReader struct {
	r          io.Reader
	ctx        context.Context
	stats      *EngineStats
	workerID   int
	size       int64
	done       int64
	limiter    *rate.Limiter
	onProgress func(done, total int64)
}

// NewProgressReader constructs a counting reader.
// bandwidthKbps <= 0 means unlimited.
func NewProgressReader(ctx context.Context, r io.Reader, stats *EngineStats, workerID int, size int64, bandwidthKbps int) *ProgressReader {
	pr := &ProgressReader{
		r:        r,
		ctx:      ctx,
		stats:    stats,
		workerID: workerID,
		size:     size,
	}
	if bandwidthKbps > 0 {
		// Convert kbps → bytes/sec; burst = 1 second of bandwidth (min 32KiB).
		bps := float64(bandwidthKbps) * 1000 / 8
		burst := int(bps)
		if burst < 32*1024 {
			burst = 32 * 1024
		}
		pr.limiter = rate.NewLimiter(rate.Limit(bps), burst)
	}
	return pr
}

// Read implements io.Reader with optional rate limiting and stats updates.
func (p *ProgressReader) Read(buf []byte) (int, error) {
	if p.ctx != nil {
		if err := p.ctx.Err(); err != nil {
			return 0, err
		}
	}
	if p.limiter != nil {
		// Limit whole buffer request; WaitN may reduce effective chunk size.
		n := len(buf)
		if err := p.limiter.WaitN(p.ctx, n); err != nil {
			// If burst too small for n, try smaller chunks.
			if n > p.limiter.Burst() {
				n = p.limiter.Burst()
				buf = buf[:n]
				if err := p.limiter.WaitN(p.ctx, n); err != nil {
					return 0, err
				}
			} else {
				return 0, err
			}
		}
	}
	n, err := p.r.Read(buf)
	if n > 0 {
		atomicAdd := int64(n)
		p.done += atomicAdd
		if p.stats != nil {
			p.stats.BytesRead.Add(atomicAdd)
			p.stats.BytesWritten.Add(atomicAdd)
			pct := 0.0
			if p.size > 0 {
				pct = float64(p.done) / float64(p.size) * 100
			}
			p.stats.SetWorker(WorkerStatus{
				WorkerID:  p.workerID,
				SizeBytes: p.size,
				BytesDone: p.done,
				Percent:   pct,
				Active:    true,
			})
		}
		if p.onProgress != nil {
			p.onProgress(p.done, p.size)
		}
	}
	return n, err
}

// BytesRead reports bytes consumed so far.
func (p *ProgressReader) BytesRead() int64 {
	return p.done
}

// Ensure ProgressReader is always a Reader.
var _ io.Reader = (*ProgressReader)(nil)
