# Craft Workspace Draft Head S1 — Task 1 Report

## Scope

Implemented Task 1 only: domain contract and validation, explicit empty draft head creation with Workspace creation, scoped read/unresolved behavior, paired schema migrations, and focused tests. `Advance` is intentionally fail-closed with `ErrUnsupported` pending the separate reviewed Task 2 capture/CAS work. No Task 2 files or RunView/runtime wiring were changed.

## Evidence

- Initial HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Initial owned source files `craft_workspace.go` and `craft_workspace_test.go` were clean; Task 1 new Go/migration paths were absent. Other shared worktree changes were left untouched.
- Migration allocation rechecked: PG `000195` and SQLite `000116` were free after `000194` / `000115`.
- RED: `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead|Workspace' -count=1` failed at compile time because the new DraftHead types/store were not yet implemented.
- GREEN: same command passed after implementation (craft and repository packages passed).
- Migration proof: `go test ./internal/application/repository -run 'TestCraftDraftHeadMigrationUpDownUp|TestCraftDraftHeadEmptyReadIsScopedAndDurable/postgres' -count=1 -v` passed SQLite up/down/up. The PostgreSQL subtest was skipped because `TRPC_TEST_POSTGRES_DSN` is unset; PG migration/runtime behavior remains unverified.
- `git diff --check` passed with no output.
- `gofmt -d` on changed Go files passed with no output.
- SQLite behavior tests cover persisted/reopened empty revision 0, owner isolation, legacy missing-head unresolved, and exactly one initial head under concurrent Workspace creation.

## Files

- `internal/modules/craft/draft_head.go`, `internal/modules/craft/draft_head_test.go`
- `internal/application/repository/craft_draft_head.go`, `internal/application/repository/craft_draft_head_test.go`
- `internal/application/repository/craft_workspace.go`, `internal/application/repository/craft_workspace_test.go`
- `migrations/versioned/000195_craft_workspace_draft_head.{up,down}.sql`
- `migrations/sqlite/000116_craft_workspace_draft_head.{up,down}.sql`

The repository ignores all of `migrations/` (`.gitignore:96`). To avoid altering the shared index, the migration files remain ignored/untracked at the filesystem level; they are included in the checkpoint patch for the orchestrator to force-add during integration.

## Checkpoint

Exact task patch and file hashes are recorded in `2026-09-24-craft-107-workspace-draft-head-s1-task1-checkpoint.patch` and `.json`. No commit or index staging was performed.

## Remaining risks / assumptions

- PostgreSQL execution is not verified without `TRPC_TEST_POSTGRES_DSN`; only SQLite up/down/up and both dialects' shared test structure were exercised.
- Task 1's domain validator enforces canonical path, uniqueness, valid SHA-256, nonnegative size, nonempty refs and digest consistency. A product-level maximum selected-draft file count/total byte cap is not defined by the approved seam materials inspected, so no new quota was invented.
- `DraftHead` is a Go value with an exported slice; automatic defensive copy on arbitrary struct assignment is impossible. `Clone()` provides the explicit copy boundary and repository `Read` returns a clone. Callers crossing ownership boundaries must use it.
