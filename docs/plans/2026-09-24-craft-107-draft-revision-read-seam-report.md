# Craft #107 T01 R4 prerequisite — immutable draft revision read report

## Result

Implemented `DraftHeadStore.ReadRevision(ctx, scope, workspaceID, revision)` and its repository implementation. The method reads Workspace authorization, current head identity, exact immutable revision, source Run and file rows in one joined SELECT. For positive revisions it validates that the revision exists at or below the current head, the source Run still belongs to the authorized owner/session and is terminal, and the returned `DraftHead` recomputes the stored manifest digest and passes canonical path, ref, byte count and size bounds. Revision 0 returns an explicit empty predecessor only when the authorized Workspace has a valid durable head row. It does not derive files from the current head.

The focused tests prove D1 remains readable after D2 becomes current, revision 2 selects D2, and revision 0 remains explicitly empty. They also cover wrong owner/session/tenant, future/missing/negative revisions, corrupt file digest/path, revision digest and current-head metadata mismatch.

## TDD evidence

- RED: `go test ./internal/application/repository -run '^TestCraftDraftHeadReadRevision' -count=1` failed to compile with `store.ReadRevision undefined` for all new calls.
- GREEN: the same focused command passed after implementation (`ok`, 1.525s on the first pass; 1.924s on the expanded test pass).

## Verification

- `go test ./internal/application/repository -run '^TestCraftDraftHead(ReadRevision|SelectedRead|EmptyRead|LegacyRead)' -count=1` — PASS (`ok`, 3.638s).
- `go test -race ./internal/application/repository -run '^TestCraftDraftHeadReadRevision' -count=1 -timeout=120s` — PASS (`ok`, 2.822s).
- `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead' -count=1` — PASS (`craft` 0.720s; `repository` 8.418s).
- `git diff --check -- internal/modules/craft/draft_head.go internal/application/repository/craft_draft_head.go internal/application/repository/craft_draft_head_revision_test.go` — PASS (no output).
- `gofmt -w` was run on the three owned Go files.
- `TRPC_TEST_POSTGRES_DSN` was unset. PostgreSQL and PostgreSQL race coverage are therefore **not verified** in this run; the tests use the repository's SQLite integration harness.
- Package-wide `go test ./internal/modules/craft ./internal/application/repository` — FAIL in unrelated existing coverage. The output showed `internal/modules/craft.TestExcerptOfBoundsAtRuneBoundary` failing (`overshoot marker missing: "知知"`) and multiple actor admission tests failing because registered Craft admission required a Craft input manifest. The output was truncated by the runner; this package-wide failure does not change the passing focused draft-head evidence.

## Checkpoint and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Exact task-local patch: `docs/plans/2026-09-24-craft-107-draft-revision-read-seam-task-local.patch`
- Hash manifest: `docs/plans/2026-09-24-craft-107-draft-revision-read-seam-checkpoint.json`
- No commits or staged changes were made. `internal/container/craft_runtime.go`, H2-owned tests, and T19 files were not modified by this prerequisite task.

## Remaining review and limits

Independent review is required before the R4 runtime Task 1 is unblocked. PostgreSQL 17.9 behavior must be verified when the disposable database/DSN is available. No R4 materialization, output filesystem writes, promotion, or runtime enablement is included here.
