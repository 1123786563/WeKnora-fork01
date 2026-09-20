"""JSON payload templates for the T02 lab objects.

Shapes follow the pinned Lago Community v1.53.0 contracts (verified against
the getlago/lago-api source at commit 591ae90):

- POST /api/v1/features                       -- features_controller
- POST /api/v1/plans                          -- plans_controller
- POST /api/v1/plans/{code}/entitlements      -- plans/entitlements_controller
  (``entitlements`` array with ``feature_code`` + property keys)
- POST /api/v1/customers                      -- customers_controller
  (``payment_provider``, ``provider_customer_id``,
   ``provider_payment_methods`` permitted)
- POST /api/v1/subscriptions                  -- subscriptions_controller
  (``activation_rules: [[:type, :timeout_hours]]`` permitted)
- POST /api/v1/payments                       -- payments_controller
  (only ``invoice_id``, ``amount_cents``, ``reference``, ``paid_at``; the
  recorded status is always ``succeeded`` server-side and the operation is
  Premium-gated on Community)
- GraphQL ``addStripePaymentProvider``        -- input {code, name, secretKey}
"""

from __future__ import annotations

from datetime import datetime, timezone

PLAN_AMOUNT_CENTS = 5000
PLAN_AMOUNT_CURRENCY = "USD"


def _now_iso():
    return datetime.now(timezone.utc).isoformat()


def feature_payload(code, name="WeKnora T02 lab feature"):
    return {"feature": {"code": code, "name": name}}


def plan_payload(
    code,
    name="WeKnora T02 lab plan",
    interval="monthly",
    amount_cents=PLAN_AMOUNT_CENTS,
    amount_currency=PLAN_AMOUNT_CURRENCY,
    pay_in_advance=True,
):
    return {
        "plan": {
            "code": code,
            "name": name,
            "interval": interval,
            "amount_cents": amount_cents,
            "amount_currency": amount_currency,
            "pay_in_advance": pay_in_advance,
            "trial_period": 0.0,
        }
    }


def plan_entitlements_payload(feature_code):
    """Attach a feature to a plan.

    The v1.53.0 controller passes ``params[:entitlements]`` to
    ``PlanEntitlementsUpdateService``, which iterates it as a Hash keyed by
    feature code with per-privilege values (an array of ``feature_code``
    items makes the service 500). A feature without privileges attaches
    with an empty privileges object.
    """
    return {"entitlements": {feature_code: {}}}


def customer_payload(external_id, name, provider_customer_id,
                     payment_provider="stripe", provider_payment_methods=("card",)):
    return {
        "customer": {
            "external_id": external_id,
            "name": name,
            "payment_provider": payment_provider,
            "provider_customer_id": provider_customer_id,
            "provider_payment_methods": list(provider_payment_methods),
        }
    }


def subscription_payload(external_customer_id, plan_code, external_id,
                         rule_type="payment", timeout_hours=0):
    return {
        "subscription": {
            "external_customer_id": external_customer_id,
            "plan_code": plan_code,
            "external_id": external_id,
            "activation_rules": [{"type": rule_type, "timeout_hours": timeout_hours}],
        }
    }


def manual_payment_payload(invoice_id, amount_cents, reference, paid_at=None):
    """The complete REST contract of POST /api/v1/payments on v1.53.0.

    There is no ``external_id`` and no ``status`` field: the service always
    records ``succeeded`` (Premium-gated on Community). This difference from
    the ticket draft is part of the evidence.
    """
    return {
        "payment": {
            "invoice_id": invoice_id,
            "amount_cents": amount_cents,
            "reference": reference,
            "paid_at": paid_at or _now_iso(),
        }
    }


ADD_STRIPE_PROVIDER_QUERY = (
    "mutation AddStripePaymentProvider($input: AddStripePaymentProviderInput!) {"
    " addStripePaymentProvider(input: $input) { id code name } }"
)


def add_stripe_provider_variables(code, name, secret_key):
    return {"input": {"code": code, "name": name, "secretKey": secret_key}}
