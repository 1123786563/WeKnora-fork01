#!/usr/bin/env bash
# R462 A3: install the pre-push hook into THIS checkout's git dir.
#
# Worktree-aware: inside a linked worktree `git rev-parse --git-dir` resolves
# to <repo>/.git/worktrees/<name>, so the hook lands in that worktree's own
# hooks/ directory and fires only for pushes made from this checkout.
# core.hooksPath is intentionally NOT touched: other worktrees and the main
# checkout keep whatever hook configuration they already have.
#
# The installed file is a thin stub delegating to the versioned hook at
# scripts/git-hooks/pre-push, so hook-logic updates ship with the code
# (re-run this script only when the stub itself changes).
#
# Usage (from repo root): pnpm setup:hooks   (or bash scripts/setup-hooks.sh)
# Bypass for one command: SKIP_HOOKS=1 git push ...

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
GIT_DIR="$(git -C "$ROOT" rev-parse --absolute-git-dir)"
HOOKS_DIR="$GIT_DIR/hooks"
HOOK="$HOOKS_DIR/pre-push"

mkdir -p "$HOOKS_DIR"

cat > "$HOOK" <<'STUB'
#!/usr/bin/env bash
# Installed by `pnpm setup:hooks` (R462 A3). Thin stub delegating to the
# versioned hook inside the worktree. No network, no credentials in this file.
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
exec bash "$ROOT/scripts/git-hooks/pre-push" "$@"
STUB
chmod +x "$HOOK"

echo "Installed pre-push hook -> $HOOK"
echo "core.hooksPath: $(git -C "$ROOT" config core.hooksPath || echo '(unset -- repo default, untouched)')"
