# Task 2R2 independent spec and quality review

**Scope:** `ed6cf08a3ef6986322bc01681dc797e086efb180..6075c5ecb863bda60e3af0d1db7a2c0cdcfe2fd2`, clean source worktree at review start. Read the approved Career spec, ADR 0015–0019, `CONTEXT.md`, Task 2R2 brief, implementation report, independent backend validation, and changed source/tests. This review is read-only; OCR was not invoked.

## Verdict

- **Spec compliance: conditional / not yet complete.** The implemented issuer and downloader bind signed claims to tenant, owner, resource, immutable version and digest, and recheck live binding before opening bytes. Tests cover real fixture download and common denials. There is no production caller of `BindVersion` yet, so a Career material cannot traverse this flow until Task 5 publishes its exact version. The accepted #142 lifecycle should not be marked end-to-end complete before that integration.
- **Code quality: changes requested.** The two medium findings below concern owner authorization at the publisher seam and bounded use of temporary storage by repeatable public downloads. No immediate HMAC bypass or legacy Workbench regression was found in the reviewed diff. The validator's targeted Go tests passed; live PostgreSQL migration application and full production router assembly remain unverified.

## Findings

### R2R2-1 — Medium — BindVersion accepts any ready version in the tenant

**Evidence:** `internal/modules/career/repository/artifacts.go:66–88,150–164`. `BindVersion` receives an owner `Scope` and a `versionID`, but `readReadyVersion` filters only `tenant_id`, `id`, and `scan_state`. It reads `session_id` yet never checks that the artifact's Task/session belongs to this owner or to the Career resource being published. The new binding row then asserts the supplied `owner_id` as authority. `career_artifact_bindings` has a tenant/version FK but no owner or Task FK. Tests use one owner and a version with an unconstrained `session_id`.

**Impact:** When Task 5 invokes this trusted publisher hook, passing a same-tenant version ID from a different member/Task would create a valid owner binding and signed public download, bypassing the existing Task ownership boundary. The approved spec requires server-side User/Tenant/Task permission checks and a private single-member Career space. This defect is latent while no production caller exists, but the exported seam makes the wrong binding possible.

**Smallest correction:** Make publication resolve the Career resource's authorized Task/session and require the ready artifact version's `session_id` (and tenant) to match it in the same transaction, or accept a verified owner/Task authorization port and fail closed on mismatch. Add a same-tenant foreign Task version test. Task 5 must consume this checked publisher seam.

### R2R2-2 — Medium — Each public redemption can stage up to 8 GiB on disk

**Evidence:** `internal/handler/session/career_artifacts.go:110–159`. A valid five-minute URL can be replayed without authentication. Each request creates a distinct temporary file and copies up to `version.Size+1` bytes; the accepted size is as high as `8<<30`. There is no concurrent staging limit, per-tenant quota, or smaller artifact-specific cap. The file is cleaned on normal handler exit, but concurrent or slow requests retain their allocations until completion. Existing tests use only tiny fixtures.

**Impact:** A leaked or repeatedly used valid link can exhaust the server's temp volume and disrupt unrelated requests. The digest-before-success requirement makes staging reasonable, but it needs a bounded resource budget.

**Smallest correction:** Enforce a realistic Career artifact size ceiling and a process-wide or tenant-scoped semaphore/byte budget before opening the blob, with a test that excess concurrent requests fail closed and leave no temp files. Keep digest verification before writing a success response.

## Additional evidence and limits

- `CareerArtifactHandler.Download` verifies signature/expiry and live catalog state before tenant storage resolution or `GetFile`; denied paths use empty 404 responses. It stages and hashes bytes before `filetransport.Serve`, so a corrupt object does not produce a successful body. Temp file close/remove defer order is correct.
- The public GET is registered before global Auth; POST issuance is added to the authenticated v1 group. The production DI provider is registered, but the router test constructs its own Gin engine and handler rather than resolving `NewRouter` through the production container. A full assembly check remains useful before integration; this is an evidence gap, not a demonstrated route defect.
- The SQLite and versioned migration shapes include composite tenant/owner/resource/version identity and tenant/version FK. SQLite GORM tests pass. PostgreSQL DDL was not applied to a live server in this task.
- Legacy Task grant signing and W26 route code are unchanged; targeted regression tests in the validation report passed. The shared `externalURLBase` uses request host/forwarding headers, which was pre-existing behavior and should be assessed at deployment/proxy boundary.
