# Craft #107 T01 R4 prerequisite Fix — immutable revision-zero origin

## Result

Implemented a persisted, immutable D0 origin for each Workspace. New Workspace creation now writes the empty head and origin marker in one database transaction. `ReadRevision(..., 0)` requires the marker joined to the tenant, Workspace and current head in its authorized SELECT; a missing or malformed marker returns `ErrDraftHeadUnresolved`.

Added paired migrations at PostgreSQL 198 and SQLite 119. Backfill creates an origin for a valid empty head only when no revision history exists. A selected head is backfilled only when its current revision matches its immutable revision row, the complete revision sequence from 1 through the current revision exists, each source Run is terminal and matches the Workspace owner/session, and every revision has files. A legacy Workspace without a head, or any structurally incomplete history, receives no origin.

The migrations protect origins, revisions and files against update or direct deletion while their Workspace exists. Workspace teardown still cascades the history. Repository tests cover D1→D2 while preserving D0, missing-origin rejection, atomic marker creation, safe backfill and up/down/up, headless legacy refusal, source/file tampering protection, and owner transfer behavior.

## TDD evidence

- RED: `go test ./internal/application/repository -run '^TestCraftDraftHeadReadRevision(RequiresPersistedZeroOrigin|ReturnsFrozenManifestAfterHeadAdvances)$' -count=1` failed because `craft_workspace_draft_origins` did not exist.
- GREEN: the focused SQLite tests passed after adding the migration, origin write and read check.

## Verification

- SQLite: `go test ./internal/application/repository -run '^TestCraftDraftHeadReadRevision|^TestCraftWorkspaceCreationRollsBackWhenZeroOriginInsertFails|^TestCraftDraftOriginMigration' -count=1` — PASS.
- SQLite race: same focused selection with `-race` — PASS.
- SQLite/domain selectors: `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead|DraftOrigin|CraftWorkspaceCreationRollsBack' -count=1` — PASS.
- PostgreSQL 17.9 disposable database: `go test ./internal/application/repository -run '^TestCraftDraftHeadReadRevisionCannotAuthorizeThroughTransferredWorkspace|^TestCraftDraftOriginMigration' -count=1 -timeout=180s` — PASS.
- PostgreSQL 17.9 disposable database race: same selection with `-race` — PASS.
- The integration migration test performs migration down to the prior tip, up, down and up again on both engines. It verifies valid empty and selected lineages receive D0, D1 remains readable after D2, and a headless legacy Workspace remains unresolved.
- PostgreSQL test database and role were removed after both runs. The earlier temporary role and `pg_search` extension created in the default `postgres` test database were also removed. Final checks reported zero leftover temporary roles, databases or extension.
- `gofmt -d` and scoped `git diff --check` — PASS (no output).

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Migration tips verified before implementation: PostgreSQL 197 and SQLite 118. New paired tips: PostgreSQL 198 and SQLite 119.
- Exact task-local patch: `docs/plans/2026-09-24-craft-107-draft-revision-zero-fix-task-local.patch`.
- Hash and ignored migration byte manifest: `docs/plans/2026-09-24-craft-107-draft-revision-zero-fix-checkpoint.json`.
- Migration files were marked intent-to-add for visibility because `migrations/` is ignored; their contents were not staged. No commit was created.
- `internal/container/craft_runtime.go` remains at task-start SHA-256 `1d37595229ae07699af052f4fb0ca1c2f238c36054ccc57db7c986941bd2e4c2`.

## Review gate and limits

Independent review remains required before R4 runtime Task 1 resumes. Migration backfill establishes D0 from the S1 head/revision lineage invariants; it does not recompute every historical manifest digest in SQL. Positive revision reads still validate the full manifest digest and file metadata before materialization.
