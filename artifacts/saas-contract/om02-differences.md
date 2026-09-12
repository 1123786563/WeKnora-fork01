# OM-02 — V2 Metered Entitlements vs V3 Credits (runtime evidence)

- Check: OM-02 (唯一商业模型 / authoritative commercial-model selection)
- Instance: local OpenMeter v1.0.0-beta.232 at http://127.0.0.1:48888 (OM-01 verified)
- Namespace flag: `om02-20260912` (runner enforcement; instance runs a single implicit `default` namespace — isolation achieved via namespaced keys; confirmed by 409 error text "already used by an another customer in the namespace default")
- Isolated customers: `om02-20260912-v2cust` (v1/v2 API, id in artifacts) and `om02-20260912-v3cust` → id `01M29SXXWD6Y35PN7FP4FTWRPX` (v3 API)
- Method: `scripts/saas/probe_case.py` (read-only), all artifacts under `artifacts/saas-contract/om02-*` (auto-redacted `run.json` per case)

## Verdict matrix

| Aspect | V2 metered entitlement | V3 credits |
|---|---|---|
| 1. 发放 issuance | **verified** — entitlement `metered` on feature `om02_tokens`/meter `tokens_total` + grant amount 1000, 201 | **verified** — credit grant amount "100" USD, 201, `status: active` |
| 2. 消费 consumption | **verified** — 1 `prompt` event (tokens 250, subject = customer key) → `usage: 250`, balance 1000→750; subscription path (om02b): auto-grant balance 1 → 0, `overage: 249` | **verified (om02b run)** — subscription with unit rate card (0.01 USD × feature `om02_tokens`) + 250-token event → **live balance 100 → 97.5 within ~5s** (`settled` stays 100 until period-close settlement; a 60ms-later re-check still showed 100 — async lag). Adjustments endpoint still 501; burn is rate-card driven. Artifact: `om02b-v3-burn/run.json` + `om02b-v3-burn/balance-after-5s.json` |
| 3. 余额 balance | **verified** — `GET .../value`: `{balance:750, usage:250, overage:0, totalAvailableGrantAmount:1000, grantBalances:{<grantId>:750}, hasAccess:true}` | **verified** — `GET .../balance`: `{balances:[{currency:USD, live:"250", pending:"0", settled:"250"}]}`; ledger via `GET .../transactions` w/ `available_balance.before/after` |
| 4. 到期 expiry | **fields verified; time-based lapse NOT observed** — grant stores `expiration:{duration:"HOUR",count:1}` → `expiresAt 2026-09-12T04:20:00Z`; effectiveAt enforced literally (balance stayed 0 until effective time) | **fields verified; time-based lapse NOT observed** — request `expires_after:"PT1H"` → response `expires_at: 2026-09-12T04:18:38Z`, grant status enum includes `expired` (schema) |
| 5. 订阅关联 subscription association | **verified (om02b run)** — plan `om02b_20260912_v2plan4` (usage_based rate card; rate card key MUST equal featureKey; metered template needs `issueAfterReset`; last phase `duration: null`) create 201 → publish 200 → `POST /api/v1/subscriptions` 201 `status: active`, `settlementMode: credit_then_invoice`; subscription **auto-provisioned** the entitlement (annotations `subscription.id`) and **auto-issued grant amount 1** (`issueAfterReset: 1`, grant annotations `issueAfterReset: true`) — metered usage consumed it (balance 1→0, overage 249). Artifacts: `om02b-v2-subscription/run.json`, `om02b-v2-subscription-observe/run.json` | **verified (om02b run)** — plan `om02b_20260912_v3plan3` (rate card: feature reference requires `id` (ULID) not key; `billing_cadence` ≥1h required; rate card key = feature key) create 201 → publish 200 → `create-subscription` 201 `status: active` with `customer_id` + `plan_id` + `settlement_mode: credit_then_invoice`; standalone grant then **burned via subscription rate card** (live 100→97.5). 409 `only_single_subscription_allowed_per_customer_at_a_time` on a second subscription attempt. Artifacts: `om02b-v3-subscription/run.json` (attempt log), `om02b-v3-burn/run.json` |

## Observed differences

| Dimension | V2 (entitlements) | V3 (credits) |
|---|---|---|
| Units | feature/meter-coupled usage units (float `amount: 1000`, SUM over meter) | money-like decimal **strings** (`amount: "100"`, currency required, per-currency balances) |
| Model | feature → meter → entitlement → grants; usage from event stream (subject = customer key) | customer → credit grants; balance ledger w/ `live/pending/settled` + transactions ledger |
| Double-issuance guard (禁止同笔充值双发) | customer key unique per namespace (409 on duplicate); grants have **no** natural-key or idempotency dedup observed (unique ULID only) | grant body `key` is per-customer idempotency key → duplicate returns **409** (observed); **Idempotency-Key header is NOT honored** — replay with header double-issued a second 50 grant (transactions show two `funded` 50 entries, balance 150→200→250) |
| Expiry representation | `expiration:{duration:HOUR|DAY|WEEK|MONTH|YEAR, count:n}` + `effectiveAt`; rollover fields min/maxRolloverAmount on the grant; effectiveAt validated against entitlement usage period (400 if before) | `expires_after` ISO-8601 duration (e.g. `PT1H`) → server-computed `expires_at`; `funding_method` (none/invoice/external) + tax_config + `settlement/external` hooks |
| Consumption settlement | automatic via meter aggregation (async pipeline; ~seconds lag observed); `overage` when usage > grants; `isSoftLimit` | adjustments endpoint 501 on OSS beta; burn intended via subscription rate cards (`settlement_mode: credit_then_invoice | credit_only`); void + external settlement endpoints exist |
| Reset/period semantics | entitlement `usagePeriod` (interval+anchor), `reset`, `issueAfterReset`, rollover math | none observed at credit level (no periodic reset concept) |
| API surface | /api/v1 + /api/v2 (snake-free camelCase, ULID or key path params) | /api/v3/openmeter (Konnect-style pagination `data/meta.page`, labels, problem+json `kongapi.info`) |

