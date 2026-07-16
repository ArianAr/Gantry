#!/usr/bin/env bash
# Wait for a PR's required checks without hanging the agent session.
#
# Usage:
#   scripts/wait-pr-ci.sh <pr-number> [timeout_seconds]
#
# Exit codes:
#   0  all checks passed
#   1  a check failed, or usage error
#   2  timed out while still pending
#
# Design:
# - Prefer `gh pr checks --watch` (event-driven) under a hard `timeout(1)`.
# - Never use unbounded sleep loops.
# - On timeout, print current check table and exit 2 so the caller can decide
#   (retry later, report to user) without looking "stuck".

set -euo pipefail

PR="${1:-}"
TIMEOUT_SEC="${2:-600}"

if [[ -z "$PR" || ! "$PR" =~ ^[0-9]+$ ]]; then
  echo "usage: $0 <pr-number> [timeout_seconds]" >&2
  exit 1
fi

if ! command -v gh >/dev/null; then
  echo "gh CLI required" >&2
  exit 1
fi

# Snapshot helper
print_checks() {
  gh pr checks "$PR" 2>&1 || true
}

# If already complete, don't watch.
if print_checks | awk '
  BEGIN { fail=0; pending=0; pass=0; total=0 }
  NF >= 2 {
    total++
    s=$2
    if (s == "pass" || s == "success") pass++
    else if (s == "fail" || s == "failure" || s == "cancelled" || s == "error") fail++
    else pending++
  }
  END {
    if (total == 0) exit 3
    if (fail > 0) exit 1
    if (pending > 0) exit 2
    exit 0
  }
'; then
  echo "PR #$PR: all checks already green"
  print_checks
  exit 0
fi
rc=$?
if [[ $rc -eq 1 ]]; then
  echo "PR #$PR: a check already failed"
  print_checks
  exit 1
fi

echo "PR #$PR: waiting up to ${TIMEOUT_SEC}s for checks..."

# gh pr checks --watch exits 0 when all pass, non-zero on failure.
# timeout(1) exits 124 on wall-clock timeout.
set +e
if command -v timeout >/dev/null; then
  timeout --signal=TERM --kill-after=10 "${TIMEOUT_SEC}" \
    gh pr checks "$PR" --watch --interval 10 --fail-fast
  watch_rc=$?
else
  # Fallback without GNU timeout: bounded poll (max TIMEOUT_SEC).
  deadline=$((SECONDS + TIMEOUT_SEC))
  watch_rc=2
  while (( SECONDS < deadline )); do
    print_checks | awk '
      BEGIN { fail=0; pending=0; pass=0; total=0 }
      NF >= 2 {
        total++
        s=$2
        if (s == "pass" || s == "success") pass++
        else if (s == "fail" || s == "failure" || s == "cancelled" || s == "error") fail++
        else pending++
      }
      END {
        if (total == 0) exit 3
        if (fail > 0) exit 1
        if (pending > 0) exit 2
        exit 0
      }
    '
    case $? in
      0) watch_rc=0; break ;;
      1) watch_rc=1; break ;;
      *) sleep 10 ;;
    esac
  done
fi
set -e

case $watch_rc in
  0)
    echo "PR #$PR: CI green"
    print_checks
    exit 0
    ;;
  124|143)
    echo "PR #$PR: timed out after ${TIMEOUT_SEC}s (not stuck — retry or inspect)"
    print_checks
    exit 2
    ;;
  *)
    # gh often returns 1 for failed checks
    echo "PR #$PR: checks finished unsuccessfully (rc=$watch_rc)"
    print_checks
    exit 1
    ;;
esac
