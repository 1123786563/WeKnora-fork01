# T19 normal-output Task2a Task1 independent review

Date: 2026-09-24. Scope: the eight owned postimage files in the Task1 checkpoint. Read-only review of implementation, approved Craft spec, `CONTEXT.md`, T19 architecture, plan and test evidence. No OCR run.

## Evidence and verdict

- **Spec compliance: FAIL pending the seal finding.** Tenant/Task/Run/activity/exact receipt checks, bounded committed cursor reads, quota partial state, and separation of sealed from complete are present. The sink cannot guarantee the provider's required durable cutoff when sealing fails.
- **Code quality: FAIL pending the seal finding.** Repository transaction ordering is sound in the successful path; the service state machine loses a failed seal and its result cannot reach the provider. Explicit sequence retry at quota also conflicts with its documented idempotence.
- Current files and task-local patch match all eight SHA-256 values in the checkpoint, including gitignored SQLite `000124` and PostgreSQL `000203` up/down SQL. Applying the patch to an empty temporary directory reproduced all eight hashes. Migration numbers follow SQLite 123 and PostgreSQL 202 without collision in this worktree.
- Recorded focused Go and race tests pass; SQLite isolated up/down/up x2 passes. PostgreSQL tests and migration/runtime behavior are **not verified** because `TRPC_TEST_POSTGRES_DSN` is unset. I did not duplicate those tests or OCR.

## Findings

### F1 — High — failed seal is irreversible in memory but not durable

**Evidence:** `internal/application/service/craft_docker_output.go:90-111` marks `s.sealed = true` before calling `store.Seal` with a two-second timeout. On a database error or timeout it stores `sealErr`, but every later `Seal()` returns immediately. `Seal()` has no error result. A concurrently called second `Seal()` also returns while the first is still in progress. The provider's `DockerNormalExecChunkSink` contract (`internal/modules/execution/sandbox/docker_normal_exec.go:96-103`) requires that, after Seal returns, no in-flight or future Append can mutate durable output. `OpenSink` can create another sink for the same still-unsealed operation, so a failed seal leaves the row writable and the provider has no way to distinguish it from a successful cutoff. The blocked-append service test uses a mock whose `Seal` always succeeds; repository tests exercise only successful seal.

**Impact:** A storage outage or lock timeout can produce an apparently sealed transport result while later appends from another sink/process still commit. The durable cursor may change after the provider's cutoff, and a recovered coordinator could treat incomplete or mutable output as final.

**Smallest correction:** Make the durable seal outcome observable to the caller/coordinator and never report a successful cutoff before the database marker commits. Preserve retryability after a failed or in-progress seal, with a bounded wait and explicit unavailable outcome if the cutoff cannot be committed. Add a test that forces `store.Seal` to fail/time out, reopens the operation, and checks that the failure cannot be mistaken for a durable seal; test concurrent `Seal` calls.

### F2 — Medium — quota truncation breaks byte-identical `AppendSequence` replay

**Evidence:** `internal/application/repository/craft_docker_output.go:230-253` stores only the remaining prefix of an over-quota input and returns `ErrCraftDockerOutputQuota`. `AppendSequence` replay at lines 208-216 compares the saved prefix and its digest to the full original chunk, so an exact retry of the same sequence and bytes returns `ErrCraftDockerOutputConflict`. At zero remaining bytes, lines 235-240 record truncation without any sequence or input fingerprint, so the same attempted sequence is not represented at all. The plan requires same-sequence byte-identical replay or conflict; the existing replay test covers only non-truncated chunks.

**Impact:** A caller retrying an acknowledged partial append cannot tell exact retry from divergent input, and may classify a deterministic quota result as identity corruption. This weakens the advertised idempotent append contract at precisely the partial-output boundary.

**Smallest correction:** Persist the attempted input digest/length (including a zero-stored-byte quota attempt) and compare that identity for explicit-sequence replay, while keeping stored prefix/byte quota separate; or narrow the API contract to reject all replay after truncation and make the caller handle that explicit state. Add exact/different retry tests for prefix truncation and zero remaining quota.

### F3 — Medium delivery gate — SQL migrations remain ignored by Git

**Evidence:** `.gitignore:96` ignores `migrations/`; `git check-ignore -v` confirms SQLite 124 and PostgreSQL 203 are ignored. The checkpoint patch includes all four SQL files and replay passes, but ordinary `git add`/diff inspection will omit them unless they are explicitly force-added.

**Impact:** Integration or a later commit can carry Go code without the required tables, causing runtime failure after deployment. This is a delivery/checkpoint issue, not a defect in the SQL text.

**Smallest correction:** During integration explicitly include and verify all four migration files in the final delivery diff/checkpoint (for example force-add if committing is authorized). Verify PostgreSQL up/down/up with a real test database before a complete verdict.

## Additional observations

- `ReadAfter` checks exact operation scope, uses a PostgreSQL repeatable-read transaction, validates contiguous sequence, byte count and digest, and bounds pages to 200. The store marks a sealed output `Partial`; it does not claim stream/process completeness.
- The operation FK to the send journal uses `ON DELETE RESTRICT`, and chunks cascade with operation deletion. This retains receipt evidence while output exists. A policy-driven purge path is not in this Task1 scope; later integration must align operation/chunk deletion with Run/Task retention.
- The repository's successful append and seal operations serialize on the operation row. The missing behavioral case is a real blocked database append racing a seal timeout/retry (the service mock only checks cooperative context cancellation). PostgreSQL race/migration evidence is still absent.
