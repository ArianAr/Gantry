package s3

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ArianAr/Gantry/pkg/db"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// EventType classifies engine events for SSE.
type EventType string

const (
	EventLog      EventType = "log"
	EventProgress EventType = "progress"
	EventJob      EventType = "job"
	EventWorker   EventType = "worker"
	EventError    EventType = "error"
)

// Event is a structured engine notification.
type Event struct {
	Type      EventType   `json:"type"`
	JobID     string      `json:"job_id,omitempty"`
	Message   string      `json:"message,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload,omitempty"`
}

// EventEmitter broadcasts engine events (typically the SSE hub).
type EventEmitter interface {
	Emit(Event)
}

// nopEmitter discards events.
type nopEmitter struct{}

func (nopEmitter) Emit(Event) {}

// ObjectInfo is a listed object summary.
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag,omitempty"`
}

// DryRunAction classification for dry-run matrix.
type DryRunAction string

const (
	ActionAdd    DryRunAction = "add"
	ActionModify DryRunAction = "modify"
	ActionDelete DryRunAction = "delete"
	ActionSkip   DryRunAction = "skip"
)

// DryRunItem is one row in the dry-run matrix.
type DryRunItem struct {
	SourceKey string       `json:"source_key,omitempty"`
	TargetKey string       `json:"target_key,omitempty"`
	Size      int64        `json:"size"`
	Action    DryRunAction `json:"action"`
	Reason    string       `json:"reason,omitempty"`
}

// DryRunResult is the full dry-run response.
type DryRunResult struct {
	Items            []DryRunItem `json:"items"`
	AddCount         int          `json:"add_count"`
	ModifyCount      int          `json:"modify_count"`
	DeleteCount      int          `json:"delete_count"`
	SkipCount        int          `json:"skip_count"`
	TotalBytesToSync int64        `json:"total_bytes_to_sync"`
}

// transferTask is an internal worker task.
type transferTask struct {
	SourceKey string
	TargetKey string
	Size      int64
	Action    DryRunAction
}

// Engine coordinates dry-runs and streaming sync jobs.
type Engine struct {
	DB      *db.DB
	Emitter EventEmitter
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	stats   map[string]*EngineStats
}

// NewEngine creates an engine.
func NewEngine(database *db.DB, emit EventEmitter) *Engine {
	if emit == nil {
		emit = nopEmitter{}
	}
	return &Engine{
		DB:      database,
		Emitter: emit,
		cancels: make(map[string]context.CancelFunc),
		stats:   make(map[string]*EngineStats),
	}
}

// GetStats returns live stats for a job, if any.
func (e *Engine) GetStats(jobID string) *EngineStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stats[jobID]
}

// ActiveStats returns all active job stats.
func (e *Engine) ActiveStats() []*EngineStats {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]*EngineStats, 0, len(e.stats))
	for _, s := range e.stats {
		out = append(out, s)
	}
	return out
}

// CancelJob cancels a running job.
func (e *Engine) CancelJob(jobID string) bool {
	e.mu.Lock()
	cancel, ok := e.cancels[jobID]
	e.mu.Unlock()
	if ok && cancel != nil {
		cancel()
		return true
	}
	return false
}

func (e *Engine) log(jobID, msg string) {
	e.Emitter.Emit(Event{
		Type:      EventLog,
		JobID:     jobID,
		Message:   msg,
		Timestamp: time.Now().UTC(),
	})
}

// mapTargetKey rewrites source key into target prefix space.
func mapTargetKey(sourceKey, sourcePrefix, targetPrefix string) string {
	rel := sourceKey
	if sourcePrefix != "" && strings.HasPrefix(sourceKey, sourcePrefix) {
		rel = strings.TrimPrefix(sourceKey, sourcePrefix)
		rel = strings.TrimPrefix(rel, "/")
	}
	if targetPrefix == "" {
		return rel
	}
	return path.Join(strings.TrimSuffix(targetPrefix, "/"), rel)
}

