"""JSON payload templates + pure helpers for the T11 settle-trigger lab
(t10 skeleton, R-4 dual-track revision).

Shapes follow the pinned Lago Community v1.53.0 contracts (t02/t09/t10
evidence; this file carries ONLY the payloads the #82 settle probe needs —
everything else is re-exported read-only from the payment-activation lab's
fixtures via ``pa_fixtures``).

- POST /api/v1/subscriptions  -- gated create: ``activation_rules:
  [{type: "payment", timeout_hours: 0}]`` (F8: the authority never
  auto-cancels; cancellation timing stays with the coordinator)
- Stripe-side settle rail (D2'): attach settle pm + set customer default +
  ``POST /v1/payment_intents/{id}`` (update payment_method) +
  ``POST /v1/payment_intents/{id}/confirm`` — the off-session synchronous
  charge on the stuck gating intent (P-C).
- POST /webhooks/stripe/{org_id} -- the F9/F10 receive chain: the local
  harness (transport-leg stand-in ONLY, D8) computes the real
  ``Stripe-Signature`` header with the provider's real webhook_secret and
  delivers a ``payment_intent.succeeded`` event whose body is the REAL PI
  object read back from the Stripe API (P-D).
"""

from __future__ import annotations

import hashlib
import hmac
import importlib.util
import json
import sys
import time
import uuid
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


# ---------------------------------------------------------------------------
# Stripe webhook transport-leg helpers (P-D; D8 boundary: harness only)
# ---------------------------------------------------------------------------

def sign_stripe_event(secret, payload, timestamp=None):
    """The real Stripe-Signature header value for a webhook delivery.

    Stripe signs ``"{t}.{payload}"`` with HMAC-SHA256; the header carries the
    timestamp and the hex digest. Lago's ValidateIncomingWebhookService calls
    Stripe::Webhook::Signature.verify_header against the provider's stored
    webhook_secret — this helper produces exactly that wire shape.
    """
    if isinstance(payload, (bytes, bytearray)):
        raw = bytes(payload)
    else:
        raw = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    ts = int(timestamp if timestamp is not None else time.time())
    signed = f"{ts}.".encode("utf-8") + raw
    digest = hmac.new(secret.encode("utf-8"), signed, hashlib.sha256).hexdigest()
    return f"t={ts},v1={digest}"


def build_pi_succeeded_event(payment_intent, event_id=None, api_version=None):
    """A ``payment_intent.succeeded`` event wrapping the REAL PI object.

    The object is carried VERBATIM (same reference) — the receiver chain
    (PaymentIntentSucceededService) reads payment identity and status off it;
    synthesizing a fake object would falsify the probe (GC-3/D8).
    """
    event = {
        "id": event_id or f"evt_probe_{uuid.uuid4().hex[:24]}",
        "object": "event",
        "api_version": api_version or "2024-06-20",
        "created": int(time.time()),
        "livemode": False,
        "type": "payment_intent.succeeded",
        "data": {"object": payment_intent},
    }
    return event


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
