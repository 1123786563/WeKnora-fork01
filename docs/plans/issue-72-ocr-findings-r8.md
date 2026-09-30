# Issue #72 OCR R8 Findings Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair the valid high and medium findings from the completed R7 full-range OCR while preserving payment identity, retained financial evidence, exactly-once benefits, and a verifiable operator recovery path.

**Architecture:** Keep successful payment ownership in `ConfirmPayment` and the immutable outbox winner. Treat a closed-but-unbound attempt as eligible to claim its first verified success; never infer a winner when identity cannot be proven. Persist contradictory verified observations independently from the immutable primary anomaly, and use existing platform-guarded anomaly operations for disposition. Keep changes in the existing commercial repository/service/handler seams, with Python evidence-tool corrections isolated from Go changes.

**Tech Stack:** Go, GORM, Gin, SQLite unit tests, tagged PostgreSQL integration tests, Python 3 `unittest`, Markdown.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`; `docs/adr/0012-lago-as-commercial-billing-authority.md`; `CONTEXT.md`; `docs/plans/issue-72-dag.md` §8; `docs/plans/issue-72-execution-ledger.md`; prior repair `docs/plans/issue-72-ocr-findings-r7.md`; OCR report `/tmp/issue72-ocr-r7-final-base-to-ed520baa-20260929.md`; independent classifications `/tmp/issue72-r7-audit-ocr-go-20260929.md`, `/tmp/issue72-r7-audit-ocr-python-20260929.md`, `/tmp/issue72-r7-audit-ocr-ignore-20260929.md`, and architecture decisions `/tmp/issue72-r7-upgrade-architecture-20260929.md`, `/tmp/issue72-r7-conflict-attention-architecture-20260929.md`.

## Global Constraints

- Lago remains authoritative for Customer, Plan, Subscription, Entitlement, Wallet, Credits, final rating, Customer Invoice, Payment commercial state, and Credit Note.
- A verified channel success is a durable fund fact; payment success and fulfillment remain distinct states.
- Preserve exactly one immutable transaction winner per paid order and grant no benefit until the saved winner is verified against the registered attempt.
- Do not expose raw callback payloads, headers, signatures, secrets, provider response bodies, or arbitrary error strings in diagnostics or operator records.
- Payment anomaly identity remains unique on `(provider, merchant, transaction)`; the original fact is immutable and a contradictory observation must not overwrite or resolve it.
- Public payment and fulfillment states remain closed WeKnora vocabulary; every cross-space anomaly list or resolution route stays behind `RequirePlatformRefundReviewer`.
- #87 R-5 (“Base 与付费订阅分别定价”) is approved and unchanged. The independent #87 owner/lifecycle and R-3 purchase decisions remain gated in the execution ledger.
- No push, remote merge, deployment, publication, or GitHub Issue mutation is authorized.

## Review Focus

1. A first verified success after an attempt was closed must still claim and fulfill exactly once; a second success must remain an over-payment fact and cannot replace the winner.
2. Missing or conflicting winner identity on a paid top-up must create durable operator attention, leave the event replayable, and grant no Credits.
3. Same channel transaction with a different normalized observation must preserve both observations; terminal callback acknowledgment is allowed only after the contradictory observation is durably retained.
4. PostgreSQL and SQLite must use dialect-safe predicates for the reserved `transaction` column, including replay and over-payment disposal paths.
5. Pre-upgrade pending outbox events must not be silently acknowledged or dead-lettered; ambiguous historical rows remain a release gate until target-environment inventory and drain evidence exist.

---

## File map and task interfaces

| Task | Owned file groups | Interface produced |
|---|---|---|
| 1 | `repository/commercial/order.go`; `service/commercial/order_close_test.go` | `ConfirmPayment` atomically claims first success from `pending` or `closed` when transaction identity is null; `succeeded` is immutable. |
| 2 | `service/commercial/fulfillment.go`; its tests; new `commercial_fulfillment_exceptions` model/migration; commercial handler/router and tests | Invalid paid top-up winner creates a separate durable fulfillment exception; it never fabricates a verified payment anomaly; no benefit is sent; event remains replayable; platform operators can inspect it. |
| 3 | `repository/commercial/payment_anomaly.go`; `service/commercial/fulfillment.go`; repository and tagged PostgreSQL tests | Both anomaly-identity reads bind `transaction` as a GORM column, never raw SQL quoting; PostgreSQL test fixture migrates every written table. |
| 4 | anomaly-observation repository model/store; payment anomaly model/migration; callback and channel-query orchestration; admin handler/router/tests | Separate digest-idempotent conflict-observation records, written after transaction rollback from either source; amount-known provenance is explicit; callback returns terminal provider acknowledgment only for retained or exact replay; platform-only list/versioned resolve and order attention. |
| 5 | outbox model and migration; `service/commercial/fulfillment.go` and over-payment tests | Every deterministic over-payment Dead transition persists a closed terminal reason in the outbox row; logs never contain event keys or provider transaction identifiers. |
| 6 | #86 `consume_86.py`, `reconcile.py`, and `test_evidence_helpers.py` | Preflight errors are bounded and nonzero without writing into an invalid output directory; Ctrl-C before publication writes a bounded FAIL artifact and exits 130. |
| 7 | root `.gitignore` only | Comment accurately describes the exact ignored, untracked sanitized snapshot; no widening beyond the assigned exact-path exclusion. |
| 8 | `docs/plans/issue-72-outbox-upgrade-gate.md` only | Reusable operator runbook and evidence template define an auditable, quiesced old-worker drain; actual target-environment evidence remains blocked. |

## Task DAG

The SDD implementation/review loop is serial. Tasks 1–5 touch related commercial payment/outbox state and share test fixtures; Tasks 6–7 have disjoint paths but will follow in order unless independently isolated worktrees and resource checks are explicitly recorded. The existing R7 Task 3 PostgreSQL fixture repair is a prerequisite to Task 3 here because both extend the same tagged test seam. The Craft #107 task currently shares the Docker daemon; do not start or claim any disposable PostgreSQL/Docker test until its owner releases that resource.

### Task 1: Allow a closed unbound attempt to claim its first verified success

**Dependencies:** OCR Go audit finding 1 and approved close contract at `repository/commercial/order.go` around `MarkAttemptClosed`; test gap confirmed in `service/commercial/order_close_test.go:227–286`. **Owner:** backend_implementer. **Validator:** backend_validator. **Files:** modify `internal/modules/commercial/repository/commercial/order.go` and `internal/modules/commercial/service/commercial/order_close_test.go` only.

**Consumes:** `OrderStore.ConfirmPayment(ctx, PaymentFact)`; `PaymentAttemptStatePending`, `PaymentAttemptStateClosed`, and `PaymentAttemptStateSucceeded`; existing `TestLateSuccessAfterCloseAuditsWithoutSecondFulfillment`.

**Produces:** Conditional claim from `pending` or `closed` only while `provider_transaction_id IS NULL`; a succeeded attempt and its provider transaction remain immutable. A late first success after close stores the exact winning transaction and produces one fulfill event; a later distinct success continues down the existing over-payment path without a second fulfill event.

- [x] **Step 1 — RED:** Extend `TestLateSuccessAfterCloseAuditsWithoutSecondFulfillment` reverse-order case to assert the attempt becomes `succeeded`, its transaction equals `wx_txn_late`, and a fulfillment drain can validate the saved winner. Exercise both quoted and unquoted purchase routes through a fulfillment drain. Add a second-success assertion proving the original transaction remains the winner and one over-payment fact is retained. Run the focused test and record the failing state/assertion.
- [x] **Step 2 — GREEN:** Change the guarded payment-attempt claim predicate to accept `pending` and `closed` while retaining `provider_transaction_id IS NULL`; leave the succeeded/same-transaction idempotency and order-CAS guards unchanged.
- [x] **Step 3 — Verification:** Run `go test ./internal/modules/commercial/service/commercial -run '^TestLateSuccessAfterCloseAuditsWithoutSecondFulfillment$' -count=1 -v`, then the repository package suite and `git diff --check`; expect PASS and no second fulfillment identity.
- [x] **Step 4 — Report and commit:** Record RED/GREEN outputs and source/test hashes, then commit only the owned files as `fix(commercial): claim late success after close`.

### Task 2: Keep paid top-up winner failures visible and replayable

**Dependencies:** Task 1 verified; architecture report `/tmp/issue72-r7-conflict-attention-architecture-20260929.md` §2. **Owner:** backend_implementer. **Validator:** backend_validator. **Files:** `internal/modules/commercial/service/commercial/fulfillment.go`, its tests, `internal/handler/commercial.go`, `internal/router/routes_commercial.go`, handler/router tests, and new migration/model files `migrations/sqlite/000110_commercial_fulfillment_exceptions.{up,down}.sql` and `migrations/versioned/000189_commercial_fulfillment_exceptions.{up,down}.sql`.

**Consumes:** `fulfillEventPayload`; `winningPaymentTransaction`; `TopUpOrderLines`; `domain.FulfillmentKey(order.ID, "credits")`; platform middleware `RequirePlatformRefundReviewer`.

**Produces:** A separate `FulfillmentExceptionRow` keyed by the internal outbox event key (never returned or logged) stores opaque ID, tenant/order IDs, fixed reason, state, and timestamps. When a paid unquoted top-up winner cannot be proven, the worker persists/reuses one `invalid_winning_payment` exception, leaves the outbox event Pending, and makes no gateway call. It does not write `PaymentAnomalyRow`, because the external transaction is not established as the registered winner. An additive platform-only `GET /admin/fulfillment-attentions` returns only opaque exception ID, order/tenant IDs, kind, fixed reason, state and timestamps. It never returns event key, provider, merchant, transaction, payload, or credentials. No manual “resolve” action may bypass payment identity. After exact winner validation and successful benefit reconciliation, the exception is marked resolved in the same transaction that sends the event. Existing applied benefit records are never downgraded.

- [x] **Step 1 — RED:** Add an unquoted and a quoted invalid-winner test. For the unquoted case assert the current code marks the event Dead; specify desired behavior: one exception row, event Pending, no `ExternalID`, no gateway calls. Add repeated-drain idempotency, storage-failure, platform-list/space-admin-403/unauthenticated-403 tests. Capture current failure.
- [x] **Step 2 — GREEN:** Add the typed exception model and production/versioned migrations above; ensure the fulfillment service startup migrates the model. In the invalid-winner branch, create the exception idempotently and complete no external benefit. Preserve any existing Applied `FulfillmentRecord`. Add the platform-only list endpoint and redact internal event identity in every response/log. Resolve only after validated fulfillment and event-sent update commit atomically.
- [x] **Step 3 — Verification:** Run focused service and handler tests; run commercial service and handler package suites and `git diff --check`. Assert paid/fulfilled axes are not rewritten, invalid winners never reach the gateway, each tenant remains scoped, and a corrected verified event can grant once and resolve the exception.
- [x] **Step 4 — Report and commit:** Record sanitized row state, route guard outcomes, exact hashes/tests, and commit only owned files as `fix(commercial): retain top-up fulfillment attention`.

**R8 Task 2 closeout:** implementation commit `11470a6a34d9be513aafd58c0db67aed4a5e4c15`, fulfilled/open-exception fix `bbf79a784ace4dad4c470d61fe44a1c70a4bfd60`, tenant-scoped receipt fix `aaf0b4960c215f71395c12455b3016df3b6a54f2`, and supplemental RED evidence commit `32aadf55ca19a08e6ae845431024c753d54fe5bb` were integrated to `codex/issue-72-lago`. Independent review/validation reports and final hashes are in the execution Ledger. The PostgreSQL runtime migration remains an environment limitation because Craft #107 has not released the shared DB resource; do not treat SQLite service tests as PostgreSQL migration evidence.
### Task 3: Use PostgreSQL-safe anomaly identity predicates

**Dependencies:** Tasks 1–2 and R7 Task 3 (`docs/plans/issue-72-ocr-findings-r7.md`) verified; isolated DB slot released; OCR Go audit findings 2 and 4. **Owner:** backend_implementer. **Validator:** backend_validator. **Files:** modify `internal/modules/commercial/repository/commercial/payment_anomaly.go`, `internal/modules/commercial/service/commercial/fulfillment.go`, `internal/modules/commercial/repository/commercial/order_pg_test.go`, and focused repository/service tests only.

**Consumes:** `recordPaymentAnomalyTx`, `disposeOverPayment`, and the `commercial_integration` PostgreSQL test fixture.

**Produces:** Every equality lookup on the reserved `transaction` column uses a structured GORM `clause.Column`/`clause.Eq` or map predicate. PostgreSQL migration includes `PaymentAnomalyRow`; replay and over-payment disposal each have a PostgreSQL-tagged behavioral assertion.

- [ ] **Step 1 — RED:** Add tagged tests for same-fact replay, contradictory-fact lookup, and valid over-payment event disposal. Run against a fresh isolated PostgreSQL schema and record exact failure; do not substitute SQLite SQL compatibility or a skipped tagged run.
- [ ] **Step 2 — GREEN:** Replace raw backtick SQL conditions in both repository and consumer paths with dialect-safe structured predicates. Keep the existing transaction identity and exact immutable-fact comparisons unchanged.
- [ ] **Step 3 — Verification:** Run the focused tagged PostgreSQL tests against the same isolated schema with no skip; run the two commercial Go package suites, and `git diff --check`. Record the test DSN only as present/absent, never its value, and record cleanup.
- [ ] **Step 4 — Report and commit:** Save test output and schema/source hashes; commit only owned files as `fix(commercial): use dialect-safe anomaly lookup`.

### Task 4: Retain contradictory verified payment observations independently

**Dependencies:** Task 3 verified; architecture report `/tmp/issue72-r7-conflict-attention-architecture-20260929.md` §1. **Owner:** backend_implementer. **Validator:** backend_validator. **Files:** `internal/modules/commercial/repository/commercial/payment_anomaly.go`, `order.go`, `payment_anomaly_test.go`, `order_test.go`; service `order.go` and tests; callback `payment_callbacks.go` and tests; admin `commercial.go`, `routes_commercial.go`, tests; plus `migrations/sqlite/000111_commercial_payment_anomaly_conflicts.{up,down}.sql` and `migrations/versioned/000190_commercial_payment_anomaly_conflicts.{up,down}.sql`.

**Consumes:** unique primary anomaly identity `(provider,merchant,transaction)`, `ErrPaymentAnomalyConflict`, provider-specific callback ACK contracts, platform guard, and `OrderService.withAttention`.

**Produces:** `PaymentAnomalyConflictObservationRow` is append-only and digest-idempotent. It stores opaque ID, parent anomaly ID/identity, tenant/order/merchant-order identity, fixed observed kind, amount/currency, `ActualAmountKnown`, source (`callback` or `channel_query`), canonical SHA-256 digest, state/version/timestamps. A same digest under the same payment identity is exact replay; distinct normalized observations are separate records. The original `PaymentAnomalyRow` is never rewritten. Add nullable `PaymentAnomalyRow.ActualAmountKnown`: new callback writes set true; a channel query with omitted amount sets false and stores `ActualAmountFen=0`; existing migrated rows stay NULL (unknown provenance) and are not backfilled as known. Include the new conflict model and column in the production GORM migration seam `NewOrderService`, plus the numbered SQLite/versioned migrations above. `samePaymentAnomalyFact` treats NULL as unknown, not as a fabricated known face.

Both `ConfirmPayment` conflicts and channel-query `RecordPaymentAnomaly` conflicts use one normalized observation type and the same independent recorder after their respective write transaction has rolled back. The channel-query recovery path must catch the conflict too; no path may substitute the registered attempt amount for an unreported observed amount. `RecordConflictingPaymentObservation(ctx, observation) (bool, error)` verifies the primary anomaly using structured predicates, inserts or finds the exact digest in its own committed transaction, and returns retained/replayed only after that commit. Only then may callback orchestration return the retained-conflict sentinel for the exact WeChat/Alipay terminal ACK. Failure stays retryable. Add platform-only `GET /admin/payment-anomaly-conflicts` and `POST /:id/resolve` with required `expected_version`, state+version CAS, separate conflict IDs, sanitized typed amounts with `amount_known`, and order identity. Resolving the primary anomaly never resolves conflict rows or vice versa. Unresolved conflicts are included in `HasUnresolvedPaymentAnomaly` so order reads carry attention without altering paid/fulfilled primary state.

- [ ] **Step 1 — RED:** Add repository tests for known/unknown amount provenance, exact repeat (one conflict row), distinct observation (another row), original primary row immutability, migrated NULL provenance, resolved primary remaining resolved, and unknown query → known callback plus known callback → distinct query. Specifically assert a known callback does not compare equal to a migrated NULL row even when `ActualAmountFen` matches, is retained before ACK, a repeat matches the retained digest, and retention failure remains retryable. Add same-transaction over-payment conflict, repeated callback, and failed-retention tests. Add channel-query conflict tests. Add callback tests proving storage failure is retryable and retained/exact replay produces exact provider ACK bodies. Add platform list/version-CAS resolve and authorization tests. Run focused tests and capture failures.
- [ ] **Step 2 — GREEN:** Add model/schema migrations and production migration wiring. Implement canonical digest and source-tagged normalized observation. Route both transaction paths after rollback through the separate recorder. Use dialect-safe GORM column predicates. Add list/resolve routes with the exact platform guard and expected-version CAS; include unresolved observations in order attention.
- [ ] **Step 3 — Verification:** Run focused repository, recovery, callback and handler tests; all commercial repository/service/handler packages; tagged PostgreSQL tests after its isolated slot releases; migration up/down against fresh and existing SQLite/PostgreSQL schemas. Expect original facts immutable, unknown amounts never reported as known, no terminal ACK without a committed conflict row, and no cross-space access. Run `git diff --check`.
- [ ] **Step 4 — Report and commit:** Record redacted primary/conflict row snapshots, source (query/callback), amount-known values, response codes, migration and test evidence/hashes; commit only owned files as `feat(commercial): retain conflicting payment observations`.

### Task 5: Persist bounded over-payment terminal reasons

**Dependencies:** Tasks 2–4 verified. **Owner:** backend_implementer. **Validator:** backend_validator. **Files:** `internal/modules/commercial/repository/commercial/outbox.go`, `internal/modules/commercial/service/commercial/fulfillment.go`, focused tests, `migrations/sqlite/000112_commercial_outbox_terminal_reason.{up,down}.sql`, and `migrations/versioned/000191_commercial_outbox_terminal_reason.{up,down}.sql`.

**Consumes:** `disposeOverPayment`, `completeEvent`, the outbox migration path, and closed reason codes.

**Produces:** Outbox rows gain nullable/empty `terminal_reason`. Every deterministic over-payment Dead transition stores a fixed reason code in the same transaction as the state transition. Logs may name only the fixed kind/reason; they must not emit `EventKey`, provider, merchant, transaction, payload, or arbitrary parsed data. Transient lookup/storage failures remain pending and leave the reason empty.

- [ ] **Step 1 — RED:** Add representative malformed-envelope, invalid-order/winner, missing-anomaly, and contradictory-record tests asserting a fixed `terminal_reason`; add a logger capture with a secret-like transaction marker and malformed event-key marker and assert neither appears. Run focused tests and capture failure.
- [ ] **Step 2 — GREEN:** Add the outbox field and additive migrations; route deterministic terminal transitions through a transaction that writes `state=dead` and the closed reason atomically. Remove the raw event key from warning messages. Leave retryable errors unchanged.
- [ ] **Step 3 — Verification:** Run all over-payment tests and commercial repository/service suites; test migrations on fresh/upgraded databases; inspect captured logs for reason presence and zero raw marker; run `git diff --check`.
- [ ] **Step 4 — Report and commit:** Record before/after schema and source hashes plus command output; commit only owned files as `fix(commercial): persist over-payment terminal reasons`.

### Task 6: Bound #86 evidence-tool preflight and interruption failures

**Dependencies:** None; independent from Go tasks. **Owner:** implementer. **Validator:** reviewer. **Files:** `docs/plans/issue-72-flow-evidence-86/consume_86.py`, `reconcile.py`, and `test_evidence_helpers.py` only.

**Consumes:** `prepare_output_dir`, `_write_artifact`, existing bounded safe error conventions in `concurrent_consumption_86.py`, and the fresh nonexistent-output-dir requirement in the #86 README.

**Produces:** `consume_86.py` returns a fixed bounded output-dir preflight error without leaking paths or writing into an existing directory; argparse help/usage behavior is unchanged. `reconcile.py` catches Ctrl-C during uncommitted processing, best-effort writes a sanitized `RECONCILE FAIL` artifact with current stage and `reason=interrupted`, exits 130, and never overwrites an already published canonical PASS/FAIL artifact.

- [x] **Step 1 — RED:** Add tests for output path already exists, parent missing, and help; interruption before artifact publication; and Ctrl-C after publication. Assert no path/error marker leaks, no output directory is overwritten, pre-publication creates a bounded artifact and returns 130, and post-publication artifact/verdict is unchanged. Run focused selectors and capture expected failures.
- [x] **Step 2 — GREEN:** Add bounded preflight error handling in `consume_86.py`; add a specific `KeyboardInterrupt` branch in `reconcile.py` scoped to processing before canonical publication, preserving published verdict semantics. Three scoped review rounds additionally hardened exact-byte verification and atomic create-if-absent failure publication.
- [x] **Step 3 — Verification:** From the evidence directory, run all new selectors and `python3 -m unittest test_evidence_helpers -v`, `python3 -m py_compile consume_86.py reconcile.py test_evidence_helpers.py`, and `git diff --check`; expect PASS.
- [x] **Step 4 — Report and commit:** Save output/hashes and commit only the three owned files as `fix(issue-72): bound evidence runner failure paths`.

### Task 7: Describe the exact ignored #86 snapshot rule accurately

**Dependencies:** None. **Owner:** mechanical_worker. **Validator:** reviewer. **Files:** root `.gitignore` only.

**Consumes:** OCR ignore audit `/tmp/issue72-r7-audit-ocr-ignore-20260929.md`; assigned exact-path rule in `docs/plans/issue-72-unsupported-docs-r1.md` Step 5.

**Produces:** The comment says the exact rule prevents accidental normal staging of the currently untracked sanitized snapshot and is defense in depth; the exact ignore pattern remains unchanged. The medium directory-wide exposure claim is not implemented because the audit found no current raw sibling and the assignment explicitly requires the exact path.

- [x] **Step 1 — RED:** Record `git check-ignore -v docs/plans/issue-72-flow-evidence-86/ocr-r2-replay/backend.env.snapshot` and `git ls-files --error-unmatch ...` results; verify the exact rule and untracked status.
- [x] **Step 2 — GREEN:** Rewrite only the comment to say that sanitation is the first protection and the exact ignore rule prevents accidental normal staging of this untracked snapshot; state that Git ignore does not protect force-added or already-tracked files.
- [x] **Step 3 — Verification:** Re-run both path checks and `git diff --check`; verify no rule pattern changed.
- [x] **Step 4 — Report and commit:** Record before/after file hash and commit only `.gitignore` as `docs(issue-72): clarify evidence snapshot ignore rule`.

### Task 8: Add an auditable old-outbox upgrade runbook

**Dependencies:** Draft may be prepared concurrently from the pinned R8 interfaces; final query validation and commit depend on Tasks 2, 4, and 5 being verified and integrated. **Owner:** mechanical_worker. **Validator:** reviewer. **Files:** create `docs/plans/issue-72-outbox-upgrade-gate.md` only.

**Consumes:** architecture report `/tmp/issue72-r7-upgrade-architecture-20260929.md`; `commercial_outbox_events`, payment anomaly, fulfillment exception/record, and conflict-observation schemas after R8; PostgreSQL production dialect.

**Produces:** A repeatable release procedure and evidence template. It records target environment/application versions, operator, ingress-pause and worker-stop timestamps, old-worker-only drain window, lease-expiry wait, outbox counts by kind/state, active leases, old payload identity-shape counts, over-payment-to-anomaly correlation counts, sent/dead financial-event reconciliation to anomaly/benefit receipts, final zero criterion, rollback-to-paused instructions, and signoff. Queries emit counts and fixed classifications only; evidence must omit event key, transaction, merchant, payload, credentials and raw provider text. Zero Pending alone is insufficient. Any unreadable or ambiguous event blocks switching.

- [ ] **Step 1:** Verify exact table/column names and payload fields in the integrated migration/model definitions; write PostgreSQL read-only queries that produce counts by outbox kind/state and active lease status, missing attempt/provider/merchant/transaction payload identity, unmatched pending over-payment anomaly, and sent/dead financial events without expected anomaly/receipt reconciliation. Keep raw identities out of result columns.
- [ ] **Step 2:** Write quiesce → old-worker drain → lease wait → inventory/reconcile → zero criterion → new-worker enable procedure and explicit rollback to paused ingress/old binary; include a structured evidence template and owner/signoff fields.
- [ ] **Step 3:** Reviewer verifies every query is read-only, redacted, runnable against the exact schema and cannot treat a live ingress race or expired-but-active lease as zero. Run Markdown link/format checks and `git diff --check`.
- [ ] **Step 4:** Commit only the runbook as `docs(issue-72): define outbox upgrade gate`.

## Conditional upgrade compatibility release gate

OCR Go finding 6 is valid but conditional. The architecture audit `/tmp/issue72-r7-upgrade-architecture-20260929.md` confirms that some events written by base `84d17f1` already carry winner identity, while old `over_payment` rows may lack the transaction-time anomaly and older confirmation behavior could overwrite same-attempt transaction identity. Repository contents cannot prove the target deployment has no such rows.

Task 8 provides the read-only queries and structured record. Before switching an existing deployment from the old consumer to this implementation, quiesce payment-confirmation ingress and mixed workers, let the old worker drain, wait past all active leases, then use the runbook to collect redacted counts by outbox kind/state, producer shape, anomaly correlation, and fulfillment receipts. A zero queue count without ingress quiescence is insufficient. If any ambiguous historical event remains, keep the switch blocked; do not infer a winner, synthesize a payment fact, or acknowledge/dead-letter it. The actual target database inventory and drain evidence are not available in this code task and must remain explicitly blocked until supplied.

## Acceptance and finding mapping

| Finding | Planned disposition |
|---|---|
| Supplement R7-S-F1 | Prior R7 Task 3: add anomaly table to the tagged PostgreSQL race fixture; must verify on isolated PostgreSQL. |
| Supplement R7-S-F2 | Verified fixed in Task 1 commit `b5e651c9a`; source markers, full 99-test helper module, py_compile, and independent review/validation recorded. |
| Supplement R7-S-F3 | Verified historical/current scope fix in Task 2 commit `af272a214`; independent review/validation pass. |
| OCR `.gitignore` directory-scope MEDIUM | Rejected at claimed severity based on no current raw sibling and exact-path plan requirement; retain exact rule. |
| OCR `.gitignore` comment LOW | Task 7. |
| OCR Python consume preflight LOW | Task 6. |
| OCR Python reconcile interrupt LOW | Task 6. |
| OCR late closed-attempt HIGH | Task 1. |
| OCR PostgreSQL raw backtick HIGHs | Task 3. |
| OCR contradictory anomaly observation MEDIUM | Task 4. |
| OCR legacy event compatibility conditional HIGH | Task 8 supplies auditable procedure; target database evidence remains unavailable and the release gate stays open. |
| OCR over-payment reasonless Dead MEDIUM | Task 5. |
| OCR unquoted invalid-winner Dead HIGH | Task 2. |

## Plan self-check and scheduling

- Each valid OCR high/medium maps to a repair task or an explicit conditional release gate; the one medium-severity security claim without a present exposure is rejected with its evidence in `/tmp/issue72-r7-audit-ocr-ignore-20260929.md`.
- Tasks 1–5 form a serial payment/outbox chain; Task 2 consumes Task 1’s winner invariant, Task 3 supplies dialect-safe anomaly reads and the PostgreSQL fixture, Task 4 follows the shared immutable-fact seam, and Task 5 persists final terminal reasons after state behavior is settled.
- Tasks 6–7 own disjoint files and have no runtime-resource overlap; after the plan commit they may be dispatched concurrently in separate worktrees with independent review packages. Task 8 may be drafted in parallel, but its final query/schema validation and commit depend on Tasks 2, 4, and 5 being verified and integrated.
- No task changes the approved #87 R-5 price split or resolves outstanding owner/lifecycle/R-3 policy questions.
- Task 2 keeps invalid winner facts out of `PaymentAnomalyRow`; Task 4 covers conflicts from both store and query paths with an explicit tri-state amount comparator, and represents historical NULL amount provenance conservatively (known callback against NULL is never ACKed without a committed conflict observation).
- Task 5 persists only fixed reason codes and never logs channel transaction identifiers.
- The target deployment’s old outbox inventory, worker quiescence, and drain outcome cannot be inferred from local tests; the release gate stays open until matching operational evidence exists.
