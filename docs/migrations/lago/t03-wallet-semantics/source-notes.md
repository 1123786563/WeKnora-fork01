# T03 source notes — pinned lago-api citations

All citations are from `getlago/lago-api` at submodule SHA
`591ae9005110346f1c6034ec72ea9046625668cf` — the commit pinned by
`getlago/lago` tag `v1.53.0`, the same release as the digest-locked images in
`deploy/lago/images.lock.json`. Every claim below was re-verified against
that tree (and, where marked, re-observed at runtime by experiments E1–E4)
while building this verdict.

## Consumption order

- `app/models/wallet.rb` — `Wallet.in_application_order` =
  `order(:priority, :created_at)`. There is **no native expiry-based
  ordering**; wallets are picked by explicit priority, then creation time.
  Used by `Credits::AllocatePrepaidCreditsByWalletsService` (invoice
  finalization) via `AppliedPrepaidCreditsService`'s wallet scope
  (`customer.wallets.active.with_positive_balance`).
- `app/models/wallet_transaction.rb` — `in_consumption_order` scope:
  `order(:priority, CASE WHEN transaction_status = granted THEN 0 ELSE 1
  END, created_at)`. Within one wallet, inbound batches drain by
  transaction priority, **granted before purchased**, then `created_at`.
- E2 proved both orders at runtime, including the case where expiry rank is
  encoded into priority and disagrees with creation order.

## Expiry

- Only `wallets.expiration_at` exists — there is **no per-transaction
  expiry** anywhere in the schema.
- `app/jobs/clock/terminate_wallets_job.rb` — `Wallet.active.expired`
  → `Wallets::TerminateService` per wallet. Termination is **wallet-level
  and lazy**: until the clock runs, an expired-but-`active` wallet is still
  selectable for consumption (E1 measured the window).
- `clock.rb` (Clockwork schedule) — `Clock::TerminateWalletsJob` runs
  **hourly at `*:45`**; `Clock::CreateIntervalWalletTransactionsJob` runs
  **hourly at `*:55`**. This is the measured termination cadence ceiling:
  a wallet whose `expiration_at` passes waits up to ~65 minutes for
  termination.
- `app/services/wallets/terminate_service.rb` — `mark_as_terminated!`,
  webhook `wallet.terminated`. **No void transaction, no carry-over, no
  balance zeroing** — the remainder is stranded on the terminated wallet
  (still visible via the API) but consumable nowhere (E1).

## Active-wallet cap

- `app/services/wallets/validate_service.rb` —
  `MAXIMUM_WALLETS_PER_CUSTOMER = 6`; failure code `wallet_limit_reached`
  (`422`). The `organizations.maximum_wallets_per_customer` override has no
  writer in v1.53.0 (Community or Premium), so **six active wallets per
  customer is a hard runtime cap** (E1 cap probe).

## Transactions, statuses, traceability

