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

grep -Fq 'GOWORK=off go test -race ./internal/agent/nativeprobe -count=20 -v' \
  docs/superpowers/plans/trpc-native/{sdk-probes.md,p0-decision.md,interfaces.md,progress.md}
grep -Fq 'v1.10.0 remains **NO-GO**' docs/superpowers/plans/trpc-native/p0-decision.md
grep -Fq '`5c77dd3a`, `da16fcf5`, `a64d2640`, `df82068b`, `7399b707`' \
  docs/superpowers/plans/trpc-native/progress.md
grep -Fq '`ea10b352`, `1d347947`, `3f78f055`' docs/superpowers/plans/trpc-native/progress.md

echo 'PASS: tRPC native P0 documentation consistency'
