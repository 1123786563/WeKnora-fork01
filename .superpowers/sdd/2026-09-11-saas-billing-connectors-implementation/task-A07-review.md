---
status: final
verdict: PASS
reviewer: independent-reviewer (A07, sync migration/cursor/pause; did NOT write this code)
commit-under-review: 0e30173 (implementation) + 6ac5118 (ledger row), worktree .worktrees/saas-billing-connectors
files-reviewed: internal/appconnector/sync.go (+209), sync_test.go (+183), internal/application/service/appconnector/sync_test.go (+216), internal/application/repository/datasource_repo.go (+72), internal/application/service/datasource_service.go (+171/−1), internal/datasource/scheduler.go (+30), 4 migration SQLs, task-A07-brief.md, ledger row
method: source-level evidence via git show + independently re-run targeted tests (one fast confirming run + explicit run of the 6 service-level tests the brief's own command does not cover). Full gate suite (vet/build/gofmt/diff-check/6-pkg regression) taken on coordinator evidence per review budget.
---

# Task A07 Independent Review — Feishu/Notion sync migration, cursor & pause

## X1 — Brief compliance: files, interfaces, Step-1 verbatim — PASS
- `git show 0e30173 --stat`: exactly the 9 brief-named code files (+923/−1); the single deletion is datasource_service.go:1234's one-line `streamSyncHandler` literal expanded in place — additive in effect. Migration numbering PG 000120 / SQLite 000040 matches the coordinator's deviation note in the brief (000119/000039 taken by A03's app_actions — verified by `ls migrations/versioned | tail`). Ledger row 6ac5118 touches only progress.md.
- `CanAdvanceCheckpoint` (sync.go:189-191) is character-for-character the brief Step-3 algorithm: `return contentCommitted && currentFence == workerFence`.
- Step-1 verbatim: sync_test.go:9-20 `TestSyncCheckpointWaitsForDurableContentAndFence` — same name, same three calls `(false,2,2)`/`(true,2,1)`/`(true,2,2)`, same fatal messages, modulo gofmt line-wrapping (the standard accepted in A05/A06 reviews). Passes.
- `SyncBinding{TenantID uint64; InstallationID, ConnectionID, DataSourceID string; AuthVersion int64}` (sync.go:80-87) — exact field set plus one additive field `RequiresReauthorization` (the brief's own 行为与边界 requires exactly this marker; not a violation).
- `ResolveSyncBinding` deviation (disclosed, non-blocking): brief Produces-line sketches `(context.Context,uint64,string)(SyncBinding,error)`; implementation is `(ctx, store SyncBindingStore, tenantID uint64, dataSourceID string, state *BindingState)` (sync.go:133-136). The 3-arg sketch cannot load anything without package-global state, so the extra params are the DI form of the same contract; name, tenant+dataSourceID keying, and `(SyncBinding, error)` return preserved; consumed correctly by the service (datasource_service.go:126) and both test files.

## X2 — Migrations — PASS
- PG 000120 up: `app_datasource_bindings(id PK, tenant_id BIGINT, datasource_id, installation_id, connection_id, auth_version BIGINT DEFAULT 1, requires_reauthorization, timestamps)` with `UNIQUE (tenant_id, datasource_id)` + 2 indexes. SQLite 000040 up: identical logical shape (INTEGER/BIGINT, BOOLEAN vs 0, DATETIME vs TIMESTAMP — dialect-appropriate only). All five brief-named columns present.
- Down files are pure reverse DROP (indexes then table), no data mutation, no DML.
- No existing migration modified — the commit's migration entries are all new-file creates (`--stat` shows no `migrations` line with deletions).

## X3 — Checkpoint discipline — PASS
- streamSyncHandler.Checkpoint (datasource_service.go:1160-1177): guard `if h.currentFence != nil && !appconnector.CanAdvanceCheckpoint(true, h.currentFence(), h.workerFence)` → warn + `return nil` WITHOUT persisting the cursor — a stale worker cannot overwrite the current holder's cursor. Each run takes a fresh fence via `TakeSyncFence` (monotonic per-ds counter under mutex, processSyncStreaming:1234-1241).
- Persisted cursor is stamped with `_sync_auth_version` + `_sync_fence` (1178-1184) so resume can tell which credential version and worker produced it; resume continues from `startCursor` in the unchanged `FetchStream` path.
- Duplicate-run protection: `SaveAppDataSourceBinding` (datasource_repo.go:197-222) updates the `(tenant_id, datasource_id)` row in place and only Creates when `RowsAffected==0`, backed by the DB `UNIQUE (tenant_id, datasource_id)` — duplicates structurally cannot accumulate.
- Note (non-blocking): the call site passes `contentCommitted=true` with an in-code justification (Emit is synchronous, so everything before a offered checkpoint is committed). The boolean is a positional argument to a tested pure function; the discipline rests on the documented Emit-before-Checkpoint ordering rather than an explicit commit signal. Improvement candidate only.

## X4 — Pause semantics — PASS
- Exactly the five reasons budget/plan/permission/quota/provider_limit as a closed vocabulary (sync.go:17-32) with `IsValidPauseReason`; distinctness + validity + unknown-rejection tested (sync_test.go:111-128).
- Plan expiry: `ShouldPauseNewSync(!planActive)` gates ManualSync (service:570), ProcessSync (service:758) and the scheduler BEFORE any sync log/task; `MaySettlePersistedSync` returns true so persisted settlements still complete — asserted together in TestManualSyncPausedByPlanExpiryBeforeScheduling (pause + logs.created==0 + enq.enqueued==0 + MaySettle(true)) and TestPlanExpiryPausesNewSyncButSettlesPersisted (both plan directions).
- Pause leaves the cursor untouched: every pause path returns before any cursor write (gate at 570/758 precedes connector fetch and streaming; scheduler gate precedes everything). SyncPausedError is a pause, not a datasource failure — ProcessSync records the reason on the sync log and returns nil (no asynq retry storm).
- Note (non-blocking): `MaySettlePersistedSync(planActive bool)` ignores its parameter (always true = "settlement is allowed in every plan state"). Semantically correct, slightly odd signature.

## X5 — Binding resolution — PASS
- `ResolveSyncBinding` (sync.go:133-166): nil state (unprovable legacy credentials) → binding resolved with `RequiresReauthorization=true` → service pauses with reason permission, NOT a silent pass (service:126-130); revoked/disabled/pending connection or installation → `bindingAuthorizesSpaceSync` false → same permission pause; personal-kind connections are refused (kind must be `ConnectionKindSpace`). All asserted: TestResolveSyncBindingLegacyCredentialsRequireReauthorization, TestResolveSyncBindingUnusableStatesRequireReauthorization (table incl. revoked state), TestManualSyncPausedForRevokedSpaceConnection.
- Missing relation row → `ErrSyncBindingNotFound` (sync.go:106) → service returns nil (legacy path) — and because the plan gate runs BEFORE the binding lookup (service:105-108), there is no silent legacy bypass of budget: proven by TestLegacyUnboundDataSourceStillGatedByPlan (plan expired + unbound → paused; plan active + unbound → legacy path allowed).

## X6 — Existing-path integrity / additive diffs — PASS
- Legacy datasources keep the old path (`ErrSyncBindingNotFound` → nil) but cannot bypass an enabled space's budget: the plan gate is unconditional and first (service:105-108), regardless of binding existence.
- Connector protocol unchanged: no edit to Connector/StreamingConnector/StreamHandler types; streamSyncHandler (their adapter) only gained fields whose zero values (`currentFence == nil`, authVersion 0) reproduce the old Checkpoint behavior exactly.
- All three modified-file diffs are additive with nil-safe gates: service fields are zero-value = legacy; `SetSyncExecution` optional components; scheduler `syncGate` nil → pre-A07 behavior (scheduler.go:50-52, 172-180). The one deleted line is the in-place expansion of the handler literal.
- Scheduler gate runs BEFORE sync log/task creation (scheduler.go:172-181 sits after ds fetch, before the `syncLog := &types.SyncLog{` literal); ManualSync gate likewise precedes "Create sync log" (service:566-580).
- internal/datasource does NOT import internal/appconnector — the gate is a plain `SyncGateFunc func(ctx, *types.DataSource) error` (scheduler.go:40-42); scheduler.go's import block is untouched in the diff.

## X7 — Source-failure handling — PASS
- `ClassifySourceFailure` (sync.go:199-208): 404/410 → source_not_found; 401/403 → source_permission_denied; anything else → source_unavailable — three distinct kinds, all asserted with the exact HTTP codes (sync_test.go:153-166, testing 404, 403, 401, 5xx).
- `LocalCopyPreserved(sourceReportsDeletion, syncDeletionsEnabled)` = keep unless an explicit source deletion notice AND operator opt-in (sync.go:213-215) — default is keep; tested for default-keep, failure-keep, and opt-in removal (sync_test.go:168-181).

## X8 — Honesty — PASS
- Ledger row (6ac5118) explicitly declares: real Feishu/Notion sync endpoints and real PostgreSQL migration execution are blocked-env ("同步行为以 SQLite/in-memory 与桩证据验证，未伪造 runtime pass") — nothing unrun is marked pass.
- The lost RED transcript is disclosed as such, with the coordinator's reconstruction method (remove sync.go → undefined: CanAdvanceCheckpoint/StoredSyncBinding/ResolveSyncBinding/BindingState/ErrSyncBindingNotFound). I verified those five symbols are exactly what sync.go defines, so the reconstruction is faithful. Same U03/C05/F05 precedent accepted in A06.
- One attribution imprecision (non-blocking): the ledger lists the 6 service/appconnector cases under the GREEN targeted command, but that command's package list (`./internal/application/service`) does not include the `.../service/appconnector` subpackage, and those test names do not match `Test(Sync|DataSource)` either — `-run "TestSync"` on that package reports "no tests to run". The tests themselves are real and pass: I ran them explicitly (see evidence). They were also covered by the 6-package full regression, so no result is fabricated — only the command-to-case attribution is loose.

## X9 — Test quality: counterexamples actually asserted — PASS
- Counterexamples per 行为与边界, all with hard assertions, not happy-path only: uncommitted content / stale fence refused (Step-1 verbatim, retained unweakened); legacy credentials marked (not failed); unusable states table; missing relation row error; five reasons distinct+valid, unknown invalid; plan expiry both directions + settlement-allowed; typed pause error carries reason; 404 vs 401/403 vs 5xx classification; default-keep vs opt-in deletion.
- Service-level tests are behavioral, not just unit: they assert SIDE-EFFECT counts (`logs.created != 0 || enq.enqueued != 0` fails a paused sync that leaked a log/task; active path requires exactly 1/1) — this pins the "gate BEFORE scheduling" ordering to observable effects. No existing assertion was modified or deleted anywhere in the diff.

## X10 — Code quality — PASS
- Plain-func gate design confirmed: no application-layer (or appconnector) import into internal/datasource; hooks installed via setters; nil = legacy.
- Race safety: fence map guarded by `syncFenceMu` on both take and read; `SetSyncGate` takes `s.mu`, and the gate is snapshotted under the lock then invoked outside it (no lock held across a user callback). Fake stores in tests are single-threaded per test (fine). Coordinator ran vet clean.
- Error handling: repo returns (nil, nil) on not-found so the caller can keep the legacy path instead of treating a missing binding as a failure; `NewSyncPausedError` coerces an invalid reason to budget rather than emitting an invalid string; Save validates nil/empty inputs.
- Non-blocking improvement notes: (a) fences are per-process in-memory — a multi-instance deployment needs a distributed fence (DB counter/lease) to make the stale-worker guarantee cross-instance; (b) `_sync_fence`/`_sync_auth_version` keys are stamped into the connector-owned cursor map (underscore prefix mitigates collision); (c) `SaveAppDataSourceBinding`/`SetSyncExecution`/`SetSyncGate` have no production wiring yet (grep: only tests call them) — acceptable for a hook-delivery task with assembly in later tasks (W01…), and the ledger does not claim runtime wiring.

## Command evidence (independently run in the worktree)
- `go test ./internal/appconnector ./internal/application/service/appconnector -run "TestSync" -count=1` → `ok internal/appconnector 1.617s`; service/appconnector "no tests to run" (name/package mismatch — see X8 note), therefore also:
- `go test ./internal/application/service/appconnector -count=1 -v -run 'TestManualSync|TestLegacyUnbound|TestCheckpointFence'` → 6/6 PASS, `ok` (TestManualSyncPausedByPlanExpiryBeforeScheduling, TestManualSyncPausedForUnprovableLegacyCredentials, TestManualSyncPausedForRevokedSpaceConnection, TestManualSyncDispatchesWithActiveSpaceConnection, TestLegacyUnboundDataSourceStillGatedByPlan, TestCheckpointFenceRefusesStaleWorker).
- `git show --stat 0e30173 / 6ac5118`; `ls migrations/versioned | tail -4` / `ls migrations/sqlite | tail -4` (000120/000040 are next-free after A03's 000119/000039); `grep -rn 'SetSyncExecution|SetSyncGate|SaveAppDataSourceBinding|FindAppDataSourceBinding'` (wiring status); `grep -n AuthorizeSyncExecution datasource_service.go` → 570 (ManualSync, pre-log) / 758 (ProcessSync, pre-connector-fetch).
- Full gate suite (vet/build 5 pkg, gofmt, diff-check, 6-pkg regression) taken on coordinator evidence per the review time budget; the only failure there is the pre-existing unrelated TestSkillPythonVerifier documented since A02.

## Verdict
PASS — no blocking finding: file set exact, checkpoint/fence algorithm verbatim, migrations paired + reverse-only downs + next-free numbering per the authorized deviation, pause semantics exactly the five reasons with cursor-preserving behavior, no budget bypass for legacy datasources, source failures classified with local copy preserved by default, additive-only modified files with nil-safe hooks, tests assert real counterexamples including side-effect counts, and the ledger is honest about blocked-env and the reconstructed RED. Eight non-blocking improvement notes recorded above (ResolveSyncBinding signature sketch deviation, hardcoded contentCommitted=true, MaySettlePersistedSync unused param, ledger command-to-case attribution, in-process fences, cursor key stamping, unwired hooks pending assembly, single deleted line being an in-place expansion).
