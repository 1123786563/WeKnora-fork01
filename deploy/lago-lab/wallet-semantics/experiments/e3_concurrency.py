"""Experiment E3 -- concurrency and retry semantics.

Real parallel HTTP from stdlib threads; determinism comes from asserting
totals and guards, never interleavings. Sub-scenarios, each on its own
synthetic customer:

- duplicate grant: the same POST /api/v1/wallet_transactions payload twice
  (pre-registered: no API idempotency -> two settled inbounds, doubled
  balance, metadata-keyed lookup recovers both) contrasted with events,
  which ARE idempotent by transaction_id.
- concurrent consumption: 6 threads fire one 50.00 consumption event each
  against a single 100.00 batch (300.00 demand vs 100.00 balance); the
  customer lock plus the DB-guarded remainder decrement must keep the total
  draw within the grant and the remainder non-negative.
- void-vs-consume race: a whole-remaining void races a finalizing
  consumption; whichever wins, drawn + voided must never exceed the grant.
- expired-during-run: draw before and after a short-TTL expiry passes;
  pre + post draws must stay within the grant (no double-draw of the same
  remainder across the expiry boundary).
"""

import threading
import time
import uuid

import harness
from experiments import trigger

SLUG = "e3-concurrency"

CONCURRENT_THREADS = 6
CONCURRENT_EVENT_CENTS = 5000
CONCURRENT_GRANTED_CENTS = 10000
EXPIRY_TTL_SECONDS = 120
EXPIRY_SETTLE_SECONDS = 30
EXPIRED_STATUS_WINDOW = 90


def outcome_of(checks, criterion):
    return harness.outcome_of(checks, criterion)


def _check(criterion, expected, observed, ok):
    return {"criterion": criterion, "expected": expected, "observed": observed,
            "outcome": "pass" if ok else "fail"}


def e3_expected(observed):
    """Pre-registered expectations -> check rows (pure; unit-tested offline)."""
    checks = []
    dup = observed.get("duplicate_grant") or {}
    granted_each = dup.get("granted_each_cents") or 0
    checks.append(_check(
        "duplicate_grant_mints_second_batch",
        "no external idempotency on wallet transactions: the identical POST "
        "replayed creates a SECOND settled inbound and doubles the balance "
        "(the coordination layer must own grant idempotency)",
        dup,
        dup.get("settled_inbounds") == 2
        and dup.get("balance_cents") == 2 * granted_each,
    ))
    checks.append(_check(
        "metadata_lookup_recovers_duplicates",
        "the transaction index filtered by the metadata batch key returns both "
        "duplicates (the documented recovery path for replayed grants)",
        {"metadata_lookup_count": dup.get("metadata_lookup_count")},
        dup.get("metadata_lookup_count") == 2,
    ))
    event = observed.get("event_idempotency") or {}
    checks.append(_check(
        "events_idempotent_by_transaction_id",
        "unlike wallet transactions, events are idempotent by transaction_id: "
        "the identical event POST replayed is rejected without creating a "
        "second event (the pattern the coordination layer must emulate for "
        "grants)",
        event,
        (event.get("second_status") or 0) >= 400
        and not event.get("second_created_event"),
    ))
    concurrent = observed.get("concurrent_consumption") or {}
    granted = concurrent.get("granted_cents") or 0
    checks.append(_check(
        "concurrent_consumption_within_balance",
        "6 parallel finalizations against one batch draw at most the granted "
        "amount in total (Customers::LockService serializes, the remainder "
        "decrement is DB-guarded); the remainder never goes negative",
        concurrent,
        concurrent.get("events_accepted") == concurrent.get("threads")
        and (concurrent.get("total_drawn_cents") or 0) <= granted
        and (concurrent.get("final_balance_cents") or 0) >= 0
        and (concurrent.get("min_remaining_cents") or 0) >= 0,
    ))
    race = observed.get("void_vs_consume") or {}
    race_granted = race.get("granted_cents") or 0
    taken = (race.get("drawn_cents") or 0) + (race.get("voided_cents") or 0)
    checks.append(_check(
        "void_vs_consume_exclusive",
        "a whole-remaining void racing a finalization cannot both succeed "
        "against the same remainder: drawn + voided <= granted and the "
        "balance never goes negative",
        race,
        taken <= race_granted and bool(race.get("balance_never_negative")),
    ))
    expired = observed.get("expired_during") or {}
    exp_granted = expired.get("granted_cents") or 0
    checks.append(_check(
        "expired_batch_never_double_drawn",
        "a batch whose expiration_at passes mid-run may be skipped or lazily "
        "terminated, but the same remainder is never drawn twice: pre-expiry "
        "draw + post-expiry draw <= granted, remainder >= 0",
        expired,
        (expired.get("total_drawn_cents") or 0) <= exp_granted
        and (expired.get("remaining_cents") or 0) >= 0,
    ))
    return checks


# ---------------------------------------------------------------------------
# Runtime scenario


