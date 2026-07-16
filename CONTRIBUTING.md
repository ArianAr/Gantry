# Contributing to Gantry

Thank you for contributing. This project is licensed under **GPL-3.0**. By submitting a contribution, you agree that your work is provided under the same license.

## Development setup

### Prerequisites

- Go 1.22+ (CI uses a current stable Go)
- Node.js 20+ and npm
- Docker (optional, for image builds)
- `git` and `gh` (recommended)

### Build & run

```bash
# Frontend
cd frontend && npm ci && npm run build && cd ..

# Backend
go test ./...
go build -ldflags "-X github.com/ArianAr/Gantry/internal/version.Version=$(cat VERSION)" -o gantry .
./gantry -addr :8080 -db ./gantry.db
```

### Docker

```bash
docker build -t gantry:local .
docker run --rm -p 8080:8080 -v gantry-data:/data gantry:local
```

## Code standards

- **Streaming only:** S3 object transfer must use `io.Pipe()` / streaming uploads. Never buffer entire objects in memory or write them to local disk.
- **No placeholders:** Do not leave `TODO`, stub handlers, or truncated files in merged code.
- **Secrets:** Never log access keys or secrets; keep API redaction intact.
- **Tests:** Add or update tests for `pkg/db`, `pkg/s3`, and `pkg/api` changes.
- **Formatting:** `gofmt` / `go vet`; frontend should build cleanly with Vite.

## Pull request process

1. Open an issue for non-trivial work when practical.
2. Create a branch from latest `main` (`feat/...`, `fix/...`, `docs/...`, `chore/...`).
3. **One feature or fix per PR.**
4. **Test everything locally before committing and before pushing:**
   - `go test ./...` and `go vet ./...`
   - Frontend: `cd frontend && npm ci && npm run build` when UI changes
   - Binary smoke: `go build -o gantry . && ./gantry -version` (plus `/healthz` when relevant)
   - Docker image build when packaging changes
5. Fill out the PR template (checklist is not optional).
6. **Every PR gets a formal code review** (GitHub review submitted — approve or request changes). Self-merge without a written review is not allowed for feature work.
7. Address review feedback and re-run the local test suite after fixes.
8. Maintainers merge only when **CI is green** and review findings are resolved.

## Review expectations

- Reviewers check correctness, streaming safety (`io.Pipe` / no full-file buffers), secret handling, API contracts, and tests.
- Bugs and security issues block merge; nits should still be fixed when easy.
- Dependabot PRs still need a brief review + green CI before merge.

## Versioning & releases

Semantic versioning. Source of truth: root **`VERSION`** file (no `v` prefix).

### When to release

| Change type | Release? |
|-------------|----------|
| Critical (security, data loss, broken sync) | Always |
| Major feature | Yes (minor/major bump) |
| Breaking API/schema | Yes |
| Docs / chore only | No |

### Release checklist

1. Bump `VERSION`.
2. Set `frontend/package.json` `"version"` to the same value.
3. Update `CHANGELOG.md` under `## [X.Y.Z] - YYYY-MM-DD`.
4. Open a release PR → CI green → merge.
5. Tag: `git tag v$(cat VERSION) && git push origin v$(cat VERSION)`.
6. Confirm GitHub Actions **Release** workflow:
   - GitHub Release + binary assets
   - Multi-arch image pushed to `ghcr.io/arianar/gantry`
7. Workflow must fail if the tag does not match `VERSION`.

## Issue labels

Use labels such as `bug`, `enhancement`, `docs`, `ci`, `security`, `frontend`, `backend`, `s3-engine` when filing issues.

## Code of conduct

Be respectful and constructive. Harassment or bad-faith contributions will not be tolerated.

## Questions

Open a GitHub Discussion or issue with the `question` label if unsure about design direction. Product intent is also captured in `specs.md`.
