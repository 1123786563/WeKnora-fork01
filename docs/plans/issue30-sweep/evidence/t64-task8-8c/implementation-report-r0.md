# Task 8C Report — Atomic Agent Run Admission

## Scope and revision

- Assigned worktree/branch: `/Users/wuyongjun/.codex/worktrees/issue30-t64-task8c/WeKnora-fork01`, `codex/issue30-t64-task8c`.
- Brief SHA256 verified: `2484309185ac2eb29980155802b33f37204b3d94e1082325aaac13dce5cbd4f8`.
- Base HEAD: `3d1d1f94901590e6dd31e84f00fbfbcf1e66159b`.
- Final HEAD: `a9b81799d341d82bbf11f920fbacda1f791656a4`.
- Owned implementation files changed: `internal/application/repository/agent_run.go`, `internal/modules/agentruntime/agent/runtime/contracts.go`, `internal/application/repository/agent_run_security_admission_test.go`.
- No migration, container, handler, service, Workbench, or unrelated source/test files changed.

## Implementation

`AgentRunStore.Admit` now enters `withTenantSecurityGuard` before its transaction accesses the session, request, Variant, Run, or slot rows. It validates session scope and engine, returns an exact idempotent replay before checking current security state, keeps the Variant retirement lock/check, and checks the trusted local Agent Version against the current published mapping and Release before reserving the slot. A trusted Release ID must equal the checked Release. New Marketplace Runs persist all four immutable security pins with `security_pin_source='admission'`. Non-Marketplace Runs omit the nullable pin columns so they remain SQL NULL rather than empty strings. Slot, Run, and message writes remain in the guarded transaction.

The new tests cover security refusal with no Run/message/slot writes, retirement refusal, replay after revocation plus changed-hash conflict, and Release/dependency revocation-first and admission-first barrier schedules. Barrier transcript: in admission-first schedules, the admission callback pauses only after the SQLite `UPDATE tenants SET id = id WHERE id = ?` succeeds; the revocation attempt is separately paused at the pre-lock callback. Releasing admission lets it commit one Run; releasing revocation then lets revocation commit. In revocation-first schedules, revocation completes before admission starts, and admission returns `ErrAgentSecurityReleaseBlocked`. No schedule deadlocked in the focused runs.

## Verification evidence

- Required pre-change RED was not captured before production edits. The first focused security/replay invocation after implementation passed; this is a workflow deviation, not represented as RED evidence.
- `go test ./internal/application/repository -run '^TestAgentRunAdmit(Security|ReplaysExistingRun)' -count=1` — PASS.
- `go test ./internal/application/repository -run '^TestAgentRunAdmitSerializesAgainst(Release|ExactDependency)RevocationBothOrders$' -count=10` — PASS (both barrier cases, ten repetitions each).
- `go test ./internal/application/repository -run '^TestAgentRunConcurrentClaim$|^TestTenantSecurityGuardSerializesDecisiveWriteFamilies$' -count=1` — PASS.
- `go test ./internal/application/repository` — FAIL by timeout after `601.324s`; Go emitted a large goroutine dump with migration reader/pipe activity and exited `FAIL github.com/Tencent/WeKnora/internal/application/repository 601.324s`. The truncated output did not identify a specific failing test. This is not counted as passing.
- `go build ./...` — PASS; linker emitted warnings that duplicate `-lc++` libraries were ignored for `cmd/desktop` and `cmd/server`.
- `git diff --check` — PASS.
- `TRPC_TEST_POSTGRES_DSN` was unset. PostgreSQL row-lock behavior is therefore not verified; successful barrier evidence is SQLite-only and is not a substitute for PostgreSQL evidence.

## Self-review and limitations

- The public `AgentRunStore.Admit(context.Context, agentruntime.Admission)` signature is unchanged; trusted `LocalAgentVersionID` and `ReleaseID` fields are additive runtime admission fields.
- Existing retirement fixture semantics are preserved by keeping lifecycle refusal before the new Marketplace predicate; the fixture has no CustomAgent/Version/Release.
- The focused tests and build pass, but the full repository package timed out and PostgreSQL runtime verification was unavailable.
- Required RED-before-production sequencing was missed; focused verification was performed after the implementation instead.
- Commit: `a9b81799d341d82bbf11f920fbacda1f791656a4` (`Secure atomic agent run admission`), containing only the three owned implementation files. This report is retained at the assigned report path as task evidence and is outside the implementation commit scope.
