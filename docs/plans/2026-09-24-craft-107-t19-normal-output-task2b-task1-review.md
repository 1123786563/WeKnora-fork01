# T19 normal output Task2b Task1 — independent review

## Scope and evidence

Reviewed the Task2b plan, Task1 report/checkpoint/patch, T19 normal-output architecture and implementation plan, the approved Craft artifact scope, relevant `CONTEXT.md` and ADR constraints, and the previously reviewed provider/output seams. This is a repository-stage and receipt review only; it does not approve coordinator integration, a physical Docker start, or release.

The task-local patch SHA-256 is `a573404cf04fc6a7c73a0a99a388b218e3f71e6ddad0ab8ee40e471f6efdbc98`. I reconstructed the two prior S2 Fix1 claim files from the checkpoint snapshot, applied the patch in a temporary directory, and compared all eight resulting files byte for byte with both the checkpoint postimage SHA-256 manifest and the integration worktree. This includes ignored SQLite 125 and versioned PostgreSQL 204 up/down migrations. All eight matched. The claim test file is byte-identical to its preimage. The report records focused and race tests passing on SQLite and isolated PostgreSQL, including migration up/down/up; I inspected those saved results but did not repeat the controller's test run. The isolated PostgreSQL schema does not establish a full migration-chain pass.

## Findings

### T19-NI-1 — High — legacy receipt bind can cross the stage boundary

**Evidence:** `internal/application/repository/craft_docker_send_claim.go:57-71` checks `normalInputExists` before opening the transaction and taking the Run lock. `Stage` takes that same lock and inserts the normal-input row at `internal/application/repository/craft_docker_normal_input.go:111-155`. Thus a legacy `BindDockerExecReceipt` can observe no row, pause, allow `Stage` to commit, then acquire the Run lock and bind only the old three-field journal receipt. The journal UPDATE has no `NOT EXISTS` guard for `craft_docker_normal_inputs`. SQLite 125 and PostgreSQL 204 validate the stage INSERT and stage receipt UPDATE, but have no journal-side guard against this legacy bind. The sequential test at `craft_docker_normal_input_test.go:99-103` cannot cover the interleaving.

**Impact:** The newly staged command/stdin/timeout can coexist with a journal receipt authorized through the old three-field path. A later normal bind to a different receipt conflicts, leaving the held operation stranded; a three-field bind has also bypassed the promised complete-receipt boundary. This violates Task1's explicit legacy-bypass exclusion and immutable exact receipt requirement. The equivalent pre-lock check in legacy `ClaimDockerExecSend` (`craft_docker_send_claim.go:114-128`) should be corrected with the bind path; its independent exploitability is narrower because Stage requires an unbound journal.

**Smallest correction:** Move the stage-existence check inside each legacy transaction, immediately after the Run lock and before journal mutation. Include a deterministic interleaving test in both database modes; a journal-side invariant would add defense against other callers.

### T19-NI-2 — Medium — encrypted create request has no aggregate byte bound

**Evidence:** `internal/application/repository/craft_docker_normal_input.go:87-110,234-251` marshals and encrypts the whole request before any aggregate-size check. `validCraftDockerNormalInput` caps stdin at 1 MiB and checks individual validity, but does not bound command argument count/bytes, environment count/bytes, user, working directory, or the canonical JSON/ciphertext length. The new migrations likewise have no ciphertext length bound.

**Impact:** A caller can allocate and persist an arbitrarily large encrypted create request and replay it through decryption, violating the plan's bounded staged-blob requirement and creating memory/database pressure before Docker creation.

**Smallest correction:** Define one explicit maximum for canonical request bytes, check it before encryption and on read, and constrain ciphertext storage to a compatible bound. Test oversized command/environment and exact boundary behavior.

## Scoped verdicts

- **Spec compliance: FAIL for Task1.** Encryption with a valid 32-byte `SYSTEM_AES_KEY` fails closed when absent or invalid; stored request digest, scope, stdin identity, receipt metadata, current Run/revision checks, and create-only replay are substantively implemented. The legacy bind interleaving breaks the required complete-receipt boundary, and the staged request lacks an aggregate byte limit.
- **Code quality: CHANGES REQUIRED.** The checked patch is reconstructible and focused, with useful SQLite/PostgreSQL tests. The Run-lock placement in the legacy methods is a concurrency defect, and the tests omit that interleaving and oversized non-stdin request coverage. The two-step journal/full-receipt write remains recoverable only through replay and should be exercised at its crash boundary during the follow-on coordinator task.
