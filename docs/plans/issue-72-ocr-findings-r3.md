# Issue #72 Final OCR Findings R3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve both valid low findings from `/tmp/issue72-ocr-r2-final-base-to-bf0423140-20260929.md` in the offline reconciliation evidence runner.

**Architecture:** Preserve the existing `_safe_reason` 80-character bound without a redundant second slice. Put output-directory preflight failures behind a bounded `RECONCILE FAIL` diagnostic while preserving argparse's normal `SystemExit` behavior for help/usage errors.

**Tech Stack:** Python 3 standard library and `unittest`.

**Spec:** Approved billing Spec `docs/specs/2026-09-20-lago-billing-migration-design.md`; ADR-0012; preceding repair plans `docs/plans/issue-72-ocr-final-findings-r1.md` and `docs/plans/issue-72-ocr-findings-r2.md`; R2 final OCR report above; R2 commit `bf0423140`.

## Global Constraints

- This is offline evidence tooling; do not access Lago/API/Docker or alter historical JSON/TXT.
- Preflight errors must not yield an uncaught traceback; preserve bounded diagnostics and nonzero status.
- `argparse` help and missing-required-argument `SystemExit` retain normal CLI semantics.
- Preserve all work outside the owned reconciliation source/test paths.

## Review Focus

- Reusing an existing output directory or supplying a missing parent returns nonzero with a diagnostic no longer than 240 characters, `RECONCILE FAIL: stage=output_preflight`, no path leakage, no traceback, and no artifact overwrite.
- Missing `--output-dir` and `--help` still surface argparse `SystemExit` rather than being converted to reconciliation errors.
- Artifact-write failure diagnostics use the single `_safe_reason` bound and remain redacted.

## Task 1: Bound preflight and failure diagnostics

**Dependencies:** Commit `bf0423140`; OCR-R2 report `/tmp/issue72-ocr-r2-final-base-to-bf0423140-20260929.md`. **Owner:** implementer (offline Python evidence tooling); **Validator:** backend_validator; **Independent reviewer:** reviewer. **Execution:** one serial task due shared reconciliation test module.

**Owned files:**

- `docs/plans/issue-72-flow-evidence-86/reconcile.py`
- `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`

**Consumes:** `prepare_output_dir`, `argparse.ArgumentParser`, `_safe_reason`, `_emit_text`, and the existing artifact-write-failure tests.

**Produces:** bounded preflight error reporting and nonredundant reason formatting.

- [x] **Step 1 — RED: existing and missing-parent output paths produce bounded failure.** Add canonical CLI tests for (a) an output directory created in advance with a sentinel file and (b) a path whose immediate parent does not exist. Invoke `reconcile.main()` with each path and assert current behavior raises `FileExistsError` or `FileNotFoundError` respectively. After GREEN, each case must return 1; captured stderr must start with `RECONCILE FAIL: stage=output_preflight`, include only the exception class (no raw path/sentinel), be at most 240 characters, and contain no traceback. The existing directory's sentinel remains byte-identical; neither case creates/changes an artifact.
- [x] **Step 2 — RED: argparse behavior remains outside conversion.** Add checks that invoking with missing `--output-dir` and with `--help` raises `SystemExit` with argparse's expected code/output; these usage exits are not rewritten as `RECONCILE FAIL`. These characterization assertions are expected to pass before and after the fix.
- [x] **Step 3 — GREEN: guard preflight only.** Keep argument parsing in its normal flow, then call `prepare_output_dir` within a narrow try block. Re-raise `SystemExit`; catch ordinary exceptions, emit the bounded `output_preflight` diagnostic via best-effort `_emit_text`, and return 1. Do not attempt to write a canonical artifact into an unavailable/conflicting directory.
- [x] **Step 4 — Characterize the real artifact-write failure diagnostic and simplify its bound.** Replace `safe[:160]` with `safe` in the artifact-write-failure message. Extend `test_reconcile_main_artifact_writer_failure_stays_nonzero_and_redacted` (the test that forces both canonical and fallback artifact writes to fail and captures stderr) to assert the exact `RECONCILE FAIL: artifact write failed (` prefix, that the reason portion is at most 80 characters, and that secret/error body text is absent. This is a maintainability cleanup with intentionally identical output because `_safe_reason` already enforces the 80-character bound; the test is a characterization of the actual branch, and the source diff itself establishes removal of the misleading redundant second slice.
- [x] **Step 5 — Verify.** Run the full `python3 -m unittest docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`; py_compile all four evidence Python files; `git diff --check`; compare all tracked historical JSON/TXT hashes under the evidence directory with BASE `bf0423140`.
- [x] **Step 6 — Review package.** Capture before/after hashes and complete incremental diffs for the two owned paths. Obtain independent Spec Compliance and Code Quality PASS bound to the same after snapshot before local commit.

## Acceptance Mapping

| OCR finding | Step | Evidence |
|---|---:|---|
| Redundant reason truncation | 4 | Fallback artifact-writer failure emits the bounded and redacted diagnostic through its actual stderr branch |
| Output-directory failure escapes main with traceback | 1–3 | Existing and missing-parent paths return 1 with bounded preflight diagnostics; usage errors retain argparse behavior |

## Plan Self-Review

- The preflight handler is deliberately narrow and does not swallow argparse control flow.
- Because the output directory is unavailable by definition, the failure is reported to stderr and no artifact write is attempted.
- Error detail remains bounded by `_safe_reason`; the formatter does not introduce a second, misleading bound.
- No runtime billing behavior, public API, or live acceptance status changes.


## Task 1 closeout

- Plan review `/tmp/issue72-ocr-r3-plan-review-20260929.md`: Spec Compliance PASS, Code Quality PASS.
- Task review `/tmp/issue72-ocr-r3-task-review-20260929.md`: Spec Compliance PASS, Code Quality PASS, hash-bound to the final two-file snapshot.
- Independent validation `/tmp/issue72-ocr-r3-validation-20260929.md`: DONE; 78 tests, four-file `py_compile`, scoped diff check, and 19 historical JSON/TXT hashes passed.
- Offline evidence tooling only; no live Lago acceptance claim.
