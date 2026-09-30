# Craft Budget Pause Extension Shape Resilience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A malformed optional `extension_action` must not suppress a valid Craft budget pause view.

**Architecture:** Keep core pause parsing strict. Parse the optional extension action independently and downgrade only that unusable optional field to `null`, so the UI can retain `{limit, used}` while withholding the extension action.

**Tech Stack:** TypeScript, API client tests, web feature tests.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`; OCR R3 report `docs/plans/craft-107-ocr-final-3.md`, finding `packages/api-client/src/craft/index.ts:392-396`.

## Global Constraints

- Never expose an extension action that failed validation.
- Preserve strict parsing of required pause fields and authorization behavior.
- Do not treat `can_extend` alone as authority to render or submit an extension action.
- No dependency or wire-contract changes.

## Review Focus

- Valid required pause plus malformed/unsafe optional extra-credit value retains the required numeric pause view and returns `extensionAction: null`.
- Missing or null optional action is represented as `null`.
- Malformed required pause fields continue to raise `INVALID_RESPONSE`.
- Valid extension action remains byte-for-byte contract compatible.
- Public type export remains usable from the package entry point.

---

### Task 1: Make optional extension parsing non-fatal

**Dependencies:** None. Exact API/client tree hash-matches current integration baseline.

**Owner role:** `frontend_implementer`.

**Validator role:** `frontend_validator`.

**Files:** `packages/api-client/src/craft/index.ts`, `packages/api-client/src/craft/index.test.ts`; optional low-risk export fix only in `packages/api-client/src/index.ts` and UI cleanup in `apps/web/src/features/craft/routes.tsx` if included in Task Brief.

**Consumes:** `parseCraftBudgetPause(body)` and `parseCraftBudgetExtensionAction(body.extension_action)`.

**Produces:** Budget pause result whose core `pause` remains strict and whose `extensionAction` is null when only the optional field is invalid.

- [ ] Add a test with valid `{limit, used}` and an unsafe/malformed `extension_action`; assert budgetPause resolves, numeric pause fields remain intact, and action is null.
- [ ] Run that test and capture RED caused by current whole-response `INVALID_RESPONSE`.
- [ ] Catch only extension-action parser failure locally; return null for that field without suppressing errors from `parseCraftBudgetPause`.
- [ ] Run focused API client test and adjacent Craft API tests; run frontend lint/type checks if package scripts make them targeted and deterministic.
- [ ] Run `git diff --check`; write report with exact file hashes, patch package, test commands/results, and unrelated baseline status.
- [ ] Independent Review gives Spec and code quality conclusions; independent validator confirms tests and snapshot hashes. No commit.

## Failure Handling

- If an invalid optional action cannot be separated from malformed required pause data, retain fail-closed behavior and report the exact contract ambiguity; do not weaken core parsing.
- If a package command is blocked by unavailable workspace dependencies, record the exact error and run the narrowest existing unit test.
