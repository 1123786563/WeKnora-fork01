# Issue #72 OCR R7 findings repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the three medium findings from the R7 supplemental review of the complete committed #72 range without weakening billing authority, evidence redaction, or recovery records.

**Architecture:** Keep the repair narrowly within the PostgreSQL race-test fixture, the #86 concurrent evidence runner and its tests, and the historical #85 readiness audit. No production billing behavior or subscription-owner rule changes. The SDD implementation and task-review loop is serial as required by the repository workflow; read-only audits and validation may run independently when their resources do not overlap.

**Tech Stack:** Go, GORM, PostgreSQL integration tests, Python 3, unittest, Markdown, git.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`; `docs/adr/0012-lago-as-commercial-billing-authority.md`; `docs/plans/issue-72-plan-84-ocr-attempt-id-fix.md`; `docs/plans/issue-72-plan-84-cross-attempt-settlement-fix-r4.md`; `docs/plans/issue-72-ocr-findings-r6.md`; `docs/plans/issue-72-dag.md`; `docs/plans/issue-72-execution-ledger.md`; R7 independent review `/tmp/issue72-r7-full-range-supplement-20260929.md`.

## Global Constraints

- “Lago Community is the authority for Customer, Plan, Subscription, Entitlement, Wallet, Credits, final rating, Customer Invoice, Payment commercial state, and Credit Note.”
- “Usage metadata contains billing dimensions and correlation identities only. It excludes prompts, outputs, knowledge content, Connector parameters, and payment secrets.”
- R6 failure artifacts must never include credentials, response bodies, request URLs, headers, or arbitrary exception text; only allowlisted diagnostic values such as the numeric HTTP status are retained.
- Keep the order-CAS winner immutable. A later successful collection remains an exact anomaly fact; this plan changes only the PostgreSQL test fixture used to exercise that path.
- The #85 audit remains a historical snapshot of checkout `cd6b0e521`; preserve its four valid acceptance gaps and its conclusion that #85 is not acceptance-ready. Current readiness must point to the integration DAG and ledger.
- No external calls, remote GitHub mutation, push, deploy, publish, or change to payment/price business rules.
- Commit policy: local task commits are authorized. Implementers commit only their owned files; the coordinator owns plan/ledger and integration bookkeeping.

## Review Focus

1. Arbitrary exception messages, embedded URLs/headers/bodies, and credential-like markers are absent from both canonical `facts.json` and stderr, including when artifact writing also fails. Numeric `WalletHTTPError.http_status` remains observable without rendering `str(exc)`.
2. The two tagged PostgreSQL order-CAS race tests create every table used by the transaction, including `PaymentAnomalyRow`; against an isolated PostgreSQL schema, each test reaches its existing 1/0 CAS and anomaly assertions without a skip or missing-relation error.
3. The #85 audit distinguishes observations made at historical checkout `cd6b0e521` from current integrated readiness; it preserves supported #85 acceptance gaps and does not revive superseded #74/#81 blocker claims.

---

## Task DAG and ownership

The three tasks are independent by source-file ownership but are dispatched serially to preserve the SDD per-task implementation/review gate.

| Task | Finding | Files owned | Depends on | Acceptance |
|---|---|---|---|---|
| Task 1 | R7-S-F2 exception text leakage | `docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py`, `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py` | None | Type/status-only sanitized diagnostics, secret marker absent from artifact and stderr, scoped and full helper tests pass |
| Task 2 | R7-S-F3 stale current-readiness wording | `docs/plans/issue-72-85-readiness-audit-2026-09-29.md` | None | Historical checkout scope is prominent; current status points to the integration DAG/ledger; valid #85 gaps are retained |
| Task 3 | R7-S-F1 PostgreSQL fixture table omission | `internal/modules/commercial/repository/commercial/order_pg_test.go` | PostgreSQL test slot is available; no source dependency | Both named `commercial_integration` CAS races pass on isolated PostgreSQL with no skip |

### Task 1: Stop arbitrary exception text in #86 failure evidence

**Dependencies:** R6 contract is approved and present at `docs/plans/issue-72-ocr-findings-r6.md`; read-only audits `/tmp/issue72-r7-full-range-supplement-20260929.md` and `/tmp/issue72-r7-audit-evidence-redaction-20260929.md` confirm that `bounded_error` currently copies arbitrary `str(exc)` into both stderr and `facts.json`. **Owner:** implementer. **Validator:** backend_validator. **Files:** modify `docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py` and `docs/plans/issue-72-flow-evidence-86/test_evidence_helpers.py` only.

**Consumes:** `WalletHTTPError.http_status: int`; `bounded_error(exc) -> dict`; `main()` failure/facts finalizer; existing `_run_failed_probe` test helper.

**Produces:** a safe error mapping containing bounded exception type, a fixed generic message for arbitrary exceptions, and `http_status` only when `exc` is a `WalletHTTPError` with an exact integer status. No exception message is copied into stderr or durable evidence.

- [ ] **Step 1 — RED:** Add `test_concurrent_failure_paths_redact_arbitrary_exception_text` that exercises both `replay-request-failure` and `observer-failure`, injects distinct URL/header/body/credential markers in the RuntimeError message, captures stderr, and asserts markers are absent from stderr and `facts.json` while failed stage, bounded exception type, and `verdict=FAIL` remain. Add `test_concurrent_secondary_artifact_failure_redacts_arbitrary_text` that makes `write_facts_atomic` raise an exception containing a distinct marker and asserts the marker is absent from stderr. Add `test_concurrent_wallet_http_error_keeps_only_numeric_status` that sends `WalletHTTPError(503)` through `main()` and asserts `http_status == 503` while its raw message is not serialized. First run exactly these three new selectors and observe failure on current behavior.
- [ ] **Step 2 — GREEN:** Change `bounded_error` in `concurrent_consumption_86.py` to construct only allowlisted fields. Use the fixed message `operation failed` for general exceptions. Preserve `http_status` only from `isinstance(exc, WalletHTTPError)` and `type(exc.http_status) is int`. Do not inspect or serialize generic exception text, args, traceback, URL, headers, or response body.
- [ ] **Step 3 — REFACTOR and verification:** From `docs/plans/issue-72-flow-evidence-86`, run `python3 -m unittest -v test_evidence_helpers.EvidenceHelpersTest.test_concurrent_failure_paths_redact_arbitrary_exception_text test_evidence_helpers.EvidenceHelpersTest.test_concurrent_secondary_artifact_failure_redacts_arbitrary_text test_evidence_helpers.EvidenceHelpersTest.test_concurrent_wallet_http_error_keeps_only_numeric_status test_evidence_helpers.EvidenceHelpersTest.test_runner_replay_request_failure_has_accurate_stage test_evidence_helpers.EvidenceHelpersTest.test_runner_observation_failure_preserves_duplicate_response`; expect all named tests PASS. Then run `python3 -m unittest test_evidence_helpers -v`, `python3 -m py_compile concurrent_consumption_86.py test_evidence_helpers.py`, and `git diff --check`; expect suite PASS, compilation PASS, clean diff. Record test count and exact hashes.
- [ ] **Step 4 — Report and commit:** Save the implementer report under this plan's SDD workspace, including RED evidence, exact commands/output, after hashes, and commit the two owned files with a focused `fix(issue-72): redact concurrent evidence failures` subject.

### Task 2: Scope the #85 readiness audit to its historical checkout

**Dependencies:** Read-only audit `/tmp/issue72-r7-audit-readiness-doc-20260929.md` confirms that the DAG/ledger absence at `cd6b0e521` is historically true, while some parent-wave and prerequisite statements are stale relative to the current integration overlay. **Owner:** mechanical_worker. **Validator:** reviewer. **Files:** modify only `docs/plans/issue-72-85-readiness-audit-2026-09-29.md`.

**Consumes:** Historical source HEAD `cd6b0e521`; current `docs/plans/issue-72-dag.md` recovery overlay and `docs/plans/issue-72-execution-ledger.md` readiness state.

**Produces:** A historical audit that prominently scopes checkout-local absences and references the current recovery sources without changing #85 acceptance conclusions.

- [ ] **Step 1 — RED:** Add an exact-text documentation check command using `rg -n` for the current-status phrases (“canonical planned DAG”, “currently has documented blocked runtime evidence”, and the “only clearly completed prerequisite evidence” language) and save the pre-edit hits to the report; it should identify all stale statements from the scoped audit. Do not treat the historically true absence at the audited `cd6b0e521` as an error.
- [ ] **Step 2 — GREEN:** Add a prominent scope note after the title: the audit is a snapshot of `cd6b0e521`, and its missing-DAG/ledger observation applies only to that checkout; direct current readiness to `issue-72-dag.md` §8 and `issue-72-execution-ledger.md`. Label `2026-09-20-lago-billing-waves.md` as the historical planning DAG, not the canonical current DAG. Reword #74/#81/#82/#83/#75 statements as checkout-scoped history plus the verified current overlay: #74 and #81 have later completion evidence; #82 code is integrated but acceptance remains unverified; #83 remains unverified; #75 a2 remains blocked; none alone clears #85. Preserve the four #85 acceptance gaps and the unready conclusion.
- [ ] **Step 3 — Verification:** Re-run the exact `rg` command and manually inspect the full audit against current DAG §8 and execution-ledger entries; assert every current-state sentence has a source pointer, the historical absence still names `cd6b0e521`, and no #85 acceptance item was removed. Run `git diff --check`; expect no stale current-status phrase or whitespace issue.
- [ ] **Step 4 — Report and commit:** Save exact before/after hashes, commands and the source lines consulted; commit only the audit document with subject `docs(issue-72): scope readiness audit to historical checkout`.

### Task 3: Migrate the payment-anomaly table in the PostgreSQL race fixture

**Dependencies:** R7 finding audit `/tmp/issue72-r7-audit-pg-fixture-20260929.md` confirms the isolated helper currently migrates `OrderRow`, `PaymentAttemptRow`, and `OutboxEvent` only, while `ConfirmPayment` writes `PaymentAnomalyRow` on a losing order-CAS claim. The currently active Craft exact-flow task shares the Docker daemon; do not start a disposable PostgreSQL container until that task releases the shared container slot. **Owner:** mechanical_worker. **Validator:** backend_validator. **Files:** modify only `internal/modules/commercial/repository/commercial/order_pg_test.go`.

**Consumes:** `testOrderPGStore(t)` isolated per-test schema; `OrderStore.ConfirmPayment` transaction path; tagged tests `TestPaymentConcurrentDistinctTransactionsOnSameAttemptKeepsOneWinner` and `TestPaymentConcurrentDistinctAttemptsStoreOnlyOrderCASWinner`.

**Produces:** The fresh PostgreSQL schema contains `PaymentAnomalyRow` so both existing races reach their CAS/anomaly assertions.

- [ ] **Step 1 — RED:** Once a dedicated isolated PostgreSQL DSN/test container is available, run both tagged tests before editing:

  ```bash
  go test -tags commercial_integration ./internal/modules/commercial/repository/commercial -run '^(TestPaymentConcurrentDistinctTransactionsOnSameAttemptKeepsOneWinner|TestPaymentConcurrentDistinctAttemptsStoreOnlyOrderCASWinner)$' -count=1 -v
  ```

  Expected before the fix: the losing-confirmation branch reports PostgreSQL `relation "commercial_payment_anomalies" does not exist`. If no isolated PostgreSQL is available, record the exact environment blocker and do not claim a RED or PostgreSQL pass; obtain the isolated local test slot before completing this task.
- [ ] **Step 2 — GREEN:** Add `&PaymentAnomalyRow{}` to the `db.AutoMigrate` call in `testOrderPGStore`. Do not change production code, indexes, barrier logic, or expected CAS/anomaly behavior.
- [ ] **Step 3 — Verification:** Re-run the exact tagged command against the same isolated PostgreSQL service; expect both named tests PASS with no skip and verify that the output has no missing-relation error. Run `go test ./internal/modules/commercial/repository/commercial -count=1` and `git diff --check`; expect package tests PASS and a clean diff. Record whether the PostgreSQL service was newly created and prove cleanup.
- [ ] **Step 4 — Report and commit:** Save the test output and fixture hash in the report and commit only `order_pg_test.go` with subject `test(commercial): migrate payment anomalies in postgres races`.

## Acceptance and issue mapping

| Source finding / acceptance | Task | Verification |
|---|---|---|
| R7-S-F2: arbitrary provider/runtime exception data must not reach evidence outputs; preserve safe numeric HTTP diagnosis | 1 | marker tests cover replay failure, observer failure, stderr, facts artifact, secondary write failure, and HTTP 503 status; full 86 helper suite |
| R7-S-F3: recovery facts must distinguish historical checkout evidence from current integrated state | 2 | scoped exact-text scan, full source comparison to DAG §8 and execution ledger, diff check |
| R7-S-F1: actual PostgreSQL race fixture must contain every table written by the transactional anomaly path | 3 | both named tagged race tests on isolated PostgreSQL without skip; repository package suite |

## Plan self-check and scheduling

- All three supplemental findings map to exactly one Task and one owned file scope (Task 1 intentionally owns its paired source/test files).
- No Task shares a source/test/document file with another Task; the common implementation/review order is serial under SDD. Root alone owns the SDD ledger and parent execution ledger.
- Task 1 emits only the unchanged `bounded_error(exc)` contract plus its safe fields; Task 2 is isolated documentation; Task 3 does not change production interfaces.
- Every test has a concrete command and expected outcome. Task 3’s PostgreSQL acceptance requires the isolated DB slot and must remain blocked-env if no such environment becomes available.
- The plan resolves no #87 price-owner, lifecycle, or R-3 payment policy. R-5 “Base 与付费订阅分别定价” is already approved and recorded; unanswered #87 dimensions remain guarded.
