# T63 Task 6 Production Wiring — Validation Report

- Result: **PASS**
- Revision: `11f6da40b1b77fd061207636cf9f6a45f512e852`
- Base: `5f121c96df241724428adb2187b9984caf333fb5`
- Plan: `docs/plans/issue30-sweep/plans/plan-t63-task6-production-wiring.md`
- Scope: production-composed retired-Agent gate on the Workbench Start HTTP path; no source or test source files modified during validation.

## Commands and results

1. `git rev-parse HEAD` — PASS; returned `11f6da40b1b77fd061207636cf9f6a45f512e852`.
2. `git status --short` — clean before validation/report creation.
3. `git diff --check 5f121c96df241724428adb2187b9984caf333fb5..11f6da40b1b77fd061207636cf9f6a45f512e852` — PASS, no whitespace errors.
4. `go test ./internal/container/ -run TestWorkbenchAdmissionRejectsRetiredAgentFromProductionProvider -count=1` — PASS (`ok .../internal/container 1.943s`). Linker warning: `ld: warning: ignoring duplicate libraries: '-lc++'`.
5. `go test ./internal/container/ ./internal/modules/workbench/service/workbench/ -run 'WorkbenchAdmission|AgentUse' -count=1` — PASS (container 3.046s; workbench service 1.494s). Same duplicate `-lc++` linker warning.
6. `go build ./...` — PASS (exit 0). Linker emitted duplicate `-lc++` warnings for `cmd/desktop` and `cmd/server`.

## Evidence and acceptance

- The new test is in `package container`, uses `wiringTestDB(t)` (full SQLite migrations plus tenant/session fixtures), seeds a persisted retired variant, constructs `NewWorkbenchAdmissionCoordinator` with real repositories, and calls `session.NewWorkbenchStartHandler(coordinator).Start` through Gin HTTP.
- The production provider at `internal/container/workbench.go` installs `SetAgentUseGate` backed by `RetiredVariantAgentExists`; the test does not manually install a gate.
- The HTTP test asserts `409 Conflict` and zero matching tenant-1 `workbench_requests` and `agent_runs` for its request ID.
- The handler normalizes tenant/user keys into context; admission checks the gate before creating a pending durable request. The focused HTTP test passed, covering its externally observable authorization context, denial status and zero-write assertions.
- No package cycle was introduced: integration test package is `container`, and the specified builds/tests compile.

## Gaps and risks

- Acceptance criteria covered; no behavioral gap found.
- The prescribed mutation-sensitivity experiment (temporarily removing provider gate registration) was not repeated as it requires editing production source; current provider registration was inspected directly and the test passed against it.
- Linker duplicate `-lc++` warnings are environmental/toolchain warnings; all commands exited successfully.
- This report is an allowed validation artifact. The worktree was clean before report creation; no production or test source was changed.
