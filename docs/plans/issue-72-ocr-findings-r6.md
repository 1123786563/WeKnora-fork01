# Issue #72 OCR R6 Evidence Runner Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close valid OCR R5 findings about partial-run resource traceability, per-request fallback credential lookup, and wallet HTTP status lost across the evidence readers.

**Architecture:** Keep all scripts fail closed and offline-testable. Store generated correlation candidates separately from resources whose successful creation was observed; never copy provider response objects into facts. Resolve fallback credentials once per `consume_86.main()` run while preserving fresh lazy lookup for direct helper calls. Introduce a status-only `WalletHTTPError` for all three wallet readers, convert only numeric response status, and surface it in canonical failure artifacts without broadening generic exception text.

**Tech Stack:** Python 3 standard library, `unittest`.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`, `docs/adr/0012-lago-as-commercial-billing-authority.md`, `docs/plans/issue-72-plan-86.md`, `docs/plans/issue-72-ocr-findings-r5.md`, OCR reports `/tmp/issue72-ocr-r5-final-base-to-f2972ce6e-20260929.md` and `/tmp/issue72-ocr-r5-workspace-20260929.md`.

## Global Constraints

- Offline evidence tooling only; no network/API/Lago/Docker calls or historical artifact edits.
- Preserve fail-closed behavior and current successful wallet page schema/pagination semantics.
- Never include credentials, response bodies, request URLs, headers, or arbitrary exception text in facts/artifacts.
- Keep `--help` free of credential discovery; one main invocation must use one stable resolved credential.
- Keep current pre-existing changes outside each Task's snapshot and Review Package. The `consume_86.py` checkpoint predates R6; record it as each task's starting snapshot, not as part of that task's delta.
- Tasks run serially because they share `test_evidence_helpers.py`; this is a file scheduling constraint, not a business dependency.

## Review Focus

- Failure artifacts distinguish generated identifier candidates from confirmed successful creations and exclude provider/secret data (Tasks 1–2).
- Fallback credential resolution runs once per main invocation, while separate direct requests observe the current environment on each call (Task 3).
- JSON and non-JSON 401/503 wallet failures retain only numeric status in reader errors and canonical artifacts, across all three readers (Task 4).
- Successful 2xx responses still pass strict page validation and malformed 2xx pages still fail (Tasks 1–4).

---

### Task 1: Trace concurrent runner setup identifiers

**Files:** `docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py`, `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`.

**Interfaces:** `run_probe(output_dir: Path, state: dict) -> int` passes `state["observations"]` to canonical FAIL facts. Add `observations["generated_candidates"]` for locally generated tag/metric/plan/subscription codes before the first POST, and `observations["created_resources"]` containing only local codes whose creation received a successful response.

- [ ] **Step 1 — RED test.** Add `EvidenceHelpersTest.test_concurrent_early_setup_failure_preserves_run_codes`. Patch `cc.call` to return 201 for metric creation and a marked non-success plan response. Run `cc.main()` in a fresh temporary output directory. Assert nonzero exit, `facts.json` has `verdict=FAIL` and `failed_stage=create_plan`, all generated candidates are labeled under `generated_candidates`, only the metric code is under `created_resources`, and provider-body/secret markers are absent.
- [ ] **Step 2 — demonstrate RED.** From `docs/plans/issue-72-flow-evidence-86`, run `python3 -m unittest test_evidence_helpers.EvidenceHelpersTest.test_concurrent_early_setup_failure_preserves_run_codes -v`; expected: missing identifier fields.
- [ ] **Step 3 — implement.** Store bounded generated codes under `generated_candidates` immediately after generation; after each successful metric/plan/subscription creation response, add only the corresponding local code under `created_resources`. Never persist Lago IDs or response bodies.
- [ ] **Step 4 — GREEN/checkpoint.** Run the focused test and `test_runner_event_post_failure_writes_redacted_failed_facts`, then the full helper suite, py_compile of the two files, and `git diff --check`. Save before/after snapshots and hashes; confirm the 19 historical JSON/TXT files remain unchanged.

### Task 2: Preserve consume runner resource identifiers in failure facts

**Depends on:** Task 1 reviewed and its checkpoint remains integrated. Shared test module requires serial execution.

**Files:** `docs/plans/issue-72-flow-evidence-86/consume_86.py`, `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`.

**Interfaces:** Existing `result` is serialized to both PASS and FAIL artifacts. Add `result["resource_codes"]`, containing only local metric/plan/subscription codes and transaction IDs whose corresponding operation returned success; never include provider Lago IDs or response objects.

- [ ] **Step 1 — RED test.** Add `EvidenceHelpersTest.test_consume_failures_preserve_progressive_resource_codes` with separate `plan_create`, `subscription_create`, and `event_post` failures. Mock successful earlier resource responses with opaque Lago ID fields and return a body/secret marker for the failing response. Assert each `consume-cny.json` has the expected failed stage and only the local codes from successful earlier operations; assert absence of all provider IDs, body markers, and secret markers.
- [ ] **Step 2 — demonstrate RED.** Run that exact method from the evidence directory; expected: early artifacts omit `resource_codes`.
- [ ] **Step 3 — implement.** After each metric, plan, subscription, and event operation succeeds, incrementally add its corresponding local code/transaction ID to `result["resource_codes"]`. Preserve existing final PASS fields and the redacted exception behavior.
- [ ] **Step 4 — GREEN/checkpoint.** Run the focused method and existing `test_runner_event_post_failure_writes_redacted_failed_facts`, full helper suite, py_compile of the two files, and `git diff --check`. Save exact before/after snapshots and hashes; verify all 19 historical JSON/TXT files unchanged.

### Task 3: Resolve fallback credentials once per consume invocation

**Depends on:** Task 2 reviewed and integrated; serial because `consume_86.py` and the helper module are shared.

**Files:** `docs/plans/issue-72-flow-evidence-86/consume_86.py`, `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`.

**Interfaces:** Change to `request(method, path, payload=None, *, key=None)`. If `key is not None`, use it; otherwise call `api_key()` for this request. Add keyword-only `key=None` to `wallets(fetch_page=None, *, key=None)` and `balance_snapshot(*, key=None)` and forward only explicit keys. `main()` resolves once after output setup and passes that key through every setup/event and wallet polling request.

- [ ] **Step 1 — RED tests.** Add `test_consume_main_resolves_key_once_and_reuses_for_all_requests`: clear `LAGO_API_KEY`, patch `api_key()` to yield `initial-run-key` then `rotated-run-key`, fake main-path HTTP/page replies, force a settlement poll, and assert exactly one resolver call and that every captured request key is `initial-run-key`. Add `test_consume_request_explicit_key_skips_lazy_lookup` by making `api_key()` raise and capturing the Authorization header from mocked `OPENER.open`; assert explicit key used. Add `test_consume_direct_requests_resolve_current_key_each_time` that changes `LAGO_API_KEY` between two direct `request()` calls and asserts the opener sees both current values respectively.
- [ ] **Step 2 — demonstrate RED.** Run those three exact test methods; expected: unsupported explicit key/main run stability or direct current-key contract failure.
- [ ] **Step 3 — implement key threading.** Implement the signatures above, resolve inside main's guarded execution after `prepare_output_dir`, and pass the one value through the run. Preserve fresh lazy resolution when direct callers omit `key`. Update every existing main-path `consume.request` fake callback in `test_evidence_helpers.py` to accept `key=None`.
- [ ] **Step 4 — GREEN/checkpoint.** Run the three tests, existing fallback credential/help tests, full helper suite, py_compile, and `git diff --check`. Save snapshots/hashes and verify historical JSON/TXT artifacts unchanged.

### Task 4: Preserve HTTP status through all readers and canonical artifacts

**Depends on:** Tasks 1–3 reviewed and integrated. This final Task owns the shared exception interface and final test edits.

**Files:** `docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py`, `docs/plans/issue-72-flow-evidence-86/consume_86.py`, `docs/plans/issue-72-flow-evidence-86/reconcile.py`, `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py`.

**Interfaces:** Add `WalletHTTPError(RuntimeError)` to `concurrent_consumption_86.py`, with a validated integer `http_status` and safe message containing only `HTTP <status>`. Each reader rejects non-2xx before page folding. `reconcile._safe_reason()` returns `HTTP <status>` only for this class; generic exceptions remain class-only. Consume FAIL artifact adds `error.http_status` only for this exact class. `consume_86.py` imports the existing shared `decode_error_body()` for best-effort HTTP error parsing.

- [ ] **Step 1 — RED reader and canonical artifact tests.** Add these eight `EvidenceHelpersTest` methods: `test_consume_wallet_http_error_without_json_preserves_status` uses mocked `OPENER.open` raising a 503 `HTTPError` with non-JSON body; `test_consume_wallet_reader_rejects_json_non_2xx` returns status 503 with JSON body markers; `test_consume_wallet_reader_accepts_200_page` returns a valid one-page response; `test_reconcile_wallet_http_error_preserves_status` raises marked 401 `HTTPError`; `test_reconcile_wallet_reader_rejects_returned_non_2xx` returns `status=503` with valid page-shaped JSON; `test_reconcile_wallet_reader_accepts_200_page` returns valid 200; `test_consume_wallet_http_status_failure_artifact_is_bounded` runs `consume.main()` and asserts `consume-cny.json` has `failed_stage=before_balance_snapshot`, `error.http_status=503`, nonzero exit, and no body/URL/header/secret markers; `test_reconcile_http_status_failure_artifact_is_bounded` runs `reconcile.main()` with temporary valid account JSON and mocked opener raising marked HTTPError 503, then asserts `reconcile-output.txt` has `stage=lago_fetch`, `reason=HTTP 503`, nonzero exit, and no markers in artifact or stderr.
- [ ] **Step 2 — demonstrate RED.** From the evidence directory, run `python3 -m unittest -v test_evidence_helpers.EvidenceHelpersTest.test_consume_wallet_http_error_without_json_preserves_status test_evidence_helpers.EvidenceHelpersTest.test_consume_wallet_reader_rejects_json_non_2xx test_evidence_helpers.EvidenceHelpersTest.test_consume_wallet_reader_accepts_200_page test_evidence_helpers.EvidenceHelpersTest.test_reconcile_wallet_http_error_preserves_status test_evidence_helpers.EvidenceHelpersTest.test_reconcile_wallet_reader_rejects_returned_non_2xx test_evidence_helpers.EvidenceHelpersTest.test_reconcile_wallet_reader_accepts_200_page test_evidence_helpers.EvidenceHelpersTest.test_consume_wallet_http_status_failure_artifact_is_bounded test_evidence_helpers.EvidenceHelpersTest.test_reconcile_http_status_failure_artifact_is_bounded`; expected: the two valid-200 page tests pass, while the other six fail on missing/non-JSON status preservation, accepted reconcile 503, or missing canonical artifact status.
- [ ] **Step 3 — implement shared status behavior.** `WalletHTTPError(status)` accepts `type(status) is int` and contains no fields from response/request beyond that code. Concurrent and consume readers discard non-2xx body. Adjust `consume_86.request()` HTTPError handling to decode error bodies best-effort with existing `decode_error_body()` so invalid JSON becomes `None` while the numeric status survives; keep valid JSON error bodies for existing POST/event callers. Reconcile checks integer `resp.status` before parsing and rejects non-2xx; catch raised HTTPError, copy only `.code`, and raise `WalletHTTPError(code) from None` without reading body, URL, or headers.
- [ ] **Step 4 — expose only the safe status.** In `_safe_reason()`, special-case only `WalletHTTPError` as `HTTP <status>`, preserving class-only behavior for all other exceptions. In consume's canonical failure builder, include integer `http_status` only for `WalletHTTPError`; leave generic error type/message unchanged.
- [ ] **Step 5 — GREEN/checkpoint.** Run all eight new named tests, existing three-reader multi-page/malformed-page tests and generic redaction tests, full helper suite, py_compile for all four files, and `git diff --check`. Confirm all 19 historical JSON/TXT files unchanged and no external call occurred. Save exact before/after snapshots and hashes.

## Acceptance Mapping

| OCR finding | Task | Verification |
|---|---|---|
| R5 final F1: concurrent partial setup lacks traceability | 1 | FAIL facts separate generated candidates from confirmed local codes |
| Workspace F2: consume early FAIL facts omit prior successful resources | 2 | Plan/subscription/event stage cases and redaction |
| Workspace F1: fallback `.env` lookup repeats each request | 3 | One resolver call per main run; direct calls see current environment |
| Workspace final F2: reconcile loses HTTP status | 4 | Raised and returned non-2xx; text artifact stage/status/redaction |
| R5 final F2: consume loses HTTP status | 4 | JSON/non-JSON HTTP failures; JSON artifact status/redaction |
| Successful wallet contract | 1–4 | Actual-boundary 200 cases plus existing multi-page/malformed page tests |
