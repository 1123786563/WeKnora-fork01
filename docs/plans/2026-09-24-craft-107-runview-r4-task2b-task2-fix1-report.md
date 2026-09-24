# Craft #107 R4 Task 2b Task 2 Fix 1 Report

## Scope

Closed independent review findings F1–F3 from `2026-09-24-craft-107-runview-r4-task2b-task2-review.md`. Only the capture repository/service and their focused tests changed. No lifecycle source, migrations, runtime wiring or publication path changed.

The repository now loads sealed file rows from the exact tenant/workspace/Run receipt in `EnsurePending` and after `Seal`, validating the nonempty file set against the sealed manifest digest. Seal completion no longer scans the first 500 global recovery receipts. Before Advance, the service verifies the receipt's sealed refs against the scoped repository state and resource catalog tenant/size/hash metadata, then reads each object through `FileService.GetFile` and checks its byte count and SHA-256 under the configured bounds. Any missing, corrupt, wrong-size, mismatched or cross-tenant ref leaves the sealed receipt unresolved and does not advance the draft.

## Changed files

- `internal/application/repository/craft_run_capture.go`
- `internal/application/repository/craft_run_capture_test.go`
- `internal/application/service/craft_run_capture.go`
- `internal/application/service/craft_run_capture_test.go`

## RED evidence

- Repository test `TestCraftRunCaptureEnsurePendingReturnsScopedSealedFiles` failed because an existing sealed receipt returned an empty `Files` list.
- Repository test `TestCraftRunCaptureSealIsNotHiddenByUnrelatedRecoveryBacklog` failed with `craft not found` after 501 older unresolved receipts hid the just-sealed row beyond the global 500-row page.
- Service test `TestCaptureSealedRetryFailsClosedForUnavailableOrInvalidRefs` failed in all four cases (missing object, corrupt bytes, wrong size, wrong-tenant object ref) because the sealed retry advanced successfully without verification.

## Final verification

- `go test ./internal/application/repository -run 'Test(CraftRunCapture|TerminalCancellationEnqueuesCapture)' -count=1` — PASS.
- `go test ./internal/application/service -run '^TestCapture' -count=1` — PASS, including real SQLite repository/service direct retry `TestCaptureSQLiteSealedRetry`.
- `go test -race ./internal/application/repository -run 'Test(CraftRunCapture|TerminalCancellationEnqueuesCapture)' -count=1` — PASS.
- `go test -race ./internal/application/service -run '^TestCapture' -count=1` — PASS.
- PostgreSQL capture transaction tests were not rerun for this fix; no migrations changed. Prior Task2 isolated PostgreSQL 000202 up/down/up evidence remains separate from these code changes.

## Checkpoint

- Exact preimage files were recovered from the previously reviewed Task2 checkpoint patch and their SHA-256 values match the prior Task2 checkpoint JSON.
- The task-local incremental patch contains only the four changed owned Go files. Its SHA-256 and the pre/post file hashes are recorded in `2026-09-24-craft-107-runview-r4-task2b-task2-fix1-checkpoint.json`.
- No commit was created.

## Remaining risks

- PostgreSQL behavior for capture receipt queries and resource ownership checks remains unverified; this fix's verification was SQLite-backed.
- Task3 must still wire the server-side quiescence proof and recovery source. This fix does not enable runtime capture or remove the existing fail-closed gate.
