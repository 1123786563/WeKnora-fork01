# Task 7 / #147 Backend Report

Status: DONE_WITH_CONCERNS
Workspace: `/Users/wuyongjun/.codex/worktrees/issue-140-t07-intake/WeKnora-fork01`
BASE: `69d5c5fe80b0ffd023771e97255453bd7ab378fc`
HEAD: `cd81dc222f31a07e5aad4a8f4022f055872f3cd2`
Commit: `cd81dc222` (local only; no push)

## Delivered

- Added tenant/owner-scoped durable Career source revisions with private stored-resource handle and extracted text, monotonic per-profile source revision, digest, safe metadata, and `processing|ready|failed` lifecycle.
- Added upload routes `GET /api/v1/career/sources` and `POST /api/v1/career/sources/upload`. Upload validates safe filename, configured size limit, extension/MIME/content signature, stores through durable `FileService` storage, binds the resource to the Career source ID, and parses through the existing `DocumentReader`. It does not use a chat session or expiring temporary-document ID.
- Added a deliberately conservative deterministic extractor. It emits proposals only for explicit one-line education, experience, project, skill, quantified achievement, and certificate labels in English or Chinese. Proposal value and exact source line are retained; ambiguous or repeated experience claims remain pending and receive a review flag. Missing categories and missing graduation year are reported on the source. No model output is fabricated or auto-confirmed.
- Successful extraction atomically transitions the source from processing to ready and writes all pending proposals, change event, profile revision, and idempotency receipt. Identical batches replay even if field order changes; changed content under the same request ID conflicts. Stale expected revision inserts no partial proposals.
- Added purpose-scoped confirmed-fact model input. Pending proposals and raw source text are excluded, identity-number facts are omitted, and identity-like numbers in retained confirmed values are redacted before JSON serialization.
- Added handler protection against a client claiming `resume_extraction` provenance through the manual propose action. Source IDs are generated on the server; regular source responses omit the resource reference and extracted text.
- Added SQLite migration `000114` and PostgreSQL migration `000193`; these follow the existing artifact-revocation migrations at `000113` / `000192` in this worktree.

## Web interface

- `GET /api/v1/career/sources` returns `{ "sources": CareerSource[] }`.
- Multipart `POST /api/v1/career/sources/upload` accepts `file` and optional `requestId` and `expectedRevision`; it returns `{ "source": CareerSource, "receipt"?: CareerReceipt }`.
- `CareerSource` includes `id`, `revision`, `fileName`, `mimeType`, `size`, `digest`, `status`, optional `errorCategory` / `errorMessage`, `missingCategories`, and `reviewFlags`.
- The batch receipt contains `proposals[]`; each proposal stays `pending`, has `source.kind = "resume_extraction"`, `source.referenceId` equal to the server-issued source ID, and `evidence` containing the matched original line.
- A CAS conflict returns the existing 409 revision-conflict contract with `currentRevision`. The source is marked failed with `revision_conflict`, stored bytes are released, and no proposal from the failed batch is committed. The user can retry by uploading again or continue with T03 manual proposals. Confirmed facts and their history are unchanged.

## Verification evidence

Commands run:

- `go test ./internal/modules/career/...` — passed during RED/GREEN implementation.
- `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` — passed after final changes.
- `git diff --check` — passed.

The final targeted run reported all listed Go packages successful. The container test binary emitted the existing macOS linker warning `ignoring duplicate libraries: '-lc++'`; the package still passed.

Focused coverage includes:

- deterministic six-category extraction, missing graduation year, duplicated experience alternatives, exact evidence retention, and rejection of unlabeled/unquantified content;
- processing-to-ready/failed state, tenant/owner isolation, failed-attempt preservation, confirmed-only purpose inputs and final serialized redaction;
- batch CAS, atomicity, stable replay under reordered fields, altered-input idempotency conflict, and stale revision preservation;
- real SQLite migration/reopen persistence of a source revision, while confirming private resource reference and extracted text are absent from public source JSON;
- upload validation, durable (non-temporary) storage, resource binding/release, parser failure behavior, handler provenance rejection, router/container wiring.

## Remaining limits and assumptions

