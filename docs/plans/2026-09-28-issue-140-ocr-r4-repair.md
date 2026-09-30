# Issue #140 OCR Round 4 Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve independently confirmed critical/high/medium OCR findings in the #140 Career implementation and verify the repaired range.

**Architecture:** Keep Career as the authority for application, export, deletion, and evidence state; keep Workbench ownership unchanged. Apply fixes in isolated Career Web/API-client, Mini Program, and Go backend modules after confirming each finding against base `76df0cee0`. Use scoped worktrees and independent validators/reviewers, then integrate serially in this worktree.

**Tech Stack:** Go/GORM/Gin, React/TypeScript, Taro 4, node:test, pnpm.

**Spec:** `docs/specs/2026-09-23-weknora-job-search-design.md`; Issue snapshots `docs/plans/issue-140/issues/`; DAG `docs/plans/issue-140/2026-09-24-issue-140-dag.md`; findings `docs/plans/issue-140/ocr/round4-highrisk-analysis.md`, `round4-resume-increment-analysis.md`, `ocr-round-4-resume2.md`.

## Global Constraints

- Do not push, merge, publish, close Issues, or modify the root workspace's pre-existing user changes.
- Preserve tenant/owner scope, immutable evidence, original request IDs, and explicit confirmation for hard-condition continuation and destructive deletion.
- Each repair stream may only write its assigned files in a separate worktree; integration happens after task review.
- Do not infer HarmonyOS, Android-device, approved-source, or production-operation acceptance from unit tests or development tools.
- Existing OCR report contents and all previous task evidence remain immutable; add new round-specific reports.

## Review Focus

1. A hard-ineligible application must not be created unless the user explicitly acknowledges continuation; unknown deletion/export outcomes must keep recovery possible without enabling a fresh request ID.
2. Deterministic mismatched receipts must terminate in a visible error state rather than loop through unknown recovery.
3. Cross-scope state and request references must be cleared before the next owner's data is read or acted on.
4. Export and purge descriptions must agree about data that can be recovered after deletion.
5. Untrusted API payloads must be decoded before they affect UI state or write revisions.

## Task DAG and File Ownership

### Task 1 — Mini Program recovery and acknowledgement gates

**Source:** #166 / T26; OCR findings in `round4-highrisk-analysis.md` and `ocr-round-4-resume-increment-analysis.md`.
**Role:** `frontend_implementer`; validator `frontend_validator`; independent reviewer `reviewer`.
**Files:** `apps/miniprogram/src/career/application-material.tsx`, `apps/miniprogram/src/career/export-deletion.tsx`, directly corresponding tests under `apps/miniprogram/tests/`, and `docs/plans/issue-140/task-26-ocr-r4-fix.md` only.
**Consumes:** Existing `createApplication`, `lifecycleGating`, `deletionRecoveryUnresolvedAfter`, `abandonPendingSpaceExport`, `abandonPendingSpaceDeletion` APIs.
**Produces:** Acknowledgement-gated hard-ineligible creation; ambiguous deletion invalidates prior partial receipt; confirmed local abandon actions for pending export/deletion intents.
**Verification:** RED then GREEN tests for all three observable behaviors; `pnpm --filter @weknora/miniprogram test`; scoped typecheck/build with pre-existing errors reported precisely; `git diff --check`.

### Task 2 — Career Web mismatch recovery and API decoder boundaries

**Source:** #140 Web Career; four VALID high findings in `round4-highrisk-analysis.md`, independently verified in `task-career-web-ocr-r4-validation.md`.
**Role:** `frontend_implementer`; validator `frontend_validator`; independent reviewer `reviewer`.
**Depends on:** None; disjoint from Task 1.
**Files:** `apps/web/src/career/ProgressPage.tsx` and its tests, `SearchPage.tsx` and tests, `ExportDeletionPage.tsx` and tests, `packages/api-client/src/career.ts` and tests, plus `docs/plans/issue-140/task-career-web-ocr-r4-fix.md` only.
**Consumes:** Existing `ReceiptMismatchError`, `decodeCareerView`, `decodeCareerChangeSet`, `scope.scope.generation`.
**Produces:** Deterministic receipt mismatch errors; fresh private state and revision on scope generation changes; decoder enforcement at `createCareerApi` read boundaries.
**Verification:** RED then GREEN tests for mismatched progress/search receipts, scope-switch revision/state reset, and malformed open/list/changes payloads; focused tests; `pnpm typecheck:web`; `git diff --check`.

### Task 3 — Career backend export/privacy and failure behavior

