# Task 1 Report — Release-Pinned Structured Evaluation Persistence

## Status

DONE_WITH_CONCERNS

## Changes

- Added immutable `AgentEvaluationEntity` and service view/contract containing exact public release, test set/version, environment class, evaluator identity, evaluation timestamp, and structured result JSON.
- Added SQLite migration `000126` and versioned migration `000205`; both reference `public_agent_releases(id)` with `ON DELETE RESTRICT`, enforce the five-field identity uniqueness, and index Release plus evaluation time. No Task, adopter, tenant, mapping, or runtime telemetry fields are stored.
- Added repository insert/read methods. Inserts validate required identity/timestamp fields and strict result JSON (`checks` only; each check has only `code` and `status`), reject unknown codes/statuses, and map duplicate identities to `ErrAgentEvaluationConflict`. The repository offers no update method.
- Added service write/read seam. The writer binds evaluator identity to its `reviewerID` argument (the HTTP SystemAdmin gate is owned by Task 4); catalog reads return only evaluation fields.
- Added migration schema and down/up verification coverage.

## Assumptions and concerns

- Initial implementation used provisional result codes/statuses because the brief omitted their values. The execution plan’s explicit ruling was later identified during review; the correction below replaces those provisional values with the approved allowlist and overall-status field.
- HTTP SystemAdmin authorization is not implemented here because routes are owned by Task 4. Service accepts a reviewer ID and overwrites any caller-supplied evaluator identity; caller authorization must be enforced at the route boundary.
- Adopter runtime error metrics remain `not_collected` as directed; no immutable runtime attribution exists.

## TDD and verification evidence

- RED: `go test ./internal/application/repository -run 'TestAgentEvaluation' -count=1` initially failed to compile because repository/entity/duplicate-conflict behavior did not exist.
- GREEN repository: `go test ./internal/application/repository -run 'TestAgentEvaluation' -count=1` — PASS.
- GREEN service: `go test ./internal/application/service -run 'TestAgentEvaluation' -count=1` — PASS.
- Migration/schema/down-up: `go test ./internal/database -run 'TestSQLiteAgentEvaluationMigrationDownUp|TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData' -count=1` — PASS.
- `git diff --check` — PASS.
- `go build ./...` — PASS (linker emitted existing duplicate `-lc++` warnings for `cmd/server` and `cmd/desktop`).
- Confirmed initial HEAD remained `93706830b78205de0c7d433097e89f33d9726513` before commit. Only Task 1 owned implementation, migration, test, and this report are included in the task commit.

## Remaining risk

- Schema migration and application tests cover SQLite; the versioned PostgreSQL twin was not run against a live PostgreSQL instance.

## Task review correction

The implementation review identified the canonical structured result contract in `docs/plans/issue30-sweep/plans/plan-t65.md` §Recorded implementation rulings. The repository now strictly accepts only overall status `pass|fail|inconclusive` and a non-empty `checks` array whose entries contain only `code` and `status`; codes are `manifest_completeness|compatibility|license|security|dependency_integrity|privacy`, and check statuses are `pass|fail|not_run`. Unknown fields, invalid enum values, missing fields, malformed JSON, and trailing JSON are rejected. The earlier provisional vocabulary has been removed.

### Correction TDD / verification

- RED: updated repository tests to require the plan allowlist; `go test ./internal/application/repository -run 'TestAgentEvaluation' -count=1` failed because the previous repository validator rejected approved codes/statuses and overall status structure.
- GREEN: `go test ./internal/application/repository -run 'TestAgentEvaluation' -count=1` — PASS; covers every approved code/status and overall status, rejects invalid enums, extra/freeform fields, missing/empty checks, malformed and trailing JSON.
- `go test ./internal/application/service -run 'TestAgentEvaluation' -count=1` — PASS.
- `go test ./internal/database -run 'TestSQLiteAgentEvaluationMigrationDownUp|TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData' -count=1` — PASS.
- `git diff --check` — PASS.
- `go build ./...` — PASS.

## Review fix R1: evaluation test evidence repair

Applied only the two test files owned by review-fix Task 1. No production code or migrations changed. Plan ruling T1-R1-4 retained the append-only application API; no SQL triggers were added.

- RED command: `go test ./internal/application/repository -run 'TestAgentEvaluation' -count=1` — FAIL as expected. The first strengthened run exposed a current-pointer assertion comparing `string` to `*string`, and showed the former “unknown overall status” case (`inconclusive` with `not_run`) is valid, not invalid.
- GREEN command: `go test ./internal/application/repository -run 'TestAgentEvaluation' -count=1` — PASS. Invalid-input cases now seed and use a real Public Release FK (except deliberately blank ReleaseID) and assert `ErrAgentEvaluationInvalid`; the positive allowlist test explicitly accepts overall `inconclusive` with check status `not_run`. The pinning test creates a second approved immutable Release on the same Listing, advances the pointer with the first Release ID as expected prior, verifies the pointer moved, then reads both release buckets and confirms evidence remains only under the first Release.
- `go test ./internal/database -run '^TestSQLiteMigrationsCreateVersionedSchema$' -count=1` — PASS; fresh schema asserts all eight Evaluation table columns explicitly.
- `git diff --check` — PASS.
- `go build ./...` — PASS.

## Controller exact validation evidence on review-fix HEAD

- Command: `go test ./internal/application/repository -run '^TestAgentEvaluation' -count=1` — exit 0

  ```text
  ok   github.com/Tencent/WeKnora/internal/application/repository 3.751s
  ```

- Command: `go test ./internal/database -run '^TestSQLiteMigrationsCreateVersionedSchema$' -count=1` — exit 0

  ```text
  ok   github.com/Tencent/WeKnora/internal/database 3.212s
  ```

- Command: `git diff --check` — exit 0; no output.
- Command: `go build ./...` — exit 0

  ```text
  # github.com/Tencent/WeKnora/cmd/desktop
  ld: warning: ignoring duplicate libraries: '-lc++'
  # github.com/Tencent/WeKnora/cmd/server
  ld: warning: ignoring duplicate libraries: '-lc++'
  ```
