### Task 5: T05 — Selected knowledge and actual sources (#124)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #124 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/knowledge.go; internal/application/service/craft_knowledge.go; packages/views/src/craft/sources.tsx; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 source/restricted-contribution facts; current caller grant. **Produces:** Selected∩authorized retrieval and immutable actual-source record with ref,digest,time,bounded excerpt.

**Acceptance checks:**

- Retrieval scope equals selected scope intersected with current caller authorization.
- Unselected and unauthorized knowledge never enters the material bundle.
- Actual sources persist stable reference, digest, acquisition time, and bounded excerpt metadata.
- Empty and truncated source results are disclosed.
- Every source open re-resolves through the authorization chain rather than following a model URL.
- Permission revocation is honored on later access.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: select KB A while B exists; revoke A; empty/truncated results; direct source open as unauthorized Viewer. Name the Go test `TestCraftT05Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT05Journey -count=1`. Expected: `TestCraftT05Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Intersect Task selection with current caller authorization before retrieval and again at open; persist only actually used refs and bounded metadata; project empty/truncated warnings. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT05Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Knowledge -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run CraftKnowledge -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Craft.*Source' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any unselected or unauthorized source entering delegated material blocks T06 and T10. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

