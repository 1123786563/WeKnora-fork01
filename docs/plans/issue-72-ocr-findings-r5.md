# Issue #72 OCR R5 Findings Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the two valid low-severity OCR R4 findings in the Issue #86 concurrent-consumption evidence runner: preserve wallet HTTP status in failure evidence and correct its module usage documentation.

**Architecture:** Keep `call()` returning `(status, body)` and validate status at the `wallet_list()` fetch boundary before pagination parsing. Successful 2xx responses retain the existing page-fold contract; non-2xx responses raise a bounded status-only error without persisting provider response text. Update only the stale module docstring for the run-owned output directory and artifacts.

**Tech Stack:** Python 3 standard library, `unittest`.

**Spec:** Approved `docs/specs/2026-09-20-lago-billing-migration-design.md`, ADR `docs/adr/0012-lago-as-commercial-billing-authority.md`, Issue #86 evidence plan `docs/plans/issue-72-plan-86.md`, OCR R4 report `/tmp/issue72-ocr-r4-final-base-to-cf3fdb132-20260929.md`, and independent audits `/tmp/issue72-ocr-r4-status-audit-20260929.md` and `/tmp/issue72-ocr-r4-docstring-finding-audit-20260929.md`.

## Global Constraints

- Offline evidence tooling only: no network/API/Lago/Docker calls and no historical artifact changes.
- Preserve the `call(method, path, payload=None) -> (int, object | None)` contract and successful wallet pagination behavior.
- Do not include raw HTTP response bodies or credentials in persisted failure facts.
- Preserve all changes outside the two files listed in Task 1.
- Do not add a power-loss durability contract. The fsync observation is assessed as optional hardening by `/tmp/issue72-ocr-r4-fsync-audit-20260929.md`.

## Review Focus

- Non-JSON 4xx/5xx wallet responses must retain their numeric HTTP status in the raised failure; test `wallet_list()` with `(503, None)` and assert `HTTP 503` is present while no response body is included.
- Successful `(200, valid page)` response must pass a direct `wallet_list()` test with patched `call()`; the existing pagination fold test `test_all_three_readers_follow_lago_page_metadata` must continue to pass for multi-page behavior.
- The module header must describe the required `--output-dir`, automatic transcript file, and machine-readable facts file; validate with `py_compile`.

---

### Task 1: Preserve wallet fetch status and correct runner documentation

**Depends on:** R4 OCR finding audit; both source reviews are complete and identify no business-rule or external-interface ambiguity.

**Files:**
- Modify: `docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py`
- Modify: `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`

**Interfaces:**
- Consumes: `call(method: str, path: str, payload: object | None = None) -> tuple[int, object | None]` and `read_wallet_pages(fetch_page: Callable[[int], object]) -> list[dict]`.
- Produces: `wallet_list(fetch_page=None)` rejects non-2xx HTTP status with a `RuntimeError` containing the integer status before calling the pagination fold; successful 2xx page shape remains validated by `read_wallet_pages`.

- [ ] **Step 1: Add failing request-boundary regressions.** In `EvidenceHelpersTest`, add `test_wallet_list_reports_http_status` by patching `cc.call` to return `(503, None)` and asserting `cc.wallet_list()` raises `RuntimeError` containing `HTTP 503`. Add `test_wallet_list_rejects_non_2xx_without_response_content` using `(503, {"error": "provider-marker SECRET-MARKER"})`; assert neither marker is present in the exception. Add `test_wallet_list_accepts_2xx_page` using `(200, valid_single_page_body)` and assert `cc.wallet_list()` returns the wallet row. Retain `test_all_three_readers_follow_lago_page_metadata` for multi-page folding.
- [ ] **Step 2: Run the focused new test against the current implementation.** From `docs/plans/issue-72-flow-evidence-86`, run `python3 -m unittest test_evidence_helpers.EvidenceHelpersTest.test_wallet_list_reports_http_status -v`; expected before implementation: FAIL because the generic wallet-page error omits `503`.
- [ ] **Step 3: Implement status validation in `wallet_list.fetch`.** Bind `status, body = call(...)`; for status outside 200–299 raise `RuntimeError("wallet page request failed: HTTP %d" % status)`, then return `body`. Do not include or store response body text.
- [ ] **Step 4: Update the module docstring.** Replace the opening “stdout only; operator tees” wording with a concise statement that `--output-dir` is required, stdout is automatically tee'd to `<output-dir>/concurrent-output.txt`, and the verdict is published to `<output-dir>/facts.json`. Preserve all later behavioral and egress-policy documentation.
- [ ] **Step 5: Run verification.** From the repository root, run `python3 -m unittest discover -s docs/plans/issue-72-flow-evidence-86 -p test_evidence_helpers.py -v` (expected: all tests pass), `python3 -m py_compile docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py` (expected: exit 0), and `git diff --check` (expected: no output, exit 0). Confirm historical JSON/TXT artifact hashes remain unchanged and no live service was contacted.
- [ ] **Step 6: Self-review and report.** Check only the two owned files changed for this Task, report exact test outputs, and provide the before/after snapshot hashes required by the SDD Ledger. Do not commit before independent task review and validation.

## Acceptance Mapping

| OCR finding | Task | Verification |
|---|---|---|
| R4-F2: wallet HTTP status discarded | Task 1 | 503 non-JSON regression; existing valid pagination fold; complete offline helper test module |
| R4-F3: stale stdout-only module docstring | Task 1 | Exact docstring inspection; Python `py_compile` |
| R4-F1: fsync durability | Deferred optional hardening; not an approved process-level artifact contract | Audit `/tmp/issue72-ocr-r4-fsync-audit-20260929.md`; no power-loss guarantee is claimed |
