"""Shared deterministic consumption trigger and response parsing.

Consumption trigger (confirmed against the pinned lago-api source at
591ae9005110346f1c6034ec72ea9046625668cf before relying on it, per the plan):

    POST /api/v1/billable_metrics   (sum_agg over an "units" field)
    POST /api/v1/plans              (weekly plan, one charge:
                                     pay_in_advance + invoiceable + standard
                                     model at 1.00 per unit)
    POST /api/v1/subscriptions      (customer subscribes, active immediately)
    POST /api/v1/events             (properties.units = dollars)
      -> Events::PayInAdvanceService
      -> Invoices::CreatePayInAdvanceChargeJob (invoiceable: true)
      -> Invoices::CreatePayInAdvanceChargeService
      -> Credits::AppliedPrepaidCreditsService.call!   (wallets drawn, in
         in_application_order, under Customers::LockService, idempotent per
         invoice via wallets_already_applied?)

A fee of N units x 1.00 = N*100 cents draws N*100 cents of wallet credits
(rate_amount "1" -> 1 credit per cent). The invoice is created asynchronously
by the api-worker, so the runtime callers poll wallet state after each event.

All money crosses the harness as integer cents; credits cross the API as
decimal strings. No binary floating point anywhere.
"""

import time
import uuid
from decimal import Decimal, InvalidOperation

import harness

PER_UNIT_AMOUNT = "1.00"  # standard model: 1 unit of "units" = 1.00 = 100 cents


# ---------------------------------------------------------------------------
# Money conversions (integer cents <-> decimal credit strings)


def credits_to_cents(value):
    if value is None:
        return None
    try:
        amount = Decimal(str(value))
    except InvalidOperation:
        raise ValueError(f"not a decimal credits value: {value!r}") from None
    cents = amount * 100
    if cents != cents.to_integral_value():
        raise ValueError(f"sub-cent credits precision not supported: {value!r}")
    return int(cents)


def cents_to_credits(cents):
    if cents != int(cents):
        raise ValueError("cents must be an integer")
    amount = Decimal(int(cents)) / Decimal(100)
    text = format(amount.normalize(), "f")
    return text


def event_units_for_cents(cents):
    """Whole-dollar fees only: units x 1.00 = cents, so units = cents // 100."""
    if cents % 100 != 0:
        raise ValueError(f"consumption must be whole dollars (cents % 100 == 0): {cents}")
    return cents // 100


# ---------------------------------------------------------------------------
# Payload builders


def metric_payload(code, name):
    return {
        "billable_metric": {
            "code": code,
            "name": name,
            "description": "WeKnora T03 lab consumption metric",
            "aggregation_type": "sum_agg",
            "field_name": "units",
        }
    }


def plan_payload(code, name, metric_lago_id, amount_cents=100):
    """Plan with one pay-in-advance standard charge over the metric.

    Runtime findings recorded while confirming this trigger on v1.53.0:
    - the plan-level ``pay_in_advance`` boolean is REQUIRED (Plan validates
      inclusion in [true, false]; nil is rejected with value_is_invalid);
    - a charge embedded in the plan create payload must reference the metric
      by ``billable_metric_id`` (its lago_id) -- ``billable_metric_code`` is
      not resolved on this path (Charges::CreateService looks up by id only);
    - ``invoiceable`` is Premium-gated in Charges::CreateService; Community
      keeps the DB default (true), which is what this trigger needs.
    """
    return {
        "plan": {
            "code": code,
            "name": name,
            "interval": "weekly",
            "pay_in_advance": False,
            "amount_cents": amount_cents,
            "amount_currency": "USD",
            "charges": [{
                "billable_metric_id": metric_lago_id,
                "charge_model": "standard",
                "pay_in_advance": True,
                "invoiceable": True,
                "properties": {"amount": PER_UNIT_AMOUNT},
            }],
        }
    }


