package api

import (
	"net/http"
	"time"

	"github.com/ArianAr/Gantry/internal/version"
	"github.com/ArianAr/Gantry/pkg/db"
	"github.com/ArianAr/Gantry/pkg/s3"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CronValidator validates and computes next cron fire times (optional).
type CronValidator interface {
	ValidateCron(expr string) error
	NextRun(expr string, from time.Time) (*time.Time, error)
}

// Server holds API dependencies.
type Server struct {
	DB        *db.DB
	Engine    *s3.Engine
	Hub       *Hub
	Scheduler CronValidator // optional; used for schedule_cron validation
}

// RegisterAPI mounts REST routes on the given engine group (typically /api).
func (s *Server) RegisterAPI(r *gin.RouterGroup) {
	r.GET("/version", s.getVersion)
	r.GET("/providers", s.listProviders)
	r.POST("/providers", s.createProvider)
	r.POST("/providers/test", s.testProvider)
	r.DELETE("/providers/:id", s.deleteProvider)

	r.GET("/rules", s.listRules)
	r.POST("/rules", s.createOrUpdateRule)
	r.DELETE("/rules/:id", s.deleteRule)
	r.POST("/rules/:id/dry-run", s.dryRun)
	r.POST("/rules/:id/start", s.startJob)

	r.GET("/jobs", s.listJobs)
	r.GET("/jobs/stream", s.Hub.Stream) // before :id so "stream" is not captured as an id
	r.GET("/jobs/:id", s.getJob)
	r.POST("/jobs/:id/cancel", s.cancelJob)
}

func (s *Server) getVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":    version.Version,
		"commit":     version.Commit,
		"build_date": version.BuildDate,
	})
}

// --- Providers ---

type providerRequest struct {
	Name            string `json:"name" binding:"required"`
	ProviderType    string `json:"provider_type" binding:"required"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region" binding:"required"`
	AccessKeyID     string `json:"access_key_id" binding:"required"`
	SecretAccessKey string `json:"secret_access_key" binding:"required"`
}

func (s *Server) listProviders(c *gin.Context) {
	list, err := s.DB.ListProviders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Custom response with masked secrets visible as ********
	type providerJSON struct {
		ID              string    `json:"id"`
		Name            string    `json:"name"`
		ProviderType    string    `json:"provider_type"`
		Endpoint        string    `json:"endpoint"`
		Region          string    `json:"region"`
		AccessKeyID     string    `json:"access_key_id"`
		SecretAccessKey string    `json:"secret_access_key"`
		CreatedAt       time.Time `json:"created_at"`
	}
	resp := make([]providerJSON, 0, len(list))
	for _, p := range list {
		resp = append(resp, providerJSON{
			ID: p.ID, Name: p.Name, ProviderType: p.ProviderType,
			Endpoint: p.Endpoint, Region: p.Region, AccessKeyID: p.AccessKeyID,
			SecretAccessKey: "********", CreatedAt: p.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) createProvider(c *gin.Context) {
	var req providerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p := &db.Provider{
		Name:            req.Name,
		ProviderType:    req.ProviderType,
		Endpoint:        req.Endpoint,
		Region:          req.Region,
		AccessKeyID:     req.AccessKeyID,
		SecretAccessKey: req.SecretAccessKey,
	}
	if err := s.DB.CreateProvider(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id": p.ID, "name": p.Name, "provider_type": p.ProviderType,
		"endpoint": p.Endpoint, "region": p.Region, "access_key_id": p.AccessKeyID,
		"secret_access_key": "********", "created_at": p.CreatedAt,
	})
}

func (s *Server) testProvider(c *gin.Context) {
	var req providerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p := db.Provider{
		Name:            req.Name,
		ProviderType:    req.ProviderType,
		Endpoint:        req.Endpoint,
		Region:          req.Region,
		AccessKeyID:     req.AccessKeyID,
		SecretAccessKey: req.SecretAccessKey,
	}
	latency, buckets, err := s3.TestConnection(c.Request.Context(), p)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok": false, "error": err.Error(), "latency_ms": latency,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "latency_ms": latency, "bucket_count": buckets,
	})
}