- This is intentionally not a general natural-language resume parser. It recognizes explicit single-line labels only; unlabeled prose is left for manual entry. The existing `DocumentReader` is the only live parsing service used. There is no AI/LLM structured extractor in this backend task, and none is fabricated.
- Repeated experience entries are flagged for review rather than automatically classified as a semantic conflict; each distinct exact labeled line remains a separate pending proposal.
- A revision conflict follows the explicit failed-plus-reupload recovery path. It never commits a partial batch or replaces facts. The failure source keeps safe metadata but the raw file is released; the user reuploads or continues manually.
- Raw resume bytes and extracted text remain under Career-owned storage/DB scope. Retention duration and deletion policy remain for the already planned retention/deletion work; this task does not invent a retention duration.
- SQLite migrations and restart behavior were exercised; no live PostgreSQL instance was available for applying migration `000193`.
- No Web/TypeScript files were modified, per backend ownership. The Web contract must add batch `Receipt.proposals`, proposal `evidence`, and source `missingCategories` / `reviewFlags` before integration.

## Review fix round 1 (R0 findings)

Fix BASE: `cd81dc222f31a07e5aad4a8f4022f055872f3cd2`
Fix HEAD: `074021e5676d0c2b5f3045ed6b991a82f16ed312`
Local commits: `ca72d40c9` (`fix(career): recover idempotent resume intake uploads`), `074021e56` (`fix(career): persist source ref before catalog binding`). No push or merge.

### Findings addressed

- **F1 — same-category facts overwrote:** parser keys now include a deterministic 12-hex item identity derived from the normalized explicit source line, independent of item order. Tests create and confirm two experience facts and verify distinct keys; extraction order keeps the same keys.
- **F2 — forged direct confirmation provenance:** every handler action validates its allowed source kind. Direct confirm/propose is server-normalized to `manual`; proposal confirm/dismiss is normalized to `user_confirmation`; a client `resume_extraction` claim is rejected for direct confirm as well as propose. `confirm_proposal` stores its provenance from the existing server-side proposal.
- **F3 — identity-like model metadata leaked:** model inputs now serialize only `{key,value}` for allowed confirmed facts; source and confirmation metadata are omitted. ID-number redaction applies to fact keys and values. Test serializes the complete model DTO with identity-like numbers in key, value, source label, and reference and verifies the identifier is absent.
- **F4 — request replay created new sources:** upload now requires a nonempty `requestId`, computes the body digest and canonical intent before storage, and durably claims an owner-scoped source ID before calling FileService. Same request/digest/intent returns the original source and, when ready, the deterministic batch receipt; changed bytes or intent returns 409 before a second save. A concurrent exact request returns HTTP 202 with the existing `processing` source while the first save/parse proceeds. HTTP tests verify one save/parse and exact ready replay.
- **F5 — interrupted and uncertain upload outcome:** a 30-minute durable processing lease and `GET /career/sources` / upload-triggered stale reconciliation transition expired claims to visible `failed`/`interrupted`; known catalog-bound resources are released and the source ref is cleared only after release succeeds. The adapter persists the returned resource ref before binding the catalog owner, so later interruption can clean up the ref. Exact replay during active processing is 202; ready replay is 200 with the original receipt; changed intent is 409; after stale cleanup replay returns the terminal failed source and the user can reupload under a new request ID. On `ErrOutcomeUnknown`, the handler checks the source and receipt and returns success if the atomic batch committed; otherwise it leaves the source/resources untouched for later reconciliation. Tests cover active duplicate claim, stale transition, known-ref cleanup state, and idempotent ready replay.
- **F6 — migration rollback incomplete:** SQLite 000114 and PostgreSQL 000193 down migrations now also drop `career_proposals.evidence`; both up migrations include request identity, intent, expected revision, lease, and a scoped partial unique request index. SQLite up/down/up test verifies the source table and evidence column are removed and restored.

### Updated Web contract

- Multipart `POST /api/v1/career/sources/upload` now requires `file` and nonempty `requestId`; `expectedRevision` remains optional and defaults to the current profile revision. The same request ID is bound to the file digest, safe filename, MIME intent, and expected revision.
- HTTP 201 is the initial completed response. An exact retry after completion returns HTTP 200 with the original source and receipt. An exact retry while processing returns HTTP 202 with the same source ID and status `processing`. Reusing a request ID with changed bytes or intent returns 409 `idempotency_conflict` before storage. Stale requests observed by source listing or another upload become terminal `failed` with `errorCategory: "interrupted"`; replay returns that failed source, so the user must reupload with a new request ID or continue manually.
- Parser errors remain visible as failed source metadata and permit T03 manual proposals. Revision conflict continues to preserve confirmed facts and commits no partial proposal batch.