def _new_customer_with_wallet(client, report, **wallet_kwargs):
    customer_id = harness.new_customer_id()
    client.post("/api/v1/customers", payload=harness.customer_payload(customer_id))
    report.record_created("customer", external_id=customer_id)
    _, body = client.post("/api/v1/wallets", payload=harness.wallet_payload(
        customer_id, **wallet_kwargs))
    wallet = (body or {}).get("wallet") or {}
    report.record_created("wallet", lago_id=wallet.get("lago_id"))
    return customer_id, wallet


def _balance(client, wallet_id):
    return trigger.wallet_state(client, wallet_id).get("balance_cents")


def _inbound_transactions(client, wallet_id, batch_key=None):
    path = f"/api/v1/wallets/{wallet_id}/wallet_transactions?per_page=100"
    if batch_key:
        path += f"&metadata%5Bweknora_t03_batch%5D={batch_key}"
    return [row for row in trigger.parse_transactions_response(
        _get(client, path)) if row["transaction_type"] == "inbound"]


def _get(client, path):
    _, body = client.get(path)
    return body or {}


def run(client, report, cleanup):
    observed = {}

    # --- duplicate grant + event idempotency contrast ----------------------
    batch = harness.new_batch_key()
    customer_id, wallet = _new_customer_with_wallet(
        client, report, name="dup-grant", granted_credits="0")
    payload = harness.wallet_grant_payload(wallet["lago_id"], granted_credits="50",
                                           batch_key=batch)
    client.post("/api/v1/wallet_transactions", payload=payload)
    client.post("/api/v1/wallet_transactions", payload=payload)  # identical replay
    trigger.wait_until(
        lambda: (_balance(client, wallet["lago_id"]) or 0) >= 10000, 60)
    dup_inbounds = _inbound_transactions(client, wallet["lago_id"], batch_key=batch)
    observed["duplicate_grant"] = {
        "payload_replays": 2,
        "settled_inbounds": sum(1 for row in dup_inbounds
                                if row["status"] == "settled"),
        "granted_each_cents": 5000,
        "balance_cents": _balance(client, wallet["lago_id"]),
        "metadata_lookup_count": len(dup_inbounds),
    }

    # event idempotency contrast
    trigger_ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    event_payload = trigger.event_payload(
        f"weknora-t03-evt-{uuid.uuid4()}", customer_id,
        trigger_ctx["metric_code"], 100,
        external_subscription_id=trigger_ctx["subscription_external_id"])
    first_status, _ = client.post("/api/v1/events", payload=event_payload)
    second_status, second_error = 200, None
    try:
        client.post("/api/v1/events", payload=event_payload)
    except harness.LagoHttpError as error:
        second_status, second_error = error.status, error.reason
    observed["event_idempotency"] = {
        "first_status": first_status,
        "second_status": second_status,
        "second_error_code": second_error,
        "second_created_event": second_status < 400,
    }

    # --- concurrent consumption -------------------------------------------
    customer_id, wallet = _new_customer_with_wallet(
        client, report, name="concurrent", granted_credits="100")
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    balance_before = _balance(client, wallet["lago_id"])
    accepted = []
    errors = []

    def fire(_thread_index):
        local = harness.LagoClient(client.base_url, client.api_key,
                                   timeout=client.timeout)
        try:
            status, _ = local.post("/api/v1/events", payload=trigger.event_payload(
                f"weknora-t03-evt-{uuid.uuid4()}", customer_id,
                ctx["metric_code"], CONCURRENT_EVENT_CENTS,
                external_subscription_id=ctx["subscription_external_id"]))
            accepted.append(status)
        except Exception as error:  # noqa: BLE001 - recorded as evidence
            errors.append(harness.sanitize_text(error, (client.api_key,)))

    threads = [threading.Thread(target=fire, args=(index,))
               for index in range(CONCURRENT_THREADS)]
    for thread in threads:
        thread.start()
    for thread in threads:
        thread.join()
    trigger.wait_until(
        lambda: _balance(client, wallet["lago_id"]) == 0, 180, interval_seconds=5)
    rows = trigger.wallet_transactions(client, wallet["lago_id"])
    outbound_cents = sum(abs(row["amount_cents"] or 0) for row in rows
                         if row["transaction_type"] == "outbound")
    min_remaining = min([row["remaining_amount_cents"] for row in rows
                         if row["remaining_amount_cents"] is not None] or [None])
    observed["concurrent_consumption"] = {
        "threads": CONCURRENT_THREADS,
        "event_cents_each": CONCURRENT_EVENT_CENTS,
        "granted_cents": CONCURRENT_GRANTED_CENTS,
        "events_accepted": len(accepted),
        "accept_errors": errors,
        "total_drawn_cents": outbound_cents,
        "final_balance_cents": _balance(client, wallet["lago_id"]),
        "min_remaining_cents": min_remaining,
    }

    # --- void-vs-consume race ----------------------------------------------
    customer_id, wallet = _new_customer_with_wallet(
        client, report, name="void-race", granted_credits="100")
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    inbound = _inbound_transactions(client, wallet["lago_id"])[0]
    race_outcomes = {"drawn": 0, "voided": 0, "balance_min": None}

    def consume_thread():
        local = harness.LagoClient(client.base_url, client.api_key,
                                   timeout=client.timeout)
        local.post("/api/v1/events", payload=trigger.event_payload(
            f"weknora-t03-evt-{uuid.uuid4()}", customer_id,
            ctx["metric_code"], 10000,
            external_subscription_id=ctx["subscription_external_id"]))

    def void_thread():
        local = harness.LagoClient(client.base_url, client.api_key,
                                   timeout=client.timeout)
        try:
            local.post("/api/v1/wallet_transactions", payload={
                "wallet_transaction": {
                    "wallet_id": wallet["lago_id"],
                    "voided_transaction_id": inbound["lago_id"],
                }})
        except harness.LagoHttpError:
            pass  # the loser of the race is rejected; recorded via balances

    barrier = threading.Barrier(2)

    def guarded(thread_fn):
        def wrapper():
            barrier.wait()
            thread_fn()
        return wrapper

    threads = [threading.Thread(target=guarded(consume_thread)),
               threading.Thread(target=guarded(void_thread))]
    balances_seen = []
    for thread in threads:
        thread.start()
    deadline = time.monotonic() + 180
    while time.monotonic() < deadline:
        balances_seen.append(_balance(client, wallet["lago_id"]))
        rows = trigger.wallet_transactions(client, wallet["lago_id"])
        if sum(abs(r["amount_cents"] or 0) for r in rows
               if r["transaction_type"] == "outbound") > 0 \
                and _balance(client, wallet["lago_id"]) == 0:
            time.sleep(5)  # settle
            break
        time.sleep(2)
    for thread in threads:
        thread.join()
    rows = trigger.wallet_transactions(client, wallet["lago_id"])
    drawn = sum(abs(r["amount_cents"] or 0) for r in rows
                if r["transaction_type"] == "outbound"
                and r["transaction_status"] == "invoiced")
    voided = sum(abs(r["amount_cents"] or 0) for r in rows
                 if r["transaction_type"] == "outbound"
                 and r["transaction_status"] == "voided")
    final_balance = _balance(client, wallet["lago_id"])
    observed["void_vs_consume"] = {
        "granted_cents": 10000,
        "drawn_cents": drawn,
        "voided_cents": voided,
        "final_balance_cents": final_balance,
        "balance_never_negative": all(
            value is None or value >= 0 for value in balances_seen),
    }

    # --- expiry during the run ---------------------------------------------
    from datetime import datetime, timedelta, timezone
    expiry_iso = (datetime.now(timezone.utc)
                  + timedelta(seconds=EXPIRY_TTL_SECONDS)).strftime(
                      "%Y-%m-%dT%H:%M:%SZ")
    customer_id, wallet = _new_customer_with_wallet(
        client, report, name="expired-mid-run", granted_credits="100",
        expiration_at=expiry_iso)
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    wallet_id = wallet["lago_id"]

    balance_before = _balance(client, wallet_id)
    trigger.consume(client, ctx, 4000)
    trigger.wait_for_draw(client, wallet_id, balance_before)
    drawn_before = balance_before - _balance(client, wallet_id)

    time.sleep(EXPIRY_TTL_SECONDS + EXPIRY_SETTLE_SECONDS)  # expiry passes mid-run
    balance_mid = _balance(client, wallet_id)
    trigger.consume(client, ctx, 4000)
    drawn, _ = trigger.wait_for_draw(client, wallet_id, balance_mid)
    drawn_after = (balance_mid - _balance(client, wallet_id)) if drawn else 0

    rows = trigger.wallet_transactions(client, wallet_id)
    remainings = [row["remaining_amount_cents"] for row in rows
                  if row["remaining_amount_cents"] is not None]
    final_state = trigger.wallet_state(client, wallet_id)
    satisfied, _ = trigger.wait_until(
        lambda: trigger.wallet_state(client, wallet_id).get("status")
        == "terminated", EXPIRED_STATUS_WINDOW)
    observed["expired_during"] = {
        "granted_cents": 10000,
        "drawn_before_expiry_cents": drawn_before,
        "drawn_after_expiry_cents": drawn_after,
        "total_drawn_cents": drawn_before + drawn_after,
        "remaining_cents": min(remainings) if remainings else None,
        "wallet_status_at_end": final_state.get("status"),
        "terminated_within_short_window": satisfied,
        "note": "termination cadence is measured by E1 (hourly clock at *:45); "
                "E3 only asserts the no-double-draw guard",
    }

    for row in e3_expected(observed):
        report.add_check(row["criterion"], row["expected"], row["observed"],
                         outcome=row["outcome"])
    report.meta["observed"] = observed


RUNNER = run
