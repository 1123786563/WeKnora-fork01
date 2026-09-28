# Issue #72 Final OCR Findings R1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the three low-priority findings in the final OCR pass for the reviewed Issue #72 checkpoint, including the fail-open malformed terminated-wallet case.

**Architecture:** Keep changes in the #86 offline evidence scripts and existing helper tests. Keep `checks` only at the canonical facts top level, preserve up to eight fixed reconciliation failure reasons in the canonical text evidence, and make terminated wallet balance validation use the same required integer parser as active wallets.

**Tech Stack:** Python 3 standard library and `unittest`.

**Spec:** Final OCR report `/tmp/issue72-final-ocr-84d17f1-to-8cb618c0-20260928.md`; approved billing Spec `docs/specs/2026-09-20-lago-billing-migration-design.md`; Issue #86 offline repair plan `docs/plans/issue-72-ocr-r2-python-findings.md`; ADR-0012.

## Global Constraints

- Preserve fail-closed evidence semantics; malformed or missing required wallet fields must never produce a PASS.
- `facts.json` remains the canonical JSON result, with `checks` stored once at the top level.
- Reconciliation error strings are fixed internal labels; preserve a bounded number of complete labels instead of truncating one concatenated string.
- No live Lago/API/Docker calls; no historical evidence artifact changes.
- Keep the documented #86 live acceptance gate blocked; this is offline evidence tooling only.

## Review Focus

- Missing `balance_cents` on a terminated wallet raises a validation error and cannot become a zero-balance PASS.
- Multiple reconciliation errors remain individually readable up to the documented cap of eight.
- Failed concurrent facts contain top-level `checks` and no duplicate `observations.checks` entry.

## Task 1: Correct final OCR evidence findings

**Dependencies:** Commit `8cb618c0e29edadf6032e24407790e5f2d6e2e1e`; final OCR report above. **Owner:** implementer (Python behavior); **Validator:** backend_validator; **Independent reviewer:** reviewer. **Execution:** serial, shared helper test module.

**Files:**

- Modify `docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py`
- Modify `docs/plans/issue-72-flow-evidence-86/reconcile.py`
- Modify `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`

**Consumes:** `write_facts_atomic(output_dir, facts)`, `assert_no_terminated_residuals(wallets)`, `_integer(value, label)`, and `reconcile.main()` check construction.

**Produces:** canonical failed facts with top-level checks only; required integer parsing for each terminated wallet balance; readable first eight reconciliation error labels in the canonical report.

- [x] **Step 1 — RED: canonical terminated-wallet failure.** Use the test's fake Lago wallet transport and canonical `reconcile.main()` path with a terminated wallet lacking `balance_cents`; assert `main()` exits nonzero and the emitted canonical report contains `RECONCILE FAIL`, `stage=validation`, and `ValueError`, with no `[PASS]` row for terminated-wallet validation. The missing required integer raises before check rows are constructed, so the expected artifact is the validation-stage failure record rather than a per-check `[FAIL]` row. Also assert explicit zero passes and positive balance fails. This must fail before the code fix because the current default silently accepts missing balance as zero.
- [x] **Step 2 — GREEN: remove the default.** In `assert_no_terminated_residuals`, call `_integer(w.get("balance_cents"), "wallet balance")` without a default. Keep the zero residual rule unchanged. Re-run the canonical-path test and expect the report to record FAIL with no false PASS.
- [x] **Step 3 — RED/GREEN: preserve bounded complete errors in the canonical report.** Introduce a small `batch_error_summary(errors, limit=8)` helper (or equivalent named test seam) and test ten distinct fixed labels. Drive `reconcile.main()` with ten simulated check failures, then assert the written canonical text report contains all labels 1–8 in full and excludes labels 9–10. This ties the helper behavior to the artifact consumed by operators. Replace the current `"; ".join(batch_errors)[:240]` so the limit applies to count, not the joined string.
- [x] **Step 4 — RED/GREEN: exercise the actual concurrent assertion-failure branch.** Extend the existing `_run_failed_probe("assertion", checks_expected=6)` test path so successful probe data passes normalization/preparation, then a post-probe assertion fails (for example, a deliberately mismatched expected wallet balance). Assert the canonical `facts.json` written by `main()` has top-level `checks`, has no `observations.checks`, and retains the six expected checks. The local `state` is not exposed by `main()`, so assertions must target the written artifact. This must fail before code change because this branch currently copies `facts` wholesale into observations. Change the merge to copy all facts except `checks` into observations.
- [x] **Step 5 — Verify.** Run the complete unittest module including canonical `reconcile.main()` and concurrent assertion-failure path; run py_compile on `consume_86.py`, `reconcile.py`, `concurrent_consumption_86.py`, and `test_evidence_helpers.py`; run `git diff --check`; confirm tracked historical JSON/TXT files under the evidence directory remain byte-identical.
- [x] **Step 6 — Review package.** Capture task before/after hashes and complete incremental diffs for all changed files. Obtain independent Spec Compliance and Code Quality PASS bound to the same after hashes before creating a local commit.

## Acceptance Mapping

| OCR finding | Step | Evidence |
|---|---:|---|
| Duplicate `checks` under observations | 4 | Canonical `facts.json` from the assertion-failure branch has one checks source |
| Joined error detail truncated at 240 characters | 3 | Canonical reconciliation report keeps complete first eight labels and excludes overflow |
| Missing terminated balance silently treated as zero | 1–2 | Canonical report records FAIL without false PASS; zero passes and nonzero fails |

## Plan Self-Review

- All three findings from the final OCR report have one named task step and executable evidence.
- Every changed interface remains private to the evidence scripts; no billing runtime/API contract changes.
- Malformed-wallet and bounded-error behavior is asserted through the canonical artifact writers; helper-only checks are supplemental, not acceptance evidence.
- This plan applies only to offline tooling; no live acceptance promotion is possible from its results.


## Task 1 closeout

- Plan review: `/tmp/issue72-final-ocr-findings-plan-rereview-r2-20260928.md` — Spec Compliance PASS, Evidence Quality PASS.
- Task review: `/tmp/issue72-final-ocr-r1-task-review-20260928.md` — Spec Compliance PASS, Code Quality PASS; source hashes match the after snapshot.
- Independent validation: `/tmp/issue72-final-ocr-r1-validation-20260928.md` — all acceptance paths pass; 75 tests, `py_compile`, and `git diff --check` pass.
- Historical artifacts: 19 tracked JSON/TXT hashes unchanged from BASE. No live calls; #86 remains unverified for live acceptance.
