# T08 B5 Task1 fix1: real auth boundary and precise route assertions

> **For Codex:** Execute the scoped test evidence corrections with RED→GREEN→REFACTOR, exact uncommitted checkpoint and independent re-review. No production changes or commit.

**Sources:** `2026-09-24-craft-107-t08-b5-joined-http-plan.md`, Task1 report/checkpoint and `2026-09-24-craft-107-t08-b5-task1-review.md`; approved Spec #107/#126. Capture live HEAD and full new-test preimage/diff before edit. Full successful preview remains T14-dependent.

## Global Constraints

Keep real persistent access service, migrated DB, generic direct Session path, version/file handlers and production route guards. Prefer production `middleware.Auth` at the route with only credential verification/tenant-member backing fixtures; never inject actor identity directly if claiming real authentication. If real auth cannot be mounted without changing production source, report precise seam/blocker and narrow the test claim, then create a separate reviewed task for that seam. The `/p/` disabled-state assertion must use the configured preview host. Do not set production browser/network gates to true or claim successful preview.

## Review Focus

Auth JWT/API-key tenant/member construction; configured preview host; 200 list omission; exact grant/revoke target audit sequence; Gin global restoration; zero denied bytes/opens; no new source edits.

## Task 1 — close two Medium and three Low review findings

**Depends on:** failed Task1 independent review. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned file:** `internal/router/craft_b5_joined_test.go` only, plus new checkpoint/report. Other source/tests read-only. **Consumes:** production router/auth middleware and persistent service fixtures. **Produces:** exact Task1 joined route proof or a bounded seam blocker.

1. RED: demonstrate current synthetic auth route cannot prove production credential-to-principal mapping. Test configured preview-host `/p/` denial (currently wrong host), require list HTTP 200 with parsed ID omission, exact `(action,target)` audit multiset, and restored Gin mode.
2. Replace `X-Test-User`/`X-Test-Key` identity injection with actual `middleware.Auth` and narrowly mocked credential-verification/storage seams, or `NewRouter` where feasible. Seed active/stale/cross-tenant memberships through the same authenticated context. Preserve real RBAC/API-key route middleware and persistent Craft ACL. If Auth cannot be assembled test-only, state the exact dependency and fail the full production-auth claim; do not silently keep synthetic context and mark PASS.
3. Run targeted router selector, affected service/handler/container selectors after R3 compile stabilizes, `git diff --check`; save full pre/post test content/hashes, task-local patch, report and independent review.

**Acceptance:** Task1 claims match actual route/auth/preview-host path and exact audit/list behavior. **Failure handling:** an actual production auth seam blocker is recorded as such; no production policy edit is allowed in this task.
