#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VALIDATOR_RELATIVE_PATH="scripts/validate-trpc-native-p0-docs.sh"
FIXTURE="$(mktemp -d)"
GATE='GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v'
trap 'rm -rf "${FIXTURE}"' EXIT

copy_fixture() {
  local destination="$1"

  mkdir -p "${destination}/scripts" "${destination}/docs/superpowers/plans/trpc-native"
  cp "${ROOT_DIR}/${VALIDATOR_RELATIVE_PATH}" "${destination}/${VALIDATOR_RELATIVE_PATH}"
  for path in \
    features.tsv \
    sdk-probes.md \
    recovery-gaps.md \
    sdk-capabilities.tsv \
    interfaces.md \
    p0-decision.md \
    progress.md; do
    cp "${ROOT_DIR}/docs/superpowers/plans/trpc-native/${path}" \
      "${destination}/docs/superpowers/plans/trpc-native/${path}"
  done
  (
    cd "${destination}"
    git init -q
  )
}

expect_failure() {
  local name="$1"
  local fixture="$2"

  if (
    cd "${fixture}"
    bash "${VALIDATOR_RELATIVE_PATH}"
  ) >/dev/null 2>&1; then
    echo "expected ${name} fixture to fail validation" >&2
    return 1
  fi
}

failures=0

missing_gate_fixture="${FIXTURE}/missing-gate"
copy_fixture "${missing_gate_fixture}"
grep -Fv "${GATE}" \
  "${missing_gate_fixture}/docs/superpowers/plans/trpc-native/interfaces.md" \
  > "${missing_gate_fixture}/interfaces.md"
mv "${missing_gate_fixture}/interfaces.md" \
  "${missing_gate_fixture}/docs/superpowers/plans/trpc-native/interfaces.md"
if ! expect_failure "interfaces.md missing repeated race gate" "${missing_gate_fixture}"; then
  failures=$((failures + 1))
fi

missing_ledger_fixture="${FIXTURE}/missing-ledger"
copy_fixture "${missing_ledger_fixture}"
awk '!/^\| P0-3 /' \
  "${missing_ledger_fixture}/docs/superpowers/plans/trpc-native/progress.md" \
  > "${missing_ledger_fixture}/progress.md"
mv "${missing_ledger_fixture}/progress.md" \
  "${missing_ledger_fixture}/docs/superpowers/plans/trpc-native/progress.md"
if ! expect_failure "progress.md missing P0-3 ledger row" "${missing_ledger_fixture}"; then
  failures=$((failures + 1))
fi

if ((failures > 0)); then
  exit 1
fi

echo "tRPC native P0 documentation validator regression tests passed"
