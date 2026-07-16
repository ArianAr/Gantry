# Gantry Roadmap

This document is the product plan for Gantry. Status reflects `main` as of the latest milestone update.

**Versioning:** see [VERSION](./VERSION), [CHANGELOG.md](./CHANGELOG.md), [CONTRIBUTING.md](./CONTRIBUTING.md).  
**Image:** `ghcr.io/arianar/gantry`

---

## Milestone overview

| Milestone | Theme | Status |
|-----------|--------|--------|
| **M0** | Foundations | **Done** |
| **M1** | Core engine | **Done** |
| **M2** | API surface | **Done** |
| **M3** | Dashboard & image | **Done** |
| **M4** | Release & registry | **Done** (v0.1.0) |
| **M5** | Hardening & operations | **In progress** |
| **M6** | Observability & reliability | Planned |
| **M7** | Advanced sync | Planned |

---

## M0 — Foundations ✅

- [x] Repository, GPL-3.0 license, SECURITY, CONTRIBUTING
- [x] CI skeleton, issue/PR templates, Dependabot
- [x] VERSION + CHANGELOG
- [x] Branch protection on `main` (PR + required checks)
- [x] Shared agent guide (`AGENTS.md`)

## M1 — Core engine ✅

- [x] SQLite models (Provider, SyncRule, JobRun)
- [x] Multi-provider S3 clients (AWS, R2, MinIO, B2, Wasabi, …)
- [x] Streaming worker pool (`io.Pipe`, ProgressReader, rate limit)
- [x] Dry-run classification (add / modify / delete / skip)

## M2 — API surface ✅

- [x] REST providers / rules / jobs
- [x] Dry-run comparison matrix endpoint
- [x] SSE live metrics stream (`/api/jobs/stream`)
- [x] Provider connection test + latency

## M3 — Dashboard & image ✅

- [x] React console (Progress / Rules / Providers)
- [x] `go:embed` single binary
- [x] Multi-stage Dockerfile (BUILDPLATFORM cross-compile for multi-arch)

## M4 — Release & registry ✅

- [x] `release.yml` (tag → GitHub Release + GHCR multi-arch)
- [x] Polished README with badges
- [x] First release **v0.1.0**

---

## M5 — Hardening & operations 🚧

Goal: make network-exposed and always-on deployments safer and more operationally useful without multi-tenant complexity.

### M5.1 Operator authentication
- [x] Optional shared API token (`GANTRY_API_TOKEN` / `-api-token`)
- [x] Reverse-proxy identity headers (`Remote-User` / `X-Remote-User` / `X-Forwarded-User`)
- [x] Dashboard token prompt; SSE token via query (`access_token`)
- [x] `/healthz` remains unauthenticated for probes
- [x] Document threat model updates in SECURITY.md / README

### M5.2 Secret hygiene
- [ ] Encryption at rest for provider secrets (local key via env / file)
- [ ] Key rotation notes and migrate path for existing `gantry.db`
- [ ] Audit that secrets never appear in logs, SSE, or dry-run payloads

### M5.3 Scheduled sync
- [ ] Cron expression (or simple interval) per `SyncRule`
- [ ] Enable/disable schedule without deleting the rule
- [ ] In-process scheduler; skip overlapping runs for the same rule
- [ ] Surface next run time + last scheduled job in API/UI

### M5.4 Ops polish
- [ ] Graceful job cancel from UI (API already has cancel path — wire end-to-end)
- [ ] Configurable listen / DB via documented env (already partial)
- [ ] Structured logging option (JSON) for journald / containers

**Exit criteria:** auth optional but production-documented; secrets encrypted at rest; at least one schedule mechanism working end-to-end; v0.2.0 release.

---

## M6 — Observability & reliability (planned)

- [ ] Prometheus metrics (`/metrics`): job counts, bytes, errors, active workers
- [ ] Optional OpenTelemetry traces for transfer spans
- [ ] Job history retention / purge policy
- [ ] Stronger cancel + resume semantics
- [ ] Health/readiness split if needed (`/healthz` vs `/readyz`)
- [ ] Chaos-friendly timeouts and retry policy documentation

**Exit criteria:** scrapeable metrics; retention controls; documented ops runbook; v0.3.0.

---

## M7 — Advanced sync (planned)

- [ ] Object integrity modes (size + ETag default; optional checksum)
- [ ] Multi-job queue prioritization UI
- [ ] Multi-target fan-out (one source → N destinations)
- [ ] Bidirectional sync (explicit, carefully scoped)
- [ ] Bandwidth schedules / maintenance windows

**Exit criteria:** at least integrity mode + multi-target or queue UX; major version only if API breaks.

---

## Explicit non-goals (near term)

- Full multi-user RBAC / SSO productization (proxy auth + shared token is enough for v0.x)
- Clustered multi-node workers
- Replacing cloud-native replication products (Gantry is a coordinator/UI for S3-compatible endpoints)

---

## Release cadence (reminder)

| Change type | Release? |
|-------------|----------|
| Critical (security, data loss, broken sync) | Always |
| Major feature (milestone chunk) | Yes (minor pre-1.0) |
| Breaking API/schema | Yes (document in CHANGELOG) |
| Docs / chore only | Usually no |

Tag must match `VERSION`. Image tags: `vX.Y.Z`, `X.Y`, `X`, `latest`.
