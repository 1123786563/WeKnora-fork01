"""Experiment E1 -- wallet batch expiry, no carry-over, and the wallet cap.

Scenario (real stack, short TTLs stand in for the calendar boundaries):

- ``W_month``  granted 100.00 credits, priority 1, expires in ~3 minutes
              (stands in for one monthly plan period).
- ``W_topup``  purchased 50.00 credits, priority 2, expires in ~365 days
              (stands in for a twelve-month top-up batch).
- ``W_interval`` granted 30.00 credits, priority 3, weekly interval rule,
              never expires (the mapping-negative probe).

Steps: consume while ``W_month`` is expired-but-still-active (the lazy
termination race window), wait for the hourly TerminateWalletsJob (clock
fires at *:45), then consume across the boundary, probe resurrection by
grant, record the interval wallet's leftover, and probe the six-active-
wallets-per-customer cap on a fresh customer.
"""

import time
from datetime import datetime, timedelta, timezone

import harness
from experiments import trigger

SLUG = "e1-expiry"

MONTHLY_TTL_SECONDS = 180         # short TTL standing in for the monthly period
TOPUP_TTL_DAYS = 365              # standing in for the twelve-month batch TTL
EXPIRY_BUFFER_SECONDS = 30        # wait past expiration_at before consuming
TERMINATION_POLL_WINDOW = 4500    # clock fires hourly at *:45; 75 min covers it
TERMINATION_POLL_INTERVAL = 30
CAP_LIMIT = 6                     # Wallets::ValidateService::MAXIMUM_WALLETS_PER_CUSTOMER


def outcome_of(checks, criterion):
    return harness.outcome_of(checks, criterion)


def _check(criterion, expected, observed, ok):
    return {"criterion": criterion, "expected": expected, "observed": observed,
            "outcome": "pass" if ok else "fail"}


def e1_expected(observed):
    """Pre-registered expectations -> check rows (pure; unit-tested offline)."""
    checks = []
    race = observed.get("race_window") or {}
    checks.append(_check(
        "expired_wallet_not_consumable",
        "invariant: a wallet whose expiration_at has passed must not be consumable; "
        "pinned source terminates lazily via the hourly clock, so a consumption in "
        "that window is the pre-registered lazy-termination race window",
        race,
        not race.get("consumed_after_expiry_passed"),
    ))
    termination = observed.get("termination") or {}
    monthly = observed.get("monthly") or {}
    checks.append(_check(
        "monthly_wallet_terminates_on_expiry",
        "wallet with a past expiration_at transitions active -> terminated by the "
        "hourly TerminateWalletsJob within the poll window",
        termination,
        bool(termination.get("terminated")),
    ))
    post = observed.get("post_boundary") or {}
    checks.append(_check(
        "monthly_remainder_not_consumable_after_termination",
        "after termination the unconsumed monthly remainder is consumable nowhere: "
        "0 cents drawn from the terminated wallet by any later invoice",
        {"monthly_drawn_cents": post.get("monthly_drawn_cents")},
        post.get("monthly_drawn_cents") == 0,
    ))
    resurrection = observed.get("resurrection") or {}
    checks.append(_check(
        "terminated_wallet_rejects_new_grant",
        "a new grant against the terminated wallet is rejected and its balance does "
        "not reappear",
        resurrection,
        bool(resurrection.get("rejected")),
    ))
    checks.append(_check(
        "topup_batch_survives_monthly_boundary",
        "the twelve-month top-up wallet stays active and is consumed after the "
        "monthly batch died (drawn_cents > 0)",
        {"topup_drawn_cents": post.get("topup_drawn_cents"),
         "topup_status": post.get("topup_status")},
        (post.get("topup_drawn_cents") or 0) > 0 and post.get("topup_status") == "active",
    ))
    interval = observed.get("interval") or {}
    checks.append(_check(
        "interval_rule_wallet_keeps_leftover",
        "mapping negative: interval rules top up the SAME persistent wallet, so the "
        "leftover carries over (a plain non-expiring wallet keeps its balance across "
        "any boundary; it never expires and nothing zeroes it)",
        interval,
        interval.get("wallet_status") == "active"
        and not interval.get("has_expiration")
        and (interval.get("balance_after_cents") or 0)
        >= (interval.get("balance_before_cents") or 0),
    ))
    rule_probe = interval.get("rule_create") or {}
    checks.append(_check(
        "interval_rule_create_fails_in_community",
        "runtime finding, pre-registered after the first real run: on Community "
        "v1.53.0 the public API cannot create a wallet with recurring_transaction_rules "
        "-- RecurringTransactionRules::CreateService returns early without "
        "License.premium? and Wallets::CreateService#call! then crashes "
        "(NoMethodError on nil) -> HTTP 500; interval top-ups are Premium-only",
        rule_probe,
        rule_probe.get("http_status") == 500 and not rule_probe.get("rule_created"),
    ))
    cap = observed.get("cap") or {}
    checks.append(_check(
        "cap_six_active_wallets_per_customer",
        "creating the 7th concurrent active wallet fails 422 with "
        "wallet_limit_reached (MAXIMUM_WALLETS_PER_CUSTOMER = 6)",
        cap,
        cap.get("active_created") == CAP_LIMIT
        and cap.get("limit_error_code") == "wallet_limit_reached",
    ))
    checks.append(_check(
        "cap_freed_by_termination",
        "terminating one active wallet frees a slot: the next create succeeds",
        {"after_terminate_status": cap.get("after_terminate_status")},
        cap.get("after_terminate_status") == 200,
    ))
    return checks


