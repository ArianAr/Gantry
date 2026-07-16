# Gantry operations notes

## Probes

| Endpoint | Purpose | Auth |
|----------|---------|------|
| `GET /healthz` | Liveness — process is running | None |
| `GET /readyz` | Readiness — SQLite accepts queries | None |
| `GET /metrics` | Prometheus scrape | None (protect at network edge) |

## Retention

```bash
export GANTRY_JOB_RETENTION_DAYS=30
./gantry
```

Every hour (and once at startup), terminal job runs (`completed`, `failed`, `cancelled`) with `completed_at` older than N days are deleted. Active/queued jobs are never purged. `0` disables retention.

## Timeouts & retries

- HTTP server uses a short `ReadHeaderTimeout` (10s) to mitigate slowloris; long transfers are body streams and are not subject to a global read deadline.
- Object transfers use context cancel for job stop; a cancelled job does **not** resume mid-object — re-run the rule (dry-run skips already-synced keys when size/ETag match).
- S3 SDK default retries apply to API calls; bandwidth limits are enforced via a token-bucket on the progress reader.
- Prefer health checks on `/healthz` and readiness on `/readyz` in orchestrators so traffic only hits ready instances.

## Auth & secrets

- Set `GANTRY_API_TOKEN` for network-facing deploys.
- Set `GANTRY_SECRETS_KEY` to encrypt provider secrets at rest.
- Scrape `/metrics` only from trusted networks (or terminate TLS/auth at the proxy).
