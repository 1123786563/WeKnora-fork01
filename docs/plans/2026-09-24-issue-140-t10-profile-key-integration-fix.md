# T10 confirmed graduation fact key integration fix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the reviewed evaluator recognize the graduation fact created by the shipped Career profile form, while keeping ambiguous or malformed confirmed facts as unknown.

**Architecture:** Normalize a narrow list of known confirmed graduation fact keys at the Career Office evaluation boundary: `education.graduation_year`, `graduation_year`, the shipped manual form's `毕业时间`, and resume intake's `education.graduation_date`. Parse only recognized year/date formats. Compare all recognized confirmed candidates; any malformed or conflicting year yields `unknown`, otherwise cite the selected original immutable fact version. Do not rewrite old profile facts or change API output shape.

**Tech Stack:** Go Career Office tests, SQLite Lite runtime; Web browser acceptance on integrated code.

**Spec:** #150 and approved `docs/specs/2026-09-23-weknora-job-search-design.md` §6.1/acceptance 17. Original T10 plan Task 1; live test at integrated SHA `2fd6e3350` on 2026-09-24.

## Systematic debugging evidence

- Reproduction: fresh disposable SQLite Lite and local Web, register disposable account, use shipped Career page default `毕业时间` to directly confirm `2026`, paste standalone `仅限2027届`, evaluate. Browser shows `待确认` and reason `confirmed_graduation_year_missing`, not required `不符合`.
- Boundary trace: Browser Career page shows revision 1 and confirmed `毕业时间=2026`; SQLite `career_facts` row is `毕业时间|2026|1`. The fixed JD is canonical and the evaluation cites its exact raw span, so JD parsing succeeded. `evaluation.go` currently only accepts `graduation_year` and `education.graduation_year`; thus it excludes the confirmed UI fact before comparing years. This is a cross-component fact-key contract gap, not a parser ambiguity or authentication failure.
- Pattern comparison: resume intake uses `education.graduation_date` for missing-category semantics, while the shipped manual form and its tests use `毕业时间`. The evaluator's existing tests create only English-key facts, so they missed the public UI path. Hypothesis: accepting the actual confirmed key with existing year normalizer yields an ineligible cited outcome for the live case. No code was changed to test this hypothesis yet.

## Global Constraints

- Isolated backend worktree. Own only `internal/modules/career/evaluation.go` and `evaluation_test.go`. Preserve others' edits. No Web, migration, route, guard or data rewrite.
- Local task commit authorized; no push, merge, deploy. Use only confirmed pinned facts; never infer a graduation year from an unconfirmed proposal or unsupported date. Preserve exact original fact key/version in evidence.
- Canonical JD hard grammar remains deliberately strict. If multiple recognized confirmed graduation facts disagree, or any recognized fact has an unparseable value, return unknown; do not pick a favorable one.

## Review Focus

- Manual form `毕业时间=2026` confirmed at revision 1 + JD `仅限2027届` => ineligible with JD span and original Chinese-key fact revision. Missing/unconfirmed-only => unknown.
- Recognized `education.graduation_date=2026-06-30` => same comparison; confirmed 2027 => rule eligible. Multiple aliases with equal years cite deterministic preferred version; conflicting third/fourth candidate or malformed recognized value => unknown.
- Old evaluation remains unknown/readable; a fresh request after fix creates a new immutable evaluation. Scope, request replay, source citation, and no score remain unchanged.

---

### Task 1: Recognize confirmed graduation aliases safely

**Depends:** reviewed T10 backend and integrated UI. **Owner/validator:** backend_implementer / backend_validator. **Files:** `evaluation.go`, `evaluation_test.go`. **Consumes:** pinned confirmed `Fact` rows from Career profile and fixed JD snapshot. **Produces:** cited three-valued graduation rule.

- [ ] RED: Add an Office/public seam test that confirms a `毕业时间=2026` fact through `Office.Act`, evaluates canonical 2027-only JD, and expects ineligible with original key/revision; confirm current unknown. Add table tests for date alias, matched year, absent/unconfirmed, malformed, and conflicts among three or more aliases.
- [ ] GREEN: Implement explicit known-key recognition and all-candidate consistency check in the evaluator. Prefer a deterministic cited fact when equal aliases coexist. Keep the JD parser and persistence untouched.
- [ ] VERIFY: Focused tests repeated and `-race`, broader Career/router/database/handler/container suites, `git diff --check`. Independent Spec/quality review and backend validator before integration. Controller restarts disposable API and repeats live browser 2026→2027-only and profile-edit new-vs-old flow.

## Shared-file and interface preflight

Only the backend implementer writes the two Career files. Reviewer/validator are read-only; no parallel code writer. HTTP JSON shape, TypeScript contract and Web UI remain stable. Current browser server still runs old integrated code and must be stopped/restarted after this reviewed fix before live acceptance is repeated.
