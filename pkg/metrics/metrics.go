package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Process-level sync metrics for Prometheus scrapes.
var (
	JobsStarted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gantry_jobs_started_total",
		Help: "Total sync jobs started",
	})
	JobsCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gantry_jobs_completed_total",
		Help: "Total sync jobs completed successfully",
	})
	JobsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gantry_jobs_failed_total",
		Help: "Total sync jobs that failed",
	})
	JobsCancelled = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gantry_jobs_cancelled_total",
		Help: "Total sync jobs cancelled",
	})
	BytesTransferred = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gantry_bytes_transferred_total",
		Help: "Total bytes transferred by completed object streams",
	})
	FilesTransferred = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gantry_files_transferred_total",
		Help: "Total files successfully transferred or deleted",
	})
	FilesFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gantry_files_failed_total",
		Help: "Total file operations that failed",
	})
	ActiveJobs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gantry_active_jobs",
		Help: "Currently running sync jobs",
	})
	ActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gantry_active_workers",
		Help: "Workers currently transferring objects across all jobs",
	})
)
