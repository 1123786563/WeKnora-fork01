# T10 whole-JD graduation grammar review fix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent a hard graduation rejection when any later JD text permits the candidate's year or otherwise qualifies the apparent restriction.

**Architecture:** Classify the entire normalized JD before making a hard conclusion. Recognize exactly one full-line positive `仅限 YYYY 届` clause. Every other nonblank line must be a complete, clearly soft `技能：...` or `项目：...` field with no graduation year, graduation lexeme, alternative, exception, or negation. Any unrecognized or contradictory line makes the graduation result `unknown`. Never stop at the first skill/project line. Keep the exact raw source span for a recognized clause and the existing ruleset version semantics.

**Tech Stack:** Go Career deterministic evaluation parser and tests.

**Sources:** #150, approved Job Search Spec §3/§6.1, T10 backend plan, independent review high finding on `仅限2027届\n技能：Go\n2026届亦可`, and read-only architecture diagnosis. This replaces the prior adjacent-block approach in `2026-09-24-issue-140-t10-graduation-block-fix.md`.

## Root cause

The second parser repair stops scanning when it encounters a skill/project label. A later line can still permit 2026 graduates, so the stop treats an incomplete JD as an exclusive requirement. The classification boundary must be the full JD. The validator's existing tests pass at `d70792a3` but do not cover this continuation, and the independent reviewer has rejected it.

## Global Constraints

- Implement only in the isolated T10 backend worktree. Own `internal/modules/career/evaluation.go` and `evaluation_test.go`; preserve unrelated edits. No route, migration, Web, or architectureguard changes.
- Local task commit authorized. No push, merge, or deploy. Keep immutable JD evidence and its exact raw citation offsets. Ambiguity yields `unknown`, never `ineligible`.
- Do not broaden the hard-decision grammar beyond the enumerated forms during this repair. Ordinary JD text that is not fully classified may yield unknown.

## Review Focus

- Confirmed 2026 + standalone `仅限2027届` => ineligible; appending only `技能：Go` and/or `项目：X` remains ineligible with the exact clause citation.
- `仅限2027届\n技能：Go\n2026届亦可`, `仅限2027届\n技能：Go，2026届亦可`, and LF/CRLF variants => unknown.
- Same-line alternatives/negation, `备注：2026届可报`, `项目：2026届亦可`, `技能：不限届别`, another year or graduation lexeme anywhere, publication-year text, and unrecognized continuation lines => unknown.
- Prior fixes for missing-source citation, `Go` within `Google`, and concurrent changed-intent 409 remain intact. No hiring-probability score is introduced.

---

### Task 1: Whole-JD conservative grammar

**Depends:** T10 backend second review checkpoint `d70792a3`. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/evaluation.go`, `evaluation_test.go`. **Consumes:** raw immutable JD, profile fact/version. **Produces:** exclusive graduation mismatch with exact source span, or unknown.

- [ ] RED: Add table-driven tests for the full matrix above; demonstrate the later-line and embedded skill-line counterexamples fail on current parser. Cover LF and CRLF.
- [ ] GREEN: Normalize only for classification while retaining raw offsets. Scan all lines and accept a hard clause only if it is unique, standalone, positive, and every other nonblank line is a complete allowed soft field with no disallowed lexeme/year/qualifier. A soft field must not absorb a following unlabeled line. Return unknown before profile comparison on any ambiguous text. No early break.
- [ ] REFACTOR: Keep grammar small and explicit; avoid a growing list of permissive exceptions. Document what is recognized and unknown in code near the decision.
- [ ] VERIFY: Focused matrix repeated; relevant Career `-race` tests; broader Career/router/database/handler/container suite; `git diff --check`. Report code SHA and known conservative limits. Independent Spec/quality reviewer and backend validator must inspect the same checkpoint before integration.

## Shared-file and interface preflight

Only the backend implementer writes the parser/tests now. The evaluation API, persistence shape and migration remain stable; T10 guard and Web work wait for a reviewed backend checkpoint. The original and repair commits remain in the backend worktree for review and sequential cherry-pick after approval.
