# S1 Task 2 independent Spec and quality review

## Scope and evidence

Reviewed the approved Craft web artifact Spec (current Workspace continuation and failed Run draft rules), `CONTEXT.md`, ADR-0004/0008/0009, the corrected draft seed seam, S1 plan, Task 1 fix1 review, Task 2 report, the incremental patch, and the four owned live Go files. The live file SHA-256 values and patch SHA-256 exactly match the Task 2 checkpoint JSON: domain `ae779d96` / test `f49ece8e`, repository `d85f9c69` / test `82f193be`, patch `be4f3c52` (full values in the checkpoint). No migration or other source file is in this Task 2 delta. Independently ran `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead' -count=1`: both packages passed (repository 27.599s). This local run exercised SQLite; the Task 2 report records a separate disposable PostgreSQL 17 run and race run, which I did not repeat. No OCR was invoked.

## Finding

**Low — CAS test does not distinguish the winning manifest.** `TestCraftDraftHeadAdvanceSameRevisionCASLeavesOneManifest` in `internal/application/repository/craft_draft_head_test.go` submits the same `sourceRunID` and `files` from both goroutines (lines 269–281), then checks one success and one revision/file row (lines 286–301). That establishes single-winner behavior for identical requests, but cannot detect a bug that persists the losing contender's distinct source Run or object ref while returning the other contender's result. This misses the S1 plan's explicit “no losing manifest becomes current” assertion and weakens the PostgreSQL/SQLite parity claim for that behavior. **Smallest correction:** race two distinct terminal Runs with different canonical manifests, identify the successful result, then assert `Read` and the sole revision/file rows exactly match that winner and contain no loser Run/ref. Keep the existing rollback injection test for partial inserts.

## Verified boundaries and residual limit

- `Advance` checks the scoped Workspace under transaction, locks and checks the source Run's tenant/owner/session and terminal status, serializes the session active slot, and rejects unfinished tool calls and delegations. A Run in another session and a wrong owner are exercised in the tests. The `(tenant_id, session_id)` unique Workspace binding and source Run checks supply the Workspace/Run relationship.
- Canonical path, duplicate path, SHA-256, byte count, 8 GiB aggregate limit, nonempty object ref and digest validation precede writes. The chosen 8 GiB aggregate ceiling is an S1 policy bound borrowed from the existing per-object artifact ceiling; the approved Spec does not prescribe a different draft ceiling.
- The head CAS, revision and file inserts share one transaction. A failed insert rolls back all three in the SQLite injection test. Reusing a source Run is rejected; `Read` recomputes the digest and rejects a tampered selected head. Input and returned slices are copied. A failed terminal Run is permitted without writing `craft_versions`.
- The repository cannot establish live RunView quiescence or that object refs resolve to sealed bytes. Its method comment explicitly requires a trusted caller to verify stopped/idle capture while holding the Workspace writer fence. Task 2 adds no production caller; S3 must enforce this precondition and verify object bytes on materialization. A terminal DB status alone must never be treated as this proof.

## Verdict

**Spec compliance: PASS for the S1 Task 2 repository seam, conditional on the documented S3 caller fence/capture contract.** No evidence in this delta promotes a failed draft to the default preview, chooses a historical Version, or treats a legacy missing head as empty. This verdict does not certify S2 admission, S3 capture/materialization, or full product acceptance.

**Code quality: PASS with one Low behavioral test gap.** The independently run focused SQLite suite passed, checkpoint hashes matched, and the reported PostgreSQL/race runs were not independently repeated. The CAS test should be strengthened before relying on it as proof that a losing distinct manifest can never appear as current.
