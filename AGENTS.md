# Agent guide — Gantry

Instructions for automated coding agents working on this repository. Human contributors should follow [CONTRIBUTING.md](./CONTRIBUTING.md).

## Product constraints (non-negotiable)

1. **Memory-safe streaming** — S3 transfers use `io.Pipe()` (GetObject → ProgressReader → uploader). Never buffer whole objects in RAM or write object payloads to disk.
2. **SQLite** — Persistence via GORM + pure-Go driver (`glebarez/sqlite`). No CGO.
3. **Embedded frontend** — Vite build under `frontend/`; embed `frontend/dist` with `go:embed` in `main.go`.
4. **No placeholders** — Do not merge TODOs, stubs, or truncated files.

## Layout

| Path | Role |
|------|------|
| `pkg/db/` | Provider, SyncRule, JobRun |
| `pkg/s3/` | Clients, ProgressReader, worker-pool engine |
| `pkg/api/` | Gin REST + SSE |
| `internal/version/` | Version/Commit/BuildDate (ldflags) |
| `frontend/` | React + Tailwind + Lucide SPA |
| `VERSION` | Semver source of truth (no `v` prefix) |
| `CHANGELOG.md` | Keep a Changelog |
| `specs.md` | Product specification |
| `.github/workflows/ci.yml` | PR/main checks |
| `.github/workflows/release.yml` | Tag → GitHub Release + GHCR |

## Local verification (before every commit)

```bash
go test ./...
go vet ./...
# if frontend touched:
cd frontend && npm ci && npm run build && cd ..
go build -ldflags "-X github.com/ArianAr/Gantry/internal/version.Version=$(cat VERSION)" -o gantry .
./gantry -version
# packaging changes:
docker build -t gantry:local .
```

Smoke when backend changes: start the binary and hit `/healthz`, `/api/version`.

## PR process

1. One feature or fix per PR.
2. Test locally **before** committing and pushing.
3. Open a PR; fill the template.
4. Submit a **formal GitHub review** (`gh pr review` or UI) before merge.
5. Merge only when required CI checks are green (branch protection on `main`).

### Waiting for CI (agents)

- **Snapshot first:** `gh pr checks <N>` or `gh pr view <N> --json statusCheckRollup`.
- If all green → merge immediately. If failed → logs + fix. Do not spin.
- If pending → optional **short** wait only (`timeout 120 gh pr checks <N> --watch --interval 10 --fail-fast`), then snapshot again.
- **Never** use long unbounded `sleep` loops; they hang agent sessions.

## Releases

1. Bump `VERSION`, `frontend/package.json`, and `CHANGELOG.md` together.
2. Merge release PR → tag `v$(cat VERSION)` → `release.yml` publishes binaries + `ghcr.io/arianar/gantry`.
3. Tag must match `VERSION` file contents.

Major/critical changes get a release; docs-only usually does not.

## Docker multi-arch

Dockerfile builds frontend/Go on `$BUILDPLATFORM` and cross-compiles with `GOARCH=$TARGETARCH` (`CGO_ENABLED=0`). Do not reintroduce QEMU-based Go compiles for arm64.

## Security

- Redact provider secrets in API responses; never log secrets.
- v1 is single-operator / no multi-user auth — document exposure risks in SECURITY.md.

## Out of scope for agents

- Do **not** commit personal agent skill files, local helper scripts, or tool caches (e.g. `.grok/`). Those stay on the operator machine only.
- Shared agent guidance for this repo belongs in **this file** (`AGENTS.md`).
