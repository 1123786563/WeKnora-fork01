# T63 Task 6 Implementation Report

- Task: `docs/plans/issue30-sweep/plans/plan-t63.md` Task 6, lifecycle acceptance evidence.
- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t63`
- Base HEAD: `1186048286c6fdddd2fef76e4e0c93699ba85910` (confirmed before edits).
- Scope: only `internal/router/routes_agent_marketplace_lifecycle_test.go` and this report.
- Dependencies: Tasks 1–3 implementations are present in the base history (`4385f6d67`, `a4ab413a3`, `512ff27cb`, `c28e783bb`); Task 4 has implementation, validation and review reports; Task 5 has the implementation report and accepted r3 review/validation records. The coordinating task dispatched Task 6 after recording Tasks 1–5 as verified.

## Changes

- Added an AC1 lifecycle conservation test exercising Retire, End, Unlist and Deprecate over the real HTTP lifecycle routes. It snapshots and compares Release, review, version, Adoption, Variant, local Agent, license, submission, Run, workbench request and artifact-version row counts; it creates real historical Run and Artifact rows before exit and verifies the published local Agent is not soft-deleted.
- Added an AC2 behavior matrix proving Unlisted listings disappear from catalog and reject new adoptions while existing adoption upgrade proposals continue; Deprecated releases remain catalog-visible, allow adopting the non-deprecated current release, and reject explicit adoption/variant creation against the deprecated release with successor guidance.
- Added an AC3 real HTTP workbench admission test proving an available published Agent admits work before retirement, disappears from available-agents after retirement, and receives HTTP 409 for a new start with no durable request or Run row.
- Added only test-local helpers. No production source changed.

## Commands and results

1. Initial acceptance test run exposed two fixture assumptions, not product failures: the AC1 test tried a list endpoint not registered by `newLifecycleTestApp`, and the AC2 scenario published a new Release on the same intentionally-unlisted Listing. The test was corrected to directly assert the persisted Agent row and to use a fresh real-stack fixture for the Deprecated comparison.
2. `go test ./internal/router/ -run 'TestLifecycleExitDeletesNothing|TestLifecycleUnlistedVersusDeprecated|TestLifecycleRetireBlocksNewWork' -count=1 -v` — PASS, all three acceptance tests.
3. `go test ./internal/router/ -run 'AgentMarketplaceLifecycle|Lifecycle|TestLifecycleRetireBlocksNewWork' -count=1` — PASS, `ok github.com/Tencent/WeKnora/internal/router 7.315s`.
4. `go test ./internal/modules/workbench/service/workbench/ -run 'TestAdmission' -count=1` — PASS, `ok .../service/workbench 9.198s`.
5. `go test ./internal/handler/session/ -run 'WorkbenchStartHTTPIntegration' -count=1` — PASS, `ok .../handler/session 2.015s`.
6. `go build ./...` — PASS (exit 0); linker emitted duplicate `-lc++` warnings for `cmd/desktop` and `cmd/server`.
7. `git diff --check` — PASS with no output.

## Assumptions and limits

- AC1's audit-history evidence uses the Marketplace review table (`agent_release_reviews`), consistent with the T63 plan's scope ruling that lifecycle operations do not add Marketplace `audit_logs` entries. Existing Task/Artifact history is represented by a real workbench Run and `artifact_versions` row.
- The real admission route is mounted in the test engine with the same repository predicate seam used by production wiring. The check covers new work; it intentionally does not test an already-admitted Run's execution lifecycle.
- Independent Task 6 review and validation remain pending. No commit has been made.

**Status: DONE — awaiting independent review and validation.**
