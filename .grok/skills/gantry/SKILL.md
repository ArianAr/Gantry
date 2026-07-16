---
name: gantry
description: >
  Work on the Gantry multi-provider S3 sync engine and dashboard. Use for
  streaming S3→S3 pipelines, SQLite config, Gin REST/SSE API, embedded React
  UI, Docker/GHCR releases, and version bumps. Triggers: /gantry, Gantry,
  S3 sync engine, sync rules, dry-run matrix, worker pool, GHCR release.
---

# Gantry Project Skill

## Product

Gantry is a high-performance, self-hosted, multi-provider S3 synchronization engine with a React dashboard. Single Go binary with embedded SPA, SQLite persistence, and memory-safe streaming between buckets.

## Non-negotiable constraints

1. **Memory-safe streaming** — Transfer with `io.Pipe()` (GetObject → ProgressReader → Uploader). Never buffer whole files in RAM or write object payloads to disk.
2. **SQLite** — Config, rules, and job logs via GORM + pure-Go driver (`glebarez/sqlite`). No CGO.
3. **Embedded frontend** — Build `frontend/` with Vite; embed `frontend/dist` via `go:embed` in `main.go`.
4. **No placeholders** — Production-ready files only; no TODO stubs in shipped code.

## Layout

| Path | Role |
|------|------|
| `pkg/db/` | Provider, SyncRule, JobRun models + Open/AutoMigrate |
| `pkg/s3/` | Dynamic S3 clients, ProgressReader, worker-pool engine |
| `pkg/api/` | Gin REST handlers + SSE hub |
| `internal/version/` | Version/Commit/BuildDate (ldflags) |
| `frontend/` | React + Tailwind + Lucide SPA |
| `VERSION` | Semver source of truth (no `v` prefix) |
| `CHANGELOG.md` | Keep a Changelog |
| `specs.md` | Product specification |
| `.github/workflows/ci.yml` | PR/main merge gate |
| `.github/workflows/release.yml` | Tag → GitHub Release + GHCR multi-arch |

## Commands

```bash
# Backend tests
go test ./...

# Frontend
cd frontend && npm ci && npm run build

# Local binary (after frontend build)
go build -ldflags "-X github.com/ArianAr/Gantry/internal/version.Version=$(cat VERSION)" -o gantry .
./gantry -addr :8080 -db ./gantry.db
./gantry -version

# Docker
docker build -t gantry:local .
docker run --rm -p 8080:8080 -v gantry-data:/data gantry:local
```

## Image & registry

- **GHCR:** `ghcr.io/arianar/gantry`
- Tags on release: `vX.Y.Z`, `X.Y`, `X`, `latest`
- Pull: `docker pull ghcr.io/arianar/gantry:latest`

## Version bump checklist (every major/critical release)

1. Bump `VERSION` (semver, no `v`).
2. Bump `frontend/package.json` `"version"` to match.
3. Update `CHANGELOG.md` (`## [X.Y.Z] - YYYY-MM-DD`).
4. Open PR → CI green → merge to `main`.
5. Tag and push: `git tag v$(cat VERSION) && git push origin v$(cat VERSION)`.
6. Confirm `release.yml`: GitHub Release assets + GHCR multi-arch image.
7. Fail release if git tag ≠ `VERSION` file contents.

**When to release:** security/data-loss fixes (critical), major features, breaking API/schema. Skip for docs/chore-only.

## PR workflow (mandatory)

1. **Test before every commit** (local, not only CI):
   - `go test ./...`
   - `go vet ./...`
   - `cd frontend && npm ci && npm run build` (if frontend touched)
   - `go build -o gantry .` and smoke (`-version`, `/healthz`, basic API) when backend/UI packaging changes
   - `docker build` when Dockerfile/CI packaging changes
2. **One feature/fix per PR.**
3. **Formal PR review required** before merge:
   - Post a GitHub review via `gh pr review` (or UI) covering correctness, streaming safety, secrets, tests, and docs
   - Address all review findings; re-test after fixes
4. **Merge only when CI is green** and the review is submitted (approve or request-changes resolved).
5. Prefer sequential merges to `main` (branch protection enforced).

## Waiting for CI (do not get stuck)

**Never** use long unbounded `sleep` loops. They hit agent/tool timeouts and look hung.

### Agent policy (mandatory)

1. **Snapshot first** (single command, no wait):
   ```bash
   gh pr checks <N>
   # or
   gh pr view <N> --json statusCheckRollup --jq \
     '{checks:[.statusCheckRollup[]?|{name,status,conclusion}]}'
   ```
2. **If already all SUCCESS** → merge immediately. Do not watch.
3. **If any FAILURE** → fetch logs and fix. Do not watch.
4. **If pending** → either:
   - Run a **bounded** wait with a **short** wall clock (≤ **120s** per agent step), then snapshot again; **or**
   - Tell the user CI is still running and continue other work; re-check later with another snapshot.
5. **Never** chain multi-minute `sleep` loops or `timeout` > 180s inside a single agent tool call (the tool wrapper will background/kill and look stuck).
6. Prefer multiple short turns over one long block.

### Helper script (local / human use)

```bash
scripts/wait-pr-ci.sh <N> 600   # 0=green, 1=fail, 2=timeout
# or:
timeout 120 gh pr checks <N> --watch --interval 10 --fail-fast
```

For agents, cap at **`scripts/wait-pr-ci.sh <N> 120`** (or plain snapshot). On exit 2, re-snapshot once; if still pending, report and stop.

## API surface (quick ref)

- `GET/POST /api/providers`, `POST /api/providers/test`, `DELETE /api/providers/:id`
- `GET/POST /api/rules`
- `POST /api/rules/:id/dry-run`, `POST /api/rules/:id/start`
- `GET /api/jobs`, `GET /api/jobs/stream` (SSE)
- `GET /api/version`

## UI tabs

1. **Active Progress** — global %, MB/s, ETA, worker grid, console log
2. **Rules** — pipeline config, safe vs mirror, dry-run matrix, start job
3. **Providers** — credential cards, test connection + latency

## License

GPL-3.0 — see `LICENSE`, `SECURITY.md`, `CONTRIBUTING.md`.
