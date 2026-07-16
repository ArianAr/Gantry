# Gantry

**High-performance, self-hosted multi-provider S3 synchronization engine with a real-time web dashboard.**

Stream objects directly between S3-compatible stores (AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, and more) without buffering whole files in RAM or writing them to disk.

[![License: GPL-3.0](https://img.shields.io/badge/License-GPL%203.0-blue.svg)](./LICENSE)
[![CI](https://github.com/ArianAr/Gantry/actions/workflows/ci.yml/badge.svg)](https://github.com/ArianAr/Gantry/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ArianAr/Gantry?include_prereleases&sort=semver)](https://github.com/ArianAr/Gantry/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/ArianAr/Gantry)](./go.mod)
[![GHCR](https://img.shields.io/badge/ghcr.io-arianar%2Fgantry-blue?logo=docker)](https://github.com/ArianAr/Gantry/pkgs/container/gantry)

---

## Features

- **Memory-safe streaming** — `io.Pipe()` connects GetObject to multipart upload; no full-file buffers, no temp files
- **Multi-provider** — AWS S3, Cloudflare R2, MinIO, Backblaze B2, Wasabi, GCS (S3-compatible endpoints)
- **Rule engine** — Prefix routing, include/exclude patterns, size bounds, modified-after filters
- **Safe sync vs strict mirror** — Optional delete-on-target for true mirrors
- **Dry-run matrix** — See adds, modifies, deletes, and skips before transferring a byte
- **Worker pool** — Configurable concurrency (1–32) and optional bandwidth limits
- **Live dashboard** — SSE-powered progress, worker grid, throughput, ETA, and console log
- **Single binary** — React SPA embedded with `go:embed`; SQLite for config and job history
- **Container-first** — Multi-stage image published to GitHub Container Registry

---

## Quick start

### Docker (recommended)

```bash
docker pull ghcr.io/arianar/gantry:latest

docker run --rm -p 8080:8080 \
  -v gantry-data:/data \
  ghcr.io/arianar/gantry:latest
```

Open [http://localhost:8080](http://localhost:8080).

Pinned versions:

```bash
docker pull ghcr.io/arianar/gantry:v0.1.0
```

### Binary from GitHub Releases

Download the archive for your OS/arch from the [Releases](https://github.com/ArianAr/Gantry/releases) page, extract, and run:

```bash
./gantry -addr :8080 -db ./gantry.db
```

### Build from source

```bash
git clone https://github.com/ArianAr/Gantry.git
cd Gantry

cd frontend && npm ci && npm run build && cd ..

go build -ldflags "-X github.com/ArianAr/Gantry/internal/version.Version=$(cat VERSION)" -o gantry .
./gantry -addr :8080 -db ./gantry.db
```

---

## Dashboard

Dark single-page console with three tabs:

| Tab | Purpose |
|-----|---------|
| **Active Progress** | Global progress bar, MB/s throughput, rolling ETA, live worker grid, scrolling console log |
| **Rules** | Build S3→S3 pipelines, filters, safe sync vs mirror, dry-run comparison matrix, start jobs |
| **Providers** | Credential vault cards, custom endpoints, test connection with latency diagnostics |

---

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:8080` | HTTP listen address |
| `-db` | `gantry.db` | SQLite database path |
| `-api-token` | empty | Shared API token; empty disables auth |
| `-secrets-key` | empty | Encrypt provider secrets at rest (AES-256-GCM); empty = plaintext in DB |
| `-trust-proxy-headers` | `false` | Trust `Remote-User` / `X-Remote-User` / `X-Forwarded-User` |
| `-log-json` | `false` | Emit process logs as JSON lines (stdout) |
| `-version` | — | Print version and exit |

Environment (optional):

| Variable | Description |
|----------|-------------|
| `GANTRY_ADDR` | Overrides listen address if flag is default |
| `GANTRY_DB` | Overrides database path if flag is default |
| `GANTRY_API_TOKEN` | Shared API token (same as `-api-token`) |
| `GANTRY_SECRETS_KEY` | Passphrase for at-rest encryption of provider secrets |
| `GANTRY_TRUST_PROXY_HEADERS` | `true`/`false` — reverse-proxy identity headers |
| `GANTRY_LOG_JSON` | `true`/`false` — JSON process logs |

### Authentication

By default Gantry is **open** (local operator model). To require a shared token:

```bash
export GANTRY_API_TOKEN='long-random-secret'
./gantry -addr :8080
# or: docker run -e GANTRY_API_TOKEN=... -p 8080:8080 ghcr.io/arianar/gantry:latest
```

Clients may send:

- `Authorization: Bearer <token>`
- `X-API-Key: <token>`
- `?access_token=<token>` (for browser EventSource / SSE)

`/healthz` stays open for probes. The dashboard prompts for the token when it receives HTTP 401.

Behind a trusted reverse proxy you can set `-trust-proxy-headers` so a non-empty `Remote-User` / `X-Remote-User` / `X-Forwarded-User` is accepted. **Only enable this when untrusted clients cannot reach Gantry directly.**

### Encrypting secrets at rest

```bash
export GANTRY_SECRETS_KEY='long-random-passphrase'
./gantry -db ./gantry.db
```

When set, provider `secret_access_key` values are stored as AES-256-GCM ciphertext (`gantry1:…`). Existing plaintext rows are re-encrypted on startup. Without a key, secrets remain plaintext in SQLite (local lab default).

---

## API overview

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/providers` | List providers (secrets redacted) |
| `POST` | `/api/providers` | Create provider |
| `POST` | `/api/providers/test` | Validate credentials (`ListBuckets` + latency) |
| `DELETE` | `/api/providers/:id` | Delete provider |
| `GET` | `/api/rules` | List sync rules |
| `POST` | `/api/rules` | Create or update rule |
| `POST` | `/api/rules/:id/dry-run` | Dry-run matrix (no writes) |
| `POST` | `/api/rules/:id/start` | Start background job |
| `GET` | `/api/jobs` | Recent job runs |
| `GET` | `/api/jobs/stream` | SSE live metrics |
| `GET` | `/api/version` | Build version info |

---

## Architecture

```
       +---------------------------------------------+
       |             Gantry Web Dashboard            |
       |     (React / Tailwind CSS / Lucide Icons)   |
       +----------------------+----------------------+
                              |  REST + SSE
       +----------------------+----------------------+
       |                Go Backend Engine            |
       |  SQLite · Gin · S3 clients · Worker pool    |
       +-------+-----------------------------+-------+
               |                             |
               v         stream (pipe)       v
        Source Bucket  =================> Target Bucket
```

Object data is streamed in-process: **GetObject → ProgressReader → io.Pipe → multipart Uploader**.

Full product detail: [`specs.md`](./specs.md).

---

## Supported providers

Any S3-compatible API. Common configurations:

| Provider | Notes |
|----------|--------|
| AWS S3 | Empty endpoint; set region |
| Cloudflare R2 | Account endpoint; region often `auto` |
| MinIO | Custom endpoint; path-style addressing |
| Backblaze B2 | S3-compatible endpoint + region |
| Wasabi | Regional Wasabi endpoint |
| GCS | HMAC keys + XML API endpoint |

---

## Versioning & releases

- Semver source of truth: [`VERSION`](./VERSION)
- Changelog: [`CHANGELOG.md`](./CHANGELOG.md)
- Tags: `vX.Y.Z` trigger GitHub Release + multi-arch GHCR publish
- Image: **`ghcr.io/arianar/gantry`**

Major or critical changes land via PR, then a version-bump release (see [CONTRIBUTING.md](./CONTRIBUTING.md)).

---

## Security

Gantry is a **local-operator** tool: v1 has no built-in multi-user auth. Protect the HTTP port and SQLite file. See [SECURITY.md](./SECURITY.md) to report vulnerabilities privately.

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) and [ROADMAP.md](./ROADMAP.md). One feature per PR; CI must be green before merge.

---

## License

[GNU General Public License v3.0](./LICENSE)
