# Craft #107 T01 R4 Task2b — Task 2 Report

## Scope and interface

Implemented only the durable post-terminal draft capture seam. The receipt repository is `NewCraftRunCaptureStore(db)` and provides terminal enqueue/recovery, durable attempt digest, immutable seal, and idempotent Advance receipt updates. `CraftRunCaptureService.CaptureTerminal(ctx, scope, workspaceID, runID, kind, source)` takes a server-constructed `QuiescentRunArtifactSource`; that source extends Task1's `RunBoundSandboxArtifactSource` with `VerifyCraftCaptureQuiescent(ctx) error`. Recovery uses the injected server-side resolver.

The capture receipt freezes predecessor revision, state, source Run, and digest from the immutable admission `craft_workspace_seed`. Terminal triggers enqueue only when the bound generation and current durable head still match that seed. The recovery scan applies the same comparison. A missing/unknown seed, mismatched head, unbound generation, active slot, unfinished tool/delegation, or missing quiescence proof stays fenced and does not advance a draft. A later Run admission is blocked while a prior RunView-bound terminal Run lacks an advanced receipt.

Object upload is preceded by a durable capture-attempt manifest digest over path/content-hash/size. If a process stops after upload and before seal, retry can continue only with the same manifest. Once sealed, retries use stored refs. A retry after draft Advance but before the receipt update adopts only the exact next `ReadRevision` from the same Run with the same digest, even if the sandbox source is unavailable. Capture has no `VersionStore.Publish` path.

## Changed files

- `migrations/sqlite/000123_craft_run_capture.{up,down}.sql`
- `migrations/versioned/000202_craft_run_capture.{up,down}.sql`
- `internal/application/repository/craft_run_capture.go`
- `internal/application/repository/craft_run_capture_test.go`
- `internal/application/repository/agent_run_lifecycle_test.go` (terminal enqueue/admission fence test only; production lifecycle source remains untouched)
- `internal/application/service/craft_run_capture.go`
- `internal/application/service/craft_run_capture_test.go`

The numbered migration files are ignored by repository `.gitignore`; their exact bytes and hashes will be preserved in the Task2 checkpoint/patch for the parent to force-track.

## Verification record

Final scoped verification:

- `TRPC_TEST_POSTGRES_DSN='postgres://postgres:craft107_local_test_only@127.0.0.1:32771/craft107_r4?sslmode=disable' go test ./internal/application/repository -run '^TestCraftRunCapturePostgresMigrationUpDownUp$' -count=1 -v` — PASS against a unique isolated PostgreSQL schema and a single-migration fixture. The fixture deliberately avoids applying the whole repository chain/extensions; it verifies 000202 up/down/up only.
- `go test ./internal/application/repository -run 'Test(CraftRunCapture|TerminalCancellationEnqueuesCapture)' -count=1 -v` — PASS. Includes isolated SQLite migration up/down/up, terminal-trigger enqueue and next-Run admission fence, active/unknown-generation/pending-tool rejection, missed-receipt recovery, and attempt/seal/receipt immutability. Its Postgres subtest skips when no DSN is present; the dedicated isolated Postgres command above passes.
- `go test ./internal/application/service -run 'TestCapture(Quiescent|Replays|Conflicts)' -count=1 -v` — PASS. Includes quiescence fail-closed, terminal D2 advance without Version publication, sealed-ref replay after Advance/receipt failure, and changed-byte conflict after upload-before-seal.
- `TRPC_TEST_POSTGRES_DSN='postgres://postgres:craft107_local_test_only@127.0.0.1:32771/craft107_r4?sslmode=disable' go test -race ./internal/application/repository -run 'Test(CraftRunCapture|TerminalCancellationEnqueuesCapture)' -count=1` — PASS on the final postimage, including PG migration lifecycle. One earlier final-postimage attempt was blocked by concurrent unused imports in an out-of-scope test file; its owner fixed them and this rerun passed.
- `go test -race ./internal/application/service -run 'TestCapture(Quiescent|Replays|Conflicts)' -count=1` — PASS.
- `gofmt` on owned Go files — PASS.
- Whitespace scan on all Task2 Go/migration source files and `git diff --check -- internal/application/repository/agent_run_lifecycle_test.go` — PASS. Migration files are ignored by `.gitignore`; their exact bytes are included in the Task2 checkpoint and SHA-256 inventory.

Repository fixture RED diagnosis: the initial focused repository run failed before the capture assertions because the test's Craft session was not registered. Registering the fixture row allowed `AgentRunStore.Admit` to derive its authoritative Workspace seed; the final rerun above passes.

## Integration gates and remaining risks

- Do not apply SQLite 000123 or PostgreSQL 000202 to a shared database until R5 predecessor migrations 000121/000200 are integrated and reviewed. The SQLite tests use `openRunTestDB` temporary databases; PostgreSQL tests use an isolated schema.
- Migration integration order remains gated: R5 000120/000199 and then 000121/000200 must land and be reviewed before applying 000123/000202. The capture test intentionally migrates only its isolated SQLite DB to 000123; it does not touch a shared DB.
- PostgreSQL paired migration up/down/up is verified against the parent-provided isolated PG17 database/schema. Full PostgreSQL repository/capture transaction behavior is **NOT VERIFIED**: the focused Postgres evidence covers migration lifecycle only, not terminal triggers/recovery behavior against PG.
- Task3 must implement `VerifyCraftCaptureQuiescent` on the server-created Run/generation source and wire the capture service/recovery resolver. Until then no source can prove quiescence, so the service remains fail-closed and this task alone does not enable runtime capture.
- Terminal enqueue is a paired database trigger so Finalize, status failure, and cancellation share the same transaction boundary. The owning production `agent_run_lifecycle.go` file was not changed.
- Task2 checkpoint: `docs/plans/2026-09-24-craft-107-runview-r4-task2b-task2-checkpoint.json` and `.patch`; ignored SQL file bytes and hashes are included.
