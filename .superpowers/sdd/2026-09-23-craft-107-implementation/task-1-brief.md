### Task 1: T01 — Input recognition and decision (#120)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend/frontend implementer. **Validator:** backend_validator + frontend_validator. **Acceptance:** #120 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/input.go; internal/application/service/craft_inputs.go; packages/views/src/craft/files.tsx; matching `*_test.go`/`.test.tsx`. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 `InputRecognition` and typed workbench input slot. **Produces:** Persist immutable input ref plus accepted/understood state and one continue/cancel command.

**Acceptance checks:**

- Unknown extension is accepted within 20 MiB/file, 20 files/round, and 100 MiB/round quotas.
- Accepted and understood are persisted and projected as separate outcomes.
- Executable binary content is not misclassified as harmless parsed text.
- Cancel submits no Run; continue submits exactly once and retains the opaque read-only reference.
- Empty, oversized, over-count, over-total, digest-mismatch, and hostile-name inputs fail closed.
- Content-addressed identity and read-only materialization remain intact.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: unknown extension within quotas accepted/unrecognized; binary not parsed; continue once; cancel zero Runs; empty, hostile name, digest mismatch and all quota edges rejected. Name the Go test `TestCraftT01Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT01Journey -count=1`. Expected: `TestCraftT01Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Implement quota and digest checks at upload seam; store opaque recognition separately from acceptance; require a server-acknowledged decision before Run admission; project warning with TDesign control. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT01Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Input -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run 'CraftInput|Attachment' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run 'Attachment|CraftInput' -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Failure handling:** Any submitted Run after cancel or duplicate Run after continue blocks verification; fix admission idempotency before integration. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

