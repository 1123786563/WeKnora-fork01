# Issue 140 Task5 Review Repairs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement these tasks. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close Task5's deletion privacy, confirmed internship model-input, and behavioral test coverage findings.

**Architecture:** Keep Career Office authoritative. A confirmed deletion synchronously invalidates all mounted Career child UI state and fences in-flight reads. Confirmed internship company facts flow through the existing safe model-input allowlist. Frontend and backend changes use disjoint files and isolated worktrees.

**Tech Stack:** Go, React/TypeScript, Vitest, GORM.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`, `CONTEXT.md`, ADR-0015 through ADR-0018, parent plan `docs/superpowers/plans/2026-09-30-issue140-career-review-repairs-r2.md` Task5, findings `/tmp/issue140-r2-task5-review.md`.

## Global Constraints

- Keep deletion privacy scope and owner/tenant isolation intact; stale responses from a pre-deletion generation cannot repopulate private UI state.
- A confirmed internship fact must be available to qualification, profile summary, and material generation through the established model-input seam.
- Preserve user-confirmation provenance and existing model-input safety allowlists; do not admit unconfirmed parser data by broadening trust.
- Local commits are authorized; no push, shared merge, deployment, publication, or GitHub Issue mutation.

## Review Focus

- Deletion succeeds while Inbox read is pending: the late response cannot restore private reminders or references.
- Deletion succeeds while Export/Deletion page remains mounted: prior archive and download controls disappear immediately while the deletion receipt remains.
- Reload after deletion rejects: all profile facts, source names, reminder text, export/archive data, and download actions stay cleared.
- Manual project, internship, and skill submissions produce the exact intended keys/values and confirmed facts.
- An internship company fact appears in safe model inputs while unrelated/unapproved keys remain excluded.
- Subscription retry after revision advances retains the original request ID and expected revision.

---

## Task 27: Clear and fence all mounted private Career UI after deletion

**Dependency:** Task5 commit `44bb18a490ba76a01820775512e246102b98cbe0`; findings T5-1 High and T5-3 Medium from `/tmp/issue140-r2-task5-review.md`.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Owned files:** `apps/web/src/career/CareerPage.tsx`, `InboxPage.tsx`, `ExportDeletionPage.tsx`, and their focused tests only. Do not edit Go files or UI outside Career.

**Consumes / produces:** Existing `onCareerDeleted` callback fired only after a confirmed `deleted` receipt. Add/extend child clear-generation props or remount keys so Inbox and ExportDeletionPage synchronously clear private state, invalidate in-flight callbacks, and preserve only the deletion receipt/retention disclosure. CareerPage's generation fence remains the parent source of truth.

**Steps:**

- [ ] Add tests for each manual project/internship/skill entry selecting, submitting and asserting exact action key/value.
- [ ] Add deletion regression with failed reload and pre-deletion private profile facts/source names; assert immediately cleared.
- [ ] Add delayed Inbox read resolving after deletion; assert old todos/references remain absent.
- [ ] Add mounted export/archive state case; assert downloads/archive actions disappear immediately while deletion receipt is retained.
- [ ] Strengthen subscription retry regression to explicitly refresh to a new profile revision before retry and compare full initial/retry payloads.
- [ ] Run new tests RED, then implement child purge + in-flight response fence and the entry/retry test corrections.
- [ ] Install frozen-lockfile Web dependencies in the isolated worktree if absent; run CareerPage, InboxPage, ExportDeletionPage focused suites, Web TypeScript check and `git diff --check`.
- [ ] Commit only owned frontend paths and report exact verification evidence.

**Acceptance:** On a confirmed deletion, all mounted private Career child UI state disappears before any reload; stale reads cannot restore it. Manual fields and frozen-revision retries have executable behavior coverage.

## Task 28: Admit confirmed internship company facts into safe model inputs

**Dependency:** Task5 commit `44bb18a490ba76a01820775512e246102b98cbe0`; independent of Task27 files.

**Role:** `backend_implementer`; validator `backend_validator`; reviewer `reviewer`.

**Owned files:** `internal/career/model_input.go` and its focused tests only.

**Consumes / produces:** Existing `BuildModelInput` safe allowlist and career facts with user-confirmation provenance. Add exact key `internship.company` to the approved company/experience input mapping without changing which source statuses are trusted.

**Steps:**

- [ ] Add a failing test where a user-confirmed `internship.company` fact is included in `BuildModelInput`.
- [ ] Add a paired test that a parser-proposed/unconfirmed `internship.company` and unrelated unapproved keys remain excluded.
- [ ] Run tests RED and implement the narrow safe-key mapping.
- [ ] Run focused model-input tests, Career package as feasible and `git diff --check`.
- [ ] Commit owned backend paths and report exact verification evidence.

**Acceptance:** Confirmed internship company facts reach downstream model consumers; unconfirmed or unapproved data remains excluded.

## Task 29: Complete CareerPage deletion and manual-entry behavior evidence

**Dependency:** Task27 commit `6bf56e0bebadd065d8ca62f4e9fa07f138a3b310`; unresolved T5-3 evidence gap and parent deletion reload-failure gap documented in `/tmp/issue140-task27-report.md`.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Owned files:** `apps/web/src/career/CareerPage.tsx`, `CareerPage.test.tsx`, and if needed only the narrow Career child props/tests touched by Task27. Preserve the separate Task28 Go changes.

**Consumes / produces:** CareerPage's confirmed deleted receipt callback must synchronously clear parent facts, proposals, source names and upload/action state, increment `deletionGeneration`, and fence all old async callbacks before starting `load()`. A failed reload must not restore private state. Manual fields must be submitted through the same `doAction` behavior users invoke, with exact key, value, and user confirmation source.

**Steps:**

- [ ] Reproduce the current Task27 manual-entry test failure and inspect the rendered DOM plus CareerDesk load/validation path to identify why no manual form exists before interaction; do not weaken the test to pass by removing the submission assertion.
- [ ] Add a controlled parent integration test that begins with confirmed facts and named sources, drives the actual confirmed deletion receipt callback, makes the subsequent `open/list/sources` refresh fail, and asserts facts/proposals/source names and upload controls stay cleared while the deletion receipt remains visible.
- [ ] Repair the manual-entry test setup or production defect at its root; select and submit project, internship and skill values and assert exact action/key/value/source for each.
- [ ] Run those tests RED before a production behavior fix when the failure is behavioral; implement the smallest correction and retain all Task27 race/purge tests.
- [ ] Run all CareerPage, InboxPage and ExportDeletionPage focused suites, Web TypeScript check and `git diff --check`; expected all focused tests pass.
- [ ] Commit Task29 and append actual commands/results to `/tmp/issue140-task27-report.md`.

**Acceptance:** Task27's existing 37 passing cases remain green; manual project/internship/skill actions are proven end to end; failed reload after confirmed deletion leaves no private parent or child UI data and retains the deletion receipt.
