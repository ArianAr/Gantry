package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ArianAr/Gantry/pkg/secrets"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Job status values.
const (
	JobStatusQueued     = "queued"
	JobStatusDryRunning = "dry_running"
	JobStatusActive     = "active"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
	JobStatusCancelled  = "cancelled"
)

// Provider stores S3-compatible endpoint credentials.
type Provider struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	Name            string    `gorm:"not null" json:"name"`
	ProviderType    string    `gorm:"not null" json:"provider_type"`
	Endpoint        string    `json:"endpoint"`
	Region          string    `gorm:"not null" json:"region"`
	AccessKeyID     string    `gorm:"not null" json:"access_key_id"`
	SecretAccessKey string    `gorm:"not null" json:"-"`
	CreatedAt       time.Time `json:"created_at"`
}

// TableName returns the providers table name.
func (Provider) TableName() string { return "providers" }

// Redacted returns a copy safe for API responses (secret masked).
func (p Provider) Redacted() Provider {
	out := p
	if out.SecretAccessKey != "" {
		out.SecretAccessKey = "********"
	}
	return out
}

// SyncRule stores an S3-to-S3 pipeline configuration.
type SyncRule struct {
	ID                 string     `gorm:"primaryKey;size:36" json:"id"`
	Name               string     `gorm:"not null" json:"name"`
	SourceProviderID   string     `gorm:"not null;index" json:"source_provider_id"`
	SourceBucket       string     `gorm:"not null" json:"source_bucket"`
	SourcePrefix       string     `json:"source_prefix"`
	TargetProviderID   string     `gorm:"not null;index" json:"target_provider_id"`
	TargetBucket       string     `gorm:"not null" json:"target_bucket"`
	TargetPrefix       string     `json:"target_prefix"`
	// ExtraTargets is a semicolon-separated fan-out list of "bucket" or "bucket:prefix"
	// on the same target provider as TargetProviderID.
	ExtraTargets       string     `json:"extra_targets"`
	IncludePatterns    string     `json:"include_patterns"` // semicolon-separated
	ExcludePatterns    string     `json:"exclude_patterns"`
	MinSizeBytes       *int64     `json:"min_size_bytes"`
	MaxSizeBytes       *int64     `json:"max_size_bytes"`
	ModifiedAfter      *time.Time `json:"modified_after"`
	DeleteOnTarget     bool       `gorm:"default:false" json:"delete_on_target"`
	ConcurrencyLimit   int        `gorm:"default:4" json:"concurrency_limit"`
	BandwidthLimitKbps int        `gorm:"default:0" json:"bandwidth_limit_kbps"`
	// CompareMode is "etag" (default: size+etag) or "size" (size only).
	CompareMode string `gorm:"default:etag" json:"compare_mode"`
	// Priority: higher values start before lower when the job queue has free slots (default 0).
	Priority int `gorm:"default:0" json:"priority"`
	// ScheduleCron is a standard 5-field cron expression (min hour dom mon dow). Empty = no schedule.
	ScheduleCron    string     `json:"schedule_cron"`
	ScheduleEnabled bool       `gorm:"default:false" json:"schedule_enabled"`
	LastScheduledAt *time.Time `json:"last_scheduled_at,omitempty"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ExtraTarget is one fan-out destination (same target provider).
type ExtraTarget struct {
	Bucket string
	Prefix string
}

// ParseExtraTargets parses "bucket" or "bucket:prefix" entries separated by ';'.
func ParseExtraTargets(s string) []ExtraTarget {
	parts := ParsePatterns(s)
	if len(parts) == 0 {
		return nil
	}
	out := make([]ExtraTarget, 0, len(parts))
	for _, p := range parts {
		bucket, prefix := p, ""
		if i := strings.Index(p, ":"); i >= 0 {
			bucket = strings.TrimSpace(p[:i])
			prefix = strings.TrimSpace(p[i+1:])
		}
		if bucket == "" {
			continue
		}
		out = append(out, ExtraTarget{Bucket: bucket, Prefix: prefix})
	}
	return out
}

// NormalizeCompareMode returns etag or size.
func (r *SyncRule) NormalizeCompareMode() string {
	switch strings.ToLower(strings.TrimSpace(r.CompareMode)) {
	case "size":
		return "size"
	default:
		return "etag"
	}
}

// TableName returns the sync_rules table name.
func (SyncRule) TableName() string { return "sync_rules" }

// ClampConcurrency ensures concurrency is between 1 and 32.
func (r *SyncRule) ClampConcurrency() {
	if r.ConcurrencyLimit < 1 {
		r.ConcurrencyLimit = 1
	}
	if r.ConcurrencyLimit > 32 {
		r.ConcurrencyLimit = 32
	}
}

// ParsePatterns splits semicolon-separated include/exclude patterns.
func ParsePatterns(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// JobRun logs an active or historical sync execution.
type JobRun struct {
	ID                   string     `gorm:"primaryKey;size:36" json:"id"`
	SyncRuleID           string     `gorm:"not null;index" json:"sync_rule_id"`
	Status               string     `gorm:"not null;index" json:"status"`
	IsDryRun             bool       `gorm:"not null" json:"is_dry_run"`
	Priority             int        `gorm:"default:0;index" json:"priority"`
	TotalFilesDiscovered int64      `gorm:"default:0" json:"total_files_discovered"`
	TotalBytesDiscovered int64      `gorm:"default:0" json:"total_bytes_discovered"`
	FilesTransferred     int64      `gorm:"default:0" json:"files_transferred"`
	BytesTransferred     int64      `gorm:"default:0" json:"bytes_transferred"`
	FilesSkipped         int64      `gorm:"default:0" json:"files_skipped"`
	FilesFailed          int64      `gorm:"default:0" json:"files_failed"`
	ErrorMessage         string     `json:"error_message,omitempty"`
	StartedAt            *time.Time `json:"started_at"`
	CompletedAt          *time.Time `json:"completed_at"`
}

// TableName returns the job_runs table name.
func (JobRun) TableName() string { return "job_runs" }

// DB wraps a GORM connection.
type DB struct {
	gorm       *gorm.DB
	secretsKey string // optional AES key material for provider secrets
}

// Gorm exposes the underlying *gorm.DB for advanced queries.
func (d *DB) Gorm() *gorm.DB { return d.gorm }

// Open initializes SQLite at path and runs auto-migrations.
// secretsKey encrypts provider secrets at rest when non-empty (AES-256-GCM).
func Open(path string, secretsKey ...string) (*DB, error) {
	if path == "" {
		path = "gantry.db"
	}
	// Ensure parent directory exists (Docker default is /data/gantry.db; missing
	// /data yields SQLITE_CANTOPEN, often misreported as "out of memory (14)").
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory %q: %w", dir, err)
		}
	}
	key := ""
	if len(secretsKey) > 0 {
		key = secretsKey[0]
	}
	// WAL mode and busy timeout for concurrent readers/writers.
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	g, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w (ensure the directory exists and is writable by the process user)", path, err)
	}
	d := &DB{gorm: g, secretsKey: key}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	if key != "" {
		if err := d.migratePlaintextSecrets(); err != nil {
			return nil, fmt.Errorf("migrate secrets: %w", err)
		}
	}
	return d, nil
}

func (d *DB) migrate() error {
	if err := d.gorm.AutoMigrate(&Provider{}, &SyncRule{}, &JobRun{}); err != nil {
		return fmt.Errorf("auto-migrate: %w", err)
	}
	return nil
}

// Close closes the underlying SQL connection.
func (d *DB) Close() error {
	sqlDB, err := d.gorm.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// NewID returns a new UUID string.
func NewID() string {
	return uuid.NewString()
}

// --- Providers ---

// CreateProvider inserts a provider (assigns ID/CreatedAt if empty).
func (d *DB) CreateProvider(p *Provider) error {
	if p.ID == "" {
		p.ID = NewID()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	enc, err := secrets.Encrypt(p.SecretAccessKey, d.secretsKey)
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}
	// Store encrypted form; keep caller's struct with plaintext for immediate use if needed.
	store := *p
	store.SecretAccessKey = enc
	if err := d.gorm.Create(&store).Error; err != nil {
		return err
	}
	p.ID = store.ID
	p.CreatedAt = store.CreatedAt
	return nil
}

// ListProviders returns all providers ordered by name (secrets decrypted when key set).
func (d *DB) ListProviders() ([]Provider, error) {
	var list []Provider
	err := d.gorm.Order("name asc").Find(&list).Error
	if err != nil {
		return nil, err
	}
	for i := range list {
		if err := d.decryptProvider(&list[i]); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// GetProvider loads a provider by ID (secret decrypted when key set).
func (d *DB) GetProvider(id string) (*Provider, error) {
	var p Provider
	if err := d.gorm.First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if err := d.decryptProvider(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DB) decryptProvider(p *Provider) error {
	plain, err := secrets.Decrypt(p.SecretAccessKey, d.secretsKey)
	if err != nil {
		return fmt.Errorf("provider %s: %w", p.ID, err)
	}
	p.SecretAccessKey = plain
	return nil
}

// migratePlaintextSecrets re-encrypts any legacy plaintext provider secrets.
func (d *DB) migratePlaintextSecrets() error {
	if d.secretsKey == "" {
		return nil
	}
	var list []Provider
	if err := d.gorm.Find(&list).Error; err != nil {
		return err
	}
	for _, p := range list {
		if secrets.IsEncrypted(p.SecretAccessKey) {
			continue
		}
		enc, err := secrets.Encrypt(p.SecretAccessKey, d.secretsKey)
		if err != nil {
			return err
		}
		if err := d.gorm.Model(&Provider{}).Where("id = ?", p.ID).
			Update("secret_access_key", enc).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteProvider removes a provider if no rules reference it.
func (d *DB) DeleteProvider(id string) error {
	var count int64
	if err := d.gorm.Model(&SyncRule{}).
		Where("source_provider_id = ? OR target_provider_id = ?", id, id).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("provider is referenced by %d sync rule(s)", count)
	}
	res := d.gorm.Delete(&Provider{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// --- Sync rules ---

// CreateOrUpdateRule creates a rule or updates by ID when set and existing.
func (d *DB) CreateOrUpdateRule(r *SyncRule) error {
	r.ClampConcurrency()
	r.CompareMode = r.NormalizeCompareMode()
	if r.ID == "" {
		r.ID = NewID()
		if r.CreatedAt.IsZero() {
			r.CreatedAt = time.Now().UTC()
		}
		return d.gorm.Create(r).Error
	}
	var existing SyncRule
	err := d.gorm.First(&existing, "id = ?", r.ID).Error
	if err == gorm.ErrRecordNotFound {
		if r.CreatedAt.IsZero() {
			r.CreatedAt = time.Now().UTC()
		}
		return d.gorm.Create(r).Error
	}
	if err != nil {
		return err
	}
	r.CreatedAt = existing.CreatedAt
	return d.gorm.Save(r).Error
}

// ListRules returns all sync rules ordered by name.
func (d *DB) ListRules() ([]SyncRule, error) {
	var list []SyncRule
	err := d.gorm.Order("name asc").Find(&list).Error
	return list, err
}

// GetRule loads a sync rule by ID.
func (d *DB) GetRule(id string) (*SyncRule, error) {
	var r SyncRule
	if err := d.gorm.First(&r, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteRule removes a sync rule by ID.
func (d *DB) DeleteRule(id string) error {
	res := d.gorm.Delete(&SyncRule{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// --- Job runs ---

// CreateJobRun inserts a job run row.
func (d *DB) CreateJobRun(j *JobRun) error {
	if j.ID == "" {
		j.ID = NewID()
	}
	if j.Status == "" {
		j.Status = JobStatusQueued
	}
	return d.gorm.Create(j).Error
}

// UpdateJobRun persists the full job run state.
func (d *DB) UpdateJobRun(j *JobRun) error {
	return d.gorm.Save(j).Error
}

// GetJobRun loads a job by ID.
func (d *DB) GetJobRun(id string) (*JobRun, error) {
	var j JobRun
	if err := d.gorm.First(&j, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &j, nil
}

// ListJobRuns returns recent jobs, newest first.
func (d *DB) ListJobRuns(limit int) ([]JobRun, error) {
	if limit <= 0 {
		limit = 50
	}
	var list []JobRun
	// rowid desc is reliable on SQLite for insertion order.
	err := d.gorm.Order("rowid desc").Limit(limit).Find(&list).Error
	return list, err
}

// ListActiveJobs returns jobs currently queued or active (queued first by priority desc).
func (d *DB) ListActiveJobs() ([]JobRun, error) {
	var list []JobRun
	err := d.gorm.Where("status IN ?", []string{
		JobStatusQueued, JobStatusDryRunning, JobStatusActive,
	}).Order("CASE status WHEN 'active' THEN 0 WHEN 'dry_running' THEN 0 ELSE 1 END, priority desc, rowid asc").Find(&list).Error
	return list, err
}

// ListQueuedJobs returns only queued jobs ordered for dispatch (highest priority first).
func (d *DB) ListQueuedJobs() ([]JobRun, error) {
	var list []JobRun
	err := d.gorm.Where("status = ?", JobStatusQueued).
		Order("priority desc, rowid asc").Find(&list).Error
	return list, err
}

// CountJobsByStatus counts jobs with the given status.
func (d *DB) CountJobsByStatus(status string) (int64, error) {
	var n int64
	err := d.gorm.Model(&JobRun{}).Where("status = ?", status).Count(&n).Error
	return n, err
}

// PurgeOldJobs deletes terminal job runs whose completed_at is older than cutoff.
// Returns the number of rows deleted. Never touches queued/active/dry_running jobs.
func (d *DB) PurgeOldJobs(cutoff time.Time) (int64, error) {
	res := d.gorm.Where(
		"status IN ? AND completed_at IS NOT NULL AND completed_at < ?",
		[]string{JobStatusCompleted, JobStatusFailed, JobStatusCancelled},
		cutoff.UTC(),
	).Delete(&JobRun{})
	return res.RowsAffected, res.Error
}

// Ping checks the underlying SQL connection (for readiness probes).
func (d *DB) Ping() error {
	sqlDB, err := d.gorm.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