### Verification evidence for this fix

RED evidence before fixes:

- `go test ./internal/modules/career -run 'TestExtractResumeFields' -count=1` failed because same-category keys were identical (`experience.details`).
- Focused tests for direct confirm and model DTO failed with HTTP 200 instead of 400 and leaked the test identity number through fact key/source metadata.

GREEN and final verification on fix HEAD `074021e5676d0c2b5f3045ed6b991a82f16ed312`:

- `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` — passed all packages.
- `git diff --check` — passed with no output.
- SQLite up/down/up and HTTP duplicate upload tests are included in the full passing suite. PostgreSQL migration SQL was updated but no live PostgreSQL instance was available for an apply/down run. The container test binary emitted the existing macOS linker warning `ignoring duplicate libraries: '-lc++'`; its package passed.

### Remaining storage crash window

Career now persists the reference immediately after `FileService.SaveBytes` returns and before adding the Career catalog binding. This closes the later ref-persistence/bind/parse/atomic-completion window: after the returned reference is durable, stale reconciliation can release it, including when no Career binding was added yet. A remaining lower-level window exists inside `SaveBytes`: it can physically write a timestamp-addressed object and register it in the catalog before returning the reference. If the process dies after the physical write but before `SaveBytes` returns, Career has only its durable claim and does not know the generated physical reference; the generic FileService/ResourceCatalog API has no lookup by source-derived filename or owner for this not-yet-bound object. The object can remain orphaned and retry is not guaranteed to overwrite it. This needs a generic storage/catalog recovery API or orphan scan; this fix does not claim to close it.

## Review fix round 2 (R1 findings)

Fix BASE: `074021e5676d0c2b5f3045ed6b991a82f16ed312` (code HEAD; report-only follow-up `dbca655f5a30d9870ef01df7a1002f1ac215b151`).
Fix code HEAD: `5d1b7b2f1a4d39852ad45e4455c3560ca744620a` (`fix(career): fence intake upload cleanup and retries`).
Report update is committed separately; final report HEAD is recorded in Git after that commit.

### Findings addressed

- **F1 — T03 `source.kind="user"` wire alias:** all four manual profile actions accept the Web alias and normalize it server-side to manual/user-confirmation provenance. The server still rejects a direct client claim of `resume_extraction`; clients cannot submit resume-derived fields as extracted facts.
- **F2 — omitted-revision exact upload retry:** the handler resolves an existing request claim and compares digest/file metadata before defaulting an omitted `expectedRevision`. Exact retries reuse the stored revision, source ID, and receipt; an explicitly changed revision remains an idempotency conflict. Test verifies the retry after the profile revision has advanced and that storage runs once.
- **F3 — claim-token fencing and lease takeover:** processing claims carry a private random token. At expiry, takeover rotates it with a conditional update that rechecks lease expiry in SQL (`julianday` for SQLite); stale workers cannot persist resource refs, finish, or complete a batch. Expiry is the 30-minute lease boundary, without an extra stale grace period. Takeover preserves and reparses the existing resource ref, digest-checking bytes from FileService rather than saving another blob. Tests cover stale writer rejection and renewed-lease takeover race.
- **F4 — cleanup state survives errors:** upload failure no longer unconditionally deletes in the adapter. The claim owner marks cleanup-pending and retains its private resource ref until `Release`/`Delete` succeeds; source listing retries cleanup and only then clears the ref and publishes the terminal error category. Tests inject release and delete failures and verify the ref remains durable until a later successful retry.
- **F5 — final model-input redaction:** identity-number patterns are redacted from both fact keys and values, including IDs adjacent to ASCII letters; passport-like identifiers are also redacted. Serialized model input includes only allowed confirmed `{key,value}` facts, without source metadata. Focused DTO test verifies identifiers do not survive final serialization.

### Verification evidence for this fix

- Focused regression tests were added for the T03 alias, omitted-revision exact retry, claim takeover and stale writer fencing, renewed-lease/takeover race, cleanup retry after Release/Delete failure, and final serialized model redaction. Before the fixes, the alias returned HTTP 400, omitted-revision retry returned 409 after revision advancement, and the adjacent-letter identity example survived redaction.
- `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` — passed all packages, including SQLite migration tests. The container test binary emitted the existing macOS linker warning `ignoring duplicate libraries: '-lc++'`; the package passed.
- `git diff --check` — passed with no output.

