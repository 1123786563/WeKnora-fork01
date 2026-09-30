# Task 1 Implementation Report — Pin original version provenance

- **Scope:** `internal/router/routes_agent_marketplace_lifecycle_test.go` only.
- **Change:** In `TestLifecycleExitDeletesNothingAcrossGovernanceRows`, compare each reloaded Release's `AgentVersionID` to its pre-exit seeded Release value, and each reloaded Submission's `AgentVersionID` to its own pre-exit seeded Submission value. Existing Release-to-Submission equality, tenant-scoped reads, and history assertions remain intact.
- **Commit:** `5f121c96df241724428adb2187b9984caf333fb5` (`test(marketplace): pin lifecycle version provenance`).

## Verification evidence

- `go test ./internal/router/ -run 'TestLifecycleExitDeletesNothingAcrossGovernanceRows|TestLifecycleUnlistedVersusDeprecatedBehaviorDiffers|TestLifecycleRetireBlocksNewWorkEndToEnd' -count=1` — PASS (`ok github.com/Tencent/WeKnora/internal/router 3.533s`).
- `git diff --check` — PASS (clean).
- `git status --short` after commit — clean.

## RED attempt and limitation

Attempted a temporary SQLite update that changed a Release's `AgentVersionID` and its linked Submission's `AgentVersionID` together, to demonstrate the gap in the prior cross-row equality assertion. SQLite rejected the first Release update with `FOREIGN KEY constraint failed` at the injected update. The temporary mutation was removed; no schema or production changes were made. Thus the fixture's FK prevented a persisted mutation-based RED run, while the requested assertions directly pin both separately captured values and the targeted tests pass.
