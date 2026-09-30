# T10 narrow viewport shell repair

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the Career evaluation verdict, evidence and action readable at a 390px viewport without horizontal clipping.

**Architecture:** Remove the platform shell's 600px minimum width at the narrow breakpoint and start a newly mounted shell in its collapsed navigation state when the viewport is narrow. Keep the existing user-operated expand/collapse behavior and desktop persisted preference. The narrow rail must leave enough width for the page; the evaluation cards already wrap their own text and stack at <=640px.

**Tech Stack:** React PlatformShell, shell CSS and focused UI tests; live in-app browser viewport measurement.

**Spec:** #150 and approved Job Search Spec Web accessibility/three-end visibility; T10 Task 4 review focus. Live browser at integrated `60214147a` on 2026-09-24.

## Systematic debugging evidence

- At 390×844 in authenticated fixed evaluation detail, screenshot shows only the left part of verdict/action/evidence cards. Even after collapsing navigation, right content is clipped.
- Read-only DOM measurement: `innerWidth=390`, document scroll width 600. `main.wk-evaluation-detail` and `.plat-shell__outlet` width 540 starting at x=60; parent `.wk-shell-1` computed `min-width:600px` from `apps/web/src/platform/platform-u.css:593`. The page cards have `min-width:0` and wrap text; the parent shell is the first overflowing boundary.
- The sidebar state initializes solely from `localStorage.sidebar_collapsed` in `PlatformShell.tsx`, so a fresh 390px session may start with the 260px expanded sidebar. Hypothesis: a narrow min-width override plus narrow initial collapse brings `document.scrollWidth` within 390 and displays the verdict/action/evidence. No production code was changed while diagnosing.

## Global Constraints

- Isolated frontend worktree. Own only `apps/web/src/platform/PlatformShell.tsx`, its focused tests, `platform-u.css` or `platform-shell.td.css` as needed. Do not edit Career result/business code. Preserve others' edits.
- Local task commit authorized; no push, merge, deploy. Preserve desktop sidebar persistence and explicit narrow toggle behavior. Avoid hiding navigation without an accessible reopen control.
- This is a platform-shell correction needed for T10 narrow acceptance; verify it does not break other shell routes or overflow at desktop widths.

## Review Focus

- Fresh 390px mount with no collapsed preference starts with a usable narrow rail and Career main content width <=viewport; verdict, action, JD and profile citation wrap without horizontal clipping.
- Collapsed-to-expanded navigation remains reachable, keyboard accessible, and does not permanently overwrite desktop preference merely due to narrow mount. At 768px+ the existing sidebar behavior remains.
- A resize across breakpoint does not strand the user with a hidden or unusable navigation control; no global 600px overflow remains at 390px.

---

### Task 1: Remove narrow shell width floor and initialize usable navigation

**Depends:** integrated T10 UI and live defect reproduction. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** PlatformShell component/tests and narrow shell CSS. **Consumes:** current sidebar collapsed state/localStorage. **Produces:** responsive shell width and reachable navigation.

- [ ] RED: Add focused shell test for narrow initial state and toggle; test desktop stored preference remains. Add a CSS assertion or layout harness for absence of 600px floor at narrow width; demonstrate current failure. Verify DOM viewport evidence separately in controller browser after integration.
- [ ] GREEN: Override shell min-width under an appropriate narrow media query. Initialize or derive navigation to a collapsed usable state at narrow mount without persisting a preference change caused solely by viewport. Keep explicit toggle working and screen-reader name accurate.
- [ ] VERIFY: Focused shell/router tests, `pnpm typecheck:web`, `pnpm build:web`, `git diff --check`; run full Web suite once if feasible, document unrelated stall precisely. Independent Spec/quality review and frontend validator before integration; controller repeats 390px browser screenshot/width measurement.

## Shared-file and interface preflight

Only one frontend implementer writes shell files. T10 Career result source remains unchanged and already reviewed. Reviewer/validator are read-only; no parallel Web build in the same worktree. This shell task is serial before T10 narrow browser acceptance can be marked verified.