### Web contract / lifecycle details

- `source.kind="user"` is accepted by the four existing T03 actions and normalized by the backend. `resume_extraction` remains server-owned.
- Upload source statuses remain `processing`, `ready`, and `failed`. Same active request returns HTTP 202; a ready exact replay returns HTTP 200 and the original receipt; changed request intent returns 409. Parser failure remains visible as `failed` and the user can continue with T03 manual proposals.
- A processing lease lasts 30 minutes. An exact upload request that takes over an expired claim reuses the persisted resource ref when present. Stale sources discovered by GET/list cleanup or upload reconciliation become failed; cleanup failure is represented as `cleanup_pending_*` with the ref retained and retried on a later source-list request.
- Batch completion remains transactional and claim-token-fenced: a superseded parser cannot create any proposal, and all proposals for the current claim commit together or not at all.

### Residual risk

The generic `FileService.SaveBytes` physical-write/catalog-registration crash window described above remains. Also, if an object has just been returned by `SaveBytes` but persisting its ref loses a claim race, the handler attempts a bounded detached release; a failure of that release can leave an unbound object for the same generic orphan-recovery mechanism. No behavior here claims deterministic overwrite or automatic discovery for every FileService backend. No live PostgreSQL instance was available to apply migration 000193; its DDL was inspected and the full local SQLite migration suite passed.

## Review fix round 3 (R2 findings)

Fix code BASE: `5d1b7b2f1a4d39852ad45e4455c3560ca744620a` (report-only HEAD `8fdc30f2e`).
Fix code HEAD: `53b30d2472fa80c6d7598ce2812f54791fd43511` (`fix(career): recover cataloged intake uploads safely`). Local commit only; no push, merge, deployment, or Web/TS edits. Independent backend validation and Spec/quality review remain the parent workflow's gates.

### Findings addressed

- **T07-R2-F1 — returned resource ref recovery:** `SaveBytes`' registered ref is bound to the server-issued Career source ID before the separate source-ref CAS. A lost claim or uncertain write/read response never triggers destructive release. Exact retry discovers an active catalog row by owner-scoped source ID, Tenant, original name, SHA-256 digest, size and persistent lifecycle, checks other owner bindings, and CAS-adopts it before parsing; no second save is needed. Terminal sources retain their active ref. Surplus exact candidates are only cleaned after a terminal state/read check; a failed Release/Delete leaves the active catalog row retryable. An unrecorded candidate on a failed source is first reflected as `cleanup_pending_*` in the source row, then retried and cleared after successful deletion. No physical path enters a Career source response or cleanup log.
- **T07-R2-F2 — model input privacy:** final model JSON includes only known safe manual keys and deterministic server-extracted per-item keys allowed for the requested purpose. Arbitrary category-prefixed document fields are dropped. Remaining values redact contiguous or space/hyphen-separated long IDs and passport-like numbers; ordinary school, year, and quantified achievement facts remain.
- **T07-R2-F3 — cleanup availability:** failed old-resource Release/Delete remains pending and logged by opaque source ID/stage. Source history still returns HTTP 200 and a separate new upload progresses. Failure in the new upload's own persistence remains an unknown outcome rather than being hidden as a background cleanup failure.

### RED/GREEN evidence

- `TestModelInputExcludesNestedDocumentFieldsAndSeparatedNumbers`: RED final JSON included `education.passport_number` and `AB-12345678`; GREEN after safe-key selection and separated-number redaction.
- `TestFailedUploadRetainsReferenceUntilReleaseAndDeleteSucceed`: RED source GET returned 500 while Release failed; GREEN source GET stayed 200 through Release/Delete failures, retained pending ref, and an unrelated upload saved/progressed.
- `TestUploadRetryAdoptsCatalogedRefAfterClaimLoss`: RED exact retry returned 202 after a second SaveBytes call; GREEN exact retry returned 201 using the first cataloged ref with one save total.
- `TestStaleSourceWithUnrecordedCatalogRefShowsCleanupPending`: RED source row showed only `interrupted`; GREEN it exposes `cleanup_pending_interrupted`, then clears to `interrupted` after a later successful cleanup.
- Added deterministic coverage for a committed ref update with lost response and failed follow-up read (HTTP 504, no deletion), other Tenant/wrong name/digest/size rejection, ready-source active-ref preservation, foreign binding preservation, and retry after failed physical deletion.

