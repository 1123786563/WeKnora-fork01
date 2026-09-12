#!/usr/bin/env bash
# O02 unified recovery-test entry (isolated env; exits non-zero when the
# isolated PG is unavailable - never skips).
set -euo pipefail
cd "$(dirname "$0")/../.."

DSN="${SEMANTIC_TEST_PG_DSN:-postgresql://semantic:semantic@127.0.0.1:15432/semantic_test}"
if ! uv run --project semantic python -c "import psycopg; psycopg.connect('$DSN').close()" 2>/dev/null; then
  echo "ERROR: isolated PG unreachable at $DSN - start the test environment first" >&2
  exit 1
fi

if [ $# -gt 0 ]; then
  exec uv run --project semantic python -m pytest "$@"
else
  exec uv run --project semantic python -m pytest semantic/tests/integration -q
fi