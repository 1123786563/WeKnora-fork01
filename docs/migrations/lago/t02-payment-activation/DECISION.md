# DECISION — how an external payment activates a payment-gated Subscription (Ticket #74)

Decision document for Tickets #81/#82 (purchase-flow recording path).
Evidence links point at [`./`](./README.md) (runtime evidence bundle) and
the pinned Lago Community `v1.53.0` source
(`getlago/lago-api` @ `591ae9005110346f1c6034ec72ea9046625668cf`).
Everything not proven at runtime in this lab is explicitly marked
**unproven at runtime**.

## 1. Question

For the WeKnora purchase flow, should a verified external channel payment
(WeChat Pay / Alipay, owned and verified by WeKnora) be recorded in Lago as

- a **manual Payment** via the REST/GraphQL API, or
- through a **Lago-supported external Payment Provider** integration,

such that the payment-gated Subscription activates **exactly once** and
Entitlement becomes usable?

## 2. Manual path verdict — Community-blocked (Premium-gated)

- **Source verdict (pinned v1.53.0):** `Payments::ManualCreateService#check_preconditions`
  ends with `return result.forbidden_failure! if !License.premium?`
  (`app/services/payments/manual_create_service.rb`). Both
  `POST /api/v1/payments` and GraphQL `CreatePayment` call this service, so
  **every manual payment registration returns 403 Forbidden on Community**.
  `License.premium?` requires a server-verified `LAGO_LICENSE`
  (`config/initializers/license.rb`,
  `lib/lago_utils/lago_utils/license.rb`).
- **Runtime evidence (re-run with a Stripe TEST key):**
  [`t02-manual.json`](./t02-manual.json) reports `pass` —
  `POST /api/v1/payments` against a real lab invoice returned HTTP 403
  Forbidden and left invoice + subscription state unchanged. On v1.53.0
  the 3DS gate invoice stays API-invisible (open/closed are
  INVISIBLE_STATUS), so the probe targets customer B's settled activation
  invoice — the Premium gate fires before any invoice-state check, so the
  403 evidences the Community gate on any real invoice. The Community 403
  is now **runtime-proven**, matching the source verdict.
- **Actual REST contract (source-verified, differs from the ticket
  draft):** `POST /api/v1/payments` accepts only `invoice_id`,
  `amount_cents`, `reference`, `paid_at` (`payments_controller#create_params`).
  There is **no `external_id` and no `status` field**; status is always
  recorded `succeeded` server-side. `paid_at` is optional. The lab records
  this contract verbatim in `t02-manual.json` (`observed.param_contract`)
  once the phase runs.
- **Mechanism exists but is license-gated:** with Premium, a manual payment
  flows through `Invoices::UpdateService` — total paid is recomputed, and
  once `total_paid_amount_cents` reaches `total_amount_cents` the invoice
  `payment_status` becomes `succeeded`, which is exactly the signal
  `handle_payment_gated_activation` → `Payment::ResolveJob` uses to satisfy
  the payment activation rule, finalize the invoice, and activate the
  subscription. I.e. the manual path and the provider path converge on the
  same activation mechanism; only the gate differs.
- **Cost:** a Premium license — out of scope for this migration phase by
  spec (*Lago Premium features … are out of scope*), and the lab
  deliberately kept `LAGO_LICENSE` empty.

## 3. Provider path verdict — the supported Community path (runtime proof pending one env input)

- **Source chain (pinned v1.53.0):** a subscription created with
  `activation_rules: [{type: "payment", timeout_hours: N}]` is
  `mark_as_incomplete!`-gated with an open, numberless, `payment_status:
  pending` pay-in-advance invoice; the invoice auto-triggers
  `Invoices::Payments::CreateService` → provider Payment + confirmed
  off-session PaymentIntent (synchronous success for Stripe — no webhook
  needed); succeeded/failed status flows through
  `Invoices::UpdateService#handle_payment_gated_activation` →
  `Payment::ResolveJob` → rule satisfied → invoice **finalized** →
  subscription **active**; failed → invoice closed → subscription
  **canceled** (`cancellation_reason: payment_failed`).
