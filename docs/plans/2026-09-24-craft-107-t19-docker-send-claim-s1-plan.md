# T19 Docker send claim S1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist a Docker exec receipt and one irreversible, exclusive send claim for each existing charge-start journal operation.

**Architecture:** Extend `craft_charge_start_journal` additively. A focused repository component binds an immutable Docker receipt and atomically changes a prepared row to claimed before any caller may invoke `ExecStart`. This slice exposes persistence only; the later coordinator owns transport and resolution.

**Tech Stack:** Go, GORM, SQLite, PostgreSQL SQL migrations.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md` (§Task Budget and recovery); design `docs/plans/2026-09-23-craft-107-t19-docker-exec-recovery-design.md`; parent issue #107 / T19 issue #138.

## Global Constraints

- `CONTEXT.md` Task Budget: spending is bounded by the Task authorization; exhaustion pauses the Run durably.
- ADR `docs/adr/0004-task-is-session.md`: keep Run/Task ownership and recovery identity stable.
- Existing journal migration `000191` / `000112` and reviewed StartBinding journal/hold are prerequisites. Preserve `intent|unknown` as unresolved Run fences; `started` means resolved outcome, not permission to send.
- Commit the hold before `ExecCreate`, the receipt before the claim, and the claim before `ExecStart`. Never hold a SQL transaction across Docker I/O.
- A claimed row is observation only on replay. `Inspect Running=false`, `ExitCode=0`, and HTTP 404 do not authorize another send or a new exec ID.
- S1 does not assert full T19 recovery, output capture, daemon restart liveness, or provider support beyond Docker.

## Review Focus

1. Two workers claiming one prepared row: exactly one gets permission to send (Task 1 race test).
2. Crash after claim but before physical send: replay stays claimed and cannot resend (Task 1 persistence test).
3. Different receipt for the same operation or same receipt for a different operation: reject without changing the first binding (Task 1 identity tests).
4. Failed bind/claim write and stale Run revision: no permission to send; original `intent`/hold and fence remain (Task 1 rollback tests).
5. `Inspect` false/zero/404 and daemon-lost receipt: repository never translates observations into pre-send authority (Task 1 state test; coordinator later owns observation).

---

## File structure and ownership

S1 exclusively owns four new files `migrations/versioned/000196_craft_docker_exec_send_claim.{up,down}.sql`, `migrations/sqlite/000117_craft_docker_exec_send_claim.{up,down}.sql`, and two new Go files `internal/application/repository/craft_docker_send_claim.go` and `internal/application/repository/craft_docker_send_claim_test.go`. The migration version is based on inspected tips PG `000195` and SQLite `000116` on 2026-09-24; recheck immediately before dispatch, and amend this plan if either tip has advanced. `migrations/` is ignored by Git: capture its exact content and hashes in the SDD checkpoint.

Do not edit `internal/application/service/craft_budget.go`, `agent_run.go`, their tests, sandbox files, or the active S2/H1 write sets. The existing service `CraftChargeStartJournalRow` can continue reading the original columns; the repository component uses explicit SQL columns and its own receipt DTO. S2 may consume the new API only after S1 review/checkpoint integration.

### Task 1: Add immutable receipt and one-owner send claim

**Depends on:** reviewed start-journal migration and existing intent/hold commit; no dependency on active S2/H1 implementation. **Role:** `backend_implementer`; `backend_validator` and independent `reviewer` after implementation. **Status on handoff:** ready if file ownership above remains disjoint and migration tips remain as inspected.

**Files:** the six S1-owned files listed above.

**Interfaces:**

- Consumes: committed `craft_charge_start_journal` row keyed by `(tenantID uint64, runID string, activityKey string)`, `state='intent'`, `run_revision` as the prepared Run epoch.
- Produces: `DockerExecReceipt{Provider string, ContainerID string, ExecID string}`; `BindDockerExecReceipt(ctx context.Context, key CraftChargeStartKey, expectedRunRevision int64, receipt DockerExecReceipt) error`; `ClaimDockerExecSend(ctx context.Context, key CraftChargeStartKey, expectedRunRevision int64, receipt DockerExecReceipt) (claimed bool, err error)` on a `CraftDockerSendClaimRepository` constructed from `*gorm.DB`. Define `CraftChargeStartKey{TenantID uint64, RunID, ActivityKey string}` locally in the new file. `claimed=true` is the sole permission token for one physical `ExecStart`; `false` means no send and must be accompanied by a typed conflict or persisted-state result documented by implementation. No public method clears the claim or rebinds a receipt.
- Persistence: nullable `provider`, `container_id`, `exec_id`, `send_claimed_at` columns on the existing journal. `provider='docker'` only for this slice. All three receipt columns are either all null or all nonempty, with `send_claimed_at` requiring a complete receipt; unique `(provider, container_id, exec_id)` for bound rows. SQL constraints should reject partial receipt and duplicate IDs; repository CAS prevents overwriting a complete receipt. Do not change the existing four `state` values or unresolved scan predicate.

- [ ] **Step 1 — RED schema tests:** In `craft_docker_send_claim_test.go`, create a committed journal fixture with its Run/reservation. Assert the new receipt columns exist after migration; partial receipt fails; a duplicate provider/container/exec triple across two operation keys fails; same-key divergent receipt is rejected. Run `go test ./internal/application/repository -run '^TestCraftDockerSendClaim' -count=1`; expected failure before migration/repository methods exist.
- [ ] **Step 2 — Additive migrations:** Add the four migration files with `ALTER TABLE` columns and unique index. PostgreSQL can use a table `CHECK` for all-null/all-populated and claim implication. SQLite `ALTER TABLE ADD COLUMN` cannot add a table check; add focused insert/update triggers or equivalent SQLite-supported enforcement, including old-row compatibility. Down migration must reverse only these new columns/indexes/triggers and retain journal rows; test down/reapply on disposable DBs. Before implementation, confirm the repo's migration runner permits SQLite `DROP COLUMN` and account for dependent index/trigger order.
- [ ] **Step 3 — RED repository tests:** Exercise bind idempotence for the exact same receipt, rejection of divergent rebinding, claim once, concurrent claim race using separate DB connections, replay after repository reconstruction, stale `run_revision`, wrong tenant/Run/key, and write failure. Assert losers never receive `claimed=true`, the journal remains `intent`, and an existing hold remains present. Add an explicit test that an observation payload equivalent to false/zero or 404 supplies no repository transition to resend.
- [ ] **Step 4 — GREEN repository:** Implement bind as one conditional `UPDATE` scoped by full operation key, `state='intent'`, expected `run_revision`, and null receipt, followed by exact-match read for idempotent repeat; a conflicting receipt returns conflict. Implement claim as one conditional `UPDATE ... SET send_claimed_at=<DB timestamp>` scoped by the same key/epoch, exact receipt, `state='intent'`, and `send_claimed_at IS NULL`; return `claimed=true` only for `RowsAffected==1`. Zero affected rows are never send permission. Do not use process locks as the authority. Keep DB operations short and independent of provider callbacks.
- [ ] **Step 5 — Verify:** Run focused tests on SQLite and a disposable PostgreSQL database (`TRPC_TEST_POSTGRES_DSN` when configured), including real concurrent connections; run migration up/down/reapply on both engines, `go test ./internal/application/repository -run '^TestCraftDockerSendClaim|TestCraftCharge' -count=1`, and `git diff --check`. Record exact commands/results, DB engine versions, migration hashes and all six file hashes in the SDD checkpoint. If PostgreSQL is unavailable, report it as an unverified risk; do not claim cross-engine completion.
- [ ] **Step 6 — Review gate:** Independent reviewer checks Spec compliance and quality against the exact checkpoint, especially SQL atomicity, SQLite uniqueness/trigger behavior, `run_revision` semantics, and absence of any path that resets claim. Backend validator independently reproduces crash/race and both DB migration behavior. Integrate the reviewed checkpoint before coordinator work consumes this API.

## Interface and task consistency preflight

S1 writes only its six files; S2/H1 may edit current service and runtime files concurrently, but neither may alter S1 migrations or repository API until review. The binding identity is tenant + Run + activity key + expected Run revision + exact Docker receipt. `send_claimed_at` is independent of `state`: the unresolved Run scan still sees claimed `intent` rows. If the Run revision can legitimately advance between prepare and bind, S2 must pass the prepared journal revision and apply its own current Run fence before use; S1 must not silently relax the epoch predicate. Transport invocation and outcome resolution are outside this task.

## Coverage and dispatch decision

This plan covers only the S1 persistence prerequisite of the Docker recovery design. It does not cover coordinator prepare/resolve, detached start/output semantics, production wiring, or reconciliation. S1 can dispatch now independently of active S2/H1 only if the stated write-set separation is retained and the migration tips still end at PG `000195` and SQLite `000116`. Its checkpoint must be reviewed before any downstream send path uses `claimed=true`.
