# T08 Actor Membership Admission Fix Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint and independent Spec/quality re-review. No commit.

**Goal:** Resolve review finding T08-A1 (Medium): durable Run admission must validate the initiating actor's current active tenant membership rather than their home tenant field.

**Sources:** `t08-actor-admission-review.md`, `t08-actor-principal-plan.md` Task 1, `craft_access.go` activeMemberID, `tenant_members` model/repository, approved Spec #107/#126.

**Global Constraints:** Preserve owner storage/session/InputClaim scope, immutable ActorUserID, cross-actor replay conflict and generic Run compatibility. TaskWrite remains the Craft service Task 2 check; repository verifies only active actor/user tenant membership. No file ownership overlap: this fix owns `agent_run.go` and focused actor admission tests only until independent re-review.

**Review Focus:** cross-home-tenant invited Collaborator accepted with active membership; suspended/removed/deleted membership and inactive/deleted user rejected; same-actor replay behavior and T01 claim tests unchanged. Query current membership in the same admission transaction; no `users.tenant_id` shortcut.

## Task 1 — correct actor membership predicate

**Depends on:** T08 actor admission Task 1 first review FAIL and exact checkpoint. **Role:** backend_implementer. **Owned files:** `internal/application/repository/agent_run.go`, `agent_run_actor_test.go`, existing `agent_run_test.go` only if a test fixture needs targeted adjustment; report/checkpoint. **Produces:** `actorBelongsToTenant` based on active undeleted `tenant_members` joined to active undeleted `users`.

1. RED tests: actor home tenant A with active membership B can admit B Craft Run; home tenant B with suspended/removed B membership cannot; inactive/deleted user cannot; generic cross-home active membership stays valid.
2. GREEN: replace the home-tenant predicate with an exact `(member.tenant_id, member.user_id)` active membership + active user join in the existing transaction. Follow the canonical member status/soft-delete semantics used by `CraftAccessService.Role`; keep TaskWrite out of this repository helper.
3. Run focused admission/T01/runtime tests, `git diff --check`, save full before/after content hashes and task delta. Independent reviewer rechecks T08-A1 and the original Task 1 invariants. Only after PASS may T08 Task 2 and T19 budget-pause repository edits take `agent_run.go` ownership.

**Failure handling:** If fixtures lack `tenant_members`, adjust only test setup; do not weaken production membership predicate to restore test green. PostgreSQL repository behavior remains an explicit evidence gap when DSN is absent.
