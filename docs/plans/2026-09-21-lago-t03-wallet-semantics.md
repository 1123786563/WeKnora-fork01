# Lago T03 — Wallet 批次到期、消费顺序与撤回语义 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Using real Lago Community `v1.53.0` wallets and traceable wallet transactions, prove or refute — with runtime evidence — that Lago native objects can carry the approved Credits invariants: (a) monthly plan credits expire per period without carry-over while each top-up batch expires twelve months after its effective date, (b) consumption across concurrent active batches is earliest-expiry-first with earliest-granted tie-break, (c) concurrent consumption, expiry, and retries never double-deduct or create a negative refundable balance, and (d) void and refund withdraw only the unconsumed remainder. Any invariant that cannot hold must be recorded as an explicit migration blocker.

**Architecture:** A self-contained evidence lab under `deploy/lago-lab/wallet-semantics/` that starts its OWN pinned Lago stack (Compose project `weknora-lago-75`, loopback API port `48893`, frontend `48894`) by generating a worktree-local, git-ignored `deploy/lago/.env` and delegating lifecycle to the shared, read-only `deploy/lago/lago.sh`. A stdlib-only Python harness drives the pinned API through scripted experiments E1–E4, one per acceptance criterion, each producing sanitized JSON evidence plus a pass/fail/blocked verdict per check. Final evidence and the per-invariant PASS/BLOCKED verdict document live in `docs/migrations/lago/t03-wallet-semantics/`. Nothing here implements product billing behavior; the outcome decides whether later Commercial Platform tickets may map Credits batches onto Lago wallets.

