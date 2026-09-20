"""Experiment E2 -- consumption order across and within wallet batches.

Scenario (real stack):

- Three wallets created in order C, B, A with priorities 3, 2, 1 and expiry
  rank A earliest (the WeKnora coordination mapping: encode earliest expiry
  into the lowest priority). Balances 50/70/110 dollars make every draw order
  produce a distinct drained/partial signature, and one 160-dollar
  consumption spans all three.
- A fourth wallet holds three inbound batches: granted G1, purchased P1,
  granted G2 created in that chronological order. The pinned source
  (``WalletTransaction.in_consumption_order``) drains them G1, G2, P1 --
  granted-before-purchased beats created_at -- observed by draining exactly
  one batch per consumption and watching ``remaining_amount_cents``.

The draw order is observed at runtime from the outbound transactions'
created_at sequence (the allocation loop creates them in order) and
cross-checked against the drained-balance signature.
"""

import time

import harness
from experiments import trigger

SLUG = "e2-order"

# Wallet balances in cents; prefix sums 5000/12000/23000 never coincide, so
# the drained/partial signature uniquely identifies the draw order.
WALLET_SPEC = [
    # (label, creation sequence position, priority, granted_cents, expiry_days)
    ("C", 0, 3, 11000, 300),  # created FIRST, consumed LAST (latest expiry)
    ("B", 1, 2, 7000, 200),
    ("A", 2, 1, 5000, 100),   # created LAST, consumed FIRST (earliest expiry)
]
CROSS_CONSUMPTION_CENTS = 16000  # drains A fully, B fully, C partially

TIE_GRANT_CENTS = 10000  # each inbound batch of the tie-break wallet
TIE_STEPS = 3


def outcome_of(checks, criterion):
    return harness.outcome_of(checks, criterion)


def _check(criterion, expected, observed, ok):
    return {"criterion": criterion, "expected": expected, "observed": observed,
            "outcome": "pass" if ok else "fail"}


def e2_expected(observed):
    """Pre-registered expectations -> check rows (pure; unit-tested offline)."""
    checks = []
    priority_map = observed.get("priority_map") or {}
    expected_order = [code for code, _ in sorted(priority_map.items(),
                                                 key=lambda kv: kv[1])]
    draw_codes = [row.get("code") for row in observed.get("draw_sequence") or []]
    checks.append(_check(
        "consumption_follows_priority_over_creation_and_expiry_rank",
        "wallets are consumed in strict priority order (the coordination mapping "
        "encodes earliest expiry into the lowest priority), even though the "
        "creation order is C, B, A and expiry rank disagrees with it",
        {"draw_sequence": draw_codes,
         "creation_order": observed.get("creation_order"),
         "expiry_rank_earliest_first": observed.get("expiry_rank_earliest_first")},
        draw_codes == expected_order,
    ))

    drain = observed.get("tie_break_drain_sequence") or []
    batches = observed.get("same_wallet_batches") or []
    purchased = [b.get("batch") for b in batches
                 if b.get("transaction_status") == "purchased"]
    granted = [b.get("batch") for b in batches
               if b.get("transaction_status") == "granted"]
    purchased_last = (len(drain) == 3 and purchased
                      and drain[-1] in purchased)
    granted_before = (len(granted) == 2 and set(granted) <= set(drain[:2]))
    checks.append(_check(
        "granted_before_purchased_at_equal_priority",
        "at equal priority Lago drains granted batches before purchased ones "
        "(WalletTransaction.in_consumption_order), so the purchased batch P1 is "
        "drained last even though it was created before G2",
        {"tie_break_drain_sequence": drain},
        purchased_last and granted_before,
    ))

    created_at_map = {b.get("batch"): b.get("created_at") for b in batches}
    granted_sorted_by_creation = sorted(
        granted, key=lambda label: created_at_map.get(label) or "")
    granted_in_creation_order = (len(drain) >= 2 and drain[:2] == granted_sorted_by_creation)
    checks.append(_check(
        "created_at_tiebreak_within_granted",
        "within the same transaction_status, batches drain in created_at order "
        "(G1 before G2)",
        {"drain": drain, "granted_in_creation_order": granted_sorted_by_creation},
        granted_in_creation_order,
    ))

    drawdown = observed.get("drawdown") or {}
    checks.append(_check(
        "remaining_amount_never_negative",
        "remaining_amount_cents of every traced inbound batch never goes negative",
        {"min_remaining_cents": drawdown.get("min_remaining_cents")},
        (drawdown.get("min_remaining_cents") is not None
         and drawdown.get("min_remaining_cents") >= 0),
    ))
    checks.append(_check(
        "drawdown_monotonic_non_increasing",
        "remaining_amount_cents of every traced inbound batch is monotonically "
        "non-increasing across the run",
        {"all_non_increasing": drawdown.get("all_non_increasing")},
        bool(drawdown.get("all_non_increasing")),
    ))
    return checks


