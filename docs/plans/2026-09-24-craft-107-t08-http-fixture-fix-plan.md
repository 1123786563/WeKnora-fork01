# Craft #107 T08 HTTP Workspace Fixture Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the existing B1 handler list/version/download test on the current immutable Workspace identity schema.

**Architecture:** Use the Workspace ID returned by real Craft Session creation. The test's post-creation raw SQL rewrite changes `craft_workspaces.id` without updating the server-owned registration/origin references and invalidates its own fixture.

**Spec:** `docs/plans/2026-09-24-craft-107-t08-current-joined-validation.md`; approved Craft #107 T08 private content access.

## Global Constraints

- Test fixture only; no policy/handler/service code change. Preserve all list, version and download assertions.
- Exact pre/post checkpoint and independent review. No commit/push.

## Review Focus

- The failing test should pass using the authoritative created Workspace ID, not by weakening owner or tenant checks.
- B5 joined production-auth test remains green, and a foreign/absent Workspace still fails closed in existing tests.

---

### Task 1: Remove the fixture's Workspace key rewrite

**Depends on:** T08 current joined validation identified reproducible fixture failure. **Owner:** `mechanical_worker`; **validator:** `backend_validator`, independent `reviewer`.

**Owned file:** `internal/handler/session/craft_test.go` only.

- [ ] Record preimage and reproduce `TestCraftHTTPListAndVersionDownload` failure at the workspace lookup.
- [ ] In that test only, use `first["workspace_id"]` from `env.createSession` as the version Workspace ID and remove the raw `UPDATE craft_workspaces SET id`. No other assertion or fixture behavior changes.
- [ ] Run the one test and related handler selector, `TestCraftB5JoinedCurrentProduction`, format/diff check. Save exact report/checkpoint/patch and independent review.

**Acceptance / failure handling:** Test green without altering production policy. If the same failure persists, stop and report the actual query/fixture cause rather than changing authorization semantics.