- **Exactly-once guards (source):** at most one `pending/processing`
  provider payment per payable (partial unique index on `payments`);
  `ResolveService` no-ops unless the subscription is still `incomplete`
  and the invoice still `open`; `retry_payment` returns
  `not_allowed` once `payment_succeeded?`; re-creating a subscription with
  the same `external_id` returns `422 value_already_exist` (active) or
  `subscription_incomplete` (incomplete); the PaymentIntent carries
  `idempotency_key: "payment-#{payment.id}"`.
- **Runtime evidence (re-run with a Stripe TEST key,
  [`t02-environment.json`](./t02-environment.json)
  `run.overall: pass`):** [`t02-activation.json`](./t02-activation.json)
  reports `pass` — customer B's gated subscription became `active` with
  1 succeeded provider payment
  (`provider_payment_id` present), the invoice finalized, numbered and
  `payment_status: succeeded`, entitlements 200 with the plan feature,
  totals matched in integer cents.
  [`t02-duplicates.json`](./t02-duplicates.json) `pass` (duplicate probes
  harmless — the re-POST answered 200 with the SAME subscription,
  `retry_payment` 405, manual 403; final state byte-identical);
  [`t02-retries.json`](./t02-retries.json) `pass`
  ({"gate_not_activated": true, "no_second_payment_row": true, "no_succeeded_payment_for_gate": true, "same_identity_recovered": true});
  [`t02-decline.json`](./t02-decline.json) `pass` (a charge that cannot
  succeed never activates — with `timeout_hours: 0` the negative-control
  subscription stays `incomplete`, entitlements 404, zero succeeded
  payments; `canceled(payment_failed)` only appears after a non-zero
  timeout expires on the hourly clock, outside a lab window).
  A read-only DB observer confirmed exactly-once at the projection level:
  succeeded payments peaked at history+1 this run, customer B reached
  active without regressing, customer C never reached active.
  The end-to-end provider-path activation is therefore
  **runtime-proven on the pinned Community v1.53.0**.

## 4. Blocker statement for WeChat/Alipay channel payments

**Lago Community v1.53.0 has no supported entry point to register an
externally completed WeChat Pay / Alipay channel payment against a gated
invoice.** Specifically (all source-verified on the pinned release):

1. Manual Payment recording (the natural seam for externally verified
   facts) is Premium-gated (section 2).
2. The external Payment Provider set is fixed —
   `app/models/payment_providers/` contains exactly: stripe, gocardless,
   adyen, cashfree, flutterwave, moneyhash. There is no generic/custom
   provider type in Community.
3. Stripe payment-method types are allowlisted
   (`PaymentProviderCustomers::StripeCustomer::PAYMENT_METHODS` = card,
   sepa_debit, us_bank_account, bacs_debit, link, boleto, crypto,
   customer_balance) — **no `wechat_pay`, no `alipay`**.
4. Forging provider webhooks or writing subscription/invoice state
   directly in the database is excluded by spec (*may not force the
   Subscription active locally*; no dual-write).

So on Community, a payment-gated Subscription can only be activated by a
charge that one of the six supported providers executes itself. A WeChat/
Alipay channel success verified by WeKnora cannot reach the activation
rule through any supported API.

## 5. Options for Tickets #81/#82

| Option | What it means | Evidence / basis | Escalation needed |
|--------|---------------|------------------|------------------|
| (a) **Acquire Lago Premium** and record WeKnora-verified channel facts as **manual Payments** | WeKnora keeps owning WeChat/Alipay request/callback/refund; each verified Payment Fact becomes `POST /api/v1/payments {invoice_id, amount_cents, reference, paid_at}`; invoice `payment_status` then drives the same activation rule resolution (source-verified, section 2) | Source: `manual_create_service.rb` premium gate + paid-status propagation; activation-rule convergence (section 2/3) | Premium license cost + vendor terms — **spec/ADR owners**; Premium-manual flow itself is source-verified, not runtime-verified by this Community lab |
| (b) **Adopt a Lago-supported provider as the actual charging rail** (e.g. Stripe) where feasible; keep WeKnora channels only where the provider cannot serve them | Activation works end-to-end on Community for provider-executed charges (source-verified; runtime proof pending the Stripe test key) | Source: provider-driven activation chain (section 3); runtime: `t02-activation.json` et al. once re-run with a key | Product decision on CN payments (Stripe cannot serve WeChat/Alipay-native flows; provider set is closed) — **spec/ADR owners + product** |
| (c) **Change the spec/ADR** (e.g. drop Lago as the payment-activation authority for channel purchases, or add a compliant recording seam upstream of Lago) | Only if neither (a) nor (b) is acceptable | Blocker statement (section 4) | Full spec change — **spec/ADR owners** |

