# Issue 140 Task27 Deletion Fence Review Repairs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent pre-deletion boundary reads from restoring private state and preserve a truthful completed-deletion receipt when an old export grant is denied.

**Architecture:** Treat `deletionGeneration` as a fence for every ExportDeletionPage request, in addition to user/tenant scope generation. Refreshing a deletion boundary invalidates the old acknowledgement and blocks deletion until the new boundary is complete. Post-deletion old-grant denial is verification evidence and must not invoke generic private-state clearing on the terminal receipt.

**Tech Stack:** React, TypeScript, TDesign, Node test runner with jsdom.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md` §§2.1, 7; ADR-0015–0018; parent plan `docs/superpowers/plans/2026-09-30-issue140-career-review-repairs.md` Task27; review `/tmp/issue140-task27-review.md`.

## Global Constraints

- Confirmed `deleted` remains terminal and its receipt/retention disclosure is preserved after old-grant verification.
- No asynchronous pre-deletion callback may restore boundary, archive, export or deletion controls after the deletion generation changes.
- Scope-generation checks remain required; deletion-generation checks add a second fence and do not weaken identity isolation.
- Deletion stays disabled until a current, complete boundary is loaded and explicitly acknowledged.
- Local commits are authorized; no push, shared merge, deployment, publication or GitHub mutation.

## Review Focus

- Existing acknowledged boundary → start refresh → confirm deletion using current boundary → resolve old boundary request: old boundary remains absent and deletion controls stay hidden.
- Start deletion-boundary read → deletion completes before response: response is ignored.
- Old export-grant receipt check returns `forbidden` after a valid deleted receipt: show inaccessible verification status and retain the deleted receipt and retention disclosure.
- A real scope change during those requests still clears private state and prevents old-scope responses from updating it.

---

## Task 31: Fence boundary refreshes and preserve terminal deletion receipts

**Dependency:** Task27/29 commit `fa82b9d551e5dd20f0a9ed344e041417428ba09a`; findings T27-1 and T27-2 in `/tmp/issue140-task27-review.md`.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Owned files:** `apps/web/src/career/ExportDeletionPage.tsx` and `ExportDeletionPage.test.tsx` only.

**Consumes / produces:** `deletionGeneration` is the parent-provided monotonically increasing deletion fence. For each async boundary read, capture both the current scope snapshot and deletion generation; commit only while both remain current. Beginning a refresh invalidates the old acknowledged boundary and prevents deletion until the new read resolves successfully and is acknowledged. After a deleted receipt, `verifyOldGrants` treats `forbidden` as unreadable old authorization and updates the verification message while preserving the terminal receipt and retention details.

**Steps:**

- [ ] Add a controlled failing test: load/acknowledge a boundary; begin refresh with a deferred old response; complete deletion; resolve the old response; assert the previous boundary and deletion controls do not return.
- [ ] Add a test where post-delete export receipt verification returns `forbidden`; assert deleted status, completion timestamp, retention disclosure and verification message remain.
- [ ] Run the two tests and record their pre-fix failures.
- [ ] Add a render-updated deletion-generation ref/check to every boundary response path; require current scope plus matching deletion generation before setting boundary or state.
- [ ] On boundary refresh, clear stale boundary/acknowledgement and keep deletion disabled until current response is accepted and acknowledged.
- [ ] Handle forbidden old-grant verification as a denied/expired grant status without clearing the trusted deleted receipt; leave generic forbidden handling on pre-deletion actions unchanged.
- [ ] Run all ExportDeletionPage tests, all CareerPage/InboxPage/ExportDeletionPage focused suites, Web TypeScript check and `git diff --check`.
- [ ] Commit the scoped repair and append exact evidence to `/tmp/issue140-task27-report.md`.

**Acceptance:** T27-1 and T27-2 are fixed with controlled race tests; current boundary and terminal deletion receipt cannot be undone by late callbacks.
