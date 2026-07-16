# Gantry Roadmap

## Milestones

### M0 — Foundations (complete when PR1 merges)
- [x] Repository, GPL-3.0, SECURITY, CONTRIBUTING
- [x] CI skeleton, issue/PR templates, project skill
- [x] VERSION + CHANGELOG

### M1 — Core Engine
- [x] SQLite models (Provider, SyncRule, JobRun)
- [x] Multi-provider S3 clients
- [x] Streaming worker pool (`io.Pipe`, ProgressReader, rate limit)

### M2 — API Surface
- [x] REST providers / rules / jobs
- [x] Dry-run comparison matrix
- [x] SSE live metrics stream

### M3 — Dashboard & Image
- [x] React console (Progress / Rules / Providers)
- [x] `go:embed` single binary
- [x] Multi-stage Dockerfile

### M4 — Release & Registry
- [x] `release.yml` (tag → GitHub Release + GHCR multi-arch)
- [x] Polished README with badges
- [x] First release **v0.1.0**

## Post-v1 ideas

- Operator authentication / reverse-proxy identity headers
- Encryption at rest for provider secrets
- Cron / scheduled sync rules
- Prometheus metrics endpoint
- Multi-job queue prioritization UI
- Object integrity verification (checksum modes)
- Bidirectional or multi-target fan-out sync

## Versioning

See [CONTRIBUTING.md](./CONTRIBUTING.md) and [CHANGELOG.md](./CHANGELOG.md). Image: `ghcr.io/arianar/gantry`.
