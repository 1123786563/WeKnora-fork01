# T08 Task 4 joined authority validation

Date: 2026-09-24. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Validation is bound to integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34` plus the shared uncommitted source state below. Read-only validation; only this report was added. No source/test edits, remote issue updates, or child agents.

## Verdict

**DONE_WITH_CONCERNS — Task 4 behavior is supported by passing focused checks and same-revision Task 1–3 evidence, but the acceptance is composed of service/repository/connector seams rather than one authenticated HTTP-to-tool journey.** Do not interpret this as full T08 B1/B2/B3/B5 completion or final OCR. Remote #126 was not changed.

## Revision and relevant source hashes

HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

Initial validation hashes (the two Task 3 files whose fix3 reports establish final reviewed hashes are included):

```text
920ce165ef5c5c79a6e27cceeeca002151868905297c344d8bfb07f914883cae  internal/application/repository/agent_run.go
7dc59052fefc22a7a937fd3c77a5844b7b8dca14f6b1a179c89ff2b8938d3e391d  internal/application/service/craft_session.go
a24824636b4a8a89ff38d55e5f108dfbd8771976c23bcd5d8fd7ac572e6597fa  internal/application/service/agent_run_graph.go (final recheck)
cdd3d0e2bdfd26bfcf70cbde454b2c0a5878838eb47e9c5e3043dfafb0f4437a  internal/application/service/agent_run_graph_test.go
fe840c61d7c33222e1285b310efeb0de832148e6948f79e6a57fcf53bf86c6f1  internal/modules/appconnector/service/appconnector/action_test.go
```

The Craft session hash is `7dc59052fefc22a937fd3c77a5844b7b8dca14f6b1a179c89ff2b8938d3e391d`, matching Task 2 review. The graph source moved during validation as adjacent T05 work proceeded: initial hash `5bc1a7a7ddc882ee5b12dd068fbd96fa4c32c78ba0ef5c106288d413de90e4d0`, final recheck hash `a24824636b4a8a89ff38d55e5f108dfbd8771976c23bcd5d8fd7ac572e6597fa`. The focused worker/legacy test command was rerun after that movement and passed on the final hash. The Task 3 fix3 review's graph hash predates this T05 delta; the test file hash remains `cdd3d0e2bdfd26bfcf70cbde454b2c0a5878838eb47e9c5e3043dfafb0f4437a`. These are checkpoint observations in a shared worktree.

## Acceptance evidence

- Owner/Collaborator/Viewer/Admin Task access and Run admission: `craft_session_acl_test.go` exercises Collaborator TaskWrite and Run admission, Viewer denial, and Admin without grant denial. Task 2 report/review records the authenticated caller binding and active membership checks. The report's focused service and HTTP tests passed at this HEAD. `go test ./internal/application/service -run '^TestCraftSessionStartRunPersistsActorAndFencesContinueClaimByActor$' -count=1 -v` passed here; this asserts owner storage identity versus collaborator actor, same-actor retry, same-key owner conflict, and claim non-transfer.
- Same-key cross-actor repository replay denial and missing/legacy actor: `go test ./internal/application/repository -run '^(TestCraftRunAdmissionStoresOwnerAndAuthenticatedActorSeparately|TestCraftRunAdmissionRejectsCrossActorReplayAndCrossTenantActor|TestCraftRunAdmissionFailsClosedWithoutActorAndForLegacyReplay)$' -count=1` passed.
- Worker restart and actor/tool identity: `go test ./internal/application/service -run '^(TestExecuteDurableCraftRun(RestoresActorAcrossWorkerLeaseRecovery|RechecksTaskWriteBeforeModelResolution|RejectsLegacyMissingActorBeforeCapabilities)|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor(AfterSessionDelete)?)$' -count=1` passed. This checks W1→W2 recovery, actor context/tool metadata, revoked TaskWrite before model/capability resolution, and both active-session and deleted-session unmarked actorless historical Craft rows failing closed. The fix3 review separately records each legacy test independently passing and confirms no model/MCP calls or follow-up admission.
- Owner-private connector denied; actor-owned connection action succeeds: `go test ./internal/modules/appconnector/service/appconnector -run '^TestActionServiceRechecksPersonalConnectionAgainstPersistedCraftActor$' -count=1` passed. The test uses the persisted action actor, denies dispatch through the owner's personal connection, allows the collaborator-owned connection, and records collaborator attribution.
- `git diff --check` passed after the focused checks.

Same-revision task evidence reused: `t08-actor-task2-report.md` and `t08-actor-task2-review.md`; `t08-actor-task3-report.md`, `t08-actor-task3-review.md`, and `2026-09-24-craft-107-t08-actor-task3-fix3-review.md`. Task3 fix3 review is scoped PASS; Task 2 review is scoped PASS. The original Task 3 review's connector concern is addressed by the real `ActionService` test above. These do not collectively certify full B1 or other T08 gates.

## Commands and results

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`.