func (s *Server) deleteProvider(c *gin.Context) {
	id := c.Param("id")
	if err := s.DB.DeleteProvider(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// --- Rules ---

type ruleRequest struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name" binding:"required"`
	SourceProviderID   string  `json:"source_provider_id" binding:"required"`
	SourceBucket       string  `json:"source_bucket" binding:"required"`
	SourcePrefix       string  `json:"source_prefix"`
	TargetProviderID   string  `json:"target_provider_id" binding:"required"`
	TargetBucket       string  `json:"target_bucket" binding:"required"`
	TargetPrefix       string  `json:"target_prefix"`
	ExtraTargets       string  `json:"extra_targets"`
	IncludePatterns    string  `json:"include_patterns"`
	ExcludePatterns    string  `json:"exclude_patterns"`
	MinSizeBytes       *int64  `json:"min_size_bytes"`
	MaxSizeBytes       *int64  `json:"max_size_bytes"`
	ModifiedAfter      *string `json:"modified_after"`
	DeleteOnTarget     bool    `json:"delete_on_target"`
	ConcurrencyLimit   int     `json:"concurrency_limit"`
	BandwidthLimitKbps int     `json:"bandwidth_limit_kbps"`
	CompareMode        string  `json:"compare_mode"`
	ScheduleCron       string  `json:"schedule_cron"`
	ScheduleEnabled    bool    `json:"schedule_enabled"`
}

func (s *Server) listRules(c *gin.Context) {
	list, err := s.DB.ListRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) createOrUpdateRule(c *gin.Context) {
	var req ruleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule := &db.SyncRule{
		ID:                 req.ID,
		Name:               req.Name,
		SourceProviderID:   req.SourceProviderID,
		SourceBucket:       req.SourceBucket,
		SourcePrefix:       req.SourcePrefix,
		TargetProviderID:   req.TargetProviderID,
		TargetBucket:       req.TargetBucket,
		TargetPrefix:       req.TargetPrefix,
		ExtraTargets:       req.ExtraTargets,
		IncludePatterns:    req.IncludePatterns,
		ExcludePatterns:    req.ExcludePatterns,
		MinSizeBytes:       req.MinSizeBytes,
		MaxSizeBytes:       req.MaxSizeBytes,
		DeleteOnTarget:     req.DeleteOnTarget,
		ConcurrencyLimit:   req.ConcurrencyLimit,
		BandwidthLimitKbps: req.BandwidthLimitKbps,
		CompareMode:        req.CompareMode,
		ScheduleCron:       req.ScheduleCron,
		ScheduleEnabled:    req.ScheduleEnabled,
	}
	if req.ModifiedAfter != nil && *req.ModifiedAfter != "" {
		t, err := time.Parse(time.RFC3339, *req.ModifiedAfter)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "modified_after must be RFC3339"})
			return
		}
		utc := t.UTC()
		rule.ModifiedAfter = &utc
	}
	if s.Scheduler != nil {
		if err := s.Scheduler.ValidateCron(rule.ScheduleCron); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid schedule_cron: " + err.Error()})
			return
		}
		if rule.ScheduleEnabled && rule.ScheduleCron != "" {
			next, err := s.Scheduler.NextRun(rule.ScheduleCron, time.Now().UTC())
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid schedule_cron: " + err.Error()})
				return
			}
			rule.NextRunAt = next
		} else {
			rule.NextRunAt = nil
			rule.ScheduleEnabled = false
		}
	}
	// Preserve LastScheduledAt on update
	if rule.ID != "" {
		if existing, err := s.DB.GetRule(rule.ID); err == nil && existing != nil {
			rule.LastScheduledAt = existing.LastScheduledAt
		}
	}
	if err := s.DB.CreateOrUpdateRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (s *Server) deleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := s.DB.DeleteRule(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (s *Server) dryRun(c *gin.Context) {
	id := c.Param("id")
	rule, err := s.DB.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	result, err := s.Engine.DryRun(c.Request.Context(), rule)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) startJob(c *gin.Context) {
	id := c.Param("id")
	job, err := s.Engine.StartJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

func (s *Server) listJobs(c *gin.Context) {
	list, err := s.DB.ListJobRuns(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) getJob(c *gin.Context) {
	id := c.Param("id")
	job, err := s.DB.GetJobRun(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (s *Server) cancelJob(c *gin.Context) {
	id := c.Param("id")
	if !s.Engine.CancelJob(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not running"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cancelled": id})
}
