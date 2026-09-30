# T08 durable actor Task 3 review fix 1

> **For Codex:** Execute this scoped SDD repair with RED→GREEN→REFACTOR, exact uncommitted checkpoint and independent re-review. T05 may take the graph file only after this review passes.

**Source:** `2026-09-23-craft-107-t08-actor-task3-review.md` High/Medium findings, approved Craft Spec #107/T08, `CONTEXT.md`, Task3 report. Original execution BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; record current HEAD and pre-task owned-file hashes. No commits.

## Global Constraints

Craft classification derives from tenant/session registration, not snapshot marker, actor field, owner identity, or current access result. Legacy actorless Craft rows fail closed before model/capability and follow-up admission. Generic legacy runs may retain their existing behavior. Failed or inconsistent Craft classification fails closed. The owner remains a storage key; the collaborator actor controls capability authority. Preserve concurrent T05 graph edits.

## Review Focus

Unmarked historical Craft Run with null actor and owner TaskWrite; wrong tenant/session; lookup failure; marked snapshot with missing registration; follow-up path; real connector owner-private denial/collaborator grant and dispatcher call count.

## Task 1 — authoritative task kind before fallback

**Depends on:** T08 Task3 review FAIL. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/modules/craft/web_contracts.go`, `internal/application/service/craft_access.go`, `craft_access_test.go`, `internal/container/craft_access_wiring.go`, `internal/application/service/session.go`, `agent_run_graph.go`, `agent_run_graph_test.go` only. **Consumes:** `craft_sessions` tenant/session registration. **Produces:** typed `CraftTaskLookup` and composite `TaskRunAccess` (or equivalently narrow port), injected into existing `NewSessionService` provider without touching `container.go`.

1. RED tests with historical unmarked Craft snapshot, null actor and Owner with current TaskWrite: no model/tool call, no follow-up admission. Cover lookup error and marker/registration disagreement; ordinary generic legacy remains compatible. Service lookup tests must prove tenant/session scoping independent of grants.
2. Implement registration lookup via existing Craft access service and pass it through the current container adapter. Both durable execution and follow-up must classify before any owner fallback. Use actor C and current TaskWrite for all Craft cases. Do not treat a lookup error as generic.
3. Focused service/container tests, compile dependent packages, `git diff --check`; save exact task-local patch/hashes/report.

**Acceptance:** the old Craft owner-fallback path is impossible even without the new manifest marker. **Failure handling:** request exact repository seam before editing a new file; fail closed meanwhile.

## Task 2 — connector authority behavior evidence

**Depends on:** Task1 local GREEN. **Owner:** same backend_implementer. **Validator:** backend_validator. **Owned files:** focused test files in `internal/application/service/` and, after explicit ownership confirmation, `internal/modules/appconnector/service/appconnector/action_test.go` or `internal/modules/agentruntime/agent/tools/app_connector_test.go`; no production connector edits in this task.

1. Use real connector prepare/authorization storage and action guard, not an actor-filtering MCP mock, to seed Owner O and Collaborator C personal connections. Pass restored C from durable worker/tool metadata into the connector seam. Assert O-private connection/action is denied with zero dispatcher calls; C-permitted action succeeds and the dispatcher sees C.
2. Keep graph context-propagation test as a unit check; document exact seam between that test and real connector behavior. Run focused connector/service checks; save hashes and evidence.

**Acceptance:** collaborator cannot obtain Owner personal connector authority, and the real grant path is exercised. **Failure handling:** if a full graph→real connector test requires shared assembly changes, supply a precise test-only seam request before editing; do not claim the mock alone proves the acceptance.
