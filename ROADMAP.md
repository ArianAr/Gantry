# Gantry Roadmap

## Milestones

### M0 — Foundations (complete when PR1 merges)
- [x] Repository, GPL-3.0, SECURITY, CONTRIBUTING
- [x] CI skeleton, issue/PR templates, project skill
- [x] VERSION + CHANGELOG

### M1 — Core Engine
- [ ] SQLite models (Provider, SyncRule, JobRun)
- [ ] Multi-provider S3 clients
- [ ] Streaming worker pool (`io.Pipe`, ProgressReader, rate limit)

### M2 — API Surface
- [ ] REST providers / rules / jobs
- [ ] Dry-run comparison matrix
- [ ] SSE live metrics stream

### M3 — Dashboard & Image
- [ ] React console (Progress / Rules / Providers)
- [ ] `go:embed` single binary
- [ ] Multi-stage Dockerfile

### M4 — Release & Registry
- [ ] `release.yml` (tag → GitHub Release + GHCR multi-arch)
- [ ] Polished README with badges
- [ ] First release **v0.1.0**

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
