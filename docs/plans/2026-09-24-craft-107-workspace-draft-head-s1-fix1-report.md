# Craft Workspace Draft Head S1 Task 1 — fix1 report

## Findings addressed

- **High, Read authorization/consistency race:** replaced separate Workspace/head/files reads with one owner-, tenant- and session-scoped JOIN SELECT. The query joins current head, its immutable revision row and files in one statement snapshot; selected revision source Run and digest must match the head before the manifest is returned.
- **Medium, unresolved error hierarchy:** `ErrDraftHeadUnresolved` is now an independent sentinel and no longer matches `ErrNotFound`.
- **Medium, missing SQL identity binding:** added a supporting unique `(tenant_id,id)` key for `craft_workspaces`; heads now reference the matching tenant/workspace pair; revisions reference the matching `(tenant_id,run_id)` in `agent_runs` with `ON DELETE RESTRICT`. Go scope checks remain in Read.

No `Advance`, S2 work, or runtime wiring was added.

## RED → GREEN and verification

- RED domain: `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead' -count=1` failed because unresolved matched `ErrNotFound`.
- RED SQL: the same command failed because SQLite accepted a head whose tenant did not match its Workspace.
- GREEN SQLite: `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead|Workspace' -count=1` passed.
- GREEN PostgreSQL: with a disposable `paradedb/paradedb:v0.22.2-pg17` container and `TRPC_TEST_POSTGRES_DSN=postgres://craft:craft@localhost:55439/craft?sslmode=disable`, the same focused suite passed, including selected manifest read, tenant/source FK rejection and owner scope checks.
- Migration proof: `TestCraftDraftHeadMigrationUpDownUp` passed SQLite 000116 up/down/up and PostgreSQL 000195 up/down/up against disposable PG17 using prerequisite schemas. The migration files are tested as exact source contents; the temporary PostgreSQL source renames them to migration number 000001 only to apply them in isolation.
- `git diff --check` passed. `gofmt -d` over changed Go files returned no output.
- The selected-read test verifies exact selected files/revision/digest and asserts the store uses one SQL statement for authorization plus head/revision/files.
- Disposable database container was removed after verification.

## Checkpoint and ownership

- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Pre-fix file hashes/status are recorded in `2026-09-24-craft-107-workspace-draft-head-s1-fix1-checkpoint.json`.
- Full Task 1 post-fix patch, including all four migration SQL files, is `2026-09-24-craft-107-workspace-draft-head-s1-fix1-checkpoint.patch`; JSON records its SHA-256 and every owned-file SHA-256.
- At fix1 start the four migration files were already staged additions from Task 1. No index changes or commits were made during fix1. Existing unrelated shared changes were preserved.

## Remaining limits

The read race is closed through single-statement snapshot semantics; a timing-based owner-transfer race test was not added because a cross-dialect deterministic interleaving would require artificial test hooks. The SQL shape and statement-count test cover the consistency boundary directly. Source Run deletion is restricted while referenced by a draft revision so immutable provenance cannot disappear.
