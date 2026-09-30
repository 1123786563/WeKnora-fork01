# Craft #107 F06/F08 Migration Stack Fix

## Plan

- **Scope:** Repair only `TestCraftRunCapturePromotionMigration` for the integrated SQLite chain where F08 migration 000138 follows F06 migration 000137. Keep the isolated fixture and migration source unchanged.
- **Change:** Roll back two steps, assert both F06 promotion tables and the F08 receipt table are absent, replay two steps, and assert all three tables are restored.
- **Verification:** Reproduce the current failure first; after the edit, run F06/F08 repository migration tests, SQLite database migration tests, and `git diff --check`.

## Report and checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Base: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit created.
- RED: `go test ./internal/application/repository -run '^TestCraftRunCapturePromotionMigration/sqlite$' -count=1` failed at line 39 because `craft_run_capture_promotion_cursor` remained after `Steps(-1)` rolled back the later F08 receipt migration.
- GREEN: `go test ./internal/application/repository -run '^(TestCraftRunCapturePromotionMigration|TestCraftWebBuildReceiptSQLiteMigrationUpDownUp)$' -count=1` passed after changing the down/up step counts to -2/+2 and checking both migration-owned table sets.
- Related database checks: `go test ./internal/database -run '^(TestSQLiteMigrationsIncludeAutoTagConfig|TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData)$' -count=1` passed.
- A full `go test ./internal/application/repository -count=1` was started, then interrupted after running without output for over two minutes; the focused F06/F08 migration tests above passed.
- Scope checkpoint: one Go test file plus this plan/report; no migration or product implementation edits.
- Final hashes and the remaining requested checks are recorded in the parent task report after verification.