### Verification

- `go test -count=1 ./internal/modules/career/...` — passed after Task 1 and again after Task 2.
- `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` — passed on code HEAD, including career, router, database, handler/dto/session, and container packages. Container emitted the existing macOS linker warning `ignoring duplicate libraries: '-lc++'`.
- `git diff --check` and `git diff --cached --check` — passed with no output before the code commit.
- No migration was introduced, so no schema-reopen check was needed beyond the repository's existing database tests.

### Residual

The generic lower-level window between a physical SaveBytes write and its catalog registration remains an infrastructure limitation. This round covers registered/returned refs, including refs that Career could not persist because of claim takeover, Release failure, or an unknown DB result. No live PostgreSQL migration run was performed; this round adds no migration.

Supplementary whole-repository check: `go test -count=1 ./...` completed with exit 1. The only failed package was `tools/architectureguard`: `TestGuardCleanAtHead` reported 10 manifest/route-coverage violations (three workbench files and seven `internal/router/routes_career.go` route lines), and `TestDiscoverRealRepoRouteTotals` observed 644 routes versus its 633 baseline. These paths are outside the R3 code diff, which touches only `internal/modules/career/**`; the architectureguard baseline was not changed under this backend task. All other package results in that run were passing or had no test files. This supplementary failure is disclosed and is not represented as a passing whole-repository suite.

## Review fix round 4 (R3 findings)

Fix code BASE: `53b30d2472fa80c6d7598ce2812f54791fd43511` (report-only HEAD `b5644f3f3c123f9acf752de8f5ab3716bbe395dd`).
Fix code HEAD: `8efd70dc0e188159eb2fa80592ad93e997cd30eb` (`fix(career): guard catalog cleanup against new bindings`). Local code commit only; no push, merge, deployment, Web/TS edit, or architectureguard manifest edit. Independent validation and Spec/quality Review remain required before integration.

### Findings addressed

- **T07-R3-F1 — current Web fact keys:** the explicit safe-key taxonomy now maps the four T03 Web keys `毕业时间` and `学历` to education, and `城市` and `意向` to preference. All three current model purposes retain these confirmed facts. Arbitrary category-prefixed identity fields, including `education.passport_number`, remain excluded; separated document numbers remain redacted.
- **T07-R3-F2 — Bind/delete race:** a narrow optional `UnboundResourceDeleter` on the catalog-backed FileService claims an active, zero-binding resource as `deleting` before physical deletion. Repository `CreateBinding` and the deletion claim both reserve/lock the same resource row before inspecting state/bindings (SQLite write reservation, PostgreSQL row lock through the update). Bind after the claim returns typed `ErrResourceUnavailable`; a binding committed first causes the claim to decline deletion. The decorator uses the private physical locator from the claim, never normal active-only `Resolve`, and conditionally marks the resource `deleted` after physical success. A failed physical deletion leaves `deleting` and makes its lease retryable; a second cleaner cannot enter physical deletion while a fresh claim is active. Career release and surplus cleanup use this guarded path for catalog refs, and cleanup discovers `deleting` candidates for retry while upload adoption still only considers active candidates. Ordinary FileService/DeleteFile and ResourceCatalog.Release contracts were not changed.

### RED/GREEN evidence

- `go test -count=1 ./internal/modules/career -run TestWebConfirmedFactsReachRelevantModelPurposes`: RED final JSON had `facts:[]` for each of profile summary, qualification evaluation, and material generation; GREEN after four explicit key mappings.
- `go test -count=1 ./internal/modules/career -run TestCatalogCleanupDoesNotDeleteAfterForeignBindAtReleaseBoundary`: RED a foreign owner bound after Release/count but `files.deleted` was still the surplus ref; GREEN the guarded path preserved active bytes.
- `go test -count=1 ./internal/application/service/file -run 'TestGuardedDelete'`: RED the catalog-backed file decorator did not expose guarded deletion; GREEN real SQLite catalog/repository tests covered foreign Bind first, deletion claim first with typed Bind rejection, second cleaner exclusion, wrong Tenant, physical failure and later retry.
- `go test -count=1 ./internal/modules/career -run TestCatalogCleanupRetriesDeletingCandidateAfterPhysicalFailure`: RED Career did not discover a `deleting` surplus row; GREEN it retries through the guarded file method. Existing ready-ref preservation and source cleanup tests remain passing.

