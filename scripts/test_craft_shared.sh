#!/usr/bin/env bash
# CFT-S00-T001 craft shared-test gate.
#
# Fails when ANY of these regressions appear:
#   1. a craft test file exists on disk but is NOT matched by the
#      test:craft:shared / test:shared globs (the "0 missing items" check that
#      keeps craft from silently falling out of the gate again);
#   2. typecheck:shared drops back to the pre-craft whitelist (no craft
#      source file listed);
#   3. the pre-craft shared-test glob set shrinks (OLD_SHARED_MINIMUM is the
#      file count of the original whitelist at CFT-S00-T001 time — the old
#      test count must not drop);
#   4. the craft tests themselves fail.
set -euo pipefail
cd "$(dirname "$0")/.."
shopt -s nullglob

OLD_SHARED_MINIMUM=102 # original test:shared file count, recorded 2026-09-18

CRAFT_TEST_GLOBS=(
  'packages/contracts/src/craft/*.test.ts'
  'packages/api-client/src/craft/*.test.ts'
  'packages/domain/src/craft/*.test.ts'
  'packages/core/src/craft/*.test.ts'
  'packages/views/src/craft/*.test.ts'
  'packages/views/src/craft/*.test.tsx'
)

OLD_SHARED_GLOBS=(
  'packages/contracts/test/*.test.ts'
  'packages/domain/src/*.test.ts'
  'packages/domain/src/auth/*.test.ts'
  'packages/domain/src/access/*.test.ts'
  'packages/domain/src/knowledge/*.test.ts'
  'packages/domain/src/wiki/*.test.ts'
  'packages/domain/src/chat/*.test.ts'
  'packages/domain/src/sandbox/*.test.ts'
  'packages/api-client/src/*.test.ts'
  'packages/api-client/src/auth/*.test.ts'
  'packages/api-client/src/identity/*.test.ts'
  'packages/api-client/src/knowledge/*.test.ts'
  'packages/api-client/src/wiki/*.test.ts'
  'packages/api-client/src/chat/*.test.ts'
  'packages/api-client/src/sandbox/*.test.ts'
  'packages/api-client/src/mobile/*.test.ts'
  'packages/api-client/src/settings/*.test.ts'
  'packages/api-client/src/embed/*.test.ts'
  'packages/design-tokens/src/*.test.ts'
  'packages/i18n/test/*.test.ts'
  'packages/ui/src/*.test.tsx'
  'packages/views/src/chat/*.test.tsx'
  'packages/views/src/settings/*.test.ts'
  'packages/views/src/embed/*.test.ts'
  'packages/views/src/integrations/*.test.ts'
  'packages/views/src/integrations/*.test.tsx'
)

fail() { echo "FAIL[test_craft_shared]: $*" >&2; exit 1; }

expand() { # expand <glob...> — print one match per line, sorted
  local g matches=()
  for g in "$@"; do matches+=($g); done
  [ ${#matches[@]} -gt 0 ] || return 0
  printf '%s\n' "${matches[@]}" | sort
}

# --- 1. every craft test file on disk is collected by the craft globs --------
disk_files=$(
  for dir in packages/contracts/src/craft packages/api-client/src/craft \
             packages/domain/src/craft packages/core/src/craft \
             packages/views/src/craft; do
    find "$dir" -maxdepth 1 \( -name '*.test.ts' -o -name '*.test.tsx' \) 2>/dev/null
  done | sort
)
[ -n "$disk_files" ] || fail "no craft test files found under the craft package dirs"
echo "craft test files on disk: $(printf '%s\n' "$disk_files" | wc -l | tr -d ' ')"

glob_files=$(expand "${CRAFT_TEST_GLOBS[@]}")
[ -n "$glob_files" ] || fail "the craft globs match no files"

missing=$(comm -23 <(printf '%s\n' "$disk_files") <(printf '%s\n' "$glob_files"))
[ -z "$missing" ] || fail "craft tests on disk but NOT collected by the craft globs:
$missing"
extra=$(comm -13 <(printf '%s\n' "$disk_files") <(printf '%s\n' "$glob_files"))
[ -z "$extra" ] || fail "craft globs match files outside the enumerated craft dirs:
$extra"
echo "OK: every craft test file is collected (0 missing)"

# --- 2. package.json still declares both entries ------------------------------
node -e '
  const pkg = require("./package.json");
  const s = pkg.scripts ?? {};
  if (typeof s["test:craft:shared"] !== "string" || !s["test:craft:shared"].includes("tsx --test"))
    throw new Error("package.json scripts.test:craft:shared missing");
  for (const dir of ["contracts/src/craft","api-client/src/craft","domain/src/craft","core/src/craft","views/src/craft"]) {
    if (!s["test:craft:shared"].includes(dir)) throw new Error(`test:craft:shared does not cover ${dir}`);
    if (!s["test:shared"].includes(dir)) throw new Error(`test:shared does not cover ${dir}`);
  }
  if (typeof s["test:craft:web"] !== "string" || !s["test:craft:web"].includes("playwright"))
    throw new Error("package.json scripts.test:craft:web missing");
  const tc = s["typecheck:shared"] ?? "";
  const craftSources = ["contracts/src/craft/index.ts","api-client/src/craft/index.ts","domain/src/craft/reconnect.ts","domain/src/craft/state.ts","domain/src/craft/usage.ts","core/src/craft/controller.ts","views/src/craft/presentation.ts"];
  for (const f of craftSources) if (!tc.includes(f)) throw new Error(`typecheck:shared does not include craft source ${f}`);
  console.log("OK: package.json declares test:craft:shared/test:craft:web, craft globs in test:shared, craft sources in typecheck:shared");
' || fail "package.json craft entries incomplete"

# --- 3. the pre-craft shared set must not shrink -------------------------------
old_count=$(expand "${OLD_SHARED_GLOBS[@]}" | wc -l | tr -d ' ')
if [ "$old_count" -lt "$OLD_SHARED_MINIMUM" ]; then
  fail "pre-craft shared test set shrank: $old_count < $OLD_SHARED_MINIMUM"
fi
echo "OK: pre-craft shared set intact ($old_count >= $OLD_SHARED_MINIMUM files)"

# --- 4. run the craft tests -----------------------------------------------------
echo "running: pnpm run test:craft:shared"
pnpm run test:craft:shared
echo "OK: test:craft:shared passed"
