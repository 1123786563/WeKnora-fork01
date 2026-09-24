# T08 Task 3 — durable worker actor restoration report

Date: 2026-09-23. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Plan: `docs/plans/2026-09-23-craft-107-t08-actor-principal-plan.md`, Task 3. No commit or subagents. Actual role/model/reasoning-effort metadata was not exposed by this runtime and is not asserted.

## Result

The durable worker now restores `Run.ActorUserID` into `types.Caller`, web `Principal`, legacy `UserIDContextKey`, and `ToolExecContext.UserID` before model resolution and capability assembly. `Run.UserID` remains the owner/storage identity; lease `Run.Owner` is used only for fence validation and never as the authenticated principal.

Craft snapshots are identified by their typed `CraftInputManifest` marker. Before model lookup or `prepareAgentCapabilities`, the service requires a durable actor and a non-nil `craft.TaskAccessChecker`, then checks current `TaskWrite` using `(run tenant, actor user, run session)`. Missing/denied access returns `craft.ErrForbidden`. Actorless legacy Craft snapshots therefore fail closed. Legacy non-Craft runs retain their historical user identity fallback. MCP/resource lookup tests confirm the collaborator's view is used and the owner's private connector is not loaded.

Follow-up Run admission now copies the durable actor, fails closed if a Craft actor is absent, and checks current TaskWrite before admitting a Craft follow-up. T05 source-record revalidation is still separate; this task does not claim source authorization is covered.

## TDD and verification

RED evidence:

- Before the implementation, `go test ./internal/application/service -run '^TestExecuteDurableCraftRun(RestoresActorAcrossWorkerLeaseRecovery|RejectsLegacyMissingActorBeforeCapabilities)$' -count=1` failed: worker capability lookup observed an empty caller instead of the collaborator, and a legacy actorless Craft run returned nil instead of `craft.ErrForbidden`.
- With the TaskWrite check temporarily skipped, `go test ./internal/application/service -run '^TestExecuteDurableCraftRunRechecksTaskWriteBeforeModelResolution$' -count=1` failed as intended: expected `craft.ErrForbidden`, got nil. The bypass was removed before GREEN verification.

GREEN / verification commands:

- `go test ./internal/application/service -run '^TestExecuteDurableCraftRun(RestoresActorAcrossWorkerLeaseRecovery|RejectsLegacyMissingActorBeforeCapabilities|RechecksTaskWriteBeforeModelResolution)$' -count=1` — PASS (`5.743s`). Covers W1→W2 lease recovery, actor Caller/Principal/context restoration, actor-bound tool metadata, owner/actor identity separation, actorless legacy rejection, and TaskWrite revocation before model/capability resolution.
- `go test ./internal/application/service -run 'TestExecuteDurableRun|TestAdmitAfterFollowUps' -count=1` — PASS (`14.601s`).
- `go test ./internal/container -run '^TestCraftAccessFeatureUsesPersistentPolicyOnConstrainedRoutes$' -count=1` — PASS (`3.784s`); package compilation verifies the new session-service constructor parameter is resolvable alongside the existing `craft.TaskAccessChecker` provider. No `container.go` edit was needed.
- `git diff --check` — PASS.

## Scope and checkpoint

Task-owned changes are limited to:

- `internal/application/service/agent_run_graph.go` — actor authorization/context restoration, tool principal metadata, and follow-up actor propagation/recheck.
- `internal/application/service/session.go` — inject the existing `craft.TaskAccessChecker` port into `sessionService`.
- `internal/application/service/agent_run_graph_test.go` — recovery, legacy actorless, revoked TaskWrite, metadata, and scope assertions.

This Worktree was already shared and dirty when the task began. In particular `agent_run_graph.go` and `agent_run_graph_test.go` already contained other T05/T08 snapshot and manifest work; this task preserved it. A clean dirty-tree task-entry snapshot was not captured, so HEAD hashes below are committed baselines, not claims about the exact pre-task dirty contents.

Checkpoint: `t08-actor-task3-2026-09-23`, uncommitted at HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No commit was created.

Committed HEAD baseline SHA-256:

```text
879c91ab406ce66115a065032a12ca1642f7efc33fc99c17667ba165cf98df16  internal/application/service/session.go
3197d7d39553be193a3267153fea18e88547cfff6255acfb1cda4c21d1d1fa8f  internal/application/service/agent_run_graph.go
5231605faf003b8d4f65649ec96932d2859e5adbf994f8612f49c4f387d40ef4  internal/application/service/agent_run_graph_test.go
```

Final full-content SHA-256:

```text
49592b157b42337fc2f5fcbbc0d0986bf5a6233cb145ccf22b69ca97f6a235ab  internal/application/service/session.go
537bd72a0f2f03f8ead3105f72a2451520be8a289f4eabd347181aa1e628edde  internal/application/service/agent_run_graph.go
7bb78800f00669d84dd7a7ce36d4cde0ee521a75f44c03d3f77685cbb099ffca  internal/application/service/agent_run_graph_test.go
```

## Remaining gates

The graph and session-service files are released for independent review and subsequent T05 work. T05 source/record authorization recheck remains a separate prerequisite. Joined Craft worker validation, production DI/runtime assembly review, RunView per-Run read isolation, central Publisher integration, and dispatch/assembly binding remain outside this task. This report does not claim full T05 or T08 completion.
