# T08 durable actor Task 3: independent scoped review

Date: 2026-09-24. Scope: `t08-actor-principal-plan.md` Task 3, its report, the reviewed Task 2 contract, approved Craft Spec #107/#126, `CONTEXT.md`, ADR-0004/0009, and the three reported Task 3 files. Read-only source review; no OCR or delegation. HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34` is a committed baseline, not a clean task-entry checkpoint; the shared worktree already contained T05 snapshot work.

## Checkpoint and verdict

| Full file | SHA-256 at review |
| --- | --- |
| `internal/application/service/session.go` | `49592b157b42337fc2f5fcbbc0d0986bf5a6233cb145ccf22b69ca97f6a235ab` |
| `internal/application/service/agent_run_graph.go` | `537bd72a0f2f03f8ead3105f72a2451520be8a289f4eabd347181aa1e628edde` |
| `internal/application/service/agent_run_graph_test.go` | `7bb78800f00669d84dd7a7ce36d4cde0ee521a75f44c03d3f77685cbb099ffca` |

The hashes match the implementation report. There is no complete dirty task-entry snapshot, so this review binds to the listed final file contents and does not attribute every HEAD-relative hunk to Task 3. `git diff --check` for these files passed. Focused tests passed: `go test ./internal/application/service -run '^TestExecuteDurableCraftRun(RestoresActorAcrossWorkerLeaseRecovery|RejectsLegacyMissingActorBeforeCapabilities|RechecksTaskWriteBeforeModelResolution)$|^TestAdmitAfterFollowUps' -count=1` and `go test ./internal/container -run '^TestCraftAccessFeatureUsesPersistentPolicyOnConstrainedRoutes$' -count=1` (the latter emitted only a duplicate `-lc++` linker warning).

- **Scoped Spec compliance: FAIL.** Current typed Craft snapshots restore the persisted actor and recheck TaskWrite before model/capability resolution. However, actorless historical Craft rows without the newly introduced snapshot marker can fall back to the Task Owner, contrary to the explicit legacy fail-closed requirement.
- **Scoped code quality: FAIL.** The principal propagation and DI change compile and the focused tests pass, but the legacy classification defect is security relevant. The connector assertion is a mock-only proof and lacks a production authorization test.
- **Full T08/T05: NOT VERIFIED.** This review does not verify T05 source-record rechecks, Task 4 joined authority validation, or T08 completion.

## Findings

### 1. High — actorless historical Craft Run can execute as Task Owner

**Evidence / affected symbol:** `durableRunActorContext` at `agent_run_graph.go:538-559` determines Craft solely from `snapshot.CraftInputManifest != nil`. If that marker is absent, an empty durable `ActorUserID` falls back to `run.UserID`, the Task storage owner. `admitAfterFollowUps` repeats this fallback at `:626-640`. The marker was added in the T05 snapshot work and `BuildDurableRunSnapshot` still produces unmarked snapshots (`:65-76`); repository admission identifies Craft independently from the `craft_sessions` row (`internal/application/repository/agent_run.go:232-244,315-326`). Thus a pre-marker Craft row with nullable `actor_user_id`, or any admitted Craft row carrying a generic snapshot, is classified as generic at execution. The current legacy test at `agent_run_graph_test.go:411-440` nulls the actor only on a *new marked* snapshot, so it does not cover this case.

**Impact:** A historical Collaborator Run with no provable actor may resume with the Owner's Caller, Principal and tool metadata if the Owner still has TaskWrite, gaining the Owner's personal authorization context. Its follow-up can also be admitted under Owner identity. This violates the Task Owner/Collaborator boundary in `CONTEXT.md`, the approved Spec, and the Task 3 legacy policy.

**Smallest defensible correction:** Determine Craft from the authoritative tenant/session registration (`craft_sessions`) or another durable server-owned Run kind, not only the new snapshot field, before either actor fallback. Reject actorless Craft rows before model/capability resolution and before follow-up admission. Add a regression test with a Craft Session, an unmarked legacy snapshot, null `actor_user_id`, and an Owner whose TaskWrite would otherwise pass; assert no model/tool call and no follow-up admission.

### 2. Medium — owner-private connector denial is not behaviorally established

**Evidence / affected test:** `agent_run_graph_test.go:297-320,323-377` uses `runActorMCPService.ListMCPServices` whose result is switched on `Caller.UserID`, then asserts the mock returned `collaborator-private`. The production `mcpServiceService.ListMCPServices` lists tenant services without actor filtering (`mcp_service.go:93-100`), while the open-connector route is a separate per-call facade (`agent_service.go:303-354`). The test separately calls `durableToolExecContext` at `agent_run_graph_test.go:373-376`; no real tool action/credential lookup is executed. It proves restored context reaches this mock, but does not prove an Owner-private connection is denied or a C-owned grant succeeds.

**Impact:** The explicit Task 3 connector-authority acceptance condition remains unverified; an owner fallback or missing grant check in the production tool path would evade this test. No production credential leak is asserted from this evidence alone.

**Smallest defensible correction:** Exercise the real connector prepare/authorization seam with persisted Owner O and Collaborator C bindings or grants, running the worker with C's actor context. Assert O's personal connection is denied, C's permitted connection succeeds, and tool metadata records C. Keep the current mock test as a context propagation unit test.

## Positive evidence and limits

`ExecuteDurableRun` validates the lease fence, restores `ActorUserID`, and checks current TaskWrite before `GetChatModel` and `prepareAgentCapabilities` (`agent_run_graph.go:353-405`). It sets `Caller`, web `Principal`, legacy user context and `ToolExecContext.UserID` to the actor (`:372-375,482-491,510-528`); `run.UserID` remains available for storage. A second lease in the focused test observes C rather than worker W or Owner O, and revoked TaskWrite prevents model/capability calls. Follow-up admission copies the durable actor and checks TaskWrite for marked Craft snapshots (`:626-670`). `NewSessionService` receives `craft.TaskAccessChecker`, and the focused container test compiles the constructor wiring. These observations apply to the reviewed hashes only.
