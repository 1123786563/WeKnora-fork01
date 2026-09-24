# Craft RunView Protocol Adapter Plan

> **For Codex:** Execute with Superpowers SDD, RED → GREEN → REFACTOR, exact checkpoint, independent Review and backend validation. Do not commit.

**Goal:** Add a narrow immutable directory binding to the pinned OpenCode Go client as one prerequisite for T01/T05 per-Run execution views. This does not make the shared serve process an OS sandbox or verify a Ticket.

**Sources:** Approved Spec #107; T01/#120 and T05/#124 snapshots; `docs/plans/2026-09-23-craft-107-per-run-isolation-design.md` in the T01 Worktree; pinned OpenCode 1.18.4 protocol lock. The design found request-scoped `x-opencode-directory` support at the pinned source commit and a global `/event` stream. Integration Worktree already has T00/T08 and interim T01 source checkpoint `eecd663d...`.

**Global Constraints:** backend_implementer exclusive ownership of `internal/modules/agentruntime/agent/opencode/client.go` and focused client/protocol tests in the same package. Do not edit Craft runtime, OpenCode executor, store, container, migrations, global HTTP transport, or any other Agent files without controller amendment. No commits or subagents. Others share the integration Worktree; preserve their edits.

**Review Focus:** Bound client is immutable and cannot be retargeted between Runs; every OpenCode request that uses project/session context carries the same canonical server-selected directory header; reject empty/relative/traversal/path-list/CRLF directory input and unexpected header redirect behavior. Global `/event` remains a known separate isolation gate and must not be falsely claimed scoped. Default client behavior and pinned endpoint request shapes remain backward-compatible.

## Task 1 — immutable directory-bound client

**Depends on:** reviewed T00/OpenCode client baseline; independent of T01 fix3 files. **Consumes:** server-owned canonical RunView directory string. **Produces:** client clone or constructor option with immutable directory binding.

1. RED: Add HTTP fixture tests for CreateSession, Prompt, Messages, Status, Abort, and other instance-scoped client methods on a bound client. Assert the exact header and no header on an unbound legacy client; redirect cannot cross origin or lose binding. Add invalid-directory rejection and two cloned clients whose headers cannot cross-contaminate.
2. GREEN: Add a small `WithDirectory`/bound-client API that returns an immutable copy and sets the pinned `x-opencode-directory` header in one request-construction seam. Validate path shape and avoid an environment-derived default. Never accept this directory from model output or HTTP request parameters.
3. REFACTOR: Run focused OpenCode client/protocol tests, full `go test ./internal/modules/agentruntime/agent/opencode -count=1`, `git diff --check`. Capture full file hashes, actual HEAD/staged/unstaged/untracked Review Package and report. Explicitly say this header is routing context, not filesystem isolation.

**Failure handling:** If pinned `/event` cannot be scoped or a server test contradicts the header behavior, leave the execution path fail-closed and report the exact protocol result; do not make the header a substitute for a per-Run container or safe event router.
