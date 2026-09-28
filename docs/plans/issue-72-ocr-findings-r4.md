# Issue #72 Final OCR Findings R4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the single low maintainability finding in `/tmp/issue72-ocr-r3-final-base-to-6f520f67c-20260929.md` by removing the unreachable explicit `SystemExit` re-raise in reconciliation output preflight.

**Architecture:** Keep `argparse.parse_args()` outside the preflight try block and catch only ordinary `Exception` from `prepare_output_dir`; Python's `SystemExit` derives from `BaseException` and naturally propagates past `except Exception`.

**Tech Stack:** Python 3 standard library and `unittest`.

**Spec:** Approved billing Spec `docs/specs/2026-09-20-lago-billing-migration-design.md`; ADR-0012; R3 plan `docs/plans/issue-72-ocr-findings-r3.md`; R3 commit `6f520f67c`; OCR report cited above.

## Global Constraints

- Keep argparse help/usage exits unchanged.
- Keep ordinary preflight exceptions bounded and reported as `RECONCILE FAIL: stage=output_preflight` with nonzero exit.
- Offline evidence tooling only; no network/API/Lago/Docker and no historical artifact changes.
- Preserve all changes outside `reconcile.py`.

## Review Focus

- No unreachable `except SystemExit: raise` remains around `prepare_output_dir`.
- Existing tests continue proving missing-argument/help `SystemExit` and ordinary output-preflight error conversion.
- No change to artifact publication, `_safe_reason` or reconciliation behavior.

## Task 1: Remove unreachable branch

**Dependencies:** Commit `6f520f67c`; R3 final OCR report `/tmp/issue72-ocr-r3-final-base-to-6f520f67c-20260929.md`. **Owner:** mechanical_worker (one-line cleanup); **Validator:** backend_validator; **Independent reviewer:** reviewer. **Execution:** one serial mechanical task.

**Owned file:**

- `docs/plans/issue-72-flow-evidence-86/reconcile.py`

**Consumes:** Existing `test_reconcile_main_preserves_argparse_system_exit`, `test_reconcile_main_output_preflight_errors_are_bounded`, and the current preflight handler.

**Produces:** Preflight handler containing only the reachable ordinary-exception branch.

- [x] **Step 1 — Characterize existing behavior.** Run the argparse SystemExit test and both preflight-error tests before the edit; confirm expected help/usage propagation and bounded output errors. This is a behavior-identical cleanup; no failing test should be manufactured for unreachable code.
- [x] **Step 2 — Remove dead branch.** Delete only `except SystemExit: raise`. Leave parsing outside the try, `except Exception` conversion, diagnostics, and return unchanged.
- [x] **Step 3 — Verify.** Re-run the focused three tests and full helper test module; py_compile all four evidence Python files; `git diff --check`; verify tracked historical JSON/TXT hashes against BASE `6f520f67c`.
- [x] **Step 4 — Review package.** Capture before/after hashes and incremental diff for `reconcile.py`. Obtain read-only review PASS before local commit.

## Acceptance Mapping

| OCR finding | Step | Evidence |
|---|---:|---|
| Unreachable `SystemExit` re-raise | 1–3 | Existing argument/preflight regression tests pass after deleting the single dead clause |

## Plan Self-Review

- The deletion is behavior-identical because parsing is outside the try and `SystemExit` is not caught by `Exception`.
- Existing tests cover the relevant control-flow behavior; no artificial failing test is needed for dead code.
- The change is limited to offline evidence tooling and cannot affect live billing acceptance.


## Task 1 closeout

- Plan review `/tmp/issue72-ocr-r4-plan-review-20260929.md`: Spec Compliance PASS, Code Quality PASS.
- Task review `/tmp/issue72-ocr-r4-task-review-20260929.md`: Spec Compliance PASS, Code Quality PASS.
- Independent validation `/tmp/issue72-ocr-r4-validation-20260929.md`: DONE; exact diff/hash verified, before/after focused tests, full 78-test suite, py_compile, diff check, and 19 historical JSON/TXT hashes passed.
- The cleanup changes only the offline evidence script; no live Lago acceptance claim.
