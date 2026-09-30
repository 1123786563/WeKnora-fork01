# T08 durable actor Task 3 fix 1: independent scoped re-review

Date: 2026-09-24. Read-only review against the fix plan, Task 3 High/Medium findings, approved Craft Spec #107/T08, `CONTEXT.md`, and ADR-0004/0009. No OCR, source/test changes, or delegation. HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; the shared worktree is dirty, so HEAD-relative diff includes earlier T05/T08 work and is not a task-local patch. The report supplies before/after full-file hashes, but no full task-local patch.

## Checkpoint and verification

All nine reported final SHA-256 hashes match the files reviewed:

| Owned file | Verified final SHA-256 |
| --- | --- |
| `internal/modules/craft/web_contracts.go` | `effbfcfff1ff261c66da9a6b3367b13e1aa06fab2cae995ffaedb1f7742a4bbc` |
| `internal/application/service/craft_access.go` | `51361a099ef5111d895c73414a112629cd4a31af51c2661587d487ceb8f25eca` |
| `internal/application/service/craft_access_test.go` | `9c696fb14695e370937a8f9d6379d58bf0918032a1f7e411a02760610965effe` |
| `internal/container/craft_access_wiring.go` | `86db3aed871adeabb68075832b9a18bcdd1ebe101c6f9004e571b3d3b052c177` |
| `internal/application/service/session.go` | `1b47ca484e99fda16fedd32a237df2f2453dbcffcc2ffb3dec25576547f8740e` |
| `internal/application/service/agent_run_graph.go` | `5bc1a7a7ddc882ee5b12dd068fbd96fa4c32c78ba0ef5c106288d413de90e4d0` |
| `internal/application/service/agent_run_graph_test.go` | `e1ffc73d394e86a1e6a0a934e5032cf273aa37d6262ac3b42318d1e3096d3b1b` |
| `internal/application/service/craft_delegate_test.go` | `9fc96eec5147df2ba3dc58b0515ec59139133e7554276b5c66f3a074378329b9` |
| `internal/modules/appconnector/service/appconnector/action_test.go` | `fe840c61d7c33222e1285b310efeb0de832148e6948f79e6a57fcf53bf86c6f1` |

The three files from the previous Task 3 review match that review's final hashes as this round's reported before hashes; the remaining six before hashes are only independently available from the fix report, not a saved task-entry patch. I inspected the nine current files and relevant HEAD diff without attributing concurrent hunks to this fix. Focused service classification/tool-subject tests, the connector ActionService test, and the container DI test passed independently. `git diff --check` for the nine tracked files passed. Commands: `go test ./internal/application/service -run '^(TestCraftTaskLookupIsTenantAndSessionScopedIndependentOfGrant|TestExecuteDurableRunRejectsUnmarkedLegacyCraftRunWithoutActor|TestDurableRunClassificationLookupFailureAndManifestDisagreementFailClosed|TestExecuteDurableGenericLegacyRunRetainsOwnerFallback|TestDurableCraftActorFlowsThroughAppConnectorToolSubject)$' -count=1`; `go test ./internal/modules/appconnector/service/appconnector -run '^TestActionServiceRechecksPersonalConnectionAgainstPersistedCraftActor$' -count=1`; `go test ./internal/container -run '^TestCraftAccessFeatureUsesPersistentPolicyOnConstrainedRoutes$' -count=1` (duplicate `-lc++` linker warning only).

## Verdict

- **Scoped Spec compliance: FAIL.** The primary unmarked active-Craft case is fixed. Classification still returns “generic” for a retained Craft registration when the Session is soft-deleted, and for an absent/mismatched Session, permitting the owner fallback before any model/capability call. This conflicts with the fix plan's failed/inconsistent-classification fail-closed rule.
- **Scoped quality: FAIL.** The new linked connector tests improve coverage and the focused checks pass, but the classification query conflates “not a Craft Task” with “Craft Task whose Session is unavailable.”
- **T05/full T08: NOT VERIFIED.** No conclusion on T05 source authorization or complete T08 acceptance.

## Finding

### High — missing or soft-deleted Session is classified as generic

**Evidence / affected symbols:** `CraftAccessService.IsCraftTask` in `craft_access.go:128-140` uses an inner join from `sessions` to `craft_sessions` and filters `s.deleted_at IS NULL`. A soft delete preserves the `craft_sessions` row (the migration cascades it only on physical deletion), but this query returns `(false, nil)`. A missing Session or tenant/session mismatch also returns `(false, nil)`, rather than an error. `durableRunActorContext` at `agent_run_graph.go:545-560` then treats an unmarked actorless historical Run as generic and substitutes `run.UserID` before model/capability resolution. `admitAfterFollowUps` repeats the classification/fallback at `:637-661`. The new lookup test covers active Craft and active generic Sessions, but does not cover a retained Craft registration with `sessions.deleted_at` set or a missing Session.

**Impact:** A retained actorless Craft Run can regain the Task Owner's principal if execution is attempted after a direct service soft delete or inconsistent registration state and its lease remains valid; at minimum model and capability resolution can proceed under the wrong identity. The ordinary HTTP delete path fences Runs first, which limits that path, but the classification helper itself does not enforce the required fail-closed invariant. Wrong tenant/session data is likewise not distinguished from a valid generic Session.

**Smallest defensible correction:** Resolve the tenant/session row independently and return an error if it is missing, mismatched, or deleted; classify Craft from its durable registration even when the Session is soft-deleted, or reject deleted Sessions before any principal fallback. Only an existing active Session with no Craft registration may return `(false, nil)`. Add lookup and worker regressions for soft-deleted registered Craft and missing/wrong tenant Session, including no model/capability call and no follow-up admission.

## Resolved findings and positive evidence

For active Sessions, `IsCraftTask` reads tenant/session registration independent of grants. An unmarked actorless Craft Run is rejected before model access, and follow-up admission independently rejects it. Lookup errors and a manifest without registration are rejected; active generic legacy Runs retain owner fallback. The worker sets Caller, Principal, legacy user, and tool metadata to the durable actor before capability assembly. The constructor uses the composite port through the existing container provider and compiles.

The connector tests now link two real seams: `TestDurableCraftActorFlowsThroughAppConnectorToolSubject` passes restored C through `ToolExecContext` into the actual `AppConnectorTool` subject; `TestActionServiceRechecksPersonalConnectionAgainstPersistedCraftActor` uses persisted connection/action rows and the A02 guard to deny O's private connection with zero dispatch, then dispatch C's own connection once with C in the snapshot. They are linked seam tests rather than one graph-to-facade integration, as the report accurately states. That addresses the earlier mock-only evidence gap at the scoped behavior level.