// matchPatterns applies include/exclude semicolon patterns (suffix or substring).
func matchPatterns(key string, include, exclude []string) bool {
	base := path.Base(key)
	for _, ex := range exclude {
		if patternMatch(key, base, ex) {
			return false
		}
	}
	if len(include) == 0 {
		return true
	}
	for _, in := range include {
		if patternMatch(key, base, in) {
			return true
		}
	}
	return false
}

func patternMatch(key, base, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	// Extension style: .png
	if strings.HasPrefix(pattern, ".") && !strings.Contains(pattern[1:], ".") {
		return strings.HasSuffix(strings.ToLower(base), strings.ToLower(pattern))
	}
	// Glob-ish suffix *
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, prefix) || strings.HasPrefix(base, prefix)
	}
	return strings.Contains(key, pattern) || base == pattern
}

func applyFilters(obj ObjectInfo, rule *db.SyncRule) (bool, string) {
	include := db.ParsePatterns(rule.IncludePatterns)
	exclude := db.ParsePatterns(rule.ExcludePatterns)
	if !matchPatterns(obj.Key, include, exclude) {
		return false, "pattern filter"
	}
	if rule.MinSizeBytes != nil && obj.Size < *rule.MinSizeBytes {
		return false, "below min size"
	}
	if rule.MaxSizeBytes != nil && obj.Size > *rule.MaxSizeBytes {
		return false, "above max size"
	}
	if rule.ModifiedAfter != nil && !obj.LastModified.IsZero() && obj.LastModified.Before(*rule.ModifiedAfter) {
		return false, "modified before threshold"
	}
	return true, ""
}

// listObjects paginates all keys under prefix.
func listObjects(ctx context.Context, cli *Client, bucket, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	var token *string
	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		}
		resp, err := cli.S3.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, err
		}
		for _, obj := range resp.Contents {
			key := aws.ToString(obj.Key)
			if key == "" || strings.HasSuffix(key, "/") {
				continue // skip folder placeholders
			}
			info := ObjectInfo{
				Key:  key,
				Size: aws.ToInt64(obj.Size),
				ETag: strings.Trim(aws.ToString(obj.ETag), `"`),
			}
			if obj.LastModified != nil {
				info.LastModified = *obj.LastModified
			}
			out = append(out, info)
		}
		if !aws.ToBool(resp.IsTruncated) {
			break
		}
		token = resp.NextContinuationToken
	}
	return out, nil
}

