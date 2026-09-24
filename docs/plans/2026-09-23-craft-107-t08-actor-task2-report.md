# T08 Task 2 — Craft admission actor report

Date: 2026-09-23. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Plan: `docs/plans/2026-09-23-craft-107-t08-actor-principal-plan.md`, Task 2. No commit or subagents. Model/effort metadata was not exposed by this runtime and is not asserted.

## Result

`CraftSessionService.StartRun` now accepts an actor only from `types.CallerFromContext`, requires the authenticated tenant/user to match the supplied Craft scope, and rejects absent/mismatched identity before durable admission. The `Admission.UserID` remains the stored Task owner; `Admission.ActorUserID` carries the authenticated Run actor. Existing TaskWrite checks remain the gate for collaborator starts; Viewer/Admin without a Task grant are denied by the existing ACL path.

Same-owner/idempotency-key lookup reads the durable actor and rejects a different or legacy-unknown actor before touching T01 input claims. The T01 input claim key remains `(tenant, owner, session, purpose, decision-key)`, and the raw Run request ID is preserved for replay/recovery. The actor is mixed into the claimed decision digest so an expired claim with no durable Run cannot be rotated by another actor. Tests assert same-actor replay, cross-actor conflict with unchanged token, stale claim takeover refusal, owner storage identity, and caller/scope mismatch rejection.

Focused service-test fixtures now represent active users and membership, including a direct T01 recovery admission with an explicit owner actor. The approved handler-test fixture amendment seeds active owner membership; production handler code and HTTP assertions were not changed.

## TDD and verification

RED failures were observed during implementation: (1) actor admission/caller mismatch behavior returned the runtime conflict instead of the expected Collaborator admission or `craft.ErrForbidden`; (2) an intermediate hashed Run request ID caused same-actor T01 replay to fail with `craft conflict: admitted input claim has no matching Run`; (3) handler owner happy paths returned 409 because the fixture had no active owner membership. Corrections retained the raw Run request ID, made the digest actor-bound without changing the claim key, added valid test memberships, and supplied explicit actor in direct claim-helper tests.

The following commands passed:

- `go test ./internal/application/service -run '^TestCraftSession(StartRunPersistsActorAndFencesContinueClaimByActor|StartRunRejectsScopeThatDiffersFromAuthenticatedCaller)$' -count=1` — PASS (`2.530s`).
- `go test ./internal/application/service -run '^(TestCraftSession(StartRun|TaskAccess|TaskAccessRejects|ListPaginates)|TestCraftT01)' -count=1 && git diff --check` — PASS (`7.587s`; diff check passed).
- `go test ./internal/handler/session -run '^(TestCraftHTTP(Ow|Viewer|Routes|Body|Rejects|List|Restore|Capabilities)|TestCraftInputRoundHTTP)' -count=1 && git diff --check` — PASS (`9.112s`; diff check passed).

Exact test evidence is in `.superpowers/sdd/2026-09-23-craft-107-implementation/t08-actor-task2-checkpoint-01/test-evidence.txt`.

## Scope and checkpoint

Changed source / test files:

- `internal/application/service/craft_session.go` — caller binding, actor-aware prior-run replay check, pass ActorUserID, preserve Task owner storage identity.
- `internal/application/service/craft_inputs.go` — explicit actor parameter to input decision claim helper and actor-bound claim digest while preserving raw Run request ID and owner-scoped claim key.
- `internal/application/service/craft_session_acl_test.go` — actor admission, replay, stale-claim, and mismatch tests plus active user fixture.
- `internal/application/service/craft_session_test.go` — seed active owner memberships for service tests.
- `internal/application/service/craft_t01_test.go` — explicit actor for direct T01 claim/recovery paths and active owner membership fixture use.
- `internal/handler/session/craft_test.go` — approved fixture-only active owner membership for HTTP happy paths; assertions unchanged.

The exact final source copies and hashes are in `.superpowers/sdd/2026-09-23-craft-107-implementation/t08-actor-task2-checkpoint-01/after/` and `after.sha256`. HEAD is `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created. `baseline-head/` preserves each changed tracked file as committed at HEAD and `baseline-head.sha256` records those blob hashes.

This task resumed in a shared, already-dirty Worktree. A complete task-entry hash manifest was not captured before the task edits; therefore the archived HEAD sources are explicitly only committed baselines, not a reconstruction of the dirty pre-task Worktree. Two owned tests (`craft_session_acl_test.go`, `craft_t01_test.go`) were already untracked at task entry, so no committed baseline exists for them. Final hashes below bind the current exact content.

Final SHA-256:

```text
7dc59052fefc22a937fd3c77a5844b7b8dca14f6b1a179c89ff2b8938d3e391d  internal/application/service/craft_session.go
b9690a1bd0bd5a44a7c13aeae5e3ea268fbd1a161230c5c40740ba8198e410c5  internal/application/service/craft_session_test.go
a2ad621214e884cc4dc0009dd30b472fb23ae1e5404bc23439c07ce4840e4031  internal/application/service/craft_session_acl_test.go
bb57c4372529929d4148526847a2e24b12f0f4cb17c6ae97981f73d581571521  internal/application/service/craft_inputs.go
7e7503417a0706de93fee7d9962688985c30c079d6bdd368d8d429eb211b6ade  internal/application/service/craft_t01_test.go
ddcc0fe7fa024c468b8baf8a879178cecf5e90ca538c4ec1d732e98c8a532b48  internal/handler/session/craft_test.go
```

HEAD-relative hashes for the already tracked files before this report work were:

```text
418725eeeeeaf98e11007ed8519db99078384b8ff8e6bdb4c32375b3deb5f465  internal/application/service/craft_session.go
502e17f4bf1b7af2a50b4645f332133f88047b3177a74df1cca3bb33173d4f79  internal/application/service/craft_session_test.go
5a8982e51f27e86201f239376e95829b1fd45b357f44b802668ee40ad7304731  internal/application/service/craft_inputs.go
26415efec28939c1d00d49749522f09dafb7b498420aec350ea089df17a80882  internal/handler/session/craft_test.go
```

## Remaining gates

This is only T08 actor admission Task 2. Task 3 must restore the durable actor in worker/tool capability context and recheck current grants; Task 4 joined authority validation and independent review remain. Full Craft/T05 production remains gated on central Publisher, per-Run sandbox/read isolation, and dispatch-time authorization as tracked by integration. No worker/principal restoration or production end-to-end claim is made here.
