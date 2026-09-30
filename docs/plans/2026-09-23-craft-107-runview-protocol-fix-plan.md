# RunView Protocol Redirect Fix Plan

> **For Codex:** Execute with Superpowers SDD, RED → GREEN → REFACTOR, exact checkpoint and independent Review. Do not commit.

**Goal:** Close the Medium redirect override in `docs/plans/2026-09-23-craft-107-runview-protocol-review.md`. Pinned OpenCode 1.18.4 chooses query `directory` before `x-opencode-directory`, so a bound client must not follow a redirect that adds the query override.

**Sources:** Reviewed RunView protocol checkpoint/plan/report, pinned OpenCode source priority in per-Run isolation design. Input integration HEAD and current uncommitted `client.go/client_test.go` exact hashes are recorded in `runview-protocol-checkpoint-01`.

**Global Constraints:** backend_implementer owns only `internal/modules/agentruntime/agent/opencode/client.go`, focused `client_test.go` in integration Worktree. Wait until its separate RunView binding Task has completed before editing these files so each Task has its own checkpoint. No Craft runtime/container/store/migrations, commits or subagents. Preserve other agents' changes.

**Review Focus:** Same-origin 307/308 redirect with `?directory=/other` cannot reach target even if bound header is retained. Check redirect target after any custom `CheckRedirect` callback that may mutate URL. Encoded or duplicate directory parameters cannot bypass. Ordinary same-origin redirects without override and same bound directory remain compatible; cross-origin remains denied. No dynamic mutable directory state.

## Task 1 — reject query override

**Depends on:** initial protocol Task and independent review finding. **Produces:** fail-closed redirect policy for directory-bound clients.

1. RED: Add `httptest` fixture where `/session` redirects to `/session?directory=/other/run`, records whether target was reached, and asserts bound client returns an error with target call count zero. Include custom redirect callback adding the query and an encoded-query variant.
2. GREEN: In redirect policy after custom callback, parse target URL and reject any `directory` query parameter for a bound client regardless of value. Preserve existing same-origin/header validation and legacy unbound behavior as appropriate.
3. REFACTOR: Run focused redirect tests, full OpenCode package suite, `git diff --check`, record exact changed-file hashes, HEAD/status, test evidence and ongoing OS/global-event limits.

**Failure handling:** Do not clear or rewrite the query to mask the server's precedence; fail closed and let caller reconcile an ambiguous OpenCode request.