### Verification

- `go test -count=1 ./internal/modules/career/...` — passed after the Web mapping task.
- `go test -count=1 ./internal/modules/career/... ./internal/application/repository/... ./internal/application/service/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` — passed on code HEAD. Repository and service packages took 127.103s and 95.522s respectively; container emitted the existing macOS duplicate `-lc++` linker warning but passed.
- `go test -race -count=1 ./internal/application/service/file -run 'TestGuardedDelete'` — passed.
- `git diff --check` and staged diff check — passed before the code commit.
- No migration was introduced: the existing resource state column accepts `deleting`. The SQLite tests use the real repository and decorated FileService. No live PostgreSQL concurrency run was available.

### Residual and scope

The generic physical-write-before-catalog-registration crash window remains the previously documented infrastructure limitation. A `deleting` claim is retried after a one-minute stale lease if its cleaner crashes; physical failure explicitly makes the same state retryable sooner. A provider-specific delete error after the bytes are already absent may require provider-specific not-found classification before automated cleanup can finish; binding remains blocked while the row is `deleting`, so this is a cleanup availability limit rather than permission to delete a newly bound resource. The repository-wide architectureguard baseline failures recorded in R3 remain outside this backend-owned fix and were not represented as passing by this targeted suite.

## Review fix round 5 (R4 finding)

Fix code BASE: `8efd70dc0e188159eb2fa80592ad93e997cd30eb` (report-only HEAD `5c36749a3f2e8d7dc757e6e62fc57b09ad2475de`). Fix code HEAD: `b1dd2ca46dc34ffbf3f44ee493deec6ba3ed3303` (`fix(career): finish cleanup for deleted catalog resources`). Local code commit only; no push, merge, deployment, Web/TS edit, or architectureguard manifest edit. Independent validation and Spec/quality Review remain required before integration.

### Finding addressed

- **T07-R4-F1 — retry after source-row clear failure:** The guarded file decorator now treats a catalog resource as already cleaned only after a tenant-scoped, exact-handle, terminal `deleted` lookup. The repository lookup uses GORM `Unscoped` because successful physical deletion soft-deletes the row. Career's existing `UploadAdapter.Release` can therefore return success after its normal active-only catalog Release fails on retry, allowing the conditional `ClearSourceResource` update to remove the pending ref. Foreign Tenant, unknown handle, active/in-use resource, and DB error do not produce a deleted outcome. The physical deletion claim and Bind exclusion remain unchanged.

### RED/GREEN evidence

- `go test -count=1 ./internal/modules/career -run TestCareerReconcileClearsSourceAfterCatalogAlreadyDeleted -v`: RED second reconciliation retained `cleanup_pending_parse_failed` after the real catalog-backed physical file was deleted and the injected Career source clear failed. GREEN second reconciliation clears `ResourceRef`, retains `failed` status, and restores `parse_failed`; the test also checks the file is absent and the catalog tombstone is `deleted` after the first pass.
- `go test -race -count=1 ./internal/application/service/file -run 'TestGuardedDelete'`: passed. These real SQLite tests cover one physical deletion across two cleaners, a foreign binding before claim, Bind rejection after claim, failed physical deletion then retry, same-Tenant terminal idempotence without repeated deletion, wrong Tenant, unknown handle, active bound resource, and unavailable DB state.

### Verification

- `go test -count=1 ./internal/modules/career/... ./internal/application/repository/... ./internal/application/service/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` — passed, including Career, repository, service, file, router, database, handler/dto/session, and container. Repository took 130.392s and service took 101.776s. Container emitted the existing macOS linker warning `ignoring duplicate libraries: '-lc++'` but passed.
- `git diff --check` and staged diff check — passed with no output before the code commit.
- No migration was introduced. The existing resource state and soft-delete columns represent the terminal tombstone.

### Residual

The generic physical-write-before-catalog-registration crash window remains an infrastructure limitation outside returned Career refs. A provider-specific delete error after bytes are already absent can still leave a `deleting` claim until the provider reports not-found or a retry succeeds. No live PostgreSQL concurrency run was available; the same-row guard was exercised with SQLite and the focused file test passed under Go's race detector. Independent validation and Review are still pending.