**Tech Stack:** Docker Compose v2 (shared pinned stack from #73), Bash, Python 3 standard library (`unittest`, `urllib`, `http.server` fixtures), Lago Community `v1.53.0` public REST API, pinned-source analysis of `getlago/lago-api` submodule `591ae9005110346f1c6034ec72ea9046625668cf`.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` — *Credits and Task admission* (monthly non-carryover expiry, twelve-month top-up batches, earliest-expiry-then-earliest-grant consumption, "Lago Wallet and transaction priorities may implement this order only after real runtime verification; inability to preserve this invariant blocks migration"), *Refunds* (recredit from authoritative state, revoke only remaining value), and *Further Notes* (Credits-lot expiry and deterministic consumption order is one of the four explicit vendor blockers). Domain terms: `CONTEXT.md` 套餐内额度 / 充值额度 / 额度预占 / 退款锁定. Parent: Issue #72; ticket: #75; blocked-by: #73 (merged).

## Research basis — pinned-source facts this plan builds on

All verified by reading `getlago/lago-api` at the submodule SHA pinned by `getlago/lago` tag `v1.53.0` (`591ae9005110346f1c6034ec72ea9046625668cf`); live docs at doc.getlago.com describe later releases and are secondary only. File paths below are the citation anchors for the verdict document.

- **Consumption order is explicit priority, not expiry.** Wallets are picked in `Wallet.in_application_order` = `order(:priority, :created_at)` (`app/models/wallet.rb`), used by `Credits::AllocatePrepaidCreditsByWalletsService` and `AppliedPrepaidCreditsService` (`app/services/credits/`). Within one wallet, inbound batches consume in `in_consumption_order` = `priority ASC, granted-before-purchased, created_at ASC` (`app/models/wallet_transaction.rb`). **There is no native expiration-based ordering**; earliest-granted tie-break is native (`created_at`), and the `granted`-before-`purchased` rule fires only at equal priority.
- **Expiry is wallet-level and lazy.** Only `wallets.expiration_at` exists (no per-transaction expiry). `Clock::TerminateWalletsJob` periodically terminates `Wallet.active.expired` via `Wallets::TerminateService` (status → `terminated`, webhook `wallet.terminated`; no void transaction, no carry-over). Until the clock runs, an expired-but-still-`active` wallet remains selectable for consumption — the race window E1 must measure.
- **Active-wallet cap.** `Wallets::ValidateService::MAXIMUM_WALLETS_PER_CUSTOMER = 6` (`wallet_limit_reached`); the `organizations.maximum_wallets_per_customer` override has no writer anywhere in v1.53.0. Since per-batch expiry forces one batch per wallet, a customer holding one monthly plan wallet plus six unexpired top-up batches exceeds the cap — a prima-facie blocker for the twelve-month per-batch invariant.
- **Wallet transaction statuses in v1.53.0:** `transaction_status ∈ {purchased, granted, voided, invoiced}` (no `refunded`), `status ∈ {pending, settled, failed}`, `source ∈ {manual, interval, threshold}`; `remaining_amount_cents` per inbound transaction is tracked only for `traceable` wallets (fresh customers get traceable wallets; `Wallets::CreateService#traceable?`), with a DB-guarded non-negative decrement.
- **Consumption chain is locked and invoice-idempotent.** `Credits::AppliedPrepaidCreditsService` runs at invoice finalization under `Customers::LockService` (`scope: :prepaid_credit`), creates one outbound `invoiced` settled transaction per wallet, decrements inbound remainders in consumption order, and refuses re-application via `wallets_already_applied?` → `already_applied`. Ongoing (pre-invoice) allocation mirrors the wallet order but lets threshold-rule wallets go negative (`Wallets::Balance::AllocateOngoingUsageByWalletsService`).
- **Void is natively remainder-only.** `WalletTransactions::VoidService` (`voided_credits`, optional `voided_transaction_id`, `void_remaining`) validates `<= inbound.remaining_amount_cents` (`exceeds_remaining_transaction_amount`) and sizes a whole-remaining void under the customer lock.
- **Refund has no wallet-native status.** `WalletTransactions::RecreditService` only re-credits granted credits when an invoice is voided (`voided_invoice_id`); money refunds route through credit notes / payment providers. What a credit note on a paid-credit invoice does to wallet balance must be observed at runtime (E4).
- **Paid credits settle immediately in Community.** `Invoices::PaidCreditService` finalizes the one-off credit invoice unless `License.premium? && invoice_requires_successful_payment` — so in Community a `paid_credits` POST is spendable immediately. Acceptable only because WeKnora gates on its own Payment Fact before calling (spec: fulfillment after payment); E4 must still record it.
- **No API idempotency for wallet objects.** Neither wallets nor wallet transactions accept an external idempotency identity; correlation is via `metadata` (queryable on the transaction index), `purchase_order_number`, and returned `lago_id`. A retried POST is expected to create a second grant — E3 must prove it.
- **Interval recurring rules roll over.** Community supports `trigger: interval` rules (weekly→yearly, `already_applied_today` guard, `source: interval`) but they top up the SAME persistent wallet, so credits carry over — they cannot express the monthly non-carryover plan grant; that mapping needs one short-TTL expiring wallet per period (or coordinator voids), both of which E1 probes.

## Global Constraints

- **Own lab instance, shared assets read-only.** This lab runs its own stack: `COMPOSE_PROJECT_NAME=weknora-lago-75`, `LAGO_API_PORT=48893` (loopback), `LAGO_FRONT_PORT=48894` (loopback). Everything under `deploy/lago/` is the shared #73 asset and MUST NOT be modified or copied; isolation works by generating a worktree-local, git-ignored `deploy/lago/.env` (the per-worktree mechanism T01's README documents) and then delegating lifecycle to `./deploy/lago/lago.sh` operations (`up|down|status|config|contract-probe`). The lab never edits `deploy/lago/compose.yaml`, `images.lock.json`, or scripts.
- **Pin truth:** Lago `v1.53.0` with the digest-locked images from `deploy/lago/images.lock.json`; reported release identity always comes from the lock, never from live API text. Source citations use the pinned `lago-api` submodule SHA above.
- **Synthetic data only.** Every external identity is `weknora-t03-<uuid>`-prefixed and non-personal. The lab must not touch any pre-existing object in the shared stack (this stack starts from T01 seed state only; if unknown objects are observed, halt as `blocked-env`).
- **Secrets:** `LAGO_API_KEY` comes only from the caller environment; never from `.env` parsing, never logged, never written to reports. No usable key, password, or private material is committed. Before committing any evidence, grep the tracked tree for the live secret values — zero hits is the bar.
- **Unit tests must not require Docker.** Pure logic (harness sanitization, expected-outcome predicates, fixture builders, experiment wiring) is tested with `unittest` + in-process `http.server` fixtures, runnable offline: `python3 -m unittest discover -s deploy/lago-lab/wallet-semantics -p "test_*.py" -v`. Real-stack phases are scripted operator runs with expected outcomes per acceptance criterion; they may run off-peak when machine resources are tight, and their outputs are evidence, not unit tests.
- **Honest verdicts.** Every check reports `pass | fail | blocked-env`. A `blocked-env` (no Docker, no API key, stack not healthy) is honest environment evidence, never a pass. Per the ticket, if ANY of the four invariants cannot hold on the pinned runtime, the verdict document must explicitly mark the migration blocked for that invariant — the lab never weakens an invariant to make Lago pass.
- **Money and credits:** integer minor units end-to-end in the harness; amounts sent to the API are decimal strings; no binary floating point in assertions.
- **Lab hygiene:** `lab.sh down` preserves the lab's named volumes (mirrors T01); `lab.sh clean` removes the whole `weknora-lago-75` project including volumes and the worktree-local `.env` for a from-scratch re-run. Experiment failures must not leave unexplained synthetic state — each run records created object ids in its report and attempts customer-scoped cleanup in `finally`.

## Review Focus

- **Expiry silently carried over:** an expired wallet is still consumed because clock termination is lazy (`expiration_at` passed but status still `active`), or a terminated wallet's balance reappears through a new grant/invoice void. E1 must catch both.
- **Wrong consumption order under same-expiry:** at equal priority Lago consumes `granted` before `purchased` and breaks ties by `created_at`; a mapping that relies on wallet order without encoding expiry into `priority` (or that collides on the 1..50 priority space) consumes the wrong batch first. E2 must catch both.
- **Double-consume on retry:** no external idempotency exists — a retried grant POST mints a second batch, and a concurrent finalize+void pair could draw the same remainder twice. E3 must prove the customer-lock and `already_applied` guards and expose the duplicate-grant behavior.
- **Void/refund touching consumed portion:** a void sized above `remaining_amount_cents`, a refund that restores spendable credits beyond the unconsumed remainder, or a credit note on a paid-credit invoice that silently re-credits the wallet. E4 must catch all three.
- **Hidden capacity cap:** the hardcoded six-active-wallets-per-customer limit (`wallet_limit_reached`) breaks the one-batch-per-wallet mapping at seven concurrent batches; if untested, the blocker surfaces only in production. E1's cap probe must catch it.

---

### Task 1: Lab scaffold — isolated stack wiring and sanitized experiment harness

**Files:**
- Create: `deploy/lago-lab/wallet-semantics/lab.sh`
- Create: `deploy/lago-lab/wallet-semantics/harness.py`
- Create: `deploy/lago-lab/wallet-semantics/test_harness.py`
- Create: `deploy/lago-lab/wallet-semantics/test_lab_script.py`
- Create: `deploy/lago-lab/wallet-semantics/evidence/README.md`

**Interfaces:**
- Consumes: `deploy/lago/lago.sh` lifecycle (unchanged), `deploy/lago/images.lock.json`, `deploy/lago/health.py` semantics, caller-env `LAGO_API_KEY`.
- Produces: `./deploy/lago-lab/wallet-semantics/lab.sh env|up|status|down|clean|run <experiment>|run-all` and a reusable `harness.run_experiment` contract: `ExperimentReport(status: pass|fail|blocked-env, checks: [{criterion, expected, observed, outcome}], created_object_ids, run_id)` with sanitized JSON serialization.

- [ ] **Step 1: Write failing harness and script-wiring tests**

Use in-process `http.server` fixtures (mirror `deploy/lago/test_contract_probe.py` style). Assert, without Docker or network:

```python
def test_report_never_contains_api_key_or_raw_response_dump():
    report = ExperimentReport(run_id="weknora-t03-<uuid>", api_key="secret-for-test-only")
    report.checks.append(check_from(observed_error=Exception("Bearer secret-for-test-only rejected")))
    assert "secret-for-test-only" not in report.serialized()
```

Cover: Bearer auth sent only to the lab origin; every external id is `weknora-t03-` prefixed with a per-run uuid; `blocked-env` classification when the key is absent (zero API calls); `fail` vs `blocked-env` distinction (connection refused to a configured-but-down stack is `blocked-env`, a 4xx/5xx contract answer is `fail`); created-object ids always recorded for cleanup; cleanup runs from `finally` on later failure. Script wiring tests assert `lab.sh env` refuses to overwrite an existing `deploy/lago/.env`, writes mode-600 with `COMPOSE_PROJECT_NAME=weknora-lago-75`, `LAGO_API_PORT=48893`, `LAGO_FRONT_PORT=48894`, matching `*_URL` values, random non-sample secrets, and the seed variables from the T01 README (`LAGO_CREATE_ORG`, operator email/password/name, random `LAGO_ORG_API_KEY`); and that `up|status|down` delegate to `deploy/lago/lago.sh` without modifying any tracked file under `deploy/lago/`.

- [ ] **Step 2: Run the tests and verify the expected RED**

Run: `python3 -m unittest discover -s deploy/lago-lab/wallet-semantics -p "test_*.py" -v`

Expected: tests fail because the lab directory only contains stubs.

- [ ] **Step 3: Implement the minimal lab lifecycle and harness**

`lab.sh env` generates the worktree-local `deploy/lago/.env` following exactly the T01 init contract (random secrets via `secrets`/`openssl`, RSA key, refuse-overwrite) plus this lab's project name, ports, URLs, and seed values; it never writes anywhere else under `deploy/lago/`. `up` calls `./deploy/lago/lago.sh up` (env-driven), `status` calls `lago.sh status --json`, `down` calls `lago.sh down`, `clean` removes project `weknora-lago-75` including volumes and the generated `.env`. `harness.py` provides: a stdlib HTTP client (timeouts, no redirects across origins, key redaction in every error path), release identity read from `deploy/lago/images.lock.json`, fixture builders for customer/wallet/transaction payloads (decimal-string credits, `rate_amount` mapping, `metadata` carrying `weknora_t03_batch` correlation keys), the `ExperimentReport` model, and the run orchestrator that writes one sanitized JSON per experiment under `evidence/`. Document the evidence contract in `evidence/README.md`: verbatim sanitized outputs, no secret values, `blocked-env` clearly labeled, expected outcomes fixed before the run.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `python3 -m unittest discover -s deploy/lago-lab/wallet-semantics -p "test_*.py" -v`

Expected: sanitization, classification, id-prefixing, cleanup, and script-wiring tests pass with no Docker present.

- [ ] **Step 5: Commit the lab scaffold**

Commit only `deploy/lago-lab/wallet-semantics/` files. Verify `git status` shows no modification under `deploy/lago/`.

### Task 2: Experiments E1 (expiry, no carry-over) and E2 (consumption order) — fixtures, expected outcomes, real run

**Files:**
- Create: `deploy/lago-lab/wallet-semantics/experiments/__init__.py` (or flat `e1_expiry.py` / `e2_order.py`, matching harness imports)
- Create: `deploy/lago-lab/wallet-semantics/experiments/e1_expiry.py`
- Create: `deploy/lago-lab/wallet-semantics/experiments/e2_order.py`
- Create: `deploy/lago-lab/wallet-semantics/test_e1_expiry.py`
- Create: `deploy/lago-lab/wallet-semantics/test_e2_order.py`

**Interfaces:**
- Consumes: Task 1 harness and fixture builders; the pinned-source facts in *Research basis* as pre-registered expected outcomes.
- Produces: `evidence/e1-expiry.json` and `evidence/e2-order.json` with per-check `pass|fail|blocked-env`, plus pure functions `e1_expected(observed) -> checks` / `e2_expected(observed) -> checks` that turn a runtime observation into outcome rows (unit-tested offline).

- [ ] **Step 1: Write failing expected-outcome tests (offline)**

Pre-register each check's expected outcome from the research basis, for example:

```python
def test_expired_but_unterminated_wallet_is_reported_as_a_race_window():
    observed = {"wallet": {"status": "active", "expiration_at": "2026-09-21T00:00:00Z"},
                "consumed_after_expiry_passed": True, "clock_terminated_within": "unknown"}
    checks = e1_expected(observed)
    assert outcome_of(checks, "expired_wallet_not_consumable") == "fail"  # lazy termination gap
```

E1 checks: monthly-batch wallet (granted credits, short TTL standing in for the monthly period) becomes `terminated` with `wallet.terminated`, its remaining balance is neither consumable via a later invoice nor resurrected by a new grant; a top-up-batch wallet (purchased credits, TTL standing in for twelve months) survives the monthly boundary and is consumed after the plan batch dies; an interval-rule wallet keeps its leftover after the rule fires (proving interval rules cannot express non-carryover → recorded as a mapping negative, not a runtime failure); termination cadence measured (poll until `terminated`, record elapsed); cap probe — create batches until creation fails and assert the failure is `wallet_limit_reached` at six ACTIVE wallets per customer, and that terminating one frees a slot. E2 checks: across three wallets with priorities set from expiry rank, an invoice-sized consumption draws wallet A (earliest expiry) fully, then B, then C; at equal priority+same wallet the `granted`-before-`purchased` and `created_at` tie-breaks are observed exactly; consumption draw-down per batch is provable via the transaction `consumptions`/`fundings` endpoints (`remaining_amount_cents` monotonic non-increasing); consumption strictly follows `priority, created_at` even when expiry rank disagrees with creation order (the coordination mapping proof).

- [ ] **Step 2: Run the tests and verify the expected RED**

Run: `python3 -m unittest discover -s deploy/lago-lab/wallet-semantics -p "test_*.py" -v`

Expected: new E1/E2 tests fail; Task 1 tests stay green.

- [ ] **Step 3: Implement E1/E2 as scripted real-stack experiments**

Before implementation, confirm on the pinned release the minimal deterministic consumption trigger — candidate: subscription + one-off invoice (or advance-charge invoice) finalized through the public API so `Credits::AppliedPrepaidCreditsService` runs — and record the actually-working trigger in the experiment code comments and evidence before relying on it (same pattern T01 used for the customer create/delete confirmation). Use SHORT TTLs (minutes) instead of clock manipulation for expiry boundaries; poll wallet status with timeout and record the measured clock cadence. Each experiment: create synthetic customer → create batches per scenario metadata `weknora_t03_batch=<uuid>` → trigger consumption → read final wallet + transaction + consumption state → map through the pre-registered `*_expected` predicates → write sanitized evidence. Never assert on a check whose pre-condition (e.g., stack health) failed — classify upstream failure as `blocked-env`.

- [ ] **Step 4: Run focused tests and verify GREEN (offline)**

Run: `python3 -m unittest discover -s deploy/lago-lab/wallet-semantics -p "test_*.py" -v`

Expected: all offline tests pass without Docker.

- [ ] **Step 5: Real run against the lab stack and commit**

Run: `./deploy/lago-lab/wallet-semantics/lab.sh env && ./deploy/lago-lab/wallet-semantics/lab.sh up` (off-peak if resources are tight)

Run: `LAGO_API_KEY=<lab operator key> ./deploy/lago-lab/wallet-semantics/lab.sh run e1 && LAGO_API_KEY=<lab operator key> ./deploy/lago-lab/wallet-semantics/lab.sh run e2`

Expected: `evidence/e1-expiry.json` and `evidence/e2-order.json` record every check with an honest outcome — including the pre-registered expected gaps (lazy-termination race window, interval-rule carry-over, six-wallet cap) — with created ids and cleanup results. Commit the two experiment modules, tests, and sanitized evidence. If the stack cannot start, commit only code plus a clearly labeled `blocked-env` evidence file.

### Task 3: Experiments E3 (concurrency and retry) and E4 (void/refund semantics)

**Files:**
- Create: `deploy/lago-lab/wallet-semantics/experiments/e3_concurrency.py`
- Create: `deploy/lago-lab/wallet-semantics/experiments/e4_withdraw.py`
- Create: `deploy/lago-lab/wallet-semantics/test_e3_concurrency.py`
- Create: `deploy/lago-lab/wallet-semantics/test_e4_withdraw.py`

**Interfaces:**
- Consumes: Task 1 harness, Task 2 consumption trigger, pre-registered outcomes from *Research basis*.
- Produces: `evidence/e3-concurrency.json` and `evidence/e4-withdraw.json` with per-check outcomes; E4 additionally emits the refund-semantics observation set the verdict task consumes (what a credit note on a paid-credit invoice does to wallet state in Community).

- [ ] **Step 1: Write failing expected-outcome tests (offline)**

E3 checks (pure predicates over observed state): duplicate grant — the same `POST /api/v1/wallet_transactions` payload sent twice creates TWO settled inbound transactions and inflates balance by 2× (expected: yes, no API idempotency → coordination must own it, and `metadata`-keyed transaction lookup is the recovery path); concurrent consumption — N parallel consumption triggers against one batch draw down total ≤ granted amount with `remaining_amount_cents` never negative (the DB-guarded decrement and `exceeds_available_amount`/`already_applied` guards); void-vs-consume race — a whole-remaining void racing a finalization cannot both succeed against the same remainder (customer lock); expired-during-consumption — a batch whose `expiration_at` passes mid-run is either skipped or terminated, never double-drawn. E4 checks: partial void ≤ remaining succeeds and decrements only `remaining_amount_cents` (consumed history untouched); void of exactly the remaining amount empties the batch; void above remaining fails with `exceeds_remaining_transaction_amount` and changes nothing; `void_remaining` on a fully consumed batch is a no-op (no negative balance); after a successful channel-side refund modelled as void + credit note, no spendable credits remain attributable to the refunded batch — including what a credit note on the paid-credit invoice actually does to wallet balance in Community (observed, not assumed); paid-credit immediacy — a `paid_credits` POST is immediately spendable in Community (expected yes, recorded as a constraint on the coordination layer).

- [ ] **Step 2: Run the tests and verify the expected RED**

Run: `python3 -m unittest discover -s deploy/lago-lab/wallet-semantics -p "test_*.py" -v`

Expected: E3/E4 tests fail; earlier tasks stay green.

- [ ] **Step 3: Implement E3/E4 as scripted real-stack experiments**

E3 concurrency uses real parallel HTTP calls from threads (stdlib) with per-call request ids in evidence; determinism comes from asserting totals and guards, not interleavings. E4 walks each withdraw path with fresh batches so checks cannot contaminate each other, and captures the credit-note creation request/response (sanitized) as the refund-path evidence. Both reuse the Task 2 consumption trigger and write evidence through the Task 1 report contract.

- [ ] **Step 4: Run focused tests and verify GREEN (offline)**

Run: `python3 -m unittest discover -s deploy/lago-lab/wallet-semantics -p "test_*.py" -v`

Expected: all offline tests pass without Docker.

- [ ] **Step 5: Real run and commit**

Run: `LAGO_API_KEY=<lab operator key> ./deploy/lago-lab/wallet-semantics/lab.sh run e3 && LAGO_API_KEY=<lab operator key> ./deploy/lago-lab/wallet-semantics/lab.sh run e4`

Expected: honest per-check evidence for concurrency totals, duplicate-grant behavior, and remainder-only withdrawal. Commit experiment modules, tests, and sanitized evidence.

### Task 4: Verdict document, full-run review, and teardown evidence

**Files:**
- Create: `docs/migrations/lago/t03-wallet-semantics/verdict.md`
- Create: `docs/migrations/lago/t03-wallet-semantics/evidence/` (final sanitized copies of `e1`–`e4` reports plus the health snapshot of the lab run)
- Create: `docs/migrations/lago/t03-wallet-semantics/source-notes.md` (pinned-SHA citations from *Research basis*)
- Test: all lab unit tests (regression gate)

**Interfaces:**
- Consumes: E1–E4 evidence files, the pinned-source facts, ticket #75's acceptance criteria, and spec *Credits and Task admission*.
- Produces: the per-invariant verdict table — 套餐内额度按月到期不结转 / 充值批次十二个月到期 / 最早到期+同到期最早发放消费顺序 / 并发与重试无重复扣减无负可退余额 / void 与 refund 只撤回未消费部分 — each mapped to experiment, evidence file, and `PASS | PASS-WITH-COORDINATION | BLOCKED`.

- [ ] **Step 1: Write the verdict from evidence, never from memory**

`verdict.md` states, per invariant: what was asserted, the observed runtime behavior (with evidence-file references), and the verdict. Any invariant that cannot hold on the pinned runtime — current research-level candidates: the six-active-wallet cap against twelve-month per-batch expiry, the lazy-termination race window, absent API idempotency — gets an explicit **BLOCKED** entry with the failing evidence attached, per the ticket's "无法满足任一不变量时明确阻断迁移". `PASS-WITH-COORDINATION` is allowed only where the spec already anticipates the coordination layer (priority-maintained ordering, post-payment grant timing) and must name the exact coordination obligation handed to later tickets. Include the known-limitations ledger (no per-transaction expiry, granted-before-purchased at equal priority, paid-credit instant settlement, no `refunded` status, credit-note refund behavior) with source citations from `source-notes.md`.

- [ ] **Step 2: Run the full offline suite and the real end-to-end lab sequence**

Run: `python3 -m unittest discover -s deploy/lago-lab/wallet-semantics -p "test_*.py" -v` and `python3 -m unittest discover -s deploy/lago -p "test_*.py" -v`

Run: `./deploy/lago-lab/wallet-semantics/lab.sh env && ./deploy/lago-lab/wallet-semantics/lab.sh up && ./deploy/lago-lab/wallet-semantics/lab.sh status`

Run: `LAGO_API_KEY=<lab operator key> ./deploy/lago-lab/wallet-semantics/lab.sh run-all`

Expected: all unit tests pass offline; `run-all` regenerates E1–E4 evidence in one sitting and the copied evidence in `docs/migrations/lago/t03-wallet-semantics/evidence/` matches the fresh run (or the deltas are explained in the verdict).

- [ ] **Step 3: Stop the lab and prove clean teardown**

Run: `./deploy/lago-lab/wallet-semantics/lab.sh down`

Expected: `weknora-lago-75` containers stop; named volumes preserved; recorded in the run narrative. Optionally `clean` for a full reset, with the reset also recorded.

- [ ] **Step 4: Secret review and final commit**

Review every file under `deploy/lago-lab/wallet-semantics/evidence/` and `docs/migrations/lago/t03-wallet-semantics/` for secret leakage (grep for the live key/password values — zero hits), confirm `deploy/lago/` tracked files are untouched (`git status`), then commit documentation, verdict, and sanitized evidence only.

## Plan Self-Review

- **Spec coverage:** all four ticket criteria map to experiments — monthly non-carryover + twelve-month batch expiry → E1; earliest-expiry-then-earliest-granted order → E2 (including the spec's "priorities may implement this order only after real runtime verification"); concurrency/retry/no-negative-refundable → E3; void/refund remainder-only + explicit blocking → E4 + Task 4 verdict. The spec's four vendor blockers include "Credits-lot expiry and deterministic consumption order" — this plan is exactly that proof, and it also surfaces the newly found six-active-wallet cap as a candidate blocker the spec's coordination mapping must answer.
- **Execution detail scan:** every task names files, commands, and expected results; expected outcomes are pre-registered offline from the pinned source so a runtime surprise is unambiguous. The consumption trigger is the one unpinned implementation detail; Task 2 Step 3 makes confirming it the first implementation act, with the fallback of recording the actual trigger in evidence.
- **Interface consistency:** Task 1 exports the report/harness contract consumed by Tasks 2–3; Tasks 2–3 export evidence files consumed by Task 4; `lab.sh` is the single operator entry point; nothing under `deploy/lago/` is modified — the lab composes with it through the documented per-worktree `.env` mechanism.
- **Review focus:** the five failure cases (silent carry-over, same-expiry misordering, retry double-consume, withdrawal touching consumed value, hidden wallet cap) each have dedicated checks in E1–E4 and dedicated offline tests over the `*_expected` predicates.
- **TDD:** offline RED→GREEN precedes every module; real-stack verification is explicitly separate scripted evidence, never a unit test requiring Docker; `blocked-env` and `fail` are distinguished everywhere so environment gaps can never masquerade as contract results.
