# T08 Craft admission actor Task 2: independent scoped review

Date: 2026-09-23. Read-only review of Task 2 and its ownership amendment in `t08-actor-principal-plan.md`, `t08-actor-task2-report.md`, the reviewed Task 1 active-membership contract, approved Craft Spec #107/#126, and six exact files in the integration worktree. HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No source/test edits, staging, delegation or OCR.

## Checkpoint and verdict

| Reviewed full file | SHA-256 |
| --- | --- |
| `internal/application/service/craft_session.go` | `7dc59052fefc22a937fd3c77a5844b7b8dca14f6b1a179c89ff2b8938d3e391d` |
| `internal/application/service/craft_inputs.go` | `bb57c4372529929d4148526847a2e24b12f0f4cb17c6ae97981f73d581571521` |
| `internal/application/service/craft_session_test.go` | `b9690a1bd0bd5a44a7c13aeae5e3ea268fbd1a161230c5c40740ba8198e410c5` |
| `internal/application/service/craft_session_acl_test.go` | `a2ad621214e884cc4dc0009dd30b472fb23ae1e5404bc23439c07ce4840e4031` |
| `internal/application/service/craft_t01_test.go` | `7e7503417a0706de93fee7d9962688985c30c079d6bdd368d8d429eb211b6ade` |
| `internal/handler/session/craft_test.go` | `ddcc0fe7fa024c468b8baf8a879178cecf5e90ca538c4ec1d732e98c8a532b48` |

All full-content hashes match the implementation report and checkpoint `after.sha256`. The report explicitly lacks a complete dirty task-entry hash baseline, and two test files were already untracked; these hashes establish the reviewed final content, not a claim that every HEAD-relative line was introduced by Task 2.

- **Scoped Spec compliance: PASS.** `StartRun` takes the actor from authenticated caller context, requires caller tenant/user to match the supplied Craft scope, checks current `TaskWrite`, stores Task owner in `Admission.UserID`, and stores the initiating actor in `Admission.ActorUserID`. Same-key replay by a different or legacy-unknown actor conflicts before touching T01 claims. The actor-bound claim digest blocks an expired abandoned claim from transfer while retaining the owner/session decision key and raw durable Run request ID.
- **Scoped code quality: PASS.** The owner/actor split is explicit in the admission call; the input claim changes are limited to digest binding and helper parameter. Focused tests cover Collaborator admission, owner replay conflict, stable token, abandoned claim, caller mismatch and existing Viewer/Admin denial paths. No scoped finding was identified.
- **Full T08/B1: NOT VERIFIED.** Worker/tool principal restoration from the durable actor, current grant rechecks on restart, joined authority validation and broad T08 gates remain Task 3/4 work. This service checkpoint cannot prove capability isolation after delegation.

## Evidence and tests

`StartRun` rejects an absent/mismatched authenticated caller before session or admission work (`craft_session.go:766-774`). `writeSession` requires `TaskWrite` and the Craft Task session under the actor scope, while the loaded session determines the owner storage scope (`:257-280,778-786`). The pre-admission lookup uses the durable owner/request key but rejects a missing or different `actor_user_id` before `claimInputDecisions` (`:802-828`). It preserves the existing plain `craft-<request_id>` durable request ID, owner-scoped deterministic message IDs, owner `UserID`, and separate `ActorUserID` (`:839-871`). The repository Task 1 contract then enforces immutable cross-actor replay during atomic admission.

`claimInputDecisions` retains the owner/tenant/session/purpose/decision-key claim identity, but hashes the raw Run request ID together with the authenticated actor into the admission digest (`craft_inputs.go:381-405`). A mismatched actor cannot match the prior decision/claim digest and cannot rotate an expired claim (`:416-508`). The claim's `AdmissionRunID` and token remain durable; the helper returns the original Run ID for same-actor recovery (`:509-529`). This keeps the T01 claim key stable and does not hash or replace `Admission.RequestID`.

`TestCraftSessionStartRunPersistsActorAndFencesContinueClaimByActor` verifies Collaborator C on Owner O's Task, the returned owner/actor fields, same-actor replay, owner same-key conflict without token rotation, and a stale unadmitted claim that O cannot take over (`craft_session_acl_test.go:143-205`). The mismatch test rejects forged scope before any Run row (`:207-217`). Existing ACL tests assert Viewer and ungranted Admin cannot write while Collaborator can; service and handler happy-path fixtures now seed active memberships rather than weakening access checks. The handler change is fixture-only and preserves HTTP assertions.

I independently ran `go test ./internal/application/service -run '^(TestCraftSession(StartRun|TaskAccess|TaskAccessRejects|ListPaginates)|TestCraftT01)' -count=1` and `go test ./internal/handler/session -run '^(TestCraftHTTP(Ow|Viewer|Routes|Body|Rejects|List|Restore|Capabilities)|TestCraftInputRoundHTTP)' -count=1`; both passed. `git diff --check` on reviewed tracked files passed. These are focused SQLite service/HTTP tests, not full worker/restart/PG proof.
