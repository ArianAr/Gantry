# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| Latest release (`main` / newest tag) | Yes |
| Older tags | Best-effort fixes for critical issues only |

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Please report security issues privately:

1. Use [GitHub Security Advisories](https://github.com/ArianAr/Gantry/security/advisories/new) for this repository, **or**
2. Email the maintainer via the contact listed on the [GitHub profile](https://github.com/ArianAr).

Include:

- Description of the vulnerability and impact
- Steps to reproduce (PoC if available)
- Affected version / commit if known
- Suggested fix (optional)

You should receive an acknowledgment within **7 days**. Critical issues will be prioritized for a patched release.

## Threat model (v1+)

Gantry is designed as a **self-hosted, single-operator** tool:

- **Optional shared API token** (`GANTRY_API_TOKEN` / `-api-token`) gates the API and UI when set. Empty token = open access (local-lab default).
- Optional **reverse-proxy identity headers** (`-trust-proxy-headers`) must only be used when the proxy is the sole ingress.
- There is **no multi-user RBAC**. The token is a single shared secret, not per-user accounts.
- Provider **access keys and secrets are stored in SQLite** (`gantry.db`) on disk. Protect the host filesystem and the database file.
- API responses **redact** secret access keys; secrets must never be logged.
- Bind the service to localhost or place it behind a reverse proxy with TLS when exposing beyond a trusted network.

## Hardening recommendations

- Set a long random `GANTRY_API_TOKEN` on any network-reachable deployment.
- Run behind HTTPS (Caddy, nginx, Traefik); prefer proxy auth **plus** Gantry token for defense in depth.
- Restrict network access (firewall, VPN, private network only).
- Use least-privilege S3 credentials (scoped buckets and prefixes).
- Back up and encrypt `gantry.db` according to your org policy.
- Keep images updated: `ghcr.io/arianar/gantry`.

## Supply chain

- Dependencies are updated via Dependabot where enabled.
- Release images are published to GHCR from GitHub Actions on version tags.
- Prefer pulling signed/provenance-enabled images when available.

## Disclosure policy

We will coordinate disclosure after a fix is available in a published release whenever practical. Credit will be given to reporters who wish to be named.
