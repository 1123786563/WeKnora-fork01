# T09 DECISION — Quote→Invoice gating, the spec L121 line-item deviation, and the binding contract (#81)

Status: accepted (2026-09-23). Owner: #81 implementer per the approved plan
`docs/plans/issue-72-plan-81.md` and the 2026-09-23 user rulings (T02 option
(b) supported-provider channel; D2 option A with four mandatory conditions).

## 1. The spec text (verbatim)

From `docs/specs/2026-09-20-lago-billing-migration-design.md`, L121:

> "Before a Channel Payment Order is created, the Quote and actual Lago
> Invoice must match in Plan Version, currency, total, and line items. A
> mismatch aborts the purchase and creates no channel payment request."

## 2. Source-level evidence: why pre-payment line-item matching is IMPOSSIBLE on the pinned v1.53.0

All three reads were taken inside the running pinned container
(`weknora-lago-api-1`, release v1.53.0) during the plan-authoring session and
re-verified during this implementation (2026-09-23):

1. `/app/app/models/invoice.rb:100-101` —
   `VISIBLE_STATUS = {draft:0, finalized:1, voided:2, failed:4, pending:7}`;
   `INVISIBLE_STATUS = {generating:3, open:5, closed:6, deleted:8}`. The
   gating invoice sits at status `open` (5) while awaiting payment →
   INVISIBLE.
2. `/app/app/queries/invoices_query.rb:122-129` — `with_status` intersects
   the CALLER-provided statuses with `visible_keys`; no parameter combination
   can list an open invoice (a literal `status=open` request answers 422:
   the enum does not contain `open`).
3. `/app/app/controllers/invoices_controller.rb:46-48`
   (`invoices.visible.find_by`) and the GraphQL `InvoiceResolver`
   (`invoices.visible.find`) — both hard-filter `.visible`; a direct
   lago_id lookup of an open invoice answers 404.

Conclusion: the "actual Lago Invoice" is unreadable by EVERY API path while
it gates. A pre-payment line-item match cannot be implemented — this is an
external hard constraint, not an implementation choice.

### 2b. Additional t09 run facts (this implementation)

- F11 — the gated create requires MORE than a provider binding: a customer
  bound to a registered provider but WITHOUT a synced default payment method
  answers 422 `{"customer":["no_default_payment_method"]}`. The binding
  ensure therefore attaches a configured payment-method token, promotes it to
  default (the provider CLONES shared `pm_card_*` ids on attach — the
  customer-scoped id is the one the default update references), and polls the
  authority's payment-method import before the create.
- F12 — the gating invoice is PRORATED for a mid-period create (tenant
  creation-day purchase of a 9900-fen plan produced a 2540-fen fee; read via
  the Lago DB, the only authoritative view). Even ignoring the visibility
  wall, a pre-payment TOTAL equality check against the plan price could
  never hold. This is further evidence that option A was the only coherent
  reading of the spec on this runtime.

## 3. The replacement verification chain (what #81 DOES enforce before any channel request)

1. Hard equality on the AUTHORITATIVE SUBSCRIPTION FACE, read by identity
   with the explicit status set (the index defaults to active — F6):
   `plan_code == DeterministicPlanCode(key, version)`, `currency == "CNY"`,
   and `plan_amount_cents == quote.amount_fen` (integer fen). Any mismatch →
   `ErrInvoiceQuoteMismatch`, HTTP 409, and NO channel call (router test
   asserts the channel counter stays 0).
2. Structural line equivalence by slicing: only plans with NO usage charges
   are purchasable (`ErrPurchasePlanCharges`); publishable plans are
   pay-in-advance with no charges, so the first invoice carries exactly ONE
   subscription-fee line — the structural half of "line items" is guaranteed
   by construction, not by reading an invisible document.
3. Payment-time full re-check (the compensating half): the finalized invoice
   IS visible after payment; `PurchaseSnapshot.InvoiceFees
   []InvoiceLineSnapshot` is the explicit interface (#82 Payment recording
   and #84 abnormal payments MUST consume it for the line-item re-check).
   During the open stage it is ALWAYS empty — an invisible line is never
   fabricated. Later-issue plans' Consumes sections must reference this
   deliverable; a pre-payment non-match must never decay into "never
   compared".

## 4. Escalation trail (mandatory condition 4)

- Plan review round 1 surfaced the L121 impossibility (the plan's evidence
  baseline F3–F5) → escalated with three options.
- User ruling 2026-09-23: option A approved WITH four mandatory conditions —
  (1) this decision record; (2) a ledger Ruling with decision/basis/error
  cost; (3) the `InvoiceFees` interface deliverable for #82/#84; (4) the
  deviation applies to the pre-payment line-item comparison ONLY and sets no
  precedent for any other spec clause. The user further declared a formal
  spec-revision proposal to the spec owner; #81 does not wait for it.

## 5. Error cost of this decision (mandatory condition 2, ledger form)

If the ruling is WRONG (i.e. a pre-payment line-item match is somehow
achievable on the pinned runtime), the cost is a weakened pre-payment defense:
a drifted or tampered first invoice would be caught only AFTER the channel
payment request exists, at payment time (the #82/#84 full re-check) or by
the abnormal-payment acceptance path (#84). The structural slice (no
charges), the three-field subscription-face hard equality, and the
one-invoice-per-identity DB audit keep the residual risk bounded to the
invisible-invoice surface itself.

## 6. Scope boundary (mandatory condition 5)

This deviation covers ONLY the pre-payment line-item comparison of spec
L121. Every other clause of the spec (integer minor units, CNY-only,
credential secrecy, closed product states, outbox/idempotency, tenant
isolation, …) is implemented as written and tested as written.

## 7. Design decisions D1/D3 (as planned; verified here)

- D1 purchase identity `ExternalPurchaseSubscriptionID(t) =
  weknora-tenant-<t>-purchase` — separate from the #80 Base identity
  (`…-sub`), which never terminates; the space keeps Base fallback
  entitlements while the purchase is gated (proven: entitlements 404 while
  gated = purchase entitlements closed; base benefits untouched).
- D3 binding source precedence: provider API key (env) → placeholder prefix
  (dev/test) → `ErrPlatformUnconfigured` (fail closed). t09 adds the F11
  payment-method chain to the provider-key path; production (no test token)
  fails the gated create closed until the real card arrives through the
  provider checkout (#82/#83).
