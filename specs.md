# Specification: Gantry - Multi-Provider S3 Sync Engine & Dashboard

Gantry is a high-performance, self-hosted data migration and replication coordinator for S3-compatible cloud storage. It allows engineers to sync, mirror, and analyze bucket states across multiple providers (AWS S3, Cloudflare R2, Backblaze B2, Google Cloud Storage, Wasabi, MinIO) with real-time visual progress monitoring and granular, rule-based filtering.

---

## 1. High-Level Architecture

Gantry runs as a single, lightweight binary. It streams files directly from the source S3 endpoint to the destination S3 endpoint entirely in-memory using concurrent chunked streaming. It does not buffer entire files in memory or write them to local disk.

       +---------------------------------------------+
       |             Gantry Web Dashboard            |
       |     (React / Tailwind CSS / Lucide Icons)   |
       +----------------------+----------------------+
                              |
                     WebSocket / EventSource
                              |
       +----------------------+----------------------+
       |                Go Backend Engine            |
       |  - Embedded SQLite Database                 |
       |  - Fiber/Gin Web Server                     |
       |  - S3 Multi-Client Controller               |
       |  - Concurrent Chunk-Streaming Worker Pool   |
       +-------+-----------------------------+-------+
               |                             |
               v                             v
     +-------------------+         +-------------------+
     |  Source Bucket    | =======>| Target Bucket     |
     | (e.g., AWS S3)    | (Stream)| (e.g., MinIO)     |
     +-------------------+         +-------------------+

### Tech Stack
- Backend: Go (Golang) — utilizing official SDKs (e.g., aws-sdk-go-v2).
- Database: SQLite (via GORM or raw SQL) to track configurations, schedules, task statuses, and execution history.
- Frontend: React (SPA) + Tailwind CSS, bundled and served statically by the Go binary using go:embed.
- Inter-Process Communication: WebSockets or Server-Sent Events (SSE) for real-time progress broadcasts.

---

## 2. Database Schema (SQLite)

    -- Cloud Storage Provider Configurations
    CREATE TABLE providers (
        id TEXT PRIMARY KEY,               -- UUID or Nanoid
        name TEXT NOT NULL,                 -- Friendly label (e.g., "Prod Backups")
        provider_type TEXT NOT NULL,       -- "aws", "r2", "b2", "minio", etc.
        endpoint TEXT,                      -- Empty for official AWS, specified for custom / MinIO
        region TEXT NOT NULL,
        access_key_id TEXT NOT NULL,
        secret_access_key TEXT NOT NULL,   -- Stored securely
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    -- Sync Configuration Rules
    CREATE TABLE sync_rules (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        source_provider_id TEXT REFERENCES providers(id),
        source_bucket TEXT NOT NULL,
        source_prefix TEXT,                 -- Directory path to target (optional)
        target_provider_id TEXT REFERENCES providers(id),
        target_bucket TEXT NOT NULL,
        target_prefix TEXT,                 -- Target prefix folder (optional)
        
        -- Filter Parameters
        include_patterns TEXT,              -- Semicolon-separated file extensions (e.g., ".png;.jpg")
        exclude_patterns TEXT,              -- (e.g., ".mp4;.zip")
        min_size_bytes INTEGER,
        max_size_bytes INTEGER,
        modified_after DATETIME,
        
        -- Sync Rules
        delete_on_target BOOLEAN DEFAULT 0, -- True for strict Mirroring, False for Safe Syncing
        concurrency_limit INTEGER DEFAULT 4, -- Parallel workers (1 to 32)
        bandwidth_limit_kbps INTEGER DEFAULT 0, -- 0 for unlimited
        
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );

    -- Active & Historical Job Executions
    CREATE TABLE job_runs (
        id TEXT PRIMARY KEY,
        sync_rule_id TEXT REFERENCES sync_rules(id),
        status TEXT NOT NULL,               -- "queued", "dry_running", "active", "completed", "failed", "cancelled"
        is_dry_run BOOLEAN NOT NULL,
        total_files_discovered INTEGER DEFAULT 0,
        total_bytes_discovered INTEGER DEFAULT 0,
        files_transferred INTEGER DEFAULT 0,
        bytes_transferred INTEGER DEFAULT 0,
        files_skipped INTEGER DEFAULT 0,
        files_failed INTEGER DEFAULT 0,
        started_at DATETIME,
        completed_at DATETIME
    );

---

## 3. Streaming Engine Mechanics (The S3 Pipeline)

To transfer objects without exhausting system memory or requiring hard drive space:

1. Direct Piping: For every object sync, Gantry must spin up a Goroutine that initiates an S3 Download/GetObject request. It pipes the incoming Body stream directly into an S3 Upload/PutObject API write stream utilizing Go's `io.Pipe()`.
2. Dynamic Progress Tracking: Create a custom wrapper implementing `io.Reader` that counts the bytes read in real-time, feeding progress increments into a global thread-safe coordinator.
3. The Multi-Worker Pool: Create a Worker Pool where the pool size is dictated by `concurrency_limit`. Workers pop tasks from a channel containing individual file metadata, enabling parallel transfers of up to 32 files.
4. Rate Limiting: If `bandwidth_limit_kbps` is set to a value greater than 0, implement a token-bucket rate-limiter (e.g., `golang.org/x/time/rate`) on the file stream read actions.

---

## 4. REST API Endpoints

### Providers API
- GET `/api/providers` - Get all saved bucket configurations.
- POST `/api/providers` - Create a provider configuration.
- POST `/api/providers/test` - Takes credential variables in body, attempts an S3 bucket list operation, and returns a JSON handshake success/failure.
- DELETE `/api/providers/:id` - Delete a provider configuration.

### Sync Rules API
- GET `/api/rules` - Get all sync configurations.
- POST `/api/rules` - Create or update a configuration.
- POST `/api/rules/:id/dry-run` - Triggers a mock list evaluation of both buckets. Scans files, applies filters, calculates transfer metrics, and returns the list of files to sync/delete without writing.
- POST `/api/rules/:id/start` - Launches a background job run.

### Real-time Event Stream
- GET `/api/jobs/stream` - An active SSE (Server-Sent Events) or WebSocket connection that emits real-time sync metrics.

---

## 5. Visual Dashboard Specifications (The UI Layout)

The frontend is designed as a single-page dark-themed console with three primary views:

### Dashboard Tab: Active Progress Canvas
- Global Health Board: Displays a large progress bar showing current job completions, active transfer speed (e.g., 45.2 MB/s), and dynamic rolling ETA.
- Worker Matrix: A live grid showing active parallel workers. Each row represents a worker slot and displays the current file in transit, its total size, its source/target, and an active progress loader.
- Console Log Stream: A real-time collapsible log area showing live-scrolling file completions, skips, and connection status alerts.

### Rules Tab: Rules & Granularity Designer
- Step-by-Step Sync Configurator: Clean dropdown selectors for Source/Target bucket providers.
- Filters Inspector Card: Input forms for prefix routing, file extensions, file sizes, and date ranges.
- Dry-Run Evaluator Panel: Shows a visual breakdown of files to be updated, added, or deleted following a dry-run calculation.

    # Conceptual YAML config generated on-the-fly by the GUI
    sync_rules:
      include_paths: ["assets/images/*", "documents/2026/"]
      exclude_extensions: [".mp4", ".mov", ".zip"]
      modified_after: "2026-01-01T00:00:00Z"
      size_range:
        min: "10KB"
        max: "500MB"

### Providers Tab: Credentials Manager
- Minimal cards indicating connected servers with "Status Connected" checks. Shows custom S3 endpoint values and connection latency.
