# Issue #72 OCR R2 Python Findings Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve all verified findings from the R1 OCR recheck of the three supported #86 Python evidence scripts while preserving fail-closed and truthful-artifact behavior.

**Architecture:** Keep the repair in the shared evidence helper test module and its two production evidence scripts. Publish concurrent-run facts through one atomic writer before emitting any PASS marker; handle preflight failures with bounded diagnostics; remove the unused orphan-verdict abstraction so tests cover the real reconciliation path.

**Tech Stack:** Python 3 standard library, `unittest`, `tempfile`, `os.replace`.

**Spec:** R1 Python OCR report `/tmp/issue72-python-workspace-ocr-r1-recheck-20260928.md`; approved migration Spec `docs/specs/2026-09-20-lago-billing-migration-design.md`; ADR `docs/adr/0012-lago-as-commercial-billing-authority.md`; current R5 evidence plan `docs/plans/issue-72-ocr86-repair-r5.md`.

## Global Constraints

- Keep evidence scripts offline; do not call live Lago/API, Docker, or mutate tenant state.
- `facts.json` is the canonical result: publish complete JSON atomically, and never emit a PASS stdout marker before successful publication.
- Failure output must remain bounded and credential-redacted; malformed or unavailable output paths must not produce raw tracebacks.
- Orphan batch reconciliation must use the same integer validation and fail-closed rules as the production `reconcile_batches()` path.
- Preserve historical runtime evidence artifacts byte-for-byte; modify only the owned scripts and test module.

## Review Focus

- Abrupt process interruption leaves `facts.json` absent or complete; catchable write/close/rename failures remove unpublished temporary files. A temp file left by uncatchable termination is noncanonical and must never be mistaken for `facts.json`.
- Success artifact publication failure cannot leave a PASS stdout marker.
- Failed evidence checks retain the one canonical FAIL schema from the common finalizer.
- Argument parsing keeps argparse's standard `SystemExit`; output-directory setup errors produce sanitized diagnostics and status 2.
- Monthly and top-up orphan rows with malformed balances fail closed through the same parser as normal batches; no test-only duplicate policy remains.

## Task 1: Atomic concurrent evidence and reachable reconciliation coverage

**Dependencies:** R5 Tasks 1–3 reviewed; R1 Python OCR recheck findings. **Owner:** implementer (Python behavior); **Validator:** backend_validator; **Independent reviewer:** reviewer. **Execution:** serial because all tests share one module and the helper scripts share the same evidence artifact protocol. Local commits are authorized after review; commit only owned files.

**Files:**

- Modify `docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py`
- Modify `docs/plans/issue-72-flow-evidence-86/reconcile.py`
- Modify `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`

**Consumes:** `concurrent_consumption_86.main()` and `run_probe()`; `reconcile.reconcile_batches()`; `_integer(value, field)` parser and `_write_artifact(path, payload)` atomic publication pattern.

**Produces:** `write_facts_atomic(output_dir, facts)` with same-directory temporary file, flush/close, `os.replace`, and cleanup on failure; preflight handling that returns 2 with bounded stderr (preserving argparse `SystemExit`); one canonical facts schema on failure; actual `reconcile_batches()` orphan behavior covered by tests with the shared parser; no unused `reconciliation_orphans_verdict` import/helper/test.

