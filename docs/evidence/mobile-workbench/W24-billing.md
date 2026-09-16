# W24 billing evidence

## Scope

W24 adds the trusted remote usage boundary and exposes it through the
application container. `AllowModelSettlement` accepts only the server-side
`platform_gateway` + `platform` pair. A personal node cannot self-authorize a
model charge, and a BYOK model call does not create a second model reservation.
Non-model services remain subject to the normal commercial execution gate.

`RemoteUsageService` now has a private bound path. Public request objects cannot
enter the gate; the Paseo dispatcher derives source, funding, price, call,
attempt, revision, and dimensions from the server-owned worker fence. The
dispatcher reserves before `StartCommand` and finishes exactly once after a
successful provider start. Child admission calls the same budget tree used by
the execution gate, and explicit keys must equal the derived
`call_id:attempt_id:revision` identity.

## Evidence

The integration fix adds the actual `RemoteDispatcher` path coverage:

- `go test ./internal/application/service/workbench -run 'TestRemoteDispatcher|TestRemoteUsage' -count=1` — PASS.
- `go test -race ./internal/application/service/workbench -run 'TestRemoteDispatcher|TestRemoteUsage' -count=1` — PASS.
- `go test ./internal/application/service/workbench ./internal/container -run '^$' -count=1` — PASS (compile-only).
- `go vet ./internal/application/service/workbench ./internal/container ./internal/execution ./internal/agent/runtime` — PASS.

The tests exercise the real durable dispatch store with a fenced provider,
trusted usage Begin/Finish ordering, child budget attachment, BYOK model
no-reservation, idempotent replay, and post-provider missing-usage
reconciliation. A provider response that lacks a trusted usage observation is
left `reconciled`/unknown and is never synthesized into a billable success.

- RED was reconstructed from the task brief: before the policy file existed,
  `go test ./internal/execution -run TestRemoteUsage -count=1` failed because
  `AllowModelSettlement` was undefined.
- GREEN: `go test ./internal/application/service/workbench ./internal/application/service/commercial -count=1` passed.
- GREEN: `go test -race ./internal/application/service/workbench -count=1` passed;
  `go vet ./internal/application/service/workbench ./internal/application/service/commercial ./internal/container` passed.
- Container package compilation passed with `go test ./internal/container -run '^$' -count=1`.
- Formatting and whitespace checks passed for all W24 files.
- Container wiring adds `NewRemoteUsageServiceWithDB` behind the registered
  trusted `ExecutionGate`, injects it into `newAgentRuntime`, and constructs
  the Paseo dispatcher with the usage boundary.

## Environment boundaries

The focused remote usage tests use an in-memory fake gate; the existing
commercial tests use SQLite. The full container test suite still has an
unrelated migration transaction failure in the inherited W20/SQLite setup.
PostgreSQL migration execution, a live OpenMeter settlement, real Paseo, and
WeChat/Alipay channels were unavailable; these remain `blocked-env` and are
not claimed as runtime acceptance.

## Targeted fix evidence (2026-09-17)

- `go test ./internal/application/service/workbench -run 'TestRemoteUsage|TestRemoteDispatcher' -count=1` — PASS.
- `go test ./internal/application/service/workbench -count=1` — PASS.
- `go test ./internal/application/repository -run TestPersistUsageBindingOverridesUntrustedSnapshotFields -count=1` — PASS.
- `go test ./internal/application/service/workbench ./internal/application/repository ./internal/agent/runtime -run 'TestRemoteUsage|TestRemoteDispatcher|TestPersistUsageBinding|TestAgentRun' -count=1` — PASS.

The fix persists the server-owned usage binding into the immutable admission
snapshot and restores it into the worker `Fence`. Provider observations may
repeat the fenced revision and dimensions, but mismatched values are rejected;
they cannot replace the settlement identity. Constructor tests fail closed for
a missing gate or budget database. The real `ExecutionGateService` plus SQLite
budget/usage stores now covers one remote dispatch, child budget binding, and
same-receipt replay with one usage fact.

