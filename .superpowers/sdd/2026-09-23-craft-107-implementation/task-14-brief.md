### Task 14: T14 — Isolated no-egress preview (#129)

**Depends on:** T00 (native edges, reviewed and integrated). **Owner:** backend implementer. **Validator:** backend_validator. **Acceptance:** #129 criteria below and mapped parent stories in the DAG.

**Files owned:** internal/modules/craft/preview.go; internal/application/service/craft_preview.go; internal/handler/session/craft_preview.go; deployment preview network policy; focused tests. Paths marked “extension” may be touched only after controller grants an exclusive lock; central files and migration numbering remain T20-owned.

**Interfaces — Consumes:** T00 preview/Version typed facts and extension slot. **Produces:** Short-lived scoped ticket and isolated-origin manifest-only file service.

**Acceptance checks:**

- Preview origin differs from app origin and receives no app cookies/storage/authorization headers.
- Short-lived capability binds Task, tenant, Version, and allowed files.
- Traversal, escaped paths, cross-version and cross-tenant use fail.
- Infrastructure denies HTTP, WebSocket, DNS, internet, and enterprise-network egress; CSP is defense in depth.
- Only version-manifest files are served.
- A real browser loads valid local HTML/CSS/JS/assets.

- [ ] **Step 1 — RED:** At the highest available Craft API seam, write a success and failure/authorization/recovery test covering: same-origin cookie leak, token replay/cross-tenant/cross-version, traversal/escaped path, real browser local assets, HTTP/WS/DNS/internal egress probes. Name the Go test `TestCraftT14Journey` and front-end test file after the owned panel or domain rule; assert persisted state and externally visible response rather than private helper calls.
- [ ] **Step 2 — Verify RED:** Run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT14Journey -count=1`. Expected: `TestCraftT14Journey` fails on the missing behavior, while existing unrelated Craft tests remain green.
- [ ] **Step 3 — GREEN:** Bind capability to tenant/Task/Version/file set and expiry; serve only immutable manifest paths on separate origin with no app credentials; enforce network isolation at deployment layer plus CSP. Keep `Task ID = Session ID`, make all authority server-side, and use the T00 frozen DTO/slot names in the actual implementation.
- [ ] **Step 4 — REFACTOR:** Remove only duplication introduced in this task, keep the public seam small, then run `go test ./internal/modules/craft ./internal/application/service ./internal/handler/session ./internal/container -run TestCraftT14Journey -count=1`; expected PASS. Confirm each candidate artifact/decision/lease refers to stable Run and Version IDs where relevant.
- [ ] **Step 5 — Verify:** Run `go test ./internal/modules/craft/... -run Preview -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/application/service/... -run CraftPreview -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `go test ./internal/handler/session/... -run CraftPreview -count=1`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm test:shared`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `pnpm typecheck:web`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Step 5 — Verify:** Run `Provide browser-level denied-egress evidence`. Expected: exit 0 with all selected tests passing; record exact count/output and checkpoint hash.
- [ ] **Environment evidence:** Launch the configured preview sandbox and a real browser on the isolated origin; capture local asset load plus denied HTTP/WebSocket/DNS/enterprise-network probes. Expected: page loads and all egress probes fail at infrastructure level.
- [ ] **Failure handling:** CSP-only denial or default Docker bridge egress is failure; do not let T15 promote. Keep this node pending or blocked with the failing evidence; do not mark it verified from a typecheck alone. Independent reviewer must record Spec compliance and code quality for the exact checkpoint before integration.

