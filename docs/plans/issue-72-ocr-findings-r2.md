# Issue #72 Final OCR Findings R2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the three findings from final OCR report `/tmp/issue72-ocr-r1-final-base-to-ca76e33b-20260928.md` without changing Lago behavior or claiming Issue #86 live acceptance.

**Architecture:** Preserve canonical artifact and return-code decisions after publication even when stdout fails unexpectedly; remove an obsolete private expiry-index helper that has no production callers; make the page-only monthly orphan reason label accurate for both positive and negative nonzero values.

**Tech Stack:** Python 3 standard library and `unittest`.

**Spec:** Approved billing Spec `docs/specs/2026-09-20-lago-billing-migration-design.md`; ADR-0012; Issue #86 offline repair plan `docs/plans/issue-72-ocr-final-findings-r1.md`; final OCR report cited above. R1 implementation is commit `ca76e33b5`.

## Global Constraints

- `reconcile-output.txt` is canonical after atomic publication; post-publication stream errors must never rewrite it or change its verdict-derived exit status.
- Keep evidence tooling fail closed for malformed wallet/page data.
- Do not alter historical JSON/TXT outputs, call Lago/API/Docker, or claim live acceptance.
- Preserve all existing uncommitted work outside the two owned source/test paths.

## Review Focus

- A post-publication stdout `AttributeError` and a failing stdout-isolation attempt both leave the canonical PASS artifact and successful return code unchanged.
- `unique_by_expiry` and its helper-only test are removed only after confirming no production call sites.
- A negative nonzero page-only monthly orphan is reported as `nonzero`, not `positive`.

## Task 1: Close OCR R2 findings

**Dependencies:** R1 commit `ca76e33b5`; final OCR report `/tmp/issue72-ocr-r1-final-base-to-ca76e33b-20260928.md`. **Owner:** implementer (Python evidence tooling); **Validator:** backend_validator; **Independent reviewer:** reviewer. **Execution:** one serial task because the source and helper test module are shared.

**Owned files:**

- `docs/plans/issue-72-flow-evidence-86/reconcile.py`
- `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`

**Consumes:** `reconcile.main()`, `_emit_text`, `_isolate_failed_stdout`, `reconcile_batches`, and the existing task test fixtures.

**Produces:** a canonical artifact invariant for arbitrary stdout exceptions; no orphaned unused expiry helper; an accurate monthly orphan error label.

- [x] **Step 1 — RED: canonical PASS survives stdout and isolation exceptions.** Add canonical `reconcile.main()` coverage with valid zero-credit account input and `lago_wallets=[]`. Exercise two cases: (a) stdout lacks `write`/`flush`, causing `print` to raise `AttributeError`, while isolation succeeds; (b) stdout write raises and `_isolate_failed_stdout` is patched to raise `RuntimeError`. In each case assert return code 0, canonical artifact remains `RECONCILE PASS`, and it is not replaced by a FAIL artifact. Confirm both cases fail on the current implementation.
- [x] **Step 2 — GREEN: make output and isolation fully best-effort.** Catch `Exception` from the stream write and separately guard the `_isolate_failed_stdout` attempt so its exception cannot escape `_emit_text`. Keep artifact publication and verdict-derived return status authoritative. Re-run both Step 1 cases and expect PASS.
- [x] **Step 3 — RED/GREEN: remove dead expiry helper.** Confirm `rg` has no production caller for `unique_by_expiry`. Delete the helper and `test_identity_map_rejects_duplicate_expiry_fallback`, which only exercised the helper's obsolete rejection path. Run the focused neighboring reconciliation tests and full suite.
- [x] **Step 4 — RED/GREEN: accurately label negative monthly orphan.** Add/adjust a `reconcile_batches` test with a valid monthly page row for an unrepresented period and negative `balance_micro`; assert reconciliation fails with `nonzero page-only monthly orphan` and does not say `positive`. Change the fixed label accordingly.
- [x] **Step 5 — Verify.** Run the entire `python3 -m unittest docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`; py_compile the four evidence Python files; `git diff --check`; verify all tracked historical JSON/TXT hashes under the evidence directory match R1 BASE `ca76e33b5`.
- [x] **Step 6 — Review package.** Capture before/after hashes and complete incremental diffs for both owned paths. Obtain independent Spec Compliance and Code Quality PASS bound to the same after-snapshot hashes before committing locally.

## Acceptance Mapping

| OCR finding | Step | Evidence |
|---|---:|---|
| Post-publication stdout/isolation exception can overwrite PASS artifact | 1–2 | Canonical PASS file and return code survive print `AttributeError` and isolation `RuntimeError` |
| Unused `unique_by_expiry` helper | 3 | No production callers; helper and helper-only test removed |
| Negative page-only monthly orphan mislabeled positive | 4 | Negative fixture reports `nonzero` accurately and fails closed |

## Plan Self-Review

- Each finding has a behavior-level regression and a named code change.
- The stdout tests reach `main()` after canonical publication and inject failures in both the write and recovery/isolation paths rather than testing only the private emitter.
- The helper deletion is limited to a private function and its sole test after call-site search.
- The plan changes only offline evidence code, not runtime billing contracts or external services.


## Task 1 closeout

- Plan review: `/tmp/issue72-ocr-r2-plan-review-20260928.md` — Spec Compliance PASS, Code Quality/Evidence PASS.
- Task review: `/tmp/issue72-ocr-r2-task-review-20260929.md` — Spec Compliance PASS, Code Quality PASS, bound to the after-snapshot hashes.
- Independent validation: `/tmp/issue72-ocr-r2-validation-20260928.md` — acceptance DONE; 76 tests, py_compile, diff check, and 19 historical JSON/TXT hashes verified.
- Offline evidence tooling only; no live Lago acceptance claim.
