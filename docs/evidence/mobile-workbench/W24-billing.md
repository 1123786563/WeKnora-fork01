# W24 billing evidence

## Scope

W24 adds the trusted remote usage boundary and exposes it through the
application container. `AllowModelSettlement` accepts only the server-side
`platform_gateway` + `platform` pair. A personal node cannot self-authorize a
model charge, and a BYOK model call does not create a second model reservation.
Non-model services remain subject to the normal commercial execution gate.

`RemoteUsageService` constructs `commercial.UsageFact` from server-owned
identity, funding, price, revision, and dimensions. Unknown, partial, and
`display_only` observations never reach `ExecutionGate.Finish`; duplicate
physical attempts retain the call/attempt/revision identity and the gate's
existing idempotency key semantics. Child runs use the existing commercial
budget tree (`AttachChildRun`/root resolution) and do not receive a copied
balance.

## Evidence

- RED was reconstructed from the task brief: before the policy file existed,
  `go test ./internal/execution -run TestRemoteUsage -count=1` failed because
  `AllowModelSettlement` was undefined.
- GREEN: `go test ./internal/execution ./internal/application/service/workbench ./internal/application/service/commercial -run 'TestRemoteUsage|TestRemoteSettlement|TestExecution' -count=1` passed.
- Formatting and whitespace checks passed for all W24 files.
- Container wiring adds `NewRemoteUsageService` behind the already registered
  trusted `ExecutionGate` interface.

## Environment boundaries

The focused tests use an in-memory fake gate for the remote usage seam and the
existing commercial tests use SQLite. PostgreSQL migration execution, a live
OpenMeter settlement, real Paseo, and WeChat/Alipay channels were unavailable;
these remain `blocked-env` and are not claimed as runtime acceptance. The base
branch also contains a pre-existing W20 `RemoteDispatcher` field/method name
compile defect; validation temporarily applied the known W22 rename in an
isolated copy and restored the untouched dependency file. That dependency fix
must be present when W24 is integrated.