- [ ] **Step 1 — RED: atomic writer failures.** Add unit tests that patch the atomic publish seam to fail during serialization/write, close, and `os.replace`. Assert no truncated `facts.json` is visible, any temp file is removed for catchable exceptions, and the caller does not print PASS before artifact publication. For interruption, use a subprocess terminated during staged writing and assert only that canonical `facts.json` is absent or complete; do not assert `finally` cleanup after uncatchable termination. Run the new named tests and confirm they fail against direct `Path.write_text` behavior.
- [ ] **Step 2 — GREEN: one atomic publication path.** Implement `write_facts_atomic(output_dir, facts)` using a named temporary file in `output_dir`, JSON serialization, flush and close, then `os.replace(temp_path, output_dir / "facts.json")`; in `finally`, remove any unpublished temp file when Python regains control. Route both main's failure finalizer and successful `run_probe()` through this helper. Update `_run_failed_probe()`'s existing `Path.write_text` spy to observe the atomic rename/publish seam and assert exactly one canonical FAIL publication, including its schema and no temporary artifact after catchable completion. On success, set `verdict=PASS`, publish `facts.json`, then print `FACTS_JSON` and the PASS marker.
- [ ] **Step 3 — Prevent post-commit contradiction.** Treat successful atomic publication of final PASS facts as the result commit point. Before this point, transcript/reporting failures fail closed through the bounded FAIL finalizer. After this point, capture write, stdout write, `FACTS_JSON` reporting, flush, and close failures must not route through the FAIL finalizer or overwrite canonical PASS facts. The post-commit contract is: (a) preserve `facts.json` with `verdict=PASS`; (b) return status 0 because all evidence checks passed and only transcript recording failed; (c) never emit a `CONCURRENT CONSUMPTION FAIL` marker after commit; (d) write exactly one bounded stderr diagnostic `WARNING: evidence transcript incomplete (<ExceptionClass>)`; (e) stdout may contain no verdict marker, a complete PASS marker, or a partial PASS marker if the underlying stream accepted only part of the write, but it must never contain FAIL. Add separate injected tests for failure while recording the `FACTS_JSON` line, failure while recording the PASS line, and capture context-exit flush; assert this exact artifact/status/diagnostic contract for each case. Keep the commit boundary explicit in control flow.
- [ ] **Step 4 — RED/GREEN: preflight failure behavior.** Add tests for `FileExistsError` and `FileNotFoundError` from `prepare_output_dir()`, asserting exit status 2 and one bounded stderr diagnostic with no traceback. Add a separate argparse `--help` assertion proving argparse still exits normally with status 0. Catch setup exceptions outside the existing runtime try, re-raise `SystemExit`, and render other exceptions through `bounded_error()` to stderr; do not attempt a facts artifact when no usable output directory exists, and do not mutate a pre-existing output directory.
- [ ] **Step 5 — RED/GREEN: remove unreachable orphan policy.** Replace the direct `reconciliation_orphans_verdict()` test with tests against `reconcile_batches()` for zero-balance success, positive-balance failure, and malformed `balance_micro` failure in both monthly and top-up orphan branches. Remove the unused helper and import. Retain duplicate/ambiguous top-up face checks. Ensure regular page rows and orphan rows follow one parser/policy path without changing expected totals or output schema.
- [ ] **Step 6 — Verify.** Run `python3 -m unittest docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`; run `python3 -m py_compile` for `consume_86.py`, `reconcile.py`, `concurrent_consumption_86.py`, and `test_evidence_helpers.py`; run `git diff --check`. Verify historical JSON/TXT artifacts under `docs/plans/issue-72-flow-evidence-86` are unchanged. Record exact output, owned file hashes, and the earlier unsupported-document coverage limitation in the task report.
- [ ] **Step 7 — Independent review.** Capture after snapshot and complete incremental Review Package including added/deleted files. Obtain separate Spec Compliance and Code Quality verdicts tied to identical file hashes. Repair any real finding with scoped verification and re-review before recording the task `verified`.

## Acceptance Mapping

| R1 finding | Planned response | Evidence |
|---|---|---|
| Medium: failed finalizer writes facts non-atomically | Steps 1–2 | injected write/close/replace failures; no partial JSON/temp file |
| Medium: success writes facts non-atomically after PASS stdout | Steps 1–3 | publish failure test proves no premature PASS; post-commit failure matrix asserts canonical PASS, exit 0, no FAIL, bounded warning |
| Low: parse/output-dir preflight escapes bounded reporting | Step 4 | output-dir error tests return 2 without traceback; argparse help remains 0 |
| Low: orphan helper is dead and drifts from production parsing | Step 5 | production monthly and top-up orphan tests exercise zero, positive, malformed balances |

## Plan Self-Review

- **Coverage:** all four findings in `/tmp/issue72-python-workspace-ocr-r1-recheck-20260928.md` map to owned task steps and named observable assertions.
- **Step clarity:** each test step names the behavior, injected failure, and expected process/artifact result; implementation signatures and order are explicit.
- **Type/interface consistency:** atomic writer accepts `Path`-compatible output directory and `dict` facts; both success and failure callers pass the same schema. Orphan behavior stays inside `reconcile_batches()` and the existing `_integer` parser.
- **Review Focus:** all five failure classes have corresponding named verification in Steps 1–5, including the explicit post-commit stdout/status matrix, both orphan branches, and migration of the existing write-count test.
- **Scope:** no live acceptance is claimed; only supported Python OCR findings are addressed, and excluded Markdown/JSON files require the parallel independent document review process already recorded in the execution ledger.
