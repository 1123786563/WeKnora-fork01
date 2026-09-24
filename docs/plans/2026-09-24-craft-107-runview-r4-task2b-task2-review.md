# Craft #107 R4 Task 2b Task 2 — Independent Review

## Scope and evidence

Reviewed the Task 2 plan, architecture note, approved Craft web artifact spec, ADR-0008/0009, `CONTEXT.md`, Task 2 report, checkpoint JSON and exact 1,502-line incremental patch. The patch contains the four otherwise ignored SQLite `000123`/PostgreSQL `000202` migration files, capture repository and service files and their tests. `agent_run_lifecycle_test.go` is a shared concurrent file: its final SHA-256 is inventoried, but its mixed working-tree diff is excluded from the Task 2 apply patch. I reviewed only the focused terminal enqueue/admission-fence test as Task 2 evidence and do not attribute its other edits to this task.

The checkpoint patch SHA-256 matches `42473a9468b8def55e57e41bf019a96c84c9b6310a021273b72567d3613a5b45`. The four migration hashes match the checkpoint: SQLite up/down `5b02d84e…3971` / `d1fc271f…a395`, PostgreSQL up/down `bbe6ee78…821` / `fd866123…5053`. Current repository and service hashes also match checkpoint JSON (`b93f8751…5f14`, `5b5e264a…3e92`). This is a static independent review; I did not rerun tests or OCR. The worker reports focused SQLite repository/service and race tests passing, and isolated PostgreSQL migration up/down/up passing. Full PostgreSQL capture transactions, real runtime quiescence, Linux mount acceptance and Task 3 wiring remain unverified.

## Findings

### F1 — High — Direct sealed replay loses immutable file refs

**Evidence:** `internal/application/repository/craft_run_capture.go:176-186` returns `captureValue(existing, nil)` for an existing receipt regardless of `sealed` state. `CraftRunCaptureService.CaptureTerminal` obtains that receipt at `internal/application/service/craft_run_capture.go:65-69`; after an unadvanced sealed receipt, `capture` calls `DraftHeadStore.Advance(..., receipt.Files)` at line 208. `receipt.Files` is empty even though the sealed rows exist. The separate `RecoverPending` path loads sealed file rows; tests cover that path or an in-memory store, not a real-store direct `CaptureTerminal` retry after seal.

**Impact:** A process failure after sealing but before Advance makes direct terminal retries fail or attempt an empty manifest instead of advancing the exact sealed draft. The Run remains unresolved and a later Run is fenced; the required seal/Advance replay is incomplete.

**Smallest correction:** Have `EnsurePending` load and validate the sealed file rows inside its transaction before returning an existing sealed receipt, or route sealed direct retries through a scoped receipt loader. Add a real SQLite service/repository replay test for a sealed, unadvanced receipt with mutated source bytes.

### F2 — Medium — Seal completion depends on the first 500 global pending receipts

**Evidence:** `internal/application/repository/craft_run_capture.go:381-390` calls `RecoverPending(ctx, 500)` after committing the seal and searches that global, ordered page for its own receipt. `RecoverPending` at lines 273-291 orders all tenants' pending/capturing/sealed/blocked rows by creation time and applies the limit before returning. A valid newly sealed receipt beyond 500 older unresolved rows is absent and `Seal` returns `ErrNotFound` despite its committed seal.

**Impact:** Backlog in another tenant or workspace can indefinitely prevent this Run's immediate Advance, causing an admission fence and misleading failure after a successful seal. The unrelated global recovery insert/scan is also executed on every seal.

**Smallest correction:** Read the just-sealed receipt and its files by `(tenant, workspace, Run)` directly after the transaction, checking state and digest. Keep paginated global recovery only for workers.

### F3 — Medium — Sealed object refs are not verified before draft Advance

**Evidence:** The architecture's replay contract requires verifying a sealed manifest against object bytes. `internal/application/service/craft_run_capture.go:119-208` trusts stored `receipt.Files` on sealed replay and calls `Advance` after source quiescence and predecessor checks. `internal/application/repository/craft_run_capture.go:337-390` validates the manifest metadata/digest on seal but does not verify that each `object_ref` still resolves to its recorded SHA-256 and byte count. The service's upload path at lines 169-189 likewise receives refs from `SaveBytes` without a read-back check. Existing tests mutate source bytes, but do not remove or corrupt an object ref before sealed replay.

**Impact:** A lost, truncated or misbound object can become the durable draft head while its manifest still claims the original content. A later Run can seed a broken predecessor despite the capture reporting success.

**Smallest correction:** At the storage boundary, establish a verified durable `SaveBytes` contract or read each sealed ref and compare hash/size before Advance, scoped to the receipt tenant. Test a missing/corrupt ref on sealed replay and require the receipt to remain unresolved.

## Verdict

**Spec compliance: FAIL for Task 2 completion.** The design correctly separates draft capture from published Version, binds receipt identity to tenant/owner/session/Workspace/Run/generation, freezes the admission predecessor, enqueues terminal transitions durably, and fences pending writers. F1 breaks required seal-to-Advance replay. F3 leaves the specified sealed-object verification unproven. Task 3 must still supply the real Run/generation quiescence proof and recovery wiring; T15 promotion checks are future work, so this Task 2 candidate cannot establish T15 readiness alone.

**Code quality: FAIL pending F1–F3.** The state machine and tests cover several fail-closed cases, but a direct real-store retry path is untested, `Seal` uses an unrelated global scan for a scoped result, and object-ref integrity at replay is untested. The reported tests are useful scoped evidence, with the PostgreSQL behavior and integration gates above still open.
