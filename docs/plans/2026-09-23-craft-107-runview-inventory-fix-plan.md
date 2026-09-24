# T01 RunView Inventory Completeness Fix Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint and independent Spec/quality re-review. No commit.

**Goal:** Resolve inventory review findings INV-1/INV-2 so a RunView provider cannot treat a partial or scope-changed OpenCode session list as authoritative.

**Sources:** `runview-inventory-review.md`, `runview-provider-plan.md` Task 1, pinned OpenCode protocol research, current inventory checkpoint and shared client redirect policy.

**Global Constraints:** New inventory source/tests only; do not modify shared `client.go` or weaken its origin/directory checks. Legacy `/session` create versus v2 `/api/session` compatibility remains a live binary gate. No production provider or T01 completion claim from these unit tests.

**Review Focus:** required non-null cursor envelope on every page; empty/missing `next` valid only inside an envelope; exact project/limit/cursor query preservation through redirects; entire paginated set must be complete before success.

## Task 1 — close missing cursor and redirected scope

**Depends on:** first inventory review FAIL. **Role:** backend_implementer. **Owned files:** `internal/modules/agentruntime/agent/opencode/inventory.go`, `inventory_test.go`, report/checkpoint. **Produces:** fail-closed list semantics.

1. RED: missing/null cursor on first or later page fails; valid final page with `{cursor:{}}` succeeds. Same-origin redirects that drop or alter project, limit or cursor fail, including later pages; exact-preserving redirect may succeed if the client can prove no changed scope.
2. GREEN: require non-null cursor object for each response and enforce inventory-specific redirect scope, or disable inventory redirects entirely. Keep the existing shared directory/same-origin redirect policy intact. Validate all pages/IDs/project/directory as before.
3. Run focused and full OpenCode package tests, diff check, exact full-content checkpoint; independent reviewer rechecks INV-1/INV-2. If an upstream live route redirects incompatibly, report a production blocker rather than weakening completeness.

**Failure handling:** Any uncertain pagination or scope returns error; coordinator remains unresolved and no new session is created by that evidence.
