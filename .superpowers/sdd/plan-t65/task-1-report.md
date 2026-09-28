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

- The assigned brief says “six named check codes/status values” but does not enumerate them. Implemented codes `task_success`, `tool_accuracy`, `groundedness`, `safety`, `latency`, `cost`; statuses `passed`, `failed`, `not_applicable`, `not_collected`. These values should be aligned with Task 4/API contract if its approved enumeration differs.
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

- Confirm the code/status allowlist against the API contract before Task 4 exposes it. Schema migration and application tests cover SQLite; the versioned PostgreSQL twin was not run against a live PostgreSQL instance.