def subscription_payload(external_customer_id, plan_code):
    # v1.53.0 requires the subscription's own external_id (value_is_mandatory).
    return {
        "subscription": {
            "external_id": f"weknora-t03-sub-{uuid.uuid4()}",
            "external_customer_id": external_customer_id,
            "plan_code": plan_code,
        }
    }


def event_payload(transaction_id, external_customer_id, metric_code, cents,
                  external_subscription_id=None):
    # Without external_subscription_id the pay-in-advance service cannot
    # resolve event.subscription (-> no charge, no invoice, no wallet draw),
    # so the trigger always carries it.
    event = {
        "transaction_id": transaction_id,
        "external_customer_id": external_customer_id,
        "code": metric_code,
        "properties": {"units": event_units_for_cents(cents)},
    }
    if external_subscription_id:
        event["external_subscription_id"] = external_subscription_id
    return {"event": event}


# ---------------------------------------------------------------------------
# Response parsers (integer cents everywhere)


def parse_wallet_response(wallet):
    if not isinstance(wallet, dict):
        return {"lago_id": None, "balance_cents": None}
    balance_cents = wallet.get("balance_cents")
    if balance_cents is None:
        # Older payloads expose only credits strings.
        balance_cents = credits_to_cents(
            wallet.get("credits_balance", wallet.get("balance")))
    return {
        "lago_id": wallet.get("lago_id"),
        "code": wallet.get("code"),
        "status": wallet.get("status"),
        "priority": wallet.get("priority"),
        "balance_cents": balance_cents,
        "expiration_at": wallet.get("expiration_at"),
        "granted_credits": wallet.get("granted_credits"),
        "consumed_credits": wallet.get("consumed_credits"),
    }


def parse_transactions_response(body):
    rows = []
    for item in (body or {}).get("wallet_transactions") or []:
        rows.append({
            "lago_id": item.get("lago_id"),
            "wallet_id": item.get("lago_wallet_id"),
            "invoice_id": item.get("lago_invoice_id"),
            "transaction_type": item.get("transaction_type"),
            "transaction_status": item.get("transaction_status"),
            "status": item.get("status"),
            "source": item.get("source"),
            "amount_cents": credits_to_cents(item.get("amount")),
            "remaining_amount_cents": item.get("remaining_amount_cents"),
            "created_at": item.get("created_at"),
            "metadata": item.get("metadata"),
        })
    return rows


def drawn_from_wallet(before_balance_cents, after_balance_cents):
    """Positive cents drawn from one wallet between two balance reads."""
    if before_balance_cents is None or after_balance_cents is None:
        return None
    return before_balance_cents - after_balance_cents


# ---------------------------------------------------------------------------
# Runtime helpers (thin wrappers around LagoClient; the interesting logic is
# in the pure functions above)


def new_metric_code():
    return f"weknora-t03-metric-{uuid.uuid4()}"


def new_plan_code():
    return f"weknora-t03-plan-{uuid.uuid4()}"


def setup_consumption_trigger(client, report, external_customer_id):
    """Create metric + plan + subscription; return the trigger context."""
    metric_code = new_metric_code()
    plan_code = new_plan_code()
    _, metric_body = client.post("/api/v1/billable_metrics",
                                 payload=metric_payload(metric_code, "T03 consume"))
    metric_lago_id = ((metric_body or {}).get("billable_metric") or {}).get("lago_id")
    client.post("/api/v1/plans",
                payload=plan_payload(plan_code, "T03 lab plan", metric_lago_id))
    _, sub_body = client.post("/api/v1/subscriptions",
                              payload=subscription_payload(external_customer_id, plan_code))
    subscription = ((sub_body or {}).get("subscription") or {})
    return {"metric_code": metric_code, "metric_lago_id": metric_lago_id,
            "plan_code": plan_code,
            "subscription_external_id": subscription.get("external_id"),
            "subscription_status": subscription.get("status"),
            "external_customer_id": external_customer_id}


