# T19 Normal Output Task2a Fix1 Task1 Report

## Scope and result

Fixed the reviewed seal-result and truncated replay defects in the normal-output store path. Owned files are the repository/service output files, provider normal-output files, and SQLite 124/PostgreSQL 203 migration pairs. No S2/Run repository, production route, or Docker execution was changed.

- Provider sink `Seal` now returns an error. The attached Exec remains partial and returns the seal error when durable cutoff cannot be confirmed; the provider does not retry or attach again.
- The service closes local append admission and cancels in-flight appends before sealing. It returns seal failure to the provider and allows a later bounded `Seal` call to retry the durable marker. It does not report durable success until repository seal succeeds.
- Repository writer `Open` is create-only for an exact receipt. A second process cannot reopen an existing operation after an uncertain seal. Service `ReadAfter(scope, ...)` reads the persisted cursor independently without minting a writer.
- Every append attempt stores original input length and SHA-256 separately from the persisted prefix and records whether quota truncated that attempt. An over-quota attempt with a zero-byte stored prefix still owns its sequence, stream, digest, and length. Exact same-sequence replay returns the same quota result; changed input conflicts.
- SQLite/PG chunk schemas contain the original-input metadata and length constraint. The four migration files are ignored by `.gitignore`'s `migrations/` rule and are explicitly carried in the task-local patch/checkpoint.

## Verification

Focused repository, service, and provider tests passed, including all three focused race runs. A separate in-memory SQLite check passed migration up/down/up twice and verified exact claimed-receipt validation plus input metadata length constraints. Test output includes the expected provider RED compile failure and two intermediate fixture/storage failures with their root causes, followed by passing final commands.

The incremental patch was built from verified prior postimages: Task2a checkpoint for the eight output repository/service/migration files and provider Fix3 checkpoint for the two provider files. Reapplying the incremental patch to those reconstructed preimages reproduced all ten final hashes. Checkpoint records exact pre/post hashes and artifact hashes.

## Remaining limits

- PostgreSQL tests/migration execution were not run because `TRPC_TEST_POSTGRES_DSN` is unset; that path remains unverified.
- No live Docker run or production routing was done, as scoped by this task. The physical one-attach protocol is unchanged.
- This work guarantees output persistence cutoff and explicit partial outcome; it does not independently prove a complete Docker stream or terminal process evidence.
- Fix1 plan text says “eight-file preimage,” while its explicit owned-file list resolves to ten files. Checkpoint covers all ten listed files.