Unknown, partial, and display-only observations remain non-billable; a later
final observation can settle the retained reservation and an identical final
replay is idempotent at the service seam. PostgreSQL migrations, live
OpenMeter/Paseo execution, and provider runtime evidence remain `blocked-env`.

## Final review follow-up (2026-09-17)

- `go test ./internal/application/service/workbench -count=1` — PASS.
- `go test -race ./internal/application/service/workbench -run 'TestRemoteUsage|TestRemoteDispatcher|TestServerAdmissionBinding' -count=1` — PASS.
- `go vet ./internal/application/service/workbench ./internal/application/repository ./internal/agent/runtime ./internal/container` — PASS.
- `go test ./internal/application/repository ./internal/agent/runtime ./internal/container -run '^$' -count=1` — PASS (compile-only; container runtime migration remains separately blocked).
- `git diff --check` — PASS.

Production admission now consumes an explicit server-side binding resolver. The
resolver can carry BYOK/model, parent run, credential version, source, price,
revision, and dimensions through the immutable admission snapshot into Fence;
client JSON copies are removed. The container uses the explicit binding
constructor, while legacy tests retain a named server platform policy adapter.

Late final reconciliation reconstructs the durable physical-attempt key and
uses `ReconcileRemoteObservation` without creating a second reservation. The
real SQLite `ExecutionGateService` test covers unknown/partial/display_only,
late final, replayed final, one reservation, and one persisted usage fact.
Unsupported `RemoteCommandProvider` capability now reconciles the claimed
intent to durable `unknown` with an observed reason, so it cannot remain
`claimed`/busy. PostgreSQL, live OpenMeter, and live Paseo evidence remain
`blocked-env`.

## Final re-review follow-up (2026-09-17)

- `go test ./internal/application/service/workbench -count=1` — PASS.
- `go test -race ./internal/application/service/workbench -run 'TestRemoteUsage|TestRemoteDispatcher|TestServerAdmissionBinding|TestProductionAdmission' -count=1` — PASS.
- `go vet ./internal/application/service/workbench ./internal/application/repository ./internal/agent/runtime ./internal/container` — PASS.
- `git diff --check` — PASS.

The production container now uses `NewDatabaseAdmissionBindingResolver` with
`ExecutionTargetStore` ownership checks and `DurableTaskBudget` backed by the
commercial task-budget tables. Admission registers the actual run or attaches
it to the trusted parent before durable run creation; remote call holds still
use the same database-backed ExecutionGate. The legacy resolver is no longer
in the production container path.

A RemoteDispatcher integration test records provider usage as durable unknown,
then a fresh dispatcher/service instance delivers a late final twice and
asserts one persisted reservation and one usage fact. Production admission
coverage persists platform and server-injected BYOK/parent/credential binding
into run snapshots while client-named values are scrubbed at the repository
boundary. PostgreSQL, live OpenMeter, and live Paseo remain `blocked-env`.

## Final2 production resolver follow-up (2026-09-17)

- `go test ./internal/application/service/workbench -count=1` — PASS.
- `go test -race ./internal/application/service/workbench -run 'TestRemoteUsage|TestRemoteDispatcher|TestServerAdmissionBinding|TestProductionAdmission' -count=1` — PASS.
- `go vet ./internal/application/service/workbench ./internal/application/repository ./internal/agent/runtime ./internal/container` — PASS.
- `go test ./internal/application/repository -run 'TestExecutionTargetStore|TestExecutionTargetIdentity|TestCreateTarget' -count=1` — PASS.
- `git diff --check` — PASS.

Execution targets now persist an immutable server usage policy in
`usage_binding_json`, including BYOK/platform funding, model/connector service,
ParentRunID, price version, revision, status, and dimensions. The production
`NewDatabaseAdmissionBindingResolver` reads the tenant/owner-scoped target from
`ExecutionTargetStore`, validates target ownership and credential rotation,
and rejects request-scoped `StartInput.Binding`; platform policy is used only
for the platform target. The production admission test creates a persisted
BYOK target policy, starts platform and BYOK/parent runs through the database
resolver, and verifies the resulting snapshots.

Migration files are added for SQLite 000058 and versioned 000136. Existing
migration-head/runtime failures outside this focused target path remain
`blocked-env` and are retained as such.
