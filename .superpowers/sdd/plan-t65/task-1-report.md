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

> **T35 #65 restore 注记（2026-10-01）**：本报告记载的迁移号 versioned 000205 / sqlite 000126 为原始恢复前编号；恢复集成到新基线后实落 **000271 / 000190**（详见 .superpowers/sdd/2026-10-01-issue65-t35/task-1-report.md）。