// DryRun compares source and target without transferring.
func (e *Engine) DryRun(ctx context.Context, rule *db.SyncRule) (*DryRunResult, error) {
	srcP, err := e.DB.GetProvider(rule.SourceProviderID)
	if err != nil {
		return nil, fmt.Errorf("source provider: %w", err)
	}
	dstP, err := e.DB.GetProvider(rule.TargetProviderID)
	if err != nil {
		return nil, fmt.Errorf("target provider: %w", err)
	}
	srcCli, err := NewClient(ctx, *srcP)
	if err != nil {
		return nil, err
	}
	dstCli, err := NewClient(ctx, *dstP)
	if err != nil {
		return nil, err
	}

	srcObjs, err := listObjects(ctx, srcCli, rule.SourceBucket, rule.SourcePrefix)
	if err != nil {
		return nil, fmt.Errorf("list source: %w", err)
	}
	dstObjs, err := listObjects(ctx, dstCli, rule.TargetBucket, rule.TargetPrefix)
	if err != nil {
		return nil, fmt.Errorf("list target: %w", err)
	}

	dstByKey := make(map[string]ObjectInfo, len(dstObjs))
	for _, o := range dstObjs {
		dstByKey[o.Key] = o
	}

	result := &DryRunResult{Items: make([]DryRunItem, 0)}
	seenTarget := make(map[string]struct{})

	for _, src := range srcObjs {
		ok, reason := applyFilters(src, rule)
		tKey := mapTargetKey(src.Key, rule.SourcePrefix, rule.TargetPrefix)
		if !ok {
			result.Items = append(result.Items, DryRunItem{
				SourceKey: src.Key, TargetKey: tKey, Size: src.Size, Action: ActionSkip, Reason: reason,
			})
			result.SkipCount++
			continue
		}
		seenTarget[tKey] = struct{}{}
		if dst, exists := dstByKey[tKey]; exists {
			if dst.Size == src.Size && (dst.ETag == "" || src.ETag == "" || dst.ETag == src.ETag) {
				result.Items = append(result.Items, DryRunItem{
					SourceKey: src.Key, TargetKey: tKey, Size: src.Size, Action: ActionSkip, Reason: "already in sync",
				})
				result.SkipCount++
				continue
			}
			result.Items = append(result.Items, DryRunItem{
				SourceKey: src.Key, TargetKey: tKey, Size: src.Size, Action: ActionModify, Reason: "size or etag differs",
			})
			result.ModifyCount++
			result.TotalBytesToSync += src.Size
			continue
		}
		result.Items = append(result.Items, DryRunItem{
			SourceKey: src.Key, TargetKey: tKey, Size: src.Size, Action: ActionAdd,
		})
		result.AddCount++
		result.TotalBytesToSync += src.Size
	}

	if rule.DeleteOnTarget {
		for _, dst := range dstObjs {
			if _, keep := seenTarget[dst.Key]; keep {
				continue
			}
			// Only consider keys under target prefix that look managed
			result.Items = append(result.Items, DryRunItem{
				TargetKey: dst.Key, Size: dst.Size, Action: ActionDelete, Reason: "not present on source",
			})
			result.DeleteCount++
		}
	}
	return result, nil
}

// StartJob launches a background sync for the given rule, returning the JobRun.
func (e *Engine) StartJob(parent context.Context, ruleID string) (*db.JobRun, error) {
	rule, err := e.DB.GetRule(ruleID)
	if err != nil {
		return nil, fmt.Errorf("rule: %w", err)
	}
	rule.ClampConcurrency()

	now := time.Now().UTC()
	job := &db.JobRun{
		SyncRuleID: rule.ID,
		Status:     db.JobStatusQueued,
		IsDryRun:   false,
		StartedAt:  &now,
	}
	if err := e.DB.CreateJobRun(job); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(parent)
	e.mu.Lock()
	e.cancels[job.ID] = cancel
	stats := NewEngineStats(job.ID, rule.ID)
	e.stats[job.ID] = stats
	e.mu.Unlock()

	go e.runJob(ctx, job, rule, stats)
	return job, nil
}

