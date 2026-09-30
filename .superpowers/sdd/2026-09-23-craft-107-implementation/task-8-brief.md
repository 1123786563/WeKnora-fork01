### Task 8: T08 — Private Task roles (#126)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #126 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/access.go; internal/application/service/craft_access.go; internal/handler/session/craft_access.go; packages/views/src/craft/access.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 ACL/route/workbench slots; session authentication. **Produces:** Private-by-default Owner/Collaborator/Viewer grant and revocation audit.

**Acceptance checks:**

- Task is private to its owner by default.
- Owner can add/revoke Collaborator and Viewer roles; cross-tenant membership is refused.
- Viewer is read-only and Collaborator does not inherit owner credentials, personal connectors, or source access.
- Same-tenant membership and ordinary admin status do not grant private content access.
- Revocation applies to listing, direct access, preview, and download.
- Authorization is server enforced and audited.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: same-tenant nonmember, tenant admin, cross-tenant add, Viewer write, revoked preview/download/list/direct read. Name the Go test `TestCraftT08Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT08Journey -count=1`. Expected: `TestCraftT08Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Persist explicit membership scoped by tenant+Task; enforce on every service operation; audit add/revoke; project role-limited controls via TDesign. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT08Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'SessionShare|Craft.*Access' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Share|Craft.*Access' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any unauthorized direct HTTP access blocks all downstream access tasks. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

