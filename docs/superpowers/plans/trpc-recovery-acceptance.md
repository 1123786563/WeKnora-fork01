# tRPC recovery acceptance record

This document is the release gate for the durable tRPC run path. A passing unit
or checkpoint test does not prove process recovery: the crash cases below must
start a real graph provider, kill that process at a provider-owned barrier, and
reopen the same durable database in a new process.

## Current evidence

Recorded 2026-09-11 on macOS, branch `codex/trpc-recovery`, HEAD
`2c12fc3`. The repository has the durable Run/lease/checkpoint APIs and worker,
but no registered graph executor provider is available to this acceptance
harness. Therefore the SIGKILL matrix is **blocked by a concrete missing
provider**, and is not marked passed.

| Check | Command / setup | Result | Evidence |
|---|---|---|---|
| admission gate | `GOWORK=off go test ./internal/agent/recoverytest -run TestRecoveryAdmissionGate -count=1` | PASS | disabled admission is false; worker-only mode is drain-only; enabled admission is true |
| inconsistent config | `GOWORK=off go test ./internal/agent/recoverytest -run TestRecoveryAdmissionGateRejectsInconsistentConfig -count=1` | PASS | `AdmissionEnabled=true` with `Enabled=false` is rejected before runtime construction |
| crash harness contract | `GOWORK=off go test ./internal/agent/recoverytest -run TestCrash -count=1 -v` | SKIPPED | `TRPC_RECOVERY_GRAPH_PROVIDER` is unset; test reports the missing real provider |
| race acceptance job | `.github/workflows/agent-recovery.yml` | CONFIGURED | SQLite/PostgreSQL matrix and `go test -race`; provider is opt-in and absence is reported |

The skipped case is intentionally not evidence for `ExternalCalls`, final
status, assistant message count, or event loss. Those fields are read from a
JSON report emitted by the provider after restart.

## Provider protocol and required matrix

Set `TRPC_RECOVERY_GRAPH_PROVIDER` to an executable that owns the real graph,
saver, and migrated database. The harness invokes it once with:

```text
--recovery-case <barrier> --recovery-db <file> --recovery-barrier <file> --recovery-report <file>
```

The provider must create only test-namespace resources, touch the barrier after
the requested durable point, and keep running. The parent sends SIGKILL, then
invokes the same executable with `--recovery-resume <barrier>` and the same
arguments. Resume must print or write a JSON `CrashReport` with
`external_calls`, `final_status`, `assistant_rows`, and `lost_events`.

The release matrix must cover these barriers and assertions:

- after admission, after plan, after external side effect before result, after
  result before checkpoint, after `waiting_user`, and after finalization;
- exactly one external side effect after a committed result;
- an unknown result parks at `waiting_user`, and explicit retry alone increments
  the external count to two;
- idempotent provider redelivery leaves the external count at one;
- two workers contend on the same DB, the expired lease is reclaimed, the old
  epoch cannot write, and the unknown tool is not started twice;
- API reconnect, decision conflict, permission revocation, cancellation,
  deletion, budget persistence, graph/schema incompatibility, and sandbox
  alive/lost/destroyed fixtures;
- SQLite and isolated PostgreSQL schema. Cloud sandbox credentials are a
  separate evidence row and cannot be inferred from contract tests.

## Gate semantics

`AgentRecoveryAdmissionEnabled` is true only when both the worker and admission
flags are enabled. `Enabled=true, AdmissionEnabled=false` is the controlled
drain mode: existing queued or leased runs may finish or be reclaimed, while
new tRPC work is closed. `AdmissionEnabled=true, Enabled=false` is rejected at
startup because it would admit work without a recovery worker. A disabled
worker must never silently fall back to builtin execution for a persisted tRPC
run.

Until every required matrix row has fresh command output and a provider report,
the feature remains unavailable for rollout. The current record is therefore
partial evidence, not a completion claim.
