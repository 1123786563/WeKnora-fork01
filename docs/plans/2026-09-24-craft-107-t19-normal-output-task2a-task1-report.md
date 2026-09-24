# T19 Normal Output Task2a Task1 Report

## Scope

Implemented the durable output operation store and service adapter for the reviewed one-attach Docker output provider. The operation is bound to tenant, persisted Task/session ID, Run, activity, and the exact already-claimed Docker `(container, exec)` receipt. No production routing, Docker invocation, S2/Run repository changes, or commits were made.

## Behavior delivered

- SQLite `000124` and PostgreSQL `000203` migrations add operation and chunk tables, receipt/task scope checks, immutable identity triggers, foreign-key retention, and a unique per-operation chunk sequence.
- Repository `Open` validates the persisted Run/task and exact durable S2 Docker send claim before opening the output operation.
- `Append` serializes writers with `Seal`, commits chunk bytes, stream, digest, length and monotonic sequence transactionally, and checks cancellation before commit. `AppendSequence` accepts byte-identical replay and rejects conflicting replay/gaps.
- Output quota is persisted per operation; overflow stores only the allowed prefix and returns explicit `ErrCraftDockerOutputQuota` with partial state.
- `Seal` uses the same durable operation-row serialization as append. Late writes fail. `ReadAfter` reads committed chunks with bounded page size, validates sequence/digest/length, and reports explicit unavailable/corrupt/gap/cursor errors.
- The service sink checks exact provider receipt, admits no caller callback, cancels in-flight append contexts on seal, and exposes a separate cursor reader. A sealed output remains partial until another component supplies terminal process/stream-completeness evidence; the store does not claim completion on empty output.

## Verification

Focused repository and service tests passed, including SQLite durability/reopen, append ordering/concurrency, replay conflict/idempotency, exact scope, quota, seal/late append, integrity/gap/cursor, append failure, and service blocked-append cancellation/receipt binding. Focused race runs also passed. Isolated in-memory SQLite migration up/down/up x2 passed. Full command output is in `2026-09-24-craft-107-t19-normal-output-task2a-task1-test-output.txt`.

Checkpoint file records the exact HEAD, eight owned file SHA-256 values, patch hash, and replay check. Applying the task-local patch into an empty temporary directory reproduced all eight postimage hashes. Migration files match the reserved numbers but are hidden by `.gitignore`'s `migrations/` rule; they are explicitly included in the patch.

## Constraints and remaining verification

- PostgreSQL tests were not executed because `TRPC_TEST_POSTGRES_DSN` is unset. PostgreSQL migration/runtime behavior is NOT VERIFIED.
- This task does not wire production routing or prove provider-to-store behavior in a live Docker run; those remain later plan tasks.
- Output remains explicitly partial after sealing because this store alone cannot attest exact process termination or a complete transport stream.
