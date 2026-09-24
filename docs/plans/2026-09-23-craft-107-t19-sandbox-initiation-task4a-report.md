# T19 Task 4a — Sandbox initiation boundary checkpoint

**Status: BLOCKED / FAIL-CLOSED.** This checkpoint adds no production charging hook. The approved provider interface exposes synchronous full-operation `Create` and `Exec` methods, so calling `CraftBudgetService.StartBinding` around either call would violate Task 4's initiation-only boundary. No such wrapper was added and no claim is made that sandbox starts are budget fenced. Parent instruction: record this interface blocker and design; a separate Task 4b will own the provider API/adapters.

**Role:** `backend_implementer` as assigned by the parent. Runtime model/reasoning metadata is not exposed here; none is asserted.

**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`

**HEAD at checkpoint:** `002533c66d1bef1f371d5ba335c731bf1ce9f67a`

**Commit/staging:** none by this task.
**Ownership amendment:** Parent authorized `internal/application/service/agent_run_graph.go` and `agent_run_graph_test.go` solely for production injection. `agent_run_graph.go` already contains concurrent T05 input-manifest edits (including `BuildDurableCraftRunSnapshot`); I preserved the full current bytes and made no edit there. No `container.go`, provider adapter or shared central file was changed.

## Approved scope and source review

Read Task 4 from `2026-09-23-craft-107-t19-durable-start-plan.md`, Task 1 denial-fix review, Task 2 pending-cancel review, Task 3 model-gateway checkpoint, `docs/specs/2026-09-23-craft-web-artifact-spec.md` requirements 34/36 and its budget/network verification decisions, and `docs/adr/0004-task-is-session.md`. Task 1 and Task 2 reviews clear their scoped fixes, but explicitly do not verify T19 as a whole. Task 3 reports that a runtime caller still does not provide its required model activity header. No worker seam-map file named by earlier T19 work was present; the Task 3 report records this same limit.

The relevant Task 4 requirements are: use persisted `ToolPlan.CallID` and attempt plus the Run fence; commit charge intent/G4 hold before approved remote starts; never replay an unresolved activity; keep provider script execution/output waiting outside the charge-start callback; preserve ambiguous outcomes as unknown; and explicitly leave bootstrap, maintenance, reads and deletes ungated.

## Exact RED / interface proof

The production contract has no initiation/receipt/wait split:

```go
// internal/modules/execution/sandbox/remote_client.go
Create(context.Context, RemoteCreateRequest) (RemoteSandboxHandle, error)
Exec(context.Context, RemoteSandboxHandle, RemoteExecRequest) (*RemoteExecResult, error)
```

The implementation-level RED is structural and reproducible from these exact call paths:

- `SessionBoundManager.execShellCommandWithOutputSnapshot` calls `m.client.Exec` and obtains the final `RemoteExecResult` before returning. It also performs output snapshot reads before and after this call.
- `DockerRemoteClient.Exec` does `ExecCreate`, then `streamExec` (which attaches and drains output), then `ExecInspect`; the public method returns only after result collection.
- `CubeRemoteClient.Exec` calls `sb.Commands().Run` and returns its final stdout/stderr/exit code.
- `E2BRemoteClient.Exec` calls `sandbox.Commands.Run` and returns its final result.
- `remoteSessionLifecycle.resolveLocked` calls `l.client.Create` and expects a ready `RemoteSandboxHandle` before validation, metadata checks and durable session binding. No operation receipt is exposed before the call returns.

There is no `StartExec`, `WaitExec`, `ObserveExec`, `StartCreate`, `WaitCreate` or `ObserveCreate` member/capability on `RemoteSandboxClient`. There is no provider operation reference field or recording seam in `CraftChargeStartJournalRow`/`CraftBudgetService.StartBinding`. Therefore a callback wrapping current `m.client.Exec` would include command execution and output wait; a callback wrapping current `Create` includes provisioning through handle return. A local timeout does not convert these synchronous methods into bounded initiation, and a cancellation after provider acceptance cannot establish “not started.”

This is the RED for Task 4: the existing production surface cannot express the required behavior. The targeted existing package tests below pass as regression evidence only; they do not demonstrate charge fencing. No synthetic test was added that pretends a synchronous method is an initiation boundary.

## Concrete provider-neutral Task 4b design

This design is intentionally a proposal for the follow-up owner; it is not implemented by this checkpoint.

1. **Trusted activity identity.** At the durable graph boundary, derive the coordinator inputs from the verified `ToolDispatch` plus `RunFenceFromContext`: tenant/run/owner/epoch must match, `CallID` must equal the persisted plan, and `Attempt > 0` must equal the committed `ToolAttempt.Number`. Build a server-only activity key such as `sandbox/<operation>/<CallID>/<attempt>` (bounded to the existing activity-key limit). Never derive it from model-supplied tool arguments. Admit/resolve the Craft grant for `(tenant, run)` from trusted durable run state; never trust `Run.BudgetRef` as a Craft grant unless a reviewed contract proves that equivalence. Use a server-selected binding (`ModelID` identifies the sandbox provider/operation; `Funding` comes from server policy). Missing or stale scope fails closed before provider calls.

2. **Split transport from completion.** Add a provider-neutral operation receipt:

   ```go
   type RemoteOperationRef struct { Provider, ID, OperationKey string }
   type RemoteOperationObservation struct { State, Result ... }
   type RemoteExecStarter interface {
       StartExec(ctx, handle, request, operationKey) (RemoteOperationRef, error)
       WaitExec(ctx, handle, ref) (*RemoteExecResult, error)
       ObserveExec(ctx, handle, ref) (RemoteOperationObservation, error)
   }
   ```

   `StartExec` must return after provider acceptance with an authoritative provider reference, not after the command's output drains. The `CraftChargeService.StartBinding` callback contains only that bounded start request. Once it returns `Started`, `WaitExec` streams/waits outside the transaction and outside the callback. A transport result that may have reached the provider but has no authoritative receipt maps to `Unknown`; it is never converted to `DefinitelyNotStarted` based on a timeout alone. If a provider lacks these semantics, durable tool execution fails closed for that provider.

3. **Receipt persistence and crash window.** The current charge journal stores activity outcome but not a provider operation reference. Task 4b must either add an authenticated/fenced way to persist the receipt against the `(tenant, run, activity)` intent before waiting, or require `StartExec(operationKey)` plus `ObserveExec(operationKey)` to be provider-idempotent/authoritative across a crash between provider acceptance and receipt persistence. The stable key is the persisted tool CallID+attempt, not the Run epoch; the current fence still authorizes the start. If neither receipt durability nor authoritative key lookup exists, keep the charge journal `intent|unknown` and the ToolAttempt unresolved; do not dispatch another command.

4. **Create and lifecycle recovery.** The actual lifecycle create is inside `remoteSessionLifecycle.resolveLocked`, under the session lifecycle lock. Charge only when that code has established that no valid binding or owned sandbox exists and is about to invoke provider Create; existing-binding connect/adoption, metadata inspection, workspace bootstrap, cleanup/delete, snapshots and reads remain explicitly classified. Introduce a corresponding `StartCreate`/`WaitCreate`/`ObserveCreate` seam or a provider contract with equivalent bounded acceptance plus authoritative lookup. Tie it to the same tenant/session/config/binding generation and the initiating ToolAttempt. A crash/unknown create must be recovered by exact provider identity/metadata under the lifecycle lock before another create is allowed. Existing session metadata recovery is useful but is not by itself proof that all providers offer authoritative, unique, immediately consistent lookup. Do not start a second create when lookup reports absent but the original request can still be pending.

5. **Provider feasibility.** Docker has a real internal split (`ExecCreate`, `ExecAttach`/stream, `ExecInspect`) and is the clearest candidate for an adapter `StartExec` receipt and out-of-band wait, but its current adapter hides the ID and `ExecInspect` alone needs careful “not started vs completed” classification. Cube and E2B currently call synchronous `Commands().Run`; Task 4b must verify SDK/provider APIs for a remote execution ID/start/observe operation. If either cannot supply an authoritative operation identity/lookup, return an explicit unsupported capability and fail closed instead of invoking the synchronous method inside `StartBinding`. Create is also synchronous in the current adapter contract; provider-specific create inventory must prove stable owner metadata, uniqueness and recovery consistency before it can be enabled.

6. **Non-chargeable classifications and tool seams.** Keep `prepareSessionDirs`/bootstrap Exec, output snapshots, skill installation, artifact listing, file reads, staging, cleanup, deletes and session reconciliation outside the charge-start callback. Model-authored `sandbox_write`/`sandbox_edit` mutations also need a reviewed classification: if they are chargeable remote starts, gate only their actual remote mutation with stable tool dispatch; validation, append/read/compare and preflight stay outside. Tests must prove these maintenance/read/delete paths do not consume starts while shell/Create attempts do.

7. **Required regressions.** Fake transport tests must prove durable intent+G4 hold is visible before `StartExec`/`StartCreate`, no provider call on denial/stale epoch/foreign Run/attempt zero, same attempt cannot start twice, same-key unknown observation never starts again, first attempt gets `maySend`, rejected/no-send maps only on authoritative provider evidence, accepted/response-lost stays unknown until observation resolves it, wait/output collection occurs after StartBinding has returned, activity binding includes operation kind, lifecycle lock serializes one actual Create, and maintenance/read/delete stays outside. Add SQLite and PostgreSQL evidence for start-vs-pause ordering where configured.

## Verification performed

Read-only target regression commands completed successfully against the current shared worktree:

```text
go test ./internal/modules/agentruntime/agent/runtime -run 'TestToolExecutor' -count=1
ok github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime 3.505s

