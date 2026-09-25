"""JSON payload templates + pure helpers for the T10 settlement-trigger lab.

Shapes follow the pinned Lago Community v1.53.0 contracts (t02/t09 evidence;
this file carries ONLY the payloads the #82 trigger probe needs — everything
else is re-exported read-only from the payment-activation lab's fixtures via
``pa_fixtures``).

- POST /api/v1/subscriptions  -- gated create: ``activation_rules:
  [{type: "payment", timeout_hours: 0}]`` (F8: the authority never
  auto-cancels; cancellation timing stays with the coordinator)
- POST /api/v1/invoices/{id}/retry_payment -- the F3 trigger contract; the
  body is an EMPTY JSON object (the controller reads only the optional
  ``payment_method`` param, and the probe's link under test is the
  default-payment-method resolution path)
- GET  /api/v1/payments -- F5: the payment index answers non-succeeded
  provider payments WITH their ``invoice_ids`` even while the gating invoice
  itself is open/API-invisible.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

_PA_DIR = Path(__file__).resolve().parent.parent / "payment-activation"
if str(_PA_DIR) not in sys.path:
    sys.path.insert(0, str(_PA_DIR))

# The payment-activation lab's fixtures module, loaded under an alias so the
# bare name ``fixtures`` keeps resolving to THIS probe's payloads when the
# runner/test modules do ``import fixtures`` from this directory.
_spec = importlib.util.spec_from_file_location("pa_fixtures", _PA_DIR / "fixtures.py")
pa_fixtures = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(pa_fixtures)

# Read-only re-exports from the T02 lab (same pinned v1.53.0 shapes).
plan_payload = pa_fixtures.plan_payload
plan_entitlements_payload = pa_fixtures.plan_entitlements_payload
customer_payload = pa_fixtures.customer_payload
ADD_STRIPE_PROVIDER_QUERY = pa_fixtures.ADD_STRIPE_PROVIDER_QUERY
add_stripe_provider_variables = pa_fixtures.add_stripe_provider_variables


def gated_subscription_payload(external_customer_id, plan_code, external_id,
                               rule_type="payment", timeout_hours=0):
    """The payment-gated create (F1/F8): timeout 0 -- the authority never
    auto-cancels, the coordinator owns cancellation timing."""
    return {
        "subscription": {
            "external_customer_id": external_customer_id,
            "plan_code": plan_code,
            "external_id": external_id,
            "activation_rules": [{"type": rule_type, "timeout_hours": timeout_hours}],
        }
    }


def retry_body():
    """The exact retry_payment body under test (F3): empty object -- the
    trigger must work through the customer's RESOLVED default payment
    method, never by naming one in the request."""
    return {}


def second_pm_present(methods):
    """The P2 (c) poll predicate: the authority has imported the SECOND
    payment method (the settle pm) when the list carries at least 2
    entries."""
    return isinstance(methods, list) and len(methods) >= 2


def unsettled_payments(payments):
    """The payment rows whose status is not ``succeeded`` (the stuck
    provider intents the probe inspects)."""
    return [p for p in (payments or []) if isinstance(p, dict) and p.get("status") != "succeeded"]


def succeeded_payments(payments):
    """The payment rows whose status IS ``succeeded`` (the exactly-once
    counter of P2/P3)."""
    return [p for p in (payments or []) if isinstance(p, dict) and p.get("status") == "succeeded"]


def first_unsettled_payment(payments):
    """The first unsettled payment row carrying a non-empty ``invoice_ids``
    (F5), or None."""
    for p in unsettled_payments(payments):
        invoice_ids = p.get("invoice_ids")
        if isinstance(invoice_ids, list) and invoice_ids:
            return p
    return None


def intent_cancelable(intent_status):
    """Whether a Stripe PaymentIntent in this status accepts a cancel
    (P2 (a)): requires_action / processing-adjacent states do; already
    terminal ones are treated as success-and-continue by the caller."""
    return intent_status in (
        "requires_payment_method",
        "requires_action",
        "requires_confirmation",
        "processing",
        "unpaid",
    )


def finalized_invoice(invoices):
    """The finalized subscription invoice (the post-activation read of the
    formerly-invisible gating invoice), or None."""
    for inv in invoices or []:
        if not isinstance(inv, dict):
            continue
        if inv.get("status") == "finalized" and inv.get("invoice_type") in (None, "subscription"):
            return inv
    return None


def unsettled_intents(intents):
    """Stripe PaymentIntent rows that have NOT reached a terminal status
    (the stuck provider-side collections the settle trigger resolves)."""
    return [i for i in (intents or [])
            if isinstance(i, dict) and i.get("status") not in
            ("succeeded", "canceled", "failed", "processing_failed")]


def intent_invoice_id(intent):
    """The Lago invoice id a stuck intent carries in its metadata (the
    pinned CreateService writes ``lago_invoice_id`` onto every provider
    payment), or None."""
    meta = (intent or {}).get("metadata") or {}
    value = meta.get("lago_invoice_id")
    return value if isinstance(value, str) and value else None
