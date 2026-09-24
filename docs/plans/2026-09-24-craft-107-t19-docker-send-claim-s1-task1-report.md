# T19 Docker Send Claim S1 Task 1 Report

## Result

Implemented the S1 persistence slice. The journal now stores an immutable Docker exec receipt and a one-owner send claim. `ClaimDockerExecSend` returns `true` only for the single conditional update that records the claim; exact replay returns `false, nil`, and stale/mismatched operation identity returns `craft.ErrConflict`. Repository state remains `intent` after claiming, so the unresolved Run fence and existing dispatched reservation remain in place.

No Docker `ExecStart` was invoked. This report covers the repository prerequisite only and makes no claim about full T19 recovery.

## Changed files

- `migrations/versioned/000196_craft_docker_exec_send_claim.up.sql`
- `migrations/versioned/000196_craft_docker_exec_send_claim.down.sql`
- `migrations/sqlite/000117_craft_docker_exec_send_claim.up.sql`
- `migrations/sqlite/000117_craft_docker_exec_send_claim.down.sql`
- `internal/application/repository/craft_docker_send_claim.go`
- `internal/application/repository/craft_docker_send_claim_test.go`
- `docs/plans/2026-09-24-craft-107-t19-docker-send-claim-s1-task1-checkpoint.json` and its `.patch` / `.snapshot/` files
- This report

Migration paths are ignored by the repository-wide `migrations/` ignore rule. Their exact content is included in the checkpoint patch and snapshot and their SHA-256 values are in the checkpoint manifest.

## Behavior and constraints

- PostgreSQL enforces the all-null or complete Docker receipt shape, requires a receipt for a claim, and uniquely indexes `(provider, container_id, exec_id)` for populated rows.
- SQLite enforces the same shape on inserts and updates with triggers and uses the same partial unique index.
- Binding is a conditional update on tenant, Run, activity key, `state='intent'`, and prepared `run_revision`; only an identical receipt can replay successfully.
- Claim is a single conditional update on that full identity, exact receipt, unresolved state, matching revision, and null claim timestamp. There are no process locks or claim reset methods.
- Claim and replay do not alter journal resolution state or the reservation hold.
- Down/up migration tests retain an existing journal row. SQLite 3.54.0 was present and supports the down migration's `DROP COLUMN` statements.

## Verification evidence

Commands run in `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`:

| Command | Result |
| --- | --- |
| `go test ./internal/application/repository -run '^TestCraftDockerSendClaim' -count=1` | PASS |
| `go test ./internal/application/repository -run '^TestCraftDockerSendClaim|TestCraftCharge' -count=1` | PASS |
| `go test ./internal/application/repository -run '^TestCraftDockerSendClaimConcurrentOwnerRace$' -count=10` | PASS, all 10 race runs |
| `git diff --no-index --check /dev/null <each of the six owned implementation files>` | PASS, clean output |
| `gofmt -d internal/application/repository/craft_docker_send_claim.go internal/application/repository/craft_docker_send_claim_test.go` | PASS, no output |

The first RED run failed because the new key, receipt, repository constructor, and methods did not yet exist; after implementation the focused tests passed. Test coverage includes partial receipt and orphan claim rejection, duplicate receipt identity, exact and divergent rebinding, stale revision, wrong tenant/Run, single claim, separate-connection race, reconstructed repository replay, claim write failure, reservation preservation, unresolved `intent`, and SQLite down/up row retention.

Migration tips were rechecked immediately before writing: PostgreSQL `000195`, SQLite `000116`; target files did not exist. The focused tests apply the new migration through the repository's real full migration streams, then execute the owned down/up SQL against the migrated tables. The PostgreSQL migration subtest was skipped because `TRPC_TEST_POSTGRES_DSN` is unset; PostgreSQL execution, constraints, and down/up therefore remain unverified in this environment.

## Checkpoint

- Base HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- All six owned implementation files had absent preimages.
- Exact content and hashes are recorded in `2026-09-24-craft-107-t19-docker-send-claim-s1-task1-checkpoint.json`.
- The unified addition patch includes all six files, including ignored SQL migrations. Patch SHA-256 is recorded in the manifest.
- No commit was created.

## Remaining risks

- PostgreSQL acceptance is unverified until the migration and behavior tests are run with `TRPC_TEST_POSTGRES_DSN` configured.
- The durable claim proves exclusive permission was consumed before send; S1 intentionally does not perform the network send or reconcile a claimed row after a crash.
