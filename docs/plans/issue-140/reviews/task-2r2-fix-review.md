# Task 2R2 finding-fix independent review

**Reviewed revision:** `9c3deaa56c8db765955396d616b51482c660e43d` against `6075c5ecb863bda60e3af0d1db7a2c0cdcfe2fd2`. Read the original R2R2 review, fix report, validator report, Task 2R2 brief, approved Career spec, ADR 0015/0017, `CONTEXT.md`, relevant source and diff. This review did not invoke OCR or modify source.

## Verdict

- **R2R2-1: fixed for foreign-owner publication.** `BindVersion` now reads the ready version and checks the session's tenant, owner, and live state inside the binding transaction. The regression test changes a version to another owner's session and observes `ErrNotFound`; the matching owner's session binds successfully. `sessions.user_id` is the repository's Task owner authority.
- **R2R2-2: fixed for bounded temporary disk staging.** A Career version above 256 MiB is refused, and an admitted download reserves its declared size from a shared 512 MiB process budget before storage resolution or `GetFile`. The release is deferred after successful reservation and executes after file close/removal on every later exit, including digest rejection and response completion. `sync.Mutex` protects admission/accounting and `sync.Once` prevents double release. The targeted capacity-denial test and validator's race test passed.
- **Spec compliance: conditional. Code quality: changes requested.** The new finding below leaves deleted or reassigned Tasks downloadable through existing bindings. The Task 5 production publisher integration is also still pending, as already recorded in the initial review. Live PostgreSQL and full router assembly remain unverified evidence limits, not demonstrated defects.

## Finding

### R2R2-F1 — Medium — Redeeming a binding does not recheck the Task's current owner or deletion state

**Evidence:** `internal/modules/career/repository/artifacts.go:75–91` checks `sessions.user_id` and `deleted_at` only when binding. `Resolve` at lines 100–117, used by both grant issuance and public redemption, reads an active `career_artifact_bindings` row and a ready version but does not join/check the version's session. The normal `DeleteSession` path soft-deletes `sessions` (`internal/application/repository/session.go:441–447`) without revoking Career bindings. The existing tests cover explicit binding revocation/deletion, but not subsequent Task deletion or owner reassignment.

**Impact:** After a Task is deleted or its owner changes, an already issued five-minute URL can still download its bytes, and the old owner can obtain fresh URLs while the binding remains active. This violates the approved server-side User/Tenant/Task permission rule and the promised live authorization before blob access.

**Smallest correction:** In `Resolve`, require the version's current session to remain non-deleted and owned by the binding owner in the same tenant. Test bind → soft-delete session and bind → change `sessions.user_id`; issuance/redemption should return the same non-enumerating denial before any blob open. Keep the existing explicit binding revoke/delete checks.

## Other limits

- The process budget does not coordinate across replicas or provide per-owner fairness. Each replica can stage at most about 512 MiB of declared bytes. `Size+1` verification may temporarily exceed the declared reservation by one byte per admitted request. These are bounded follow-up capacity considerations, not reasons to reopen R2R2-2.
- The fix report has two Markdown hard-break lines that `git diff --check 6075c5e..9c3deaa` reports as trailing whitespace. This is documentation formatting only; the source diff has no whitespace error.