**The lab recommends (a)**, with (b) as the partial complement where a
supported provider can be the real rail: it is the only option that
preserves the spec's architecture (WeKnora-verified channel Payment Facts
drive Lago; Lago remains the sole activation authority; no dual-write, no
local force-active), and the activation mechanism it rides on is already
wired in the pinned release. **Explicitly unproven:** (i) ~~the AC1–AC4
runtime evidence of this lab is blocked-env pending a Stripe TEST-mode
key~~ (resolved by the re-run: all phases `pass`);
(ii) the Premium manual-payment → activation flow is source-verified only
(Premium is out of scope for this lab); (iii) commercial terms of Premium
are outside the lab's competence.

## 6. Acceptance-criterion mapping

| #74 acceptance criterion | Evidence files |
|--------------------------|----------------|
| AC1 — 付款前 Subscription 保持 incomplete，Entitlement 不可使用 | [`t02-gating.json`](./t02-gating.json) (re-run: `pass`) |
| AC2 — 可信付款登记后 Subscription 变为 active，且重复登记不重复激活 | [`t02-activation.json`](./t02-activation.json) + [`t02-duplicates.json`](./t02-duplicates.json) (re-run: `pass`) |
| AC3 — 响应丢失、超时与重试使用同一商业身份并可恢复 | [`t02-retries.json`](./t02-retries.json) (re-run: `pass`) |
| AC4 — 若 manual Payment 不能激活，证据明确给出受支持替代路径或 blocker | [`t02-manual.json`](./t02-manual.json) + [`t02-decline.json`](./t02-decline.json) (re-run: `pass`) + this document (section 4 blocker, section 5 supported path) |

## 7. Cross-ticket notes

- **No changes requested to `deploy/lago/`.** The lab reused `compose.yaml`,
  `images.lock.json`, `health.py`, and the README strictly read-only via
  `docker compose -f … --env-file … -p weknora-lago-74`.
- **Runtime contract deviation found and fixed during the real run:** the
  plan-entitlement attach endpoint
  `POST /api/v1/plans/{code}/entitlements` expects `entitlements` as a
  **hash keyed by feature code** with per-privilege values
  (`PlanEntitlementsUpdateService` iterates `feature_code,
  privilege_values`); an array of `{feature_code}` items makes the runtime
  return HTTP 500. Also, plan/subscription entitlement serializers key the
  feature by `code` (not `feature_code`). The official API reference for
  the pinned release does not make the hash shape obvious — worth a
  cross-check in any later client implementation (#81/#82 or the billing
  adapter).
- **Subscription index default filter:** `GET /api/v1/subscriptions`
  defaults to `status: ["active"]`
  (`SubscriptionIndex#subscription_index`); any recovery-by-identity GET
  for an incomplete/canceled subscription must pass explicit `status[]`
  parameters, and `GET /api/v1/subscriptions/{external_id}` likewise
  filters by `status` (default `active`). The lab's retry/recovery probes
  pass statuses explicitly.
- **Rule-timeout semantics documented from source, not observed at
  runtime:** `timeout_hours: 0` means "never expires"; a non-zero timeout
  expires gated subscriptions on the hourly clock job at :20
  (`ExpireIncompleteSubscriptionsJob` via `clock.rb`). A lab session
  cannot observe the hourly tick, so this is recorded as source knowledge
  (`app/services/subscriptions/activation_rules/…`, `clock.rb`), not
  runtime evidence.
- **Environment gap closed (2026-09 re-run):** the documented re-run with a
  Stripe TEST-mode key converted every phase report into a real verdict;
  see [`t02-environment.json`](./t02-environment.json) (`run.overall:
  pass`).
