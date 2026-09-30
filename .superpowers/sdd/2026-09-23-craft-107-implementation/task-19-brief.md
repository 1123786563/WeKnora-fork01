### Task 19: T19 — Durable Task Budget pause (#138)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #138 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/budget.go; internal/application/service/craft_budget.go; packages/views/src/craft/usage.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 BudgetPause typed record; tenant policy and Task Budget. **Produces:** Idempotent model/sandbox charge attribution and paused Run with authorized extension.

**Acceptance checks:**

- Budget admission occurs before model or sandbox activity.
- Every chargeable activity is attributed once to tenant and Task Budget.
- Limit prevents new chargeable calls and persists budget-paused reason.
- Paused Run cannot promote a version and survives reload.
- UI identifies authorized owner/billing-admin action without exposing money/secrets improperly.
- Unauthorized member cannot extend/resume; authorized extension resumes safely without replaying unknown effects.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: limit at admission; delegated sandbox/model both charge once; refresh paused; nonowner extend denied; authorized resume avoids unknown replay. Name the Go test `TestCraftT19Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT19Journey -count=1`. Expected: `TestCraftT19Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Reserve budget before chargeable work; attribute each activity once by stable ID; persist pause reason and allowed action; check owner/billing-admin grant before extension. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT19Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Budget -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftBudget|CraftExecutionBudget' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run CraftUsage -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any chargeable call after limit or paused Run promotion blocks T20. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

