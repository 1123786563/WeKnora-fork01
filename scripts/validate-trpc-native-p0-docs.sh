#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

required=(
  docs/superpowers/plans/trpc-native/features.tsv
  docs/superpowers/plans/trpc-native/sdk-probes.md
  docs/superpowers/plans/trpc-native/recovery-gaps.md
  docs/superpowers/plans/trpc-native/sdk-capabilities.tsv
  docs/superpowers/plans/trpc-native/interfaces.md
  docs/superpowers/plans/trpc-native/p0-decision.md
  docs/superpowers/plans/trpc-native/progress.md
)
for path in "${required[@]}"; do
  test -f "$path"
done

test ! -e docs/superpowers/plans/2026-09-19-trpc-native-agent-p1-storage.md
test ! -e docs/superpowers/plans/2026-09-19-trpc-native-agent-p2-governance.md

awk -F '\t' 'NR == 1 { if (NF != 9) exit 1; next } NF != 9 { exit 1 } END { if (NR != 35) exit 1 }' \
  docs/superpowers/plans/trpc-native/features.tsv
awk -F '\t' '
  NR == 1 { if (NF != 8) exit 1; next }
  NF != 8 { exit 1 }
  $5 == "verified" { verified++ }
  $5 == "source-only" { source_only++ }
  $5 == "blocked-env" { blocked_env++ }
  $5 == "incompatible" { incompatible++ }
  END { if (NR != 91 || verified != 4 || source_only != 79 || blocked_env != 2 || incompatible != 5) exit 1 }
' docs/superpowers/plans/trpc-native/sdk-capabilities.tsv

race_gate='GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v'
for path in \
  docs/superpowers/plans/trpc-native/sdk-probes.md \
  docs/superpowers/plans/trpc-native/p0-decision.md \
  docs/superpowers/plans/trpc-native/interfaces.md \
  docs/superpowers/plans/trpc-native/progress.md; do
  grep -Fq "${race_gate}" "${path}"
done

validate_ledger_row() {
  local task="$1"
  shift

  awk -F '|' -v task="${task}" -v anchors="$*" '
    function trim(value) {
      sub(/^[[:space:]]+/, "", value)
      sub(/[[:space:]]+$/, "", value)
      return value
    }
    function is_complete(value) {
      value = trim(value)
      if (value == "complete") {
        return 1
      }
      if (value !~ /^complete\([^)]*\)$/ && value !~ /^complete（[^）]*）$/) {
        return 0
      }
      return tolower(value) !~ /pending|partial|unverified|待定|部分|未验证|未完成/
    }
    function has_expected_anchors(value, expected_count, expected, actual_count, actual, remainder) {
      expected_count = split(anchors, expected, " ")
      remainder = value
      while (match(remainder, /`[^`]+`/)) {
        actual = tolower(substr(remainder, RSTART + 1, RLENGTH - 2))
        actual_count++
        if (actual_count > expected_count || actual != tolower(expected[actual_count])) {
          return 0
        }
        remainder = substr(remainder, RSTART + RLENGTH)
      }
      return actual_count == expected_count
    }
    $2 ~ "^[[:space:]]*" task "[[:space:]]" {
      rows++
      if (!has_expected_anchors($5) || !is_complete($6) || !is_complete($7) || !is_complete($8)) {
        invalid = 1
      }
    }
    END {
      if (rows != 1 || invalid) {
        exit 1
      }
    }
  ' docs/superpowers/plans/trpc-native/progress.md
}

validate_ledger_row P0-1 5c77dd3a da16fcf5 a64d2640 df82068b 7399b707
validate_ledger_row P0-2 8b1ba101 d1d18575
validate_ledger_row P0-3 d63879ab ba6dfcc6
validate_ledger_row P0-4 5b65a570 950716d6
validate_ledger_row P0-5 ea10b352 1d347947 3f78f055

echo 'PASS: tRPC native P0 documentation consistency'
