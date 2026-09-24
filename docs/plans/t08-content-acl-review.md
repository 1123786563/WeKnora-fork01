# T08 central-B B1 content ACL/list: independent scoped review

Date: 2026-09-23. Read-only review in the integration worktree at HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft web-artifact Spec, #126 acceptance mapped in the DAG, `CONTEXT.md`, T08 central-B design, B1 plan/report and exact saved checkpoint. No production/test edits, staging, OCR or test-source changes.

## Checkpoint and verdict

The checkpoint manifest `docs/plans/2026-09-23-craft-107-t08-content-acl-checkpoint.json` has SHA-256 `83c621b0919d51b3c82c7ec634477aac6ebbb611e78738eabb173924702c97a9`; the saved seven-file patch has SHA-256 `cff3ae28eed94bd767836d55b05440856e01057e671615a86e9a6536231e5978`. All seven live full-file hashes matched the manifest when reviewed:

| Scoped file | Full-content SHA-256 |
| --- | --- |
| `internal/application/service/craft_session.go` | `f20a2e28762aa2550e8567e2bf464ebed3c5aca815a8001255607b60e114613a` |
| `internal/application/service/craft_session_acl_test.go` | `2970bb54259d5316436d6c36c9ec84dccf85811537487c43357c3178eb4fbbe6` |
| `internal/application/service/craft_session_test.go` | `c47fdf43cad9b491b6e7f61bda019aa23fd9cddfdafcb9c06059350ff643762b` |
| `internal/application/service/craft_task_list_query.go` | `eadc24be2f15343c273dcb790015ea23507b28ffd7eaeb82351d9d7953399a2d` |
| `internal/container/container.go` | `9be537ce08a4b19d50af47df8740c5f5a441c5a40cc5ed6a7fb4d7bd6f115d84` |
| `internal/container/craft_session_provider_test.go` | `ec38292300c7b7779b948b40879c3bb0a4243d7131c164dd1106088da142edc6` |
| `internal/handler/session/craft_test.go` | `622dbc984d4af483057c746194f7d0c27d631dda5ceef0cbf58623f550380950` |

The saved patch includes T01 changes already present at the shared HEAD; I treated B1's new ACL/list code and narrow test expectation changes as the scoped delta. Concurrent T19/RunView edits are outside this checkpoint.

- **Scoped Spec compliance: FAIL (High finding below).** Owner/grantee/Viewer access checks and grant-aware list behavior align with most B1 acceptance, but a Collaborator who may now start a Run causes it to execute under the Task Owner's principal. #126 and `CONTEXT.md` expressly deny inheritance of the Owner's personal credentials and source access.
- **Scoped code quality: FAIL (same High finding).** Storage ownership and execution identity are conflated at admission. The new Collaborator success test checks only `Run.SessionID`, not the persisted actor or tool authority. Final B1 tests against this exact checkpoint did not compile due concurrent T19 source, per the implementation report; a prior narrower state passed focused service/container tests.
- **Full T08: NOT VERIFIED.** B2 generic direct Session metadata, B3 preview revocation, B5 joined HTTP validation and final focused Go reruns remain open. This review does not claim T08 or #126 acceptance.

## High — Collaborator-triggered Run inherits Owner execution identity

**Evidence / affected symbols:** `writeSession` authorizes a Collaborator with current `TaskWrite` (`internal/application/service/craft_session.go:257-269`); `StartRun` consumes that result (`:766-777`). It then submits `agentruntime.Admission{UserID: session.UserID}` where `session.UserID` is the persisted Task Owner, not `scope.UserID` (the Collaborator) (`:846-858`). The repository persists that value as `agent_runs.owner_id`, then exposes it as `run.UserID` (`internal/application/repository/agent_run.go:58-65, 345-348`). The durable executor forwards `run.UserID` as the tool execution user (`internal/application/service/agent_run_graph.go:469-485`). The snapshot includes `DefaultAllowedTools`, including knowledge and `search_conversations` (`craft_session.go:949-977`; `internal/modules/agentruntime/agent/tools/definitions.go:115-125`). The executor separately injects `run.Owner`, which is the *worker lease owner*, as a web-user principal (`agent_run_graph.go:361-370`); this is also not the Collaborator. The new Collaborator test only asserts that `StartRun` succeeds and `run.SessionID` matches (`internal/application/service/craft_session_acl_test.go:75-84`).

**Impact:** A Collaborator can trigger a Run persisted and presented with the Task Owner as `UserID`, and tools receive that Owner ID in their execution metadata. This can select Owner-scoped data or credentials in consumers of that metadata and misattributes the initiating actor. The separate worker-lease principal in capability assembly further fails to carry the Collaborator's authority. It also conflicts with collaborator control checks that compare `run.UserID` to the caller (`craft_control.go`), potentially making a Collaborator unable to control the Run they requested. The exact personal-data exposure requires joined execution testing, but the identity crossing is explicit and violates #126's boundary.

**Smallest defensible correction:** Keep the Owner identity solely for workspace/session storage and serialization. Carry the authenticated requesting Collaborator as a distinct durable Run actor/tool principal, and rebuild tool/source capabilities under that actor's current grants. Preserve an explicit Owner-only authority path where an operation truly requires Owner approval; do not simply replace the store owner key with the Collaborator, because existing Run/Workspace fencing and T01 claims use the Owner scope. Add a focused Collaborator admission/execution test proving actor attribution and denial of Owner-only source/connector access, plus revocation behavior before execution.

## Confirmed B1 behavior and remaining limits

`readSession`/`writeSession` check current `TaskRead`/`TaskWrite` before loading the persisted session and deriving Owner-scoped storage (`craft_session.go:232-302`). `Get`, `View`, `WorkspaceInputs`, version reads and `OpenVersionFile` follow the read path; `AssociateInput` and `StartRun` follow the write path (`:415-475, 605-630, 702-710, 766-783, 995-1041`). A denied download returns before `GetFile`, with a nil reader; the focused test also tracks zero object opens (`craft_session_acl_test.go:138-152`). Production `newCraftSessionService` injects the concrete `CraftAccessService` for both ports (`container.go:2520-2540`). The owner-only compatibility assembly no longer inherits generic Session Admin read fallback; the two Craft-specific Admin expectations were changed accordingly in `craft_session_test.go` and `handler/session/craft_test.go`. No generic Session implementation was changed.

The list query filters current active tenant membership incarnation and explicit Viewer/Collaborator grant (or Owner) in SQL before keyset pagination, capped at 101 rows (`craft_task_list_query.go:40-83`; `craft_session.go:507-567`). Tests cover owner+grant order, equal timestamps, cursor replay, revocation, stale membership, cross tenant and Admin-without-grant (`craft_session_acl_test.go:166-247`). This is a bounded query, not an in-memory filter of an already paginated owner list. The test additions in `craft_session_test.go` concerning recognition decisions and snapshot manifest preserve the separately reviewed T01 behavior; they are not evidence that B1 changed the T01 claim implementation.

`git diff --check` on the seven owned files passed. I did not rerun the focused Go packages because the report records that concurrent T19 `agent_run_lifecycle.go` fails compilation (`undefined: errors`) before B1 tests, and the exact-checkpoint Go test gate remains open. The prior passing targeted tests bind an earlier B1 state; they cannot certify the final seven-file hash set. The authenticated joined HTTP role/revocation path remains a B5 requirement.
