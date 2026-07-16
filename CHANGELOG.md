# Changelog

All notable changes to Gantry are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Prometheus metrics at `GET /metrics` (jobs, bytes, files, active workers/jobs)
- Job history retention (`GANTRY_JOB_RETENTION_DAYS` / `-job-retention-days`)
- Readiness probe `GET /readyz` (DB ping)
- Rule `compare_mode`: `etag` (default) or `size` for skip/modify decisions
- Multi-target fan-out: rule `extra_targets` (`bucket` or `bucket:prefix`, semicolon-separated) on the same target provider
- Job queue with rule `priority` and `GANTRY_MAX_CONCURRENT_JOBS` / `-max-concurrent-jobs` (default 2)
- Providers UI: known-provider dropdown (AWS, R2, ArvanCloud, MinIO, Alibaba, Parspack, Hetzner, Dunkel, …) with prefilled editable fields + manual entry
- Bidirectional sync (`bidirectional`): forward pass then reverse (reverse never deletes or fans out)
- Active hours / maintenance windows (`active_hours_utc`, e.g. `09:00-17:00`) — jobs and schedules skip outside UTC windows (pairs with bandwidth limit for bandwidth schedules)

### Changed
- Multi-arch Docker builds cross-compile Go on the host platform instead of QEMU-emulating arm64 (much faster image builds)

## [0.2.1] - 2026-07-16

### Fixed
- Docker/SQLite: create parent dirs for DB path and ship writable `/data` in the image (fixes "unable to open database file: out of memory (14)")

## [0.2.0] - 2026-07-16

### Added
- Optional operator auth: `GANTRY_API_TOKEN` / `-api-token`, reverse-proxy identity headers, dashboard token prompt, SSE `access_token` query support
- Per-rule cron schedules (`schedule_cron` / `schedule_enabled`) with in-process scheduler, overlap skip, next/last run fields
- Dashboard job cancel control; optional JSON process logs (`GANTRY_LOG_JSON` / `-log-json`)
- Optional at-rest encryption for provider secrets (`GANTRY_SECRETS_KEY` / `-secrets-key`, AES-256-GCM + PBKDF2 + migrate on open)

## [0.1.0] - 2026-07-16

### Added
- SQLite persistence for providers, sync rules, and job runs
- Multi-provider S3 clients (AWS, R2, MinIO, B2, Wasabi, etc.)
- Streaming sync engine with `io.Pipe`, worker pool, rate limiting, dry-run matrix
- REST API and SSE live metrics (`/api/jobs/stream`)
- React dashboard (progress, rules, providers)
- Single binary with embedded SPA (`go:embed`)
- Multi-stage Dockerfile and GHCR release workflow
- Project foundation: GPL-3.0, SECURITY, CONTRIBUTING, CI, branch protection

## [0.0.0] - 2026-07-16

### Added
- Initial repository scaffold prior to first functional release
