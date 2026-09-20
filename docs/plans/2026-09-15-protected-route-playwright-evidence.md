# Protected Route Playwright Evidence Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to execute this plan task-by-task with verification checkpoints.

**Goal:** Capture paired Vue/React browser evidence for protected routes, next navigation, anonymous errors, and fixed viewports; repair at most one directly evidenced web route difference, otherwise submit evidence-only.

**Architecture:** Reuse the repository's existing Vue and React dev servers, route inventory, and Playwright/browser harness. Evidence records URL, status/navigation outcome, visible state, viewport, and screenshot paths for both clients under the same anonymous/authenticated conditions. Any fix must be isolated to a single route mismatch and re-run the same pair.

**Tech Stack:** Vue/Vite, React/Vite, Playwright, existing browser evidence scripts, Markdown/JSON artifacts.

**Spec:** User request in the current task; repository guidance in `docs/agents/issue-tracker.md` and `docs/agents/domain.md`.

## Global Constraints

- Preserve unrelated dirty worktree changes.
- Treat Vue as the reference only where live evidence confirms the same scenario and environment.
- Anonymous/protected-route behavior must not be inferred from source-only route declarations.
- A build or route reachability check is not browser acceptance; record browser evidence separately.
- Make no more than one web-route fix in this task; if no safe localized fix is proven, produce evidence-only artifacts.

### Task 1: Inventory the existing parity harness and live prerequisites

**Files:**
- Read: `artifacts/browser-evidence-20260915/collect-public-route-evidence.mjs`
- Read: `artifacts/browser-evidence-20260915/results.json`
- Read: `docs/migrations/react/route-parity.csv`
- Read: `apps/web/package.json`, `frontend/package.json`

- [ ] Identify exact Vue/React base URLs, available scripts, test credentials, and route candidates.
- [ ] Check server/process readiness without mutating application data.

### Task 2: Capture paired Playwright evidence

**Files:**
- Create or modify: `artifacts/browser-evidence-20260915/protected-route-results.json`
- Create or modify: `artifacts/browser-evidence-20260915/protected-route-*.png`
- Create or modify: `artifacts/vue-react-protected-route-evidence-20260915.md`

- [ ] Run the same protected, next, anonymous-error, and viewport scenarios against Vue and React.
- [ ] Record navigation outcome, URL, visible landmarks/error text, viewport, and screenshot for each pair.
- [ ] Mark environment-blocked cases explicitly instead of treating missing auth/backend as a route mismatch.

### Task 3: Apply one evidence-backed route repair only if proven

**Files:**
- Modify: the smallest existing React/Vue route file implicated by Task 2.
- Test: the narrowest existing Playwright or route test, if available.

- [ ] Compare paired evidence and source ownership to isolate one mismatch.
- [ ] If a localized fix is proven, implement it and re-run the exact failing pair plus a focused regression.
- [ ] Otherwise leave product code unchanged and retain evidence-only status.

### Task 4: Verify and commit the scoped result

**Files:**
- Commit only the evidence files and any single verified route fix from Task 3.

- [ ] Inspect `git diff --check` and the final diff for unrelated changes.
- [ ] Re-run the evidence command or exact focused Playwright scenario after any edit.
- [ ] Commit with a message describing evidence-only or the exact route repair; do not claim broader parity.
