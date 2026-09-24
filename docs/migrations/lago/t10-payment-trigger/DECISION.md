# T10 — Settlement-trigger contract probe (#82) — DECISION

- **Date:** 2026-09-24
- **Stack:** pinned Lago Community v1.53.0 (`getlago/api@sha256:4d100177…`, digest-locked compose; lab `weknora-lago-82`, API :48895), Stripe TEST mode key (env-injected; never committed).
- **Probe code:** `deploy/lago-lab/payment-trigger/` (run `python3 run_lab.py`, evidence `t10-*.json` in this directory, run id `6f45ed2f…`).
- **Purpose:** verify the three links the #82 design decision D2 (settle trigger via the supported provider rails) depends on, BEFORE implementation (plan Task 1).

## Verdict — UP升级路径触发（P2 FAIL）

**The D2 settlement-trigger chain is FALSIFIED on the pinned stack.** There is no
supported-API path that can move a payment-gated subscription to active after a
channel payment, in the window where the gating invoice is still hidden. Per the
plan's escalation clause, implementation-side changes for Task 5+ are stopped and
the finding returns to the spec/ADR owner for a T02 §5 option re-decision.

## Link-by-link results

| Link | Expected (plan F-facts) | Observed | Verdict |
|---|---|---|---|
| gated create (3DS pm, `timeout_hours: 0`) | incomplete + default pm imported (F7) | incomplete held across the whole window; subscription NOT auto-canceled (F8 holds) | PASS (`t10-gated-3ds.json`) |
| P1 — `GET /api/v1/payments?external_customer_id=` | stuck (non-succeeded) payment row visible WITH `invoice_ids` (F5 "no visibility filter") | HTTP 200 with `payments: []` for the whole stuck window; rows land in the authority DB (`failed`/`canceled`) but stay API-invisible | **FAIL — F5 falsified** (`t10-payment-probe-p1.json`) |
| P1' — Stripe `GET /v1/payment_intents?customer=` + metadata | (probe extension) supported provider-side invoice locator | unsettled intent found; `metadata.lago_invoice_id` present (pinned `Invoices::Payments::CreateService` stamps it on every provider payment) | PASS |
| P2a — cancel the stuck provider intent | accepted, unlocks `ready_for_payment_processing?` | HTTP 200 | PASS |
| P2b — settle pm attach + default (provider side) | accepted | attach + default both 200 | PASS |
| P2c — Lago re-import of the settle pm (2nd payment method) | customer write triggers import → 2 pms | same-pcid POST upsert: no new import; fresh-pcid POST upsert (new Stripe customer + billing_configuration swap): no new import. The pinned import trigger observed in the wild is the `payment_provider_customers` row CREATION (first bind only); webhook-driven imports are unreachable from a local stack | NOT TRIGGERED (`t10-trigger-p2.json` contract_notes) |
| P2d — `POST /api/v1/invoices/{id}/retry_payment` body `{}` | 200 (F3) | **HTTP 404 `{"code":"invoice_not_found"}`** | **FAIL — F3 narrowed** (`t10-trigger-p2.json`) |
| P2e — subscription active + exactly 1 succeeded payment + finalized invoice (F1) | full activation set | never reached | NOT ACHIEVED |
| P3/P4 — duplicate/paid retry guards | 405 `invalid_status` (F2) | blocked (require P2e) | NOT RUN |

## Source evidence (pinned `getlago/lago-api@591ae900`)

1. **F5 falsified — payments index hides the stuck rows.**
   `app/queries/payments_query.rb` `base_scope` applies
   `visible_payable_condition`: a payment row is returned only when its payable
   invoice is in `Invoice::VISIBLE_STATUS` (`draft, finalized, voided, failed,
   pending`). `open` and `closed` are in `INVISIBLE_STATUS`
   (`app/models/invoice.rb:100-101`), and a stuck 3DS collection keeps its
   gating invoice open-then-closed — so the payment row is invisible exactly
   when the settle trigger needs it. Runtime: `payments_total: 0` across a 60 s
   poll while the DB holds the rows (probe run `4d334390`/`6f45ed2f`).
2. **F3 narrowed — retry_payment cannot see a hidden gating invoice.**
   `app/controllers/api/v1/invoices_controller.rb` `retry_payment`:
   `current_organization.invoices.visible.find_by(id: params[:id])` → a hidden
   (open/closed) invoice yields `not_found_error` BEFORE any service logic. The
   probe's empty-body POST with the metadata invoice id answers
   `404 invoice_not_found`. F2's `405 invalid_status` (finalized+succeeded) is
   unaffected — but it is only reachable for VISIBLE invoices, i.e. invoices
   whose payment already succeeded.
3. **No supported alternative trigger.**
   - The activation driver on this pinned version is the authority's OWN
     auto-collection (`Invoices::Payments::CreateService` via the gated
     create's after-commit job). It succeeds exactly when the customer's
     bound provider default payment method is directly chargeable — the F1
     scenario (customer B, `pm_card_visa`).
   - `CreateService.should_process_payment?` refuses `closed` invoices, and
     with the invoice closed after retry exhaustion there is no supported call
     left that can re-open the collection: retry_payment 404s (evidence 2),
     manual `POST /api/v1/payments` is Premium-gated 403 on Community
     (t02-manual.json, F9), and no webhook receiver may be assumed (plan:
     out of scope, D2 needs none — which this probe disproves).
   - The pm re-import trigger is the provider-customer binding CREATION
     (first bind); neither a same-pcid nor a fresh-pcid upsert mints a second
     import, and webhook-driven imports cannot reach a local stack.

## What this means for #82

- The plan's D2 algorithm ("read stuck payment row → invoice id → cancel
  intent → switch settle pm → `retry_payment`") has NO working window on
  pinned Community v1.53.0: the invoice is invisible exactly while a settle
  could matter, and visible only after activation (or terminal failure).
- What REMAINS valid and probe-proven on the pinned stack:
  - gated create + `timeout_hours: 0` semantics (F7/F8) — the purchase order
    flow of #81 stands;
  - the Stripe-side settle-pm attach/default mechanics (P2b PASS);
  - `metadata.lago_invoice_id` on every provider payment (a supported
    provider-side locator, should any future design need it);
  - F2's 405 `invalid_status` guard on paid invoices (exactly-once defense).
- The supported activation path on pinned Community is authority-driven ONLY
  (default pm directly chargeable). A channel-payment-driven activation
  requires either the Premium manual-payment entry (T02 §5 option ①) or a
  design where the authority's own collection settles the gating invoice —
  both are spec/ADR-owner decisions, not implementation-level substitutions.

## Escalation clause (plan Task 1)

This probe triggers the plan's upgrade path: **implementation work for #82
Task 5 (Lago settle command) and its dependents is STOPPED**; the evidence and
the lab skeleton are committed as-is. Task 2/3/4 deliverables (seam additive
command + purchase wallet identity, deterministic fake settle semantics,
finalized-stage invoice-fees read — T09 mandatory condition ③) are
trigger-mechanism-independent and stand on their own; Task 4's
`readPurchaseInvoiceFees` (lago_purchase.go) is the post-activation read the
fulfillment re-check needs under ANY re-decided design.
