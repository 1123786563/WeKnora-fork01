# Craft #107 R4 Task2b Task2 Fix1 — independent review

Date: 2026-09-24. Scope: four-file capture repository/service and focused test increment. Reviewed the approved Craft Spec, ADR-0008/0009, `CONTEXT.md`, Task2/Fix1 plans, the prior Task2 review findings F1–F3, the Fix1 report/checkpoint/patch, and exact current bytes. Concurrent lifecycle, migration and other worktree files are outside this increment. No source, test, Docker, OCR or remote issue operation was changed.

## Finding disposition

**F1 closed — direct sealed replay.** `EnsurePending` loads the exact persisted sealed file rows inside its scoped transaction, then validates their manifest digest (`craft_run_capture.go:184–195`, `:455–473`). Direct `CaptureTerminal` retry therefore reaches Advance with stored immutable refs rather than an empty list or reread source bytes. The new real SQLite service test seals, interrupts Advance, mutates source bytes and confirms the original object becomes the draft.

**F2 closed — scoped Seal result.** After committing Seal, the repository loads `(tenant_id, workspace_id, run_id)` directly, checks owner/session/generation/predecessor/digest/state, and loads those file rows (`craft_run_capture.go:385–403`). It no longer consults the first 500 global recovery receipts. The 501-older-row regression passes.

**F3 closed — object integrity before Advance.** The service requires sealed state, verifies manifest digest and scoped repository file rows/resource catalog ownership, active state, byte count and content hash, then reads each `FileService.GetFile` object under capture size bounds and recomputes size/SHA-256 before `DraftHeadStore.Advance` (`craft_run_capture.go:405–471`; service `craft_run_capture.go:207–282`). Missing, corrupt, wrong-size or wrong-tenant refs fail before Advance. `MarkPendingError` preserves the sealed state, so the admission fence remains unresolved. Uploads also pass through this same verification before their first Advance.

No new actionable finding was found within the four-file Fix1 increment.

## Checkpoint and verification

The Fix1 patch SHA-256 matched `0d0920fdd384599d7780e72ce515f5bac73ab3a9dc40e811163389c41d45e8ec`. I applied the prior reviewed Task2 checkpoint patch in an isolated temporary directory, verified the four Fix1 preimage hashes, then applied Fix1 and compared all four postimage hashes and current bytes; the report file hash also matched the manifest. The exact current hashes are repository source `0385e3ad89610ce8b1392e4ab34edd1e609abd1c4c99a6191f6d59f0ec8c5e1a`, repository test `2f7d8325de75610bd6b24b29361cef659c3b0cdd923958bc57af1b118d958cd6`, service source `d208d0e57105ef69f0ce5fcdf6dd15a1ee044d6eddd7ad2cb762467951781b7d`, and service test `254c7e04254e8a108136a1d74f8700301c828de76eea43dbfa2c27fb99e46759`.

Independently ran `go test ./internal/application/repository -run 'Test(CraftRunCapture|TerminalCancellationEnqueuesCapture)' -count=1` and `go test ./internal/application/service -run '^TestCapture' -count=1`; both passed. The worker's race results are recorded in the report but were not rerun here. PostgreSQL capture behavior was not rerun; earlier isolated migration up/down/up evidence does not establish these transaction paths on PostgreSQL.

## Scoped verdicts

**Spec compliance: PASS for Fix1 F1–F3. Code quality: PASS for this exact checkpoint.** The sealed replay and object checks now meet the assigned acceptance in the tested SQLite path. Task3 still must supply authoritative Run/generation quiescence and recovery wiring; PostgreSQL behavior and final integration remain separate open gates. This review does not authorize default Version publication or T15 promotion.
