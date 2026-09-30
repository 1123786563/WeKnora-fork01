# Craft #107 R5 Task3b Task 1 — independent durable effect review

Date: 2026-09-24. Scope: the nine-file exact checkpoint for `docker_network_create` and `docker_probe` durable effects, approved Craft Spec, Task3b plan and existing R5 effect authority. No Docker/provider action or OCR was run.

## Verdicts

- **Spec compliance: FAIL on legacy-kind API isolation (R5-3B-1).** The new kinds require a canonical-looking lowercase SHA-256 digest and demonstrate one-shot replay, drift rejection, outcome and fence checks. However, `BeginEffectWithDigest` can also create a digest-bound row for an old kind, while its existing `BeginEffect` caller can only replay with the empty digest. That can strand an existing effect and violates the plan's promise to preserve old-kind behavior.
- **Code quality: CHANGES REQUIRED for R5-3B-1.** The database/migration work is narrow and the new-kind tests are useful, but the missing old-kind method-boundary test leaves this interoperability defect unchecked.
- **Physical R5 remains blocked.** Task 1 stores a caller-supplied digest; it cannot establish that future adapters hash the complete canonical network/probe request or that any Docker operation is one-send. Adapter, coordinator, DI and physical evidence remain later gates.

## Finding

### R5-3B-1 — Medium: digest method can consume a legacy-kind claim that legacy replay cannot match

**Affected symbol:** `internal/application/repository/craft_run_view_effect.go:137-183,203-225`; tests at `craft_run_view_effect_test.go:155-233` cover new kinds but not this cross-API path.

**Evidence:** `BeginEffectWithDigest` passes any `validCraftRunViewProviderEffect(kind)` to `beginEffect`. The validation requires a digest for the two new kinds but permits a nonempty valid digest for `docker_create`, `docker_start` or `opencode_create`. A first `BeginEffectWithDigest(task, generation, docker_create, <valid digest>)` therefore inserts a pending `docker_create` row with `maySend=true`. The existing admitted coordinator calls `BeginEffect` for that kind, supplying `requestDigest=""`; its replay sees the row and returns `ErrConflict` at the digest comparison. The durable one-shot row has been consumed by an API mode that existing callers cannot replay, with no physical send required.

**Impact:** An internal caller using the newly exposed interface for an old kind can fence the current generation's established Docker/OpenCode effect path. This is an avoidable availability and contract regression; no permission is duplicated, but the old effect cannot proceed or recover through its intended interface.

**Smallest correction:** Restrict `BeginEffectWithDigest` to the two digest-required kinds and reject a nonempty digest for old kinds before any transaction; preserve legacy `BeginEffect` for old kinds. Add a focused test that the wrong method does not insert an old-kind row and that the subsequent legacy claim still receives its first permission. If old-kind digests are intended, explicitly migrate all callers and document the compatibility rule instead.

## Positive checks and exact package

The preimage archive SHA-256 is `5fd06f2e4c190efc0e5ffadbb38d2303c3053dd2409468e20e520d226fa84df9`; the postimage archive is `3a191a89db0468136a2d8e12fe5a612b9a0ab1bfdac23c415242dab897bbf42b`; and the task-local patch is `9b80b2709c276c9f086be6ba276ff742a5162ec231a7deda5100863972321856`. All match the checkpoint. Applying the patch to the retained preimage reproduced **all nine** postimage files byte for byte and matched the current integration files, including ignored SQLite `000126` and PostgreSQL `000205` up/down bytes. The domain model file was already identical in the retained preimage and postimage; the reviewed delta is in repository/test, plan/report and migration files. This is the real patch, not an empty HEAD diff.

For the two new kinds, `BeginEffect` rejects the missing digest, `validRunViewEffectRequestDigest` accepts only 64 lowercase hex digits, and the transaction locks/validates the admitted Task, Run, generation and prior allocation intent before inserting the digest-bound row. Exact replay returns the same token with `maySend=false`; a changed digest conflicts. `FinishEffect` requires an exact token/digest and nonempty success receipt, is idempotent for identical outcomes, and leaves `unknown` unresolved so a replay cannot resend. The focused tests exercise these behaviors and stale Begin fence; they do not claim that a caller's hash is truly canonical.

SQLite `000126` rebuilds the table with new kinds/digest checks, copies old rows with empty digest, and recreates the unresolved index and composite Run/RunView FKs. Its down migration uses a guard to refuse when new-kind rows exist. PostgreSQL `000205` adds a nonnull defaulted digest and paired kind/digest constraints; down raises before dropping them if new-kind rows exist. The saved test evidence reports old-row preservation, rejection before migration, refusal with new rows, and up/down/up on SQLite and isolated PostgreSQL. The PG test advances only the effect migration in a private schema and does **not** prove the vector-dependent full migration chain or PostgreSQL claim concurrency. I did not duplicate the controller's database test runs.
