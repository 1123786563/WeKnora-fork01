# OM-07 / OM-08 Experiment Report — OpenMeter v1.0.0-beta.232 (local, http://127.0.0.1:48888)

Run: 2026-09-12T04:05–04:13Z. Namespace om0708-20260912. All evidence is live HTTP against the local gateway; every request/response is captured in om0708-cases/*.json logs.
Entities: custA=01M29WMV6F84ZP0ZJ9WCAAKR46 (key om0708-20260912-custA, sub 01M29WMVBC0K52YDPAWH4M603D), custB=01M29WT91B98CZ3JMQFJ1W2YEX (key om0708-20260912-custB).

## OM-08 部分退款与撤回 (void/partial refund) — verdicts

| # | Check | Result | Evidence |
|---|-------|--------|----------|
| a | Setup: grants batch1 $40 (prio 3) + batch2 $60 (prio 5) + exp $5 (PT1M, prio 1); burn $10 (1000 tok @0.01) | **PASS** — live 105→95, exact | om08-a-setup.json, om08-b-burnsettle.json |
| b1 | Partial void via body {amount:10} | **NOT SUPPORTED** — 400 property amount is unsupported; void is all-or-nothing per grant | om08-c-voidmatrix.json |
| b2 | Full void batch1 | **PASS with caveat** — void removed full nominal $40; other batches untouched (batch2 stayed active $60). Ledger entry type=voided, amount=-40 | om08-c-voidmatrix.json |
| b3 | Alternative: update-credit-grant-external-settlement as reducer | **NOT a reducer** — body requires status only; amount → 400 unsupported; pending → 400 credit_grant_external_settlement_status_invalid (external-funding status marker, not partial refund) | om08-e-settle-final.json, om07-a-setup.json |
| b4 | Void of grant with live consumption against it (batch3: burn $5 then void) | **CAVEAT** — void booked -20 (full nominal), not -15 remaining; the $5 already burned is deducted again at live-balance level (live 85→65, not 70). Final truth re-resolves at period settlement | om08-d-settle-concurrent.json |
| c | Expired grant void (PT1M grant, status→expired polled) | **PASS (409)** — credit grant has already expired; balance unaffected (expiry itself already booked type=expired, -5) | om08-c-voidmatrix.json |
| d | Concurrent burn + void (batch4, two parallel requests) | **PASS** — burn 202 + void 200, no corruption; final balance consistent with exactly one ordering (void -20 + burn -5; live 65→40). No 409/5xx/NaN | om08-d-settle-concurrent.json |
| e | 撤回响应丢失: repeat identical void | **PASS (idempotent)** — 2nd void → 200, same resource, voided_at unchanged, balance unchanged (no double deduction). Not 409 | om08-c-voidmatrix.json |

**OM-08 verdict: PARTIAL PASS.** Per-batch precision verified (void removes exactly one grant's credits, other batches intact, ledger exact, expired/concurrent/repeat all safe). Partial-amount refund is NOT possible via void (no amount field) nor via external-settlement (status-only) — 退剩余的一部分 requires splitting grants into smaller batches at issuance or a compensating grant.

## OM-07 套餐变更 — verdicts

| # | Check | Result | Evidence |
|---|-------|--------|----------|
| a | Plans om0708_plan_b (0.02/u) + om0708_plan_c (0.03/u) create+publish; custB subscribed to om02b_20260912_v3plan4 | **PASS** — 201/200, sub 201 | om07-a-setup.json |
| base | Price on base plan | 100 tok → **-$1.00** (0.01/u) | om07-a-setup.json |
| b | Upgrade → plan_b via change-subscription | **PASS** — 200; burn → **-$2.00**; funded-grant count 1→1 (no re-issuance); grant untouched | om07-b-changes.json |
| c | Consecutive upgrade → plan_c | **PASS** — 200; burn → **-$3.00**; funded count 1→1 | om07-c-downgrade-cancel.json |
| d | Downgrade → v3plan4 | **PASS** — 200; burn → **-$1.00** (downgraded price applies immediately, no double issuance) | om07-c-downgrade-cancel.json |
| e | Cancel → burn stops, grants remain, re-subscribe | **PASS** — cancel 200; post-cancel event 202 but balance 37→37 after 12s (no deduction); grant om0708-custB-grant1 still active $50 (充值保留); re-subscribe 201 active | om07-c-downgrade-cancel.json |

**OM-07 verdict: PASS** (upgrade, consecutive upgrade, downgrade, cancel, re-subscribe; no double grant issuance, no extra charge; price switch effective immediately with timing=immediate).

## New findings (beyond accumulated knowledge)

1. void-credit-grant body is empty/absent — {} = full void 200; any amount → 400 unsupported. No partial void API in v1.0.0-beta.232.
2. Void deducts full nominal amount even when the grant has live consumption (batch3: burn $5 live, void booked -20) — over-deduction at live level until settlement reconciles.
3. Repeat void is idempotent (200, same voided_at, no balance change) — safe for lost-response retries.
4. Expired grant void → 409 has already expired; expiry auto-books type=expired transaction.
5. change-subscription requires customer, plan, timing; timing=now → 400 (invalid), immediate works. Change SUPERSEDES: response has current (old id, inactive) + next (new id, active); labels openmeter_subscription.superseding.id / previous.id. Posting change/cancel to the OLD id → 403 transition cancel in state inactive not allowed — always follow next.id.
6. Credits grant list response field is data, not items (items → silently 0).
7. External settlement statuses pending/completed rejected on funding_method=none grants; endpoint is a status marker for externally-funded grants, not a refund primitive.
8. Priority semantics: higher priority number consumed first (batch2 prio5 burned before batch1 prio3; batch3/4 prio7 first).

## Commands & exit codes

All calls via node fetch (run_code) — no shell; exit code = HTTP status per call (full log in each artifact log[]):
- Setup: create-customer 201, create-subscription 201, create-grant x5 201, ingest-events 202 x7
- void: empty-body 200 (x4 incl. repeat), amount-body 400, expired 409
- settlement/external: {} 400 (status required), pending 400, completed 400, completed+amount 400
- plans: create 201 x2, publish 200 x2; changes: 200 x3 (plus 3x 403 on superseded id, 1x 400 timing=now); cancel 200; re-subscribe 201

## Artifacts
- om0708-cases/om08-a-setup.json — grants, subscription, initial burn
- om0708-cases/om08-b-burnsettle.json — settle poll (105→95), tx snapshot, first full void probe
- om0708-cases/om08-c-voidmatrix.json — partial-void 400, expired 409, repeat-void idempotency, ledger
- om0708-cases/om08-d-settle-concurrent.json — batch3 sequential burn→void, batch4 concurrent burn+void
- om0708-cases/om08-e-settle-final.json — full ledger + settlement probe + final balance
- om0708-cases/om07-a-setup.json — plans b/c, custB, base burn -1.00, change-schema probe (incl. settlement probes)
- om0708-cases/om07-b-changes.json — upgrade→b burn -2.00, 403 evidence, supersede response
- om0708-cases/om07-c-downgrade-cancel.json — →c -3.00, downgrade -1.00, cancel, post-cancel burn flat, resubscribe 201
