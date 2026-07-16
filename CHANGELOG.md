# Changelog

All notable changes to Gantry are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Optional operator auth: `GANTRY_API_TOKEN` / `-api-token`, reverse-proxy identity headers, dashboard token prompt, SSE `access_token` query support
- Optional at-rest encryption for provider secrets (`GANTRY_SECRETS_KEY` / `-secrets-key`, AES-256-GCM + migrate on open)

### Changed
- Multi-arch Docker builds cross-compile Go on the host platform instead of QEMU-emulating arm64 (much faster image builds)

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