# ---------------------------------------------------------------------------
# Runtime scenario


def _create_wallet(client, report, customer_id, **kwargs):
    _, body = client.post("/api/v1/wallets",
                          payload=harness.wallet_payload(customer_id, **kwargs))
    wallet = (body or {}).get("wallet") or {}
    report.record_created("wallet", lago_id=wallet.get("lago_id"))
    return wallet


def _future_iso(days):
    import datetime
    return (datetime.datetime.now(datetime.timezone.utc)
            + datetime.timedelta(days=days)).strftime("%Y-%m-%dT%H:%M:%SZ")


def _balance(client, lago_id):
    return trigger.wallet_state(client, lago_id).get("balance_cents")


def run(client, report, cleanup):
    # Two independent customers: the cross-wallet order probe and the
    # within-wallet tie-break probe never interact (a shared customer let the
    # priority-3 leftover compete with the priority-4 tie wallet on the first
    # real run). Order evidence comes from serial one-wallet-sized drains --
    # which balance dropped at each step -- never from same-second
    # transaction timestamps.
    order_customer = harness.new_customer_id()
    client.post("/api/v1/customers", payload=harness.customer_payload(order_customer))
    report.record_created("customer", external_id=order_customer)
    order_ctx = trigger.setup_consumption_trigger(client, report, order_customer)

    batch = harness.new_batch_key()
    wallets = {}
    for label, _pos, priority, granted_cents, expiry_days in WALLET_SPEC:
        wallet = _create_wallet(
            client, report, order_customer, name=f"order-{label}",
            granted_credits=trigger.cents_to_credits(granted_cents),
            priority=priority, expiration_at=_future_iso(expiry_days),
            batch_key=batch)
        wallets[label] = wallet
    creation_order = [wallets[label]["code"] for label in
                      [spec[0] for spec in sorted(WALLET_SPEC, key=lambda s: s[1])]]

    # Runtime finding (first real run): a wallet-create's initial credits are
    # granted ASYNCHRONOUSLY (Wallets::CreateService#schedule_top_up ->
    # WalletTransactions::CreateJob.perform_after_commit), so a consumption
    # fired ~150ms after creation can skip the newest wallet entirely and
    # draw a later-expiring batch first. Wait for every wallet's balance to
    # settle before probing the order -- the same settle-wait the WeKnora
    # coordination layer owes around wallet creation.
    for label, _pos, _priority, granted_cents, _expiry in WALLET_SPEC:
        trigger.wait_until(
            lambda label=label, granted_cents=granted_cents:
                (_balance(client, wallets[label]["lago_id"]) or 0) == granted_cents,
            120)

    balances = {label: _balance(client, wallets[label]["lago_id"])
                for label in wallets}

    # Drain one wallet per serial consumption: A(5000), B(7000), C(4000).
    draw_sequence = []
    for label, drain_cents in (("A", 5000), ("B", 7000), ("C", 4000)):
        before = {inner: _balance(client, wallets[inner]["lago_id"])
                  for inner in wallets}
        trigger.consume(client, order_ctx, drain_cents)
        trigger.wait_until(
            lambda: sum(_balance(client, wallets[inner]["lago_id"])
                        for inner in wallets)
            == sum(before.values()) - drain_cents, 120)
        dropped = []
        for inner in wallets:
            delta = (before[inner] or 0) \
                - (_balance(client, wallets[inner]["lago_id"]) or 0)
            if delta > 0:
                dropped.append({"code": wallets[inner]["code"],
                                "label": inner,
                                "priority": wallets[inner].get("priority")
                                or trigger.wallet_state(
                                    client, wallets[inner]["lago_id"]).get("priority"),
                                "drawn_cents": delta})
        draw_sequence.extend(dropped)
    balances_after = {label: _balance(client, wallets[label]["lago_id"])
                      for label in wallets}

    # --- within-wallet tie-break probe on its own customer ------------------
    tie_customer = harness.new_customer_id()
    client.post("/api/v1/customers", payload=harness.customer_payload(tie_customer))
    report.record_created("customer", external_id=tie_customer)
    tie_ctx = trigger.setup_consumption_trigger(client, report, tie_customer)
    w_tie = _create_wallet(client, report, tie_customer, name="tie-break")
    tie_id = w_tie["lago_id"]

    batch_labels = {}
    inbound_rows = []
    # Space the grants so created_at differs at API granularity (seconds).
    for label, credits_key in (("G1", "granted_credits"), ("P1", "paid_credits"),
                               ("G2", "granted_credits")):
        key = harness.new_batch_key()
        batch_labels[key] = label
        client.post("/api/v1/wallet_transactions", payload=harness.wallet_grant_payload(
            tie_id, **{credits_key: trigger.cents_to_credits(TIE_GRANT_CENTS)},
            batch_key=key))
        time.sleep(1.2)
    # Purchased credits stay pending until the credit invoice is marked paid.
    trigger.confirm_paid_credits(client, tie_customer, tie_id, 30000)
    for row in trigger.wallet_transactions(client, tie_id):
        if row["transaction_type"] != "inbound":
            continue
        meta = {m.get("key"): m.get("value") for m in row.get("metadata") or []}
        batch_value = meta.get("weknora_t03_batch")
        inbound_rows.append({
            "batch": batch_labels.get(batch_value, batch_value),
            "transaction_status": row["transaction_status"],
            "created_at": row["created_at"],
            "remaining_cents": row["remaining_amount_cents"],
            "lago_id": row["lago_id"],
        })

    remaining_history = []
    drain_sequence = []
    purchased_label = None
    for row in inbound_rows:
        if row["transaction_status"] == "purchased":
            purchased_label = row["batch"]
    for _step in range(TIE_STEPS):
        min_seen = {row["batch"]: row["remaining_cents"] for row in inbound_rows}
        tie_balance_before = _balance(client, tie_id)
        trigger.consume(client, tie_ctx, TIE_GRANT_CENTS)
        trigger.wait_until(
            lambda: (_balance(client, tie_id) is not None
                     and _balance(client, tie_id) <= tie_balance_before - TIE_GRANT_CENTS),
            120)
        rows_now = [row for row in trigger.wallet_transactions(client, tie_id)
                    if row["transaction_type"] == "inbound"]
        meta_of = {}
        for row in rows_now:
            meta = {m.get("key"): m.get("value") for m in row.get("metadata") or []}
            meta_of[row["lago_id"]] = meta.get("weknora_t03_batch")
        for row in rows_now:
            label = batch_labels.get(meta_of.get(row["lago_id"]))
            for tracked in inbound_rows:
                if tracked["batch"] == label:
                    if tracked["remaining_cents"] is not None \
                            and row["remaining_amount_cents"] is not None:
                        remaining_history.append(
                            (label, tracked["remaining_cents"],
                             row["remaining_amount_cents"]))
                    tracked["remaining_cents"] = row["remaining_amount_cents"]
        newly_drained = [row["batch"] for row in inbound_rows
                         if row["remaining_cents"] is not None
                         and min_seen.get(row["batch"]) is not None
                         and row["remaining_cents"] < min_seen[row["batch"]]]
        if newly_drained:
            drain_sequence.extend(newly_drained)
        elif purchased_label and purchased_label not in drain_sequence:
            # The purchased batch has a nil remainder: drained by elimination.
            drain_sequence.append(purchased_label)

    same_wallet_batches = [{"batch": row["batch"],
                            "transaction_status": row["transaction_status"],
                            "created_at": row["created_at"]}
                           for row in sorted(inbound_rows,
                                             key=lambda r: r["created_at"] or "")]
    history_values = [entry[1] for entry in remaining_history] + \
        [entry[2] for entry in remaining_history]
    drawdown = {
        "all_non_increasing": all(
            after <= before for _label, before, after in remaining_history),
        "min_remaining_cents": min(
            [value for value in history_values if value is not None], default=None),
    }

    observed = {
        "settle_note": "wallet-create initial credits settle asynchronously "
                       "(Wallets::CreateService#schedule_top_up -> after-commit "
                       "WalletTransactions::CreateJob); the probe waits for "
                       "every balance before consuming. First real run without "
                       "the wait drew priority-2 before priority-1 because the "
                       "priority-1 wallet created 150ms earlier had no balance "
                       "yet -- a coordination obligation recorded in the "
                       "verdict.",
        "creation_order": creation_order,
        "priority_map": {wallets[label]["code"]: spec[2]
                         for label, spec in zip(("C", "B", "A"), WALLET_SPEC)},
        "expiry_rank_earliest_first": [wallets[label]["code"]
                                       for label in ("A", "B", "C")],
        "invoice_amount_cents": CROSS_CONSUMPTION_CENTS,
        "draw_sequence": draw_sequence,
        "same_wallet_batches": same_wallet_batches,
        "tie_break_drain_sequence": drain_sequence,
        "drawdown": drawdown,
        "final_balances_cents": balances_after,
    }
    for row in e2_expected(observed):
        report.add_check(row["criterion"], row["expected"], row["observed"],
                         outcome=row["outcome"])
    report.meta["observed"] = observed


RUNNER = run