def consume(client, trigger_ctx, cents, note=""):
    """Fire one consumption event worth `cents`; returns the event transaction id."""
    transaction_id = f"weknora-t03-evt-{uuid.uuid4()}"
    client.post("/api/v1/events", payload=event_payload(
        transaction_id, trigger_ctx["external_customer_id"],
        trigger_ctx["metric_code"], cents,
        external_subscription_id=trigger_ctx.get("subscription_external_id")))
    return transaction_id


def wallet_state(client, wallet_lago_id):
    _, body = client.get(f"/api/v1/wallets/{wallet_lago_id}")
    return parse_wallet_response((body or {}).get("wallet") or {})


def wallet_transactions(client, wallet_lago_id, per_page=100):
    _, body = client.get(
        f"/api/v1/wallets/{wallet_lago_id}/wallet_transactions?per_page={per_page}")
    return parse_transactions_response(body)


def wait_until(predicate, timeout_seconds, interval_seconds=3.0):
    """Poll until predicate() is truthy; return (satisfied, elapsed_seconds)."""
    started = time.monotonic()
    while True:
        if predicate():
            return True, time.monotonic() - started
        if time.monotonic() - started >= timeout_seconds:
            return False, time.monotonic() - started
        time.sleep(interval_seconds)


def wait_for_draw(client, wallet_lago_id, balance_before_cents, timeout_seconds=90):
    """Wait until the wallet balance drops below `balance_before_cents`."""
    def dropped():
        state = wallet_state(client, wallet_lago_id)
        balance = state.get("balance_cents")
        return balance is not None and balance < balance_before_cents
    return wait_until(dropped, timeout_seconds)


def confirm_paid_credits(client, external_customer_id, wallet_lago_id,
                         expected_balance_cents, timeout_seconds=120):
    """Mark the customer's pending paid-credit invoice as paid and wait for
    the purchased credits to settle.

    Runtime finding (first real run refuted the research claim that paid
    credits settle immediately in Community): with no payment provider,
    ``Invoices::Payments::CreateService`` skips the invoice entirely, the
    credit invoice stays ``payment_status: pending``, the purchased inbound
    stays ``pending`` and the wallet balance stays 0. Settlement happens only
    when the invoice is marked paid -- ``PUT /api/v1/invoices/:id`` with
    ``payment_status: "succeeded"`` -> ``Invoices::PrepaidCreditJob`` ->
    ``Wallets::ApplyPaidCreditsService`` (settle + balance increase). This is
    exactly the WeKnora Payment-Fact -> Lago-fulfillment seam from the spec.
    """
    # The credit invoice is created asynchronously (BillPaidCreditJob), so
    # poll for it before trying to mark it paid.
    invoice = None

    def invoice_visible():
        nonlocal invoice
        _, body = client.get(
            f"/api/v1/invoices?external_customer_id={external_customer_id}"
            "&per_page=20")
        for row in (body or {}).get("invoices") or []:
            if row.get("invoice_type") == "credit" \
                    and row.get("payment_status") != "succeeded":
                invoice = row
                return True
        return False

    wait_until(invoice_visible, 60)
    result = {"invoice_id": (invoice or {}).get("lago_id"),
              "payment_status_before": (invoice or {}).get("payment_status"),
              "confirm_http_status": None}
    if invoice is None:
        result["already_succeeded"] = True
    else:
        status, _ = client.put(f"/api/v1/invoices/{invoice['lago_id']}",
                               payload={"invoice": {"payment_status": "succeeded"}})
        result["confirm_http_status"] = status
    settled, elapsed = wait_until(
        lambda: (wallet_state(client, wallet_lago_id).get("balance_cents") or 0)
        >= expected_balance_cents, timeout_seconds)
    result.update({"settled": settled,
                   "balance_cents": wallet_state(client, wallet_lago_id).get("balance_cents"),
                   "settled_within_seconds": round(elapsed, 1)})
    return result