## Recommendation (advisory only — no committed file changed)

**Authoritative family: V3 Credits (`/api/v3/openmeter`) for money-like prepaid balance; keep V2 metered entitlements only as usage metering input.**

Rationale (from observed artifacts):
1. **双发防护 is real and testable in V3**: body `key` dedup returned 409 on reuse, while the header replay demonstrably double-issued (artifacts om02-v3-idempotency + om02-v3-final-state transactions 150→200→250). V2 grants expose no equivalent dedup key — adapter MUST generate its own external dedup ledger for V2.
2. **Money semantics**: V3 amounts are currency-scoped decimal strings with settled/pending/live split and a queryable transaction ledger — matches recharge/billing invariants; V2 values are unit-less floats tied to meters.
3. **Expiry + funding**: V3 grants carry `expires_after`→`expires_at`, `funding_method`, tax config and external-settlement hooks — the recharge lifecycle WeKnora needs.
4. **Caveats driving the adapter decision** (updated by om02b run): V3 consumption is subscription-coupled and **now verified end-to-end** — a published plan with a unit rate card (feature `om02_tokens`, 0.01 USD/token, `billing_cadence` ≥1h) + active subscription + credit grant 100 → one 250-token event dropped `live` balance 100 → 97.5 within ~5s (`settled` unchanged at 100 until period-close settlement). The standalone adjustments endpoint is still 501 on this build — the adapter must not rely on adjustments for burn; it MUST provision plan+subscription rate cards. Burn granularity = metered usage × rate-card unit price (money), not token counts.
5. **Subscription provisioning semantics (om02b)**: V2 subscriptions auto-provision entitlements + recurring grants (`issueAfterReset`) tied to the subscription; V3 enforces `only_single_subscription_allowed_per_customer_at_a_time` (409) — one active subscription per customer per family, so the adapter must model plan changes via `/change`, not parallel subscriptions.

Adapter decisions implied (for the connector spec, not implemented here): issue prepaid balance as V3 grants keyed by our recharge idempotency key in body `key`; record both `expires_at` and our own expiry sweep (time-lapse unverified); meter usage via V1 events against `tokens_total`; block any code path that issues the same recharge through both families (single-family issuance per recharge id).

## Not verified (honest gaps)
- Time-based expiry lapse (would need >1h wait or clock control) — only stored/echoed fields verified.
- Subscription change/cancel effects on credits/entitlements (inventory ops `changeSubscription`/`cancelSubscription`, `change-subscription`/`cancel-subscription` exist; not exercised — time budget).
- V3 `settled` balance drop / invoice issuance at period close (P1M cadence; would need period-end or invoice simulation).
- Whether a V2 subscription without `entitlementTemplate` still meters usage for invoicing.
- Cleanup: customers/grants intentionally left in place (namespaced test data); driver cleanup (DELETE) only supports captured write-created ids and credits guard `unsettled_transactions` — not exercised.

## Artifacts
- Cases: `artifacts/saas-contract/om02-cases/*.json` (7 executed + 1 superseded draft `om02-v3-grant-idempotency.json`)
- Runs: `artifacts/saas-contract/{om02-v3-lifecycle,om02-v2-issuance,om02-v2-consumption,om02-v2-balance-after,om02-v2-grant,om02-v3-flow,om02-v3-idempotency,om02-v3-final-state}/run.json`
- OM-02B (subscription association + V3 rate-card burn, namespace `om02b-20260912`, customers `om02b-20260912-v2cust` id `01M29T9W2HQVWYK10H0VKSNFDX` / `om02b-20260912-v3cust` id `01M29T9Z6DVA4ATC501Y44G32D`): cases `artifacts/saas-contract/om02b-cases/*.json`, runs `{om02b-v2-subscription,om02b-v2-subscription-observe,om02b-v3-subscription,om02b-v3-burn}/run.json` plus `om02b-v3-burn/balance-after-5s.json` and `om02b-v2-subscription-observe/entitlement-value-final.json`. Note: `om02b-v3-subscription/run.json` holds the last attempt (409 single-subscription) — the successful plan3 publish + subscription create 201 + grant/balance are in this session's earlier attempts of the same case file (see case JSON bodies).
