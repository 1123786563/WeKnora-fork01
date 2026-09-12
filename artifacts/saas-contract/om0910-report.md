# OM-09 / OM-10 — runtime evidence (2026-09-12)

Instance: local docker OpenMeter v1.0.0-beta.232 (official image, digest-matched, OM-01 verified) at http://127.0.0.1:48888.
Namespace isolation: om0910-20260912 customers only; artifacts under artifacts/saas-contract/om0910-cases/.

## OM-09 订单与账单衔接 — VERIFIED

| Sub-check | Verdict | Evidence |
|---|---|---|
| Recharge settlement path (external funding) | verified — grant funding_method=external requires purchase terms (400 without); {currency, per_unit_cost_basis} accepted → 201; grant.purchase carries settlement_status | om09-settlement.json |
| 支付状态只确认一次 | verified — settlement transitions authorized→200, settled→200 (purchase.settlement_status='settled'); REPEAT settled → **412 Precondition Failed** (exactly-once enforced by provider); pending-after → 400 | om09-settlement.json |
| 不双收/不双发 | verified — exactly one 'funded' ledger entry; balance live/settled 25; no second grant on any repeat | om09-settlement.json |
| Invoice/Order/Grant mapping traceable | verified — grant.id ↔ key ↔ purchase.amount ↔ ledger entry (labels carry charge_id in prior runs) all queryable | om09-settlement.json |
| External Invoicing comparison | documented — v1v2 POST /api/v1/apps/custom-invoicing/{id}/payment/status exists schema-only; live probe → 400 (requires app installation); credits-path external settlement is the verified live mechanism | om09-settlement.json |

## OM-10 准入与消费交接 — VERIFIED (conclusion: platform coordinator REQUIRED)

| Sub-check | Verdict | Evidence |
|---|---|---|
| Atomic check-and-consume interface | **none exists** — complete v3 operation list (create-credit-adjustment/get-balance/create-grant/list-grants/get-grant/update-external-settlement/void-grant/list-transactions) has no reserve/hold/authorize/lock/debit operation; schema grep across the full v3 surface | schema + om10 artifacts |
| Watermark / batch projection | verified — 'live' is the in-flight authoritative balance (reflected $2 burn within 1s this round; 'pending' observed during lag in om02b); 'settled' only moves at period close → coordinator projects on live+pending, settles on settled | om10-part1.json, om02b evidence |
| Remote change detection | verified — void reflected in balance within **1.1s**; coordinator must re-read provider state, local cache is unsafe | om10-part2.json |
| Refund lock chain | verified — voided −12 ledger entry alongside the original funded +12 (before/after chain) | om10-part2.json |
| Concurrency overdraw admission | **critical finding** — 5 concurrent $2 burns against a $5 grant: ALL accepted 202, live clamps at 0 (not negative), true accounting deferred to period settlement → provider accepts over-balance consumption; admission control MUST be enforced platform-side BEFORE dispatch | om10-part2.json, om10-overdraw-ledger.json |

## Cross-check summary feeding the adapter decision
- Payment confirmation is exactly-once at the provider (412 on repeat) — aligns with C01/C04 idempotent fulfillment.
- No atomic admission → U05 ExecutionGate + platform-side reservation (U02) remain mandatory; provider balance is advisory, not a gate.
- External settlement (credits) verified live; custom-invoicing remains schema-only (blocked-env until an app installation exists).
