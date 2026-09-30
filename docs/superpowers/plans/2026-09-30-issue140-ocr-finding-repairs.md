# Issue 140 OCR Finding Repairs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement these tasks. Keep each task in an isolated worktree; follow RED → GREEN → REFACTOR when tests apply.

**Goal:** Resolve verified medium-severity OCR findings produced by the incomplete full-range Issue #140 review session `b5b5cf36-2ff1-49bd-9303-6a9c9c8641c6`.

**Architecture:** Keep platform authorization errors precise, preserve embed utility semantics and theme accessibility, and inject disposable live-test credentials through environment variables. Do not expand the feature scope.

**Tech Stack:** TypeScript, CSS, Node.js.

**Sources:** Issue #140 DAG `docs/plans/issue-140/2026-09-24-issue-140-dag.md`; plan `docs/superpowers/plans/2026-09-29-issue140-integration-blockers.md`; OCR session artifact at `/Users/wuyongjun/.opencodereview/sessions/Users-wuyongjun-.codex-worktrees-issue-140-sweep-integration-WeKnora-fork01/b5b5cf36-2ff1-49bd-9303-6a9c9c8641c6.jsonl`; comments exported to `/tmp/issue140-ocr-fresh-comments.json`; original BASE `db234c5eb171f2dde7427d382b55b503a038f879`; review target `b3d48d5cb9cd8833ea348d4a6f37a9bef52ae13b`.

## Global Constraints

- Preserve the issue30-sweep base as an ancestor; do not edit its original worktree or push.
- Keep local commits; no GitHub mutations, merge, deploy, or publish.
- Do not put credentials in source or logs.
- Never map an arbitrary 401 to an artifact-grant error.
- Preserve documented Career, authorization, and deletion behavior.

## Review Focus

- The mini-program maps only explicit artifact grant error codes to grant messages; generic 401 remains an authentication failure.
- Embed CSS preserves `text-underline-offset: 2px`, uses brand tokens for brand states, and has readable text-button contrast in dark mode.
- T33 replay requires runtime-injected credentials and fails before interaction if they are absent.

## Task 1: Tighten Mini-program artifact 401 classification

**Dependency:** None.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/miniprogram/src/platform/files.ts`; owning test(s) in `apps/miniprogram/tests/platform-adapters.test.mjs` or existing artifact-download test.

**Consumes / Produces:** The download error JSON provides a server `code`; preserve explicit `artifact_grant_expired` and `artifact_grant_invalid` codes. Other 401 responses carry status 401 and no grant code, allowing the existing authentication message path.

**Steps:** Add a regression test for generic 401 and both explicit grant errors; confirm current generic-401 case fails. Replace nested ternary with explicit mapping; run targeted test and package typecheck; report exact commands and commit.

**Acceptance:** Grant-specific help appears only for explicit grant codes; generic 401 stays login-expired/unauthorized; targeted tests and typecheck pass.

## Task 2: Restore embed interaction style behavior

**Dependency:** None; owned files are disjoint from Tasks 1 and 3.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/embed/embed-u.css`, `apps/embed/src/styles.css`, and focused existing/new embed style assertion tests.

**Consumes / Produces:** Utility migration contract in `embed-u.css`; design tokens in `packages/design-tokens/src/styles.css` provide `--wk-color-brand`, `--wk-color-brand-hover`, `--wk-color-brand-active`.

**Steps:** Add failing assertions for `.wk-emb-34:hover` offset, token-backed primary colors, and dark-mode text button contrast. Add `text-underline-offset: 2px`; use token fallbacks for primary/hover; make dark mode text button foreground/background inherit readable current color and subtle currentColor mix. Run focused tests and relevant Web/embed package checks; commit.

**Acceptance:** Migrated hover underline keeps its 2px offset; brand states resolve via tokens with existing fallback; dark theme text controls use readable foreground/background; targeted checks pass.

## Task 3: Make T33 credentials injectable

**Dependency:** None; owned only `.superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence/wx-driver.cjs`.

**Role:** `mechanical_worker`; validator `frontend_validator`; reviewer `reviewer`.

**Consumes / Produces:** Read `T33_USER_EMAIL` and `T33_USER_PASS` only at runtime. Missing values must fail before `fillInput`; do not print them.

**Steps:** Add a pure credential loader test or node `--check` plus a small child-process check proving missing vars fail without outputting a secret; replace literals and update replay example; run test/check; commit.

**Acceptance:** No credential literal remains, no credential value is logged, missing env exits promptly with variable names only, supplied env reaches input calls.

## Integration and verification

Tasks 1–3 can run concurrently in separate worktrees because their owned files and state are disjoint. Integrate in task order 1, 2, 3 after task-local review/validation. Re-run affected focused suites, `git diff --check`, then full-range OCR from original BASE to the resulting HEAD. OCR session `b5b5cf36` is incomplete and is not a pass; these comments are provisional candidates pending code validation.