**Source:** #140 backend Career; findings in `round4-highrisk-analysis.md` and `round4-resume-increment-analysis.md`.
**Role:** `backend_implementer`; validator `backend_validator`; independent reviewer `reviewer`.
**Depends on:** None; distinct service files from Tasks 1 and 2.
**Files:** `internal/modules/career/career_export.go`, `handler.go`, and directly corresponding backend tests only, plus `docs/plans/issue-140/task-backend-ocr-r4-fix.md`.
**Consumes:** Current archive/purge and bounded error contracts.
**Produces:** Export/deletion boundary language consistent with actual exported data; generic client-facing internal errors with server-side diagnostics where safe; concurrency-safe deduplication/result for same-source reminders if confirmed.
**Verification:** RED then GREEN focused tests for boundary promise vs exported sections, raw internal error non-disclosure, and same-source concurrent SetReminder; Career and handler focused suites; `git diff --check`.

### Task 4 — Career Web definite-failure and recovery correctness

**Source:** #140 Web Career; VALID medium findings in `round4-highrisk-analysis.md` and `round4-resume-increment-analysis.md`.
**Role:** `frontend_implementer`; validator `frontend_validator`; independent reviewer `reviewer`.
**Depends on:** Task 2 for `SearchPage.tsx`; all other owned page files are disjoint. Do not edit SearchPage until Task 2 has been reviewed and integrated.
**Files:** `apps/web/src/career/MaterialPage.tsx` + tests, `SubmissionPage.tsx` + tests, `PreparationPage.tsx` + tests, `RulePage.tsx` + tests, `OpportunityPage.tsx` + tests, and `docs/plans/issue-140/task-career-web-medium-ocr-r4-fix.md` only.
**Consumes:** Existing protocol helpers, material URL/deep-link recovery, submission version choice, receipt request IDs.
**Produces:** Successful writes stay successful if follow-up reads fail; transient material URL reads retain the stable pointer and retry; material/export selection and unknown-version submission fail closed while versions are unavailable; deterministic receipt mismatches and `not_found` end in visible errors; URL imports validate receipt identity.
**Verification:** RED then GREEN regression tests for each behavior; listed Career page test files; `pnpm typecheck:web`; `git diff --check`.

### Task 5 — Career API snapshot digest decoder edge

**Source:** #140 API client; VALID medium finding in `round4-highrisk-analysis.md`.
**Role:** `frontend_implementer`; validator `frontend_validator`; independent reviewer `reviewer`.
**Depends on:** Task 2, because both own `packages/api-client/src/career.ts` and its test.
**Files:** `packages/api-client/src/career.ts`, its direct tests, and `docs/plans/issue-140/task-api-client-medium-ocr-r4-fix.md` only.
**Consumes:** Existing OpportunityEvidence decoder rules and digest syntax.
**Produces:** Any supplied `snapshotSha256` is validated as a well-formed digest even when other optional snapshot identity fields are absent.
**Verification:** RED then GREEN decoder test for malformed digest-only payload and valid optional forms; api-client Career tests; `pnpm typecheck:web`; `git diff --check`.

## Preflight and Execution Ledger

- BASE: `76df0cee0bf3ae23c14411c345b151ad518077ee`, branch `codex/issue-140-integration`.
- Commit policy: local commits are authorized by the user-provided AGENTS instructions. Agent commits must be independently reviewed before integration.
- Shared files/interfaces: no task may modify router registrations, migrations, or shared Career contracts. Backend export/error fixes remain backend-owned; Mini Program and Web/API client are separate client surfaces.
- Concurrency: Tasks 1–3 may run concurrently in isolated worktrees. Validators/reviewers are dispatched only after the implementing report and review package are available.
- Integration ledger: record each agent identity/type, worktree, BASE/HEAD, review, validation, cherry-pick, and post-integration tests in `docs/plans/issue-140/task-ocr-r4-repair-ledger.md`.
- Blockers: only verified security/data-integrity conflict or missing external system input may leave a task blocked; environment limitations remain explicit and are not silently waived.

## Acceptance Mapping Self-Check

- Hard-condition acknowledgement gate: Task 1.
- Unknown deletion/export recovery and no new-request overwrite: Task 1.
- Request ID mismatch termination: Task 2.
- Scope generation cleanup: Task 2.
- Runtime response decoder boundary: Task 2.
- Export/purge truth and server error privacy: Task 3.
- Concurrency deduplication: Task 3, only if its finding remains reproducible at BASE.
- Material/retry/irreversible-choice and deterministic-failure behavior: Task 4.
- Optional snapshot digest shape: Task 5.