func (e *Engine) runJob(ctx context.Context, job *db.JobRun, rule *db.SyncRule, stats *EngineStats) {
	defer func() {
		e.mu.Lock()
		delete(e.cancels, job.ID)
		delete(e.stats, job.ID)
		e.mu.Unlock()
	}()

	job.Status = db.JobStatusActive
	_ = e.DB.UpdateJobRun(job)
	e.Emitter.Emit(Event{Type: EventJob, JobID: job.ID, Message: "job started", Timestamp: time.Now().UTC(), Payload: job})
	e.log(job.ID, fmt.Sprintf("starting sync rule %q", rule.Name))

	result, err := e.DryRun(ctx, rule)
	if err != nil {
		e.failJob(job, err)
		return
	}

	var tasks []transferTask
	var totalBytes int64
	for _, item := range result.Items {
		switch item.Action {
		case ActionAdd, ActionModify:
			tasks = append(tasks, transferTask{
				SourceKey: item.SourceKey,
				TargetKey: item.TargetKey,
				Size:      item.Size,
				Action:    item.Action,
			})
			totalBytes += item.Size
		case ActionSkip:
			stats.FilesSkipped.Add(1)
			job.FilesSkipped++
		case ActionDelete:
			tasks = append(tasks, transferTask{
				TargetKey: item.TargetKey,
				Size:      item.Size,
				Action:    ActionDelete,
			})
		}
	}
	stats.TotalFiles.Store(int64(len(tasks)))
	stats.TotalBytes.Store(totalBytes)
	job.TotalFilesDiscovered = int64(len(result.Items))
	job.TotalBytesDiscovered = totalBytes
	job.FilesSkipped = stats.FilesSkipped.Load()
	_ = e.DB.UpdateJobRun(job)

	srcP, err := e.DB.GetProvider(rule.SourceProviderID)
	if err != nil {
		e.failJob(job, err)
		return
	}
	dstP, err := e.DB.GetProvider(rule.TargetProviderID)
	if err != nil {
		e.failJob(job, err)
		return
	}
	srcCli, err := NewClient(ctx, *srcP)
	if err != nil {
		e.failJob(job, err)
		return
	}
	dstCli, err := NewClient(ctx, *dstP)
	if err != nil {
		e.failJob(job, err)
		return
	}

	workers := rule.ConcurrencyLimit
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}

	taskCh := make(chan transferTask)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i + 1
		go func(id int) {
			defer wg.Done()
			e.workerLoop(ctx, id, taskCh, rule, srcCli, dstCli, job, stats)
		}(workerID)
	}

	// Progress ticker
	doneCh := make(chan struct{})
	go func() {
		t := time.NewTicker(400 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-doneCh:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				bps := stats.SampleSpeed()
				e.Emitter.Emit(Event{
					Type:      EventProgress,
					JobID:     job.ID,
					Timestamp: time.Now().UTC(),
					Payload: map[string]interface{}{
						"bytes_transferred": stats.BytesRead.Load(),
						"total_bytes":       stats.TotalBytes.Load(),
						"files_done":        stats.FilesDone.Load(),
						"files_failed":      stats.FilesFailed.Load(),
						"files_skipped":     stats.FilesSkipped.Load(),
						"total_files":       stats.TotalFiles.Load(),
						"bytes_per_sec":     bps,
						"active_workers":    stats.ActiveWorkers.Load(),
						"workers":           stats.SnapshotWorkers(),
					},
				})
			}
		}
	}()

	for _, t := range tasks {
		select {
		case <-ctx.Done():
			close(taskCh)
			wg.Wait()
			close(doneCh)
			e.cancelJob(job)
			return
		case taskCh <- t:
		}
	}
	close(taskCh)
	wg.Wait()
	close(doneCh)

	if ctx.Err() != nil {
		e.cancelJob(job)
		return
	}

	completed := time.Now().UTC()
	job.Status = db.JobStatusCompleted
	job.CompletedAt = &completed
	job.FilesTransferred = stats.FilesDone.Load()
	job.BytesTransferred = stats.BytesRead.Load()
	job.FilesFailed = stats.FilesFailed.Load()
	job.FilesSkipped = stats.FilesSkipped.Load()
	_ = e.DB.UpdateJobRun(job)
	e.log(job.ID, "job completed")
	e.Emitter.Emit(Event{Type: EventJob, JobID: job.ID, Message: "job completed", Timestamp: completed, Payload: job})
}