```text
git rev-parse HEAD
=> a5e9195acd6500c085c85d60c852148e7bbbbf34

sha256sum internal/application/repository/agent_run.go internal/application/service/craft_session.go internal/application/service/agent_run_graph.go internal/application/service/agent_run_graph_test.go internal/modules/appconnector/service/appconnector/action_test.go
=> hashes recorded above

go test ./internal/application/service -run '^(TestCraftSession(StartRun|TaskAccess|TaskAccessRejects)|TestExecuteDurableCraftRun(RestoresActorAcrossWorkerLeaseRecovery|RechecksTaskWriteBeforeModelResolution|RejectsLegacyMissingActorBeforeCapabilities)|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor(AfterSessionDelete)?)$' -count=1
=> PASS

go test ./internal/application/repository -run '^(TestCraftRunAdmissionStoresOwnerAndAuthenticatedActorSeparately|TestCraftRunAdmissionRejectsCrossActorReplayAndCrossTenantActor|TestCraftRunAdmissionFailsClosedWithoutActorAndForLegacyReplay)$' -count=1
=> PASS

go test ./internal/modules/appconnector/service/appconnector -run '^TestActionServiceRechecksPersonalConnectionAgainstPersistedCraftActor$' -count=1
=> PASS

go test ./internal/application/service -run '^TestCraftSessionStartRunPersistsActorAndFencesContinueClaimByActor$' -count=1 -v
=> PASS

go test ./internal/application/service -run '^(TestExecuteDurableCraftRun(RestoresActorAcrossWorkerLeaseRecovery|RechecksTaskWriteBeforeModelResolution|RejectsLegacyMissingActorBeforeCapabilities)|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor(AfterSessionDelete)?)$' -count=1
=> PASS against final `agent_run_graph.go` hash `a24824636b4a8a89ff38d55e5f108dfbd8771976c23bcd5d8fd7ac572e6597fa`

git diff --check
=> PASS

go test ./internal/application/service ./internal/application/repository ./internal/modules/appconnector/service/appconnector ./internal/container
=> INTERRUPTED after about 2 minutes; no package verdict. `repository.test` was still running. `container.test` emitted duplicate `-lc++` linker warning before interruption.
```

## Acceptance gaps and risks

- The checked evidence joins real service admission, repository replay, worker recovery, and real connector authorization across package seams, but not one authenticated HTTP request through worker restart to connector action. The Task 2 HTTP suite primarily retains owner happy-path coverage; it does not establish that entire integrated sequence.
- Admin denial is explicitly exercised at the service ACL seam. It was not re-run as part of a full auth-token HTTP path in this validation.
- The broader package command was stopped due to runtime; no full-package green result is claimed. Focused tests and same-revision reviewed task evidence passed.
- The worktree is shared and has extensive uncommitted edits. HEAD is only the committed baseline. No attempt was made to attribute all dirty hunks or to recheck unrelated source.
- Scope is only T08 actor Task 4. T08 B2/B3/B5, other product/runtime acceptance, final OCR, and updating/closing remote Issue #126 remain outside this validation.
