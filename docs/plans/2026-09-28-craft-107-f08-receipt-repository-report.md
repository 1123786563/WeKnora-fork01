# F08 Build Receipt Persistence Report

## Assignment and scope

Implement the independent persistence slice for F08 Task L in the assigned worktree. Source contract: `docs/plans/2026-09-28-craft-107-f08-execution-receipt-design.md` and the parent Task L brief. Receipt identity is immutable and includes tenant, task/session, workspace, run, activity, and request digest; command/pin identity; Docker provider/container/exec; terminal process, exit, start, transport and output completeness; and exact output generation/candidate manifest digest. The logical attempt key is `(tenant_id, task_id, workspace_id, run_id, activity_key, request_sha256)`. Exact replay is idempotent; a conflicting replay is a conflict. First terminal observation is durable and cannot be overwritten.

This task owns only the new receipt repository, its focused tests, SQLite and versioned/PostgreSQL migrations, and this report. Container dispatch, build evidence readers, promotion code, and the integration checkout are outside ownership.

## Worktree and baseline

- Worktree: `/Users/wuyongjun/.codex/worktrees/ocr-build-pipeline/WeKnora-fork01`
- Starting HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`
- Existing unrelated/reviewed changes were present at task start and are preserved.
- Migration reservation: SQLite `000138`; versioned/PostgreSQL `000217`. These are forward slots. The worktree base ends at SQLite `000135` and versioned `000214`; the intervening uncommitted `000136/000137` and `000215/000216` migration files are absent here. Do not renumber this task's reserved slots.

### Owned paths and starting state

Each owned implementation/migration path below was **ABSENT** at task start (no prior file hash):

- `internal/application/repository/craft_web_build_receipt.go`
- `internal/application/repository/craft_web_build_receipt_test.go`
- `migrations/sqlite/000138_craft_web_build_receipt.up.sql`
- `migrations/sqlite/000138_craft_web_build_receipt.down.sql`
- `migrations/versioned/000217_craft_web_build_receipt.up.sql`
- `migrations/versioned/000217_craft_web_build_receipt.down.sql`
- `docs/plans/2026-09-28-craft-107-f08-receipt-repository-report.md` (this report; absent immediately before its baseline creation)

Relevant unchanged source/migration pattern hashes at the starting checkpoint:

| Path | SHA-256 |
| --- | --- |
| `internal/application/repository/craft_candidate.go` | `cd4bff4d529cda4bcb1cb05aaeb530c342dd546aac29f54d05fb56102c13fc13` |
| `internal/application/repository/craft_candidate_test.go` | `71b13d572801594a72099c94d7a2574d909f8d929002ae6fba427b684eede24f` |
| `internal/application/repository/craft_docker_normal_input.go` | `182d3a1c82d95e8bc09744332df5fe68e8bc2a0479a83dd4d3f25706d192a0a1` |
| `migrations/sqlite/000135_craft_stop_intents.up.sql` | `66068198af980dce202517245c382b9c12c472473faf138beb4d3e5d7acd5e74` |
| `migrations/sqlite/000135_craft_stop_intents.down.sql` | `144df2d8265bb4d7f3c3ef71d1737313c7497a35b5cafb65edeec4341db4730e` |
| `migrations/versioned/000214_craft_stop_intents.up.sql` | `78693b50acfd34b5ecbe27b824313a9898ea754bb8a71cdb28c248f75aaff14c` |
| `migrations/versioned/000214_craft_stop_intents.down.sql` | `144df2d8265bb4d7f3c3ef71d1737313c7497a35b5cafb65edeec4341db4730e` |

## Implementation plan

1. Add focused repository tests first for validated receipt identity, exact replay, conflicting replay, scoped lookup, and immutable persisted terminal facts; include isolated SQLite up/down/up migration coverage and a PostgreSQL migration test gated by the configured DSN.
2. Add append-only receipt schema for both dialects at the reserved versions, enforcing the logical unique key, required digest/fact shapes, and immutable rows at the database boundary.
3. Implement a typed repository that validates complete terminal facts, inserts atomically, adopts only byte-equivalent replay, returns a conflict for changed replay, and reads only by the complete tenant/task/workspace/run/activity/request identity.
4. Run focused repository and migration tests, inspect the final owned diff, record exact commands/results and final hashes below. If the absent intervening migration files prevent the ordinary full chain, use the migration step harness against the reserved files and report the integration limitation.

## Evidence log

### Implemented

- Added `CraftWebBuildReceiptRepository.RecordTerminal` and `Read`. The logical key is tenant/task/workspace/run/activity/request digest; reads additionally require matching session scope. Exact replay returns the existing first row, while changed facts conflict. Cancellation errors remain discoverable with `errors.Is`.
- Stored command digest, runtime/toolchain/template pins, timeout/output bounds, Docker provider/container/exec, terminal process/exit/start/transport/output facts, and output generation/candidate-manifest digest.
- Added database update/delete guards and validation for terminal state, successful/failed exit facts, digest shapes, and complete-output binding.
- Added SQLite `000138` and versioned/PostgreSQL `000217` migrations. Both are forward slots and remain unchanged despite the absent intervening migration files in this worktree.
- Working assumption: `task_id` and `session_id` are separate required identity components. This follows the Task L persistence brief's explicit tenant/task/session/workspace/run/activity tuple; the parent assembly will supply both from its authoritative Task and scope.

### RED / GREEN and verification

- RED: `go test ./internal/application/repository -run '^TestCraftWebBuildReceipt' -count=1` initially failed compilation with undefined receipt API symbols, as expected before implementation.
- A later first behavior run found an invalid test mutation (successful state paired with exit 7); the fixture was corrected to change the observation timestamp while keeping the replay otherwise valid.
- RED for cancellation: `go test ./internal/application/repository -run '^TestCraftWebBuildReceiptRepositoryPreservesCancellation$' -count=1` failed because context cancellation was wrapped only as store-unavailable. The repository was changed to return `ctx.Err()` on canceled database operations.
- GREEN: `gofmt -w internal/application/repository/craft_web_build_receipt.go internal/application/repository/craft_web_build_receipt_test.go && go test ./internal/application/repository -run '^TestCraftWebBuildReceipt' -count=1 -v` passed. SQLite repository and migration cases passed, including up/down/up; PostgreSQL repository and migration cases were skipped because `TRPC_TEST_POSTGRES_DSN` is unset.
- `git diff --cached --check && git diff --check` passed. Migrations are gitignored by the repository's `migrations/` ignore rule, so the four owned migration files were force-added to the index to ensure the intended deliverable appears in the workspace diff. No commit was made.
- The migration tests copy only the reserved migration pair into golang-migrate's isolated step harness. This avoids depending on absent uncommitted `000136/000137` and `000215/000216` files; full-chain integration remains to be rerun after integration.

### Review repair round 1/5

- Finding: the DSN-gated PostgreSQL repository fixture passed the whole versioned migration body to one GORM/pgx `Exec`, which uses the extended protocol and rejects multiple statements.
- Repair: the PostgreSQL repository fixture now uses `newCraftWebBuildReceiptMigrator(...).Up()` with the exact `000217` up/down pair and the isolated schema. Added `TestCraftWebBuildReceiptPostgresRepositoryWritesAndReadsMigratedReceipt` so the DSN-gated path proceeds from migration setup to repository write/read assertions when configured. The DSN gate is unchanged.
- Verification: `gofmt -w internal/application/repository/craft_web_build_receipt_test.go && go test ./internal/application/repository -run '^TestCraftWebBuildReceipt' -count=1 -v` passed on SQLite; all PostgreSQL repository and migration cases, including the new explicit write/read path, skipped because `TRPC_TEST_POSTGRES_DSN` is unset. PostgreSQL is not claimed as locally passed.
- `git diff --cached --check && git diff --check` passed after the repair.

### Review repair round 2/5

- Finding: the migration harness still used a `database/sql` connection obtained from GORM's default PostgreSQL dialector, which retained pgx extended/cached protocol behavior for multi-statement migration SQL.
- Repair: added `craftWebBuildReceiptPostgresDialector`, configuring `postgres.New(postgres.Config{DSN: ..., PreferSimpleProtocol: true})`. Both PostgreSQL migration setup paths now obtain their `database/sql` connection from that configured dialector; repository assertions continue to use the GORM connection with the isolated schema/search path. Added `TestCraftWebBuildReceiptPostgresDialectorUsesSimpleProtocol` as a protocol-configuration assertion independent of live PostgreSQL.
- RED/GREEN: `go test ./internal/application/repository -run '^TestCraftWebBuildReceiptPostgresDialectorUsesSimpleProtocol$' -count=1` first failed because the configured dialector helper did not exist; after implementation the test passed.
- Verification: `go test ./internal/application/repository -run '^TestCraftWebBuildReceipt' -count=1 -v` passed for SQLite and the protocol-configuration assertion. DSN-gated PostgreSQL migration and repository tests remain skipped because `TRPC_TEST_POSTGRES_DSN` is unset; no live PostgreSQL pass is claimed. `git diff --cached --check && git diff --check` passed.

### Final SHA-256

| Owned path | SHA-256 |
| --- | --- |
| `internal/application/repository/craft_web_build_receipt.go` | `b7c7c7b8dcc8940da2c7598e0944032792de345b7d1f2304fce2f62da24ed421` |
| `internal/application/repository/craft_web_build_receipt_test.go` | `da1c7a5faeb850b27f4c112a764dcb038b361ac89db0246d414da6ccd0cdadac` |
| `migrations/sqlite/000138_craft_web_build_receipt.up.sql` | `2664ac738233b4ceffd781d180e8e155684aa685bce00167720f088376baea4b` |
| `migrations/sqlite/000138_craft_web_build_receipt.down.sql` | `2e407d5756f34aa7c8dbbb28865e83309a8e961ecf56ed2bfb6932a1f10d2211` |
| `migrations/versioned/000217_craft_web_build_receipt.up.sql` | `f66d285e5481cd24b09d0b9438c5a10da5d96a47c9566c0515c30967c655e066` |
| `migrations/versioned/000217_craft_web_build_receipt.down.sql` | `3ffec38519378cbaa521508771280ec70e216c53a35ee49b32c5811bf5538846` |

The report's own final hash is recorded in the Task L handoff to avoid a self-referential digest.