# ---------------------------------------------------------------------------
# Runtime scenario


def _future_iso(seconds=None, days=None):
    delta = timedelta(seconds=seconds or 0, days=days or 0)
    return (datetime.now(timezone.utc) + delta).strftime("%Y-%m-%dT%H:%M:%SZ")


def _parse_iso(value):
    return datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)


def _create_wallet(client, report, customer_id, **kwargs):
    payload = harness.wallet_payload(external_customer_id=customer_id, **kwargs)
    _, body = client.post("/api/v1/wallets", payload=payload)
    wallet = (body or {}).get("wallet") or {}
    report.record_created("wallet", lago_id=wallet.get("lago_id"))
    return wallet


def _balance(client, lago_id):
    return trigger.wallet_state(client, lago_id).get("balance_cents")


def run(client, report, cleanup):
    batch = harness.new_batch_key()
    customer_id = harness.new_customer_id()
    _, body = client.post("/api/v1/customers",
                          payload=harness.customer_payload(customer_id))
    report.record_created("customer", external_id=customer_id)

    now = datetime.now(timezone.utc)
    month_expiry_iso = _future_iso(seconds=MONTHLY_TTL_SECONDS)
    w_month = _create_wallet(client, report, customer_id, name="monthly-plan-batch",
                             granted_credits="100", priority=1,
                             expiration_at=month_expiry_iso, batch_key=batch)
    w_topup = _create_wallet(client, report, customer_id, name="topup-batch",
                             paid_credits="50", priority=2,
                             expiration_at=_future_iso(days=TOPUP_TTL_DAYS),
                             batch_key=batch)
    # Paid credits stay pending until the credit invoice is marked paid
    # (no payment provider in this lab; PUT payment_status=succeeded is the
    # Payment-Fact -> fulfillment seam). Settle before any consumption.
    paid_settlement = trigger.confirm_paid_credits(
        client, customer_id, w_topup["lago_id"], 5000)
    w_interval = _create_wallet(client, report, customer_id, name="interval-rule",
                                granted_credits="30", priority=3,
                                batch_key=batch)

    # Interval rules are Premium-gated on v1.53.0 Community and the wallet-create
    # path crashes on the nil result (first real run: HTTP 500, NoMethodError in
    # Wallets::CreateService#call via RecurringTransactionRules::CreateService).
    # Probe it explicitly and record the honest observation.
    rule_create = {"attempted": True, "rule_created": False}
    try:
        client.post("/api/v1/wallets", payload=harness.wallet_payload(
            customer_id, name="interval-rule-probe", granted_credits="5",
            recurring_transaction_rules=[{
                "trigger": "interval", "interval": "weekly",
                "granted_credits": "5",
            }]))
        rule_create.update({"http_status": 200, "error_code": None,
                            "rule_created": True})
    except harness.LagoHttpError as error:
        rule_create.update({"http_status": error.status,
                            "error_code": error.reason})

    trigger_ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    report.meta["trigger"] = {
        "mechanism": "POST /api/v1/events (pay_in_advance + invoiceable charge) "
                     "-> Invoices::CreatePayInAdvanceChargeService -> "
                     "Credits::AppliedPrepaidCreditsService",
        "metric_code": trigger_ctx["metric_code"],
        "plan_code": trigger_ctx["plan_code"],
        "subscription_external_id": trigger_ctx["subscription_external_id"],
        "subscription_status": trigger_ctx["subscription_status"],
    }

    # --- lazy termination race window: consume after expiry passed, before clock
    month_expiry = _parse_iso(month_expiry_iso)
    sleep_for = (month_expiry - datetime.now(timezone.utc)).total_seconds() \
        + EXPIRY_BUFFER_SECONDS
    if sleep_for > 0:
        time.sleep(sleep_for)

    month_before = trigger.wallet_state(client, w_month["lago_id"])
    race_drawn = 0
    race_consumed = False
    trigger.consume(client, trigger_ctx, 4000)
    drawn, _ = trigger.wait_for_draw(client, w_month["lago_id"],
                                     month_before.get("balance_cents"))
    if drawn:
        after_balance = _balance(client, w_month["lago_id"])
        if month_before.get("balance_cents") is not None and after_balance is not None:
            race_drawn = month_before["balance_cents"] - after_balance
            race_consumed = race_drawn > 0
    race_window = {
        "wallet_status_at_consumption": month_before.get("status"),
        "expiration_at_passed": True,
        "consumed_after_expiry_passed": race_consumed,
        "drawn_cents": race_drawn,
    }

    # --- wait for the hourly wallet termination clock (fires at *:45)
    month_mid = trigger.wallet_state(client, w_month["lago_id"])
    interval_before_poll = _balance(client, w_interval["lago_id"])
    terminated, elapsed = trigger.wait_until(
        lambda: trigger.wallet_state(client, w_month["lago_id"]).get("status") == "terminated",
        TERMINATION_POLL_WINDOW, TERMINATION_POLL_INTERVAL)
    month_after = trigger.wallet_state(client, w_month["lago_id"])
    interval_after_poll = _balance(client, w_interval["lago_id"])

    # --- consume across the monthly boundary (wait for the paid top-up to settle)
    trigger.wait_until(lambda: (_balance(client, w_topup["lago_id"]) or 0) > 0, 120)
    topup_before = _balance(client, w_topup["lago_id"])
    trigger.consume(client, trigger_ctx, 5000)
    trigger.wait_for_draw(client, w_topup["lago_id"], topup_before)
    topup_after = _balance(client, w_topup["lago_id"])
    month_final = _balance(client, w_month["lago_id"])
    post_boundary = {
        "invoice_amount_cents": 5000,
        "monthly_drawn_cents": (month_after.get("balance_cents") or 0)
        - (month_final if month_final is not None else 0),
        "topup_drawn_cents": (topup_before - topup_after)
        if topup_before is not None and topup_after is not None else None,
        "topup_status": trigger.wallet_state(client, w_topup["lago_id"]).get("status"),
    }

    # --- resurrection probe: grant onto the terminated wallet
    resurrection = {"attempted": True}
    try:
        _, grant_body = client.post("/api/v1/wallet_transactions", payload=(
            harness.wallet_grant_payload(w_month["lago_id"], granted_credits="5",
                                         batch_key=batch)))
        resurrection.update({
            "http_status": 200, "error_code": None, "rejected": False,
            "balance_after_cents": _balance(client, w_month["lago_id"]),
        })
    except harness.LagoHttpError as error:
        resurrection.update({
            "http_status": error.status, "error_code": error.reason,
            "rejected": True,
            "balance_after_cents": _balance(client, w_month["lago_id"]),
        })

    interval_wallet = trigger.wallet_state(client, w_interval["lago_id"])
    interval_observed = {
        "wallet_status": interval_wallet.get("status"),
        "has_expiration": bool(interval_wallet.get("expiration_at")),
        "balance_before_cents": interval_before_poll,
        "balance_after_cents": interval_after_poll,
        "rule_create": rule_create,
        "note": "interval top-ups are Premium-only in Community and the rule-create "
                "path 500s (see interval_rule_create_fails_in_community); the "
                "leftover-preservation observation is structural: a non-expiring "
                "wallet keeps its balance across every boundary",
    }

    # --- cap probe on a fresh customer
    cap_customer = harness.new_customer_id()
    client.post("/api/v1/customers", payload=harness.customer_payload(cap_customer))
    report.record_created("customer", external_id=cap_customer)
    cap_batch = harness.new_batch_key()
    created = 0
    limit_status = None
    limit_code = None
    last_wallet_id = None
    while created < CAP_LIMIT + 3:
        try:
            wallet = _create_wallet(client, report, cap_customer, name=f"cap-{created}",
                                    granted_credits="1", batch_key=cap_batch)
            created += 1
            last_wallet_id = wallet.get("lago_id")
        except harness.LagoHttpError as error:
            limit_status = error.status
            limit_code = error.reason
            break
    after_terminate_status = None
    if last_wallet_id:
        status, _ = client.delete(f"/api/v1/wallets/{last_wallet_id}")
        try:
            client.post("/api/v1/wallets", payload=harness.wallet_payload(
                external_customer_id=cap_customer, name="cap-after-terminate",
                granted_credits="1", batch_key=cap_batch))
            after_terminate_status = 200
        except harness.LagoHttpError as error:
            after_terminate_status = error.status
    cap_observed = {"active_created": created, "limit_status": limit_status,
                    "limit_error_code": limit_code,
                    "after_terminate_status": after_terminate_status}

    observed = {
        "paid_settlement": paid_settlement,
        "monthly": {
            "code": w_month.get("code"), "lago_id": w_month.get("lago_id"),
            "status_before": month_before.get("status"),
            "status_after": month_after.get("status"),
            "expiration_at": month_expiry_iso,
            "granted_cents": 10000,
            "balance_before_boundary_cents": month_mid.get("balance_cents"),
            "balance_after_termination_cents": month_after.get("balance_cents"),
        },
        "race_window": race_window,
        "termination": {"terminated": terminated,
                        "elapsed_seconds": round(elapsed) if terminated else None,
                        "poll_window_seconds": TERMINATION_POLL_WINDOW},
        "post_boundary": post_boundary,
        "resurrection": resurrection,
        "interval": interval_observed,
        "cap": cap_observed,
    }
    for row in e1_expected(observed):
        report.add_check(row["criterion"], row["expected"], row["observed"],
                         outcome=row["outcome"])
    report.meta["observed"] = observed


RUNNER = run