- `app/models/wallet_transaction.rb` — `TRANSACTION_STATUSES =
  {purchased, granted, voided, invoiced}` (there is **no `refunded` status`),
  `STATUSES = {pending, settled, failed}`, `SOURCES = {manual, interval,
  threshold}`.
- `app/services/wallet_transactions/create_service.rb` —
  `initial_remaining_amount_cents` is set **only for `granted` inbound**
  transactions on traceable wallets; purchased inbounds carry `nil`
  remainders. DB check constraint `remaining_amount_cents_non_negative`.
- `app/services/wallets/create_service.rb` — `traceable?` =
  no prior active untraceable wallet: **fresh customers get traceable
  wallets**.
- `app/services/credits/applied_prepaid_credits_service.rb` — runs under
  `Customers::LockService.call(customer:, scope: :prepaid_credit)`;
  idempotent per invoice via `wallets_already_applied?` →
  `already_applied`; per-wallet outbound `invoiced` settled transactions;
  per-inbound remainder decrement via
  `WalletTransactions::TrackConsumptionService` (granted only).
- `app/services/wallets/balance/allocate_ongoing_usage_by_wallets_service.rb`
  — ongoing (pre-invoice) allocation lets **threshold-rule wallets and the
  last applicable wallet go negative** ("absorbs the overflow").

## Void

- `app/services/wallet_transactions/void_service.rb` — validates
  `wallet_credit.amount_cents <= inbound.remaining_amount_cents` for
  traceable wallets with an `inbound_wallet_transaction`
  (`exceeds_remaining_transaction_amount`); `void_remaining` skips the
  validation and sizes the whole remainder **under the customer lock**
  (`scope: :prepaid_credit`); a zero-remainder void is a silent no-op.
- `app/services/wallet_transactions/create_from_params_service.rb` — the
  API surface: `POST /api/v1/wallet_transactions` with `voided_credits`
  (+ optional `voided_transaction_id`) or `voided_transaction_id` alone
  (whole-remaining).
- E4 proved partial/exact/excess/no-op behavior at runtime.

## Refund

- `app/services/wallet_transactions/recredit_service.rb` — re-credits
  granted credits only, and its only callers are
  `Invoices::VoidService` (via `voided_invoice_id`) and
  `WalletTransactions::RecreditJob`. A **credit note on a paid-credit
  invoice does not re-credit the wallet** (E4 observed).
- No `refunded` wallet-transaction status exists; money refunds route
  through credit notes / payment providers (`POST /api/v1/credit_notes`,
  params `invoice_id`, `reason`, `credit_amount_cents`).

## Paid credits and settlement

- `app/services/wallet_transactions/create_from_params_service.rb` — a
  `paid_credits` POST creates a **`pending` purchased inbound** and enqueues
  `BillPaidCreditJob` (high_priority, after-commit).
- `app/services/invoices/paid_credit_service.rb` — creates the `credit`
  invoice; finalizes immediately **only under
  `License.premium? && invoice_requires_successful_payment?` gating
  differences**; in Community it finalizes without payment gating, but
  finalization is not settlement.
- `app/services/invoices/payments/create_service.rb` — with **no payment
  provider configured** the service returns early (`should_process_payment?`
  false): the invoice stays `payment_status: pending` and nothing settles.
- `app/services/invoices/update_service.rb` — `PUT /api/v1/invoices/:id`
  with `payment_status: "succeeded"` on a credit invoice enqueues
  `Invoices::PrepaidCreditJob(invoice, :succeeded)` →
  `Wallets::ApplyPaidCreditsService` → `WalletTransactions::SettleService` +
  `Wallets::Balance::IncreaseService`. **This is the payment-fact →
  fulfillment seam** the WeKnora coordination layer must drive (E1/E4
  confirmed at runtime: purchased credits are NOT spendable until the
  invoice is marked paid).

## Idempotency

- Wallet and wallet-transaction APIs accept **no external idempotency
  identity** (`app/controllers/api/v1/wallet_transactions_controller.rb`
  input params). A retried grant POST mints a second settled inbound (E3).
- Correlation is via `metadata` (`?metadata[key]=value` filter on the
  wallet-scoped transaction index), `purchase_order_number`, and returned
  `lago_id` — the recovery path for duplicate grants (E3).
- Events, by contrast, ARE idempotent by `transaction_id` (E3 observed the
  rejection) — the pattern grants would need.

## Interval rules (Community boundary)

- `app/services/wallets/recurring_transaction_rules/create_service.rb` —
  `return unless License.premium?`: rule creation is **Premium-gated**, and
  the early `nil` return makes `Wallets::CreateService#call!` crash
  (`NoMethodError: undefined method 'raise_if_error!' for nil`,
  `app/services/wallets/create_service.rb:83`) — observed at runtime as
  **HTTP 500** when creating a wallet with `recurring_transaction_rules` on
  Community (E1).
- `app/services/wallets/create_interval_wallet_transactions_service.rb` —
  even with Premium, interval top-ups target the **same persistent wallet**
  (no expiry → leftovers carry over), and wallets **created today are
  excluded** (`DATE(wallets.created_at) != DATE(:today)`), so the first
  interval top-up happens on day two at the earliest.

## Misc API facts confirmed at runtime

- `POST /api/v1/plans` requires an explicit plan-level `pay_in_advance`
  boolean (`app/models/plan.rb:65`, `inclusion: {in: [true, false]}` — nil
  → `value_is_invalid`).
- A charge embedded in the plan payload must reference the metric by
  `billable_metric_id`, not code
  (`app/services/charges/create_service.rb` resolves id only).
- `charge.invoiceable` is Premium-gated in `Charges::CreateService`
  (`if License.premium?`); Community keeps the DB default `true`.
- `POST /api/v1/subscriptions` requires the subscription's own
  `external_id` (`value_is_mandatory`).
- Pay-in-advance events without `external_subscription_id` resolve no
  subscription and silently create no invoice
  (`Events::PayInAdvanceService#charges` → `event.subscription` nil).
- `DELETE /api/v1/wallets/:lago_id` terminates
  (`config/routes/shared_api.rb`: `delete "/wallets/:id", to:
  "wallets#terminate"`); grants against a terminated wallet fail with
  `wallet_is_terminated`
  (`app/services/wallet_transactions/validate_service.rb:40`).
- One-off (`POST /api/v1/invoices`) invoices do NOT apply prepaid credits on
  v1.53.0 (`Invoices::CreateOneOffService` →
  `Invoices::TransitionToFinalStatusService` → `Invoices::FinalizeService`
  — none call `Credits::AppliedPrepaidCreditsService`). The deterministic
  public-API consumption trigger is the pay-in-advance event path:
  `Events::PayInAdvanceService` → `Invoices::CreatePayInAdvanceChargeJob`
  (invoiceable charge) → `Credits::AppliedPrepaidCreditsService.call!`.
