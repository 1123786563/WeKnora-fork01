# T10 narrow sidebar resize repair

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep evaluation content readable when a shell mounted at desktop width is resized to 390px and back.

**Architecture:** Track the <=640px breakpoint after mount. Keep the desktop `sidebar_collapsed` preference separate from narrow-only navigation state; crossing into narrow starts with a collapsed rail, crossing back restores the stored desktop preference. At narrow width, an explicit expanded sidebar overlays rather than shrinks the content and retains its accessible collapse button. Do not persist automatic breakpoint changes.

**Sources:** #150 T10 narrow acceptance and approved Spec; prior narrow shell plan/code `a44679859`; independent review medium finding that `useState` initializer alone misses resize in both directions.

## Global Constraints

- Work in isolated narrow-shell worktree. Own `apps/web/src/platform/PlatformShell.tsx`, its sidebar-state tests, and narrow shell CSS only. Preserve others' edits. No Career business UI or backend changes.
- Local task commit authorized; no push, merge, deploy. Explicit desktop toggle still persists under `sidebar_collapsed`; breakpoint-driven collapse does not. Keep a keyboard accessible expand/collapse control on narrow screens.

## Review Focus

- Mount expanded at 1024px with stored false, resize to 390px: narrow rail collapses, main viewport has usable width and no 600px floor; resize back: desktop expands per stored false.
- Explicit narrow expand shows navigation as an overlay without permanently shrinking/clipping the result page; closing it returns to readable content. Resize while overlay open remains coherent.
- Stored true stays collapsed on desktop; explicit desktop toggle persists. Welcome-guide/drag expansion remains functional, no stale narrow state leaks to desktop.

---

### Task 1: Separate desktop preference from narrow navigation state

**Depends:** first narrow-shell code `a44679859`. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** PlatformShell component, sidebar-state test and CSS. **Consumes:** `sidebar_collapsed` preference and viewport breakpoint. **Produces:** responsive derived sidebar state.

- [ ] RED: Add mounted 1024→390→1024 tests for stored false and true, explicit narrow toggle, and CSS overlay/no width-floor assertions; confirm current initializer-only failure.
- [ ] GREEN: Subscribe to resize or matchMedia with cleanup; derive effective collapsed state from distinct desktop and narrow states. On entering narrow, close the expanded panel; on leaving, restore desktop preference. Keep drag/guide expansion semantics. Overlay expanded sidebar at <=640px rather than squeezing outlet.
- [ ] VERIFY: Focused shell tests, `pnpm typecheck:web`, `pnpm build:web`, full Web suite once if feasible, `git diff --check`; independent Spec/quality review and frontend validator. Controller then tests 390px live DOM widths/screenshot and reset viewport.

## Shared-file and interface preflight

One frontend implementer writes shell files serially. Career UI and API code are stable. Reviewer/validator are read-only. The live browser still runs the pre-fix bundle until reviewed integration hot reload.
