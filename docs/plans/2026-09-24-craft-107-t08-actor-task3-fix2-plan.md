# T08 actor Task 3 classification fix 2

> **For Codex:** Repair only the remaining High classification finding in `2026-09-24-craft-107-t08-actor-task3-fix1-review.md` with RED→GREEN and an exact checkpoint, then obtain independent re-review.

**Source:** approved Spec #107/T08, Task3 fix1 plan/review. Original execution BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; record current HEAD and pre-task owned-file content/hashes. No commits.

## Global Constraints

Only a proven existing active tenant/session with no Craft registration may return `isCraft=false, nil`. A retained Craft registration with a deleted Session must be rejected or classified Craft before actor fallback; missing, deleted or mismatched Sessions fail closed. Do not change owner/actor authority or generic active-session compatibility. T05 graph ownership remains parked until re-review PASS.

## Review Focus

Soft-deleted registered Craft, missing Session, wrong tenant Session, active generic Session, actorless Run model/capability and follow-up call counts. Avoid an inner join that silently converts an inconsistent row to generic.

## Task 1 — distinguish generic from absent/deleted Craft

**Depends on:** Task3 fix1 review FAIL High. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/application/service/craft_access.go`, `craft_access_test.go`, and `agent_run_graph_test.go` only. **Consumes:** tenant/session row plus `craft_sessions` registration. **Produces:** fail-closed `IsCraftTask` result with no owner fallback on missing/deleted/inconsistent Session.

1. RED lookup tests for soft-deleted registered Craft, missing/wrong-tenant Session and active generic; RED graph test for unmarked actorless retained Craft after Session soft delete, asserting zero model/capability calls and no follow-up.
2. Query tenant/session independently, require existence and active state; query Craft registration separately. Return an error for missing/deleted Session; return `false,nil` only for active generic. Keep lookup errors fail closed in both existing graph paths.
3. Run focused service/container/connector checks as affected, `git diff --check`; save task-local patch, pre/post hashes and report.

**Acceptance:** no historical Craft row can enter generic owner fallback due to Session absence/deletion. **Failure handling:** if schema prevents reliable lookup, return an error and keep execution stopped; do not infer generic from empty join result.