go test ./internal/modules/agentruntime/agent/tools -run 'Test(ShellExec|WriteSandboxFile|EditSandboxFile)' -count=1
ok github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools 2.521s
warning: ld: ignoring duplicate libraries: '-lc++'

go test ./internal/modules/execution/sandbox -run 'Test(Session|Shell|WriteSessionWorkspace|DockerClientExec)' -count=1
ok github.com/Tencent/WeKnora/internal/modules/execution/sandbox 1.610s
```

These runs compile and execute existing tool/sandbox behavior only. No new Go source or test was written; so there is no GREEN evidence for Task 4 charging. No PostgreSQL database, live provider API, cross-process create/exec recovery or end-to-end authenticated sandbox Run was exercised. The passing calls do not clear F3/T19.

## Exact checkpoint and file hashes

Task source/test files are unchanged by this checkpoint. The concurrent graph file remains modified by earlier shared work; its hash is recorded only for preservation and is not attributed to this task. Current content hashes:

```text
internal/modules/agentruntime/agent/runtime/tool_executor.go  76806d945204e7f7c668b63c0a89dfd95bc8df0904b515b9a50e28639f645d3d
internal/modules/agentruntime/agent/runtime/tool_executor_test.go  b369aaba7c35c23b9b040980f82141e52fb303fe9fd59acae2ed618b096e3b07
internal/modules/execution/sandbox/session_manager.go  471cd9c79348afc7288a1dc1a0c1bf69566ca0483e96ac2c2a5fdf6b5dedb300
internal/modules/execution/sandbox/session_manager_test.go  9a1a26341f85ec2c7924f4d1c25d34cee13ef86fb84a5250c55b2afd24e3adb2
internal/modules/execution/sandbox/remote_client.go  7b8fc6c06a7934a985625ec12aba7cfcb9b9e7661d74ec0379d31c5baeb3b46d
internal/modules/execution/sandbox/session_lifecycle.go  163c2a4b73082aca11b6b1e1aa116aa65cc3176b8b10b0b3310df99cd689f927
internal/modules/execution/sandbox/docker_remote_client.go  dfb98bc2695b863f6630c10a484c66197fa40ee44aa5912e29918e3da52e36a8
internal/modules/execution/sandbox/cube_remote_client.go  b52f45cb971cee66ebe0c09b897b61054903ee51329ba0694b8fe25d8cde38c2
internal/modules/execution/sandbox/e2b_remote_client.go  84effaf3f01545136e8a4d814941383f863d5ccac45461a89461d6a0b62c92a2
internal/modules/agentruntime/agent/tools/shell_exec.go  da258a02f99bb2550a4f5229e8414323a876356f6ed72a60e08a947c09fe7ee0
internal/modules/agentruntime/agent/tools/shell_exec_test.go  f74a1013d363c75fa5eb701dabc2ad931a259aa4988dbd9266d9dec2ff861b99
internal/modules/agentruntime/agent/tools/sandbox_write.go  623e2e6c02fd53695357424ebf6606ba3d830e2234887ede89f08964df51f796
internal/modules/agentruntime/agent/tools/sandbox_write_test.go  29fc0d678b703219d854daf287ba2fba254cf82373034cc6f7a461af6fc75589
internal/modules/agentruntime/agent/tools/sandbox_edit.go  d3199bb7043df90d9120d271518247a086e172242339829bc27ea753c9086a2a
internal/modules/agentruntime/agent/tools/sandbox_edit_test.go  c5cffe4fbeb9d82238dd83421c3ed12e0e80ef1ea3b10f94c72e0a9c2a936dba
internal/application/service/agent_run_graph.go (concurrent shared content, preserved) 3197d7d39553be193a3267153fea18e88547cfff6255acfb1cda4c21d1d1fa8f
```

This report is the only file added by this task. `git diff --check` and report hash are recorded after this document is written. No files were staged or committed.

## Remaining risks / handoff

- Sandbox Create/Exec paths are not yet integrated with `StartBinding`; current durable sandbox acceptance remains unverified and Task 4 is not complete.
- Task 4b needs exact ownership for `internal/modules/execution/sandbox/remote_client.go`, `session_lifecycle.go`, and Docker/Cube/E2B adapter files, plus any explicitly approved service journal receipt seam/schema. Until then do not wrap the synchronous methods or claim provider egress/start proof.
- The existing tool plan/journal and Task 1 G4 coordinator alone cannot prove provider start acceptance, operation completion, or recovery. The exact provider transport contract and receipt persistence are prerequisites.