func (e *Engine) workerLoop(
	ctx context.Context,
	id int,
	tasks <-chan transferTask,
	rule *db.SyncRule,
	src, dst *Client,
	job *db.JobRun,
	stats *EngineStats,
) {
	uploader := manager.NewUploader(dst.S3, func(u *manager.Uploader) {
		u.PartSize = 8 * 1024 * 1024
		u.Concurrency = 2
	})

	for task := range tasks {
		if ctx.Err() != nil {
			return
		}
		stats.ActiveWorkers.Add(1)
		stats.SetWorker(WorkerStatus{
			WorkerID:  id,
			Key:       task.SourceKey,
			Source:    rule.SourceBucket + "/" + task.SourceKey,
			Target:    rule.TargetBucket + "/" + task.TargetKey,
			SizeBytes: task.Size,
			Active:    true,
		})

		var err error
		switch task.Action {
		case ActionDelete:
			_, err = dst.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(rule.TargetBucket),
				Key:    aws.String(task.TargetKey),
			})
			if err == nil {
				stats.FilesDone.Add(1)
				e.log(job.ID, fmt.Sprintf("deleted s3://%s/%s", rule.TargetBucket, task.TargetKey))
			}
		default:
			err = e.streamCopy(ctx, id, src, dst, uploader, rule, task, stats)
			if err == nil {
				stats.FilesDone.Add(1)
				e.log(job.ID, fmt.Sprintf("synced %s -> %s (%d bytes)", task.SourceKey, task.TargetKey, task.Size))
			}
		}

		if err != nil {
			stats.FilesFailed.Add(1)
			e.log(job.ID, fmt.Sprintf("FAILED %s: %v", task.SourceKey, err))
			e.Emitter.Emit(Event{Type: EventError, JobID: job.ID, Message: err.Error(), Timestamp: time.Now().UTC()})
		}

		stats.ClearWorker(id)
		stats.ActiveWorkers.Add(-1)

		// Persist lightweight counters periodically
		job.FilesTransferred = stats.FilesDone.Load()
		job.BytesTransferred = stats.BytesRead.Load()
		job.FilesFailed = stats.FilesFailed.Load()
		_ = e.DB.UpdateJobRun(job)
	}
}

// streamCopy pipes GetObject body into multipart upload without buffering the whole object.
func (e *Engine) streamCopy(
	ctx context.Context,
	workerID int,
	src, dst *Client,
	uploader *manager.Uploader,
	rule *db.SyncRule,
	task transferTask,
	stats *EngineStats,
) error {
	getOut, err := src.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(rule.SourceBucket),
		Key:    aws.String(task.SourceKey),
	})
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer getOut.Body.Close()

	size := task.Size
	if getOut.ContentLength != nil {
		size = *getOut.ContentLength
	}

	pr, pw := io.Pipe()
	prog := NewProgressReader(ctx, getOut.Body, stats, workerID, size, rule.BandwidthLimitKbps)

	// Fill pipe from source in a goroutine so Upload can consume concurrently.
	copyErr := make(chan error, 1)
	go func() {
		defer pw.Close()
		_, err := io.Copy(pw, prog)
		if err != nil {
			_ = pw.CloseWithError(err)
		}
		copyErr <- err
	}()

	// Annotate worker with keys for UI
	stats.SetWorker(WorkerStatus{
		WorkerID:  workerID,
		Key:       task.SourceKey,
		Source:    fmt.Sprintf("%s/%s", rule.SourceBucket, task.SourceKey),
		Target:    fmt.Sprintf("%s/%s", rule.TargetBucket, task.TargetKey),
		SizeBytes: size,
		Active:    true,
	})

	_, upErr := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(rule.TargetBucket),
		Key:    aws.String(task.TargetKey),
		Body:   pr,
	})
	cErr := <-copyErr
	if upErr != nil {
		return fmt.Errorf("put object: %w", upErr)
	}
	if cErr != nil {
		return fmt.Errorf("stream copy: %w", cErr)
	}
	return nil
}

func (e *Engine) failJob(job *db.JobRun, err error) {
	completed := time.Now().UTC()
	job.Status = db.JobStatusFailed
	job.ErrorMessage = err.Error()
	job.CompletedAt = &completed
	_ = e.DB.UpdateJobRun(job)
	e.log(job.ID, "job failed: "+err.Error())
	e.Emitter.Emit(Event{Type: EventJob, JobID: job.ID, Message: "job failed: " + err.Error(), Timestamp: completed, Payload: job})
}

func (e *Engine) cancelJob(job *db.JobRun) {
	completed := time.Now().UTC()
	job.Status = db.JobStatusCancelled
	job.CompletedAt = &completed
	_ = e.DB.UpdateJobRun(job)
	e.log(job.ID, "job cancelled")
	e.Emitter.Emit(Event{Type: EventJob, JobID: job.ID, Message: "job cancelled", Timestamp: completed, Payload: job})
}

// Ensure unused import for types retained for future ACL use.
var _ = s3types.ObjectCannedACLPrivate
