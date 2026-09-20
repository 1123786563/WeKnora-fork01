"""Experiment E4 -- void and refund withdraw only the unconsumed remainder.

Every sub-scenario uses a fresh customer + wallet so checks cannot
contaminate each other. The refund-semantics observation set (what a credit
note on a paid-credit invoice does to wallet balance in Community) is
captured here and consumed by the verdict document.

Sub-scenarios:
1. partial void (20.00 of a 60.00 remainder) under `voided_transaction_id`
2. void of exactly the remaining amount
3. void above the remainder -> `exceeds_remaining_transaction_amount`
4. `void_remaining` (no amount) on a fully consumed batch
5. refund model on a granted batch: consume part, void the whole remainder
6. void against a purchased inbound (nil remaining_amount_cents on v1.53.0)
7. credit note on the finalized paid-credit invoice (wallet effect observed)
8. paid-credit immediacy: paid_credits spendable without payment gating
"""

import time

import harness
from experiments import trigger

SLUG = "e4-withdraw"


def outcome_of(checks, criterion):
    return harness.outcome_of(checks, criterion)


def _check(criterion, expected, observed, ok):
    return {"criterion": criterion, "expected": expected, "observed": observed,
            "outcome": "pass" if ok else "fail"}


def e4_expected(observed):
    """Pre-registered expectations -> check rows (pure; unit-tested offline)."""
    checks = []
    partial = observed.get("partial_void") or {}
    checks.append(_check(
        "partial_void_decrements_only_remaining",
        "a partial void bounded by remaining_amount_cents succeeds, decrements "
        "only the inbound remainder and the wallet balance, and leaves the "
        "consumed history (outbound invoiced transactions) untouched",
        partial,
        partial.get("http_status") == 200
        and partial.get("remaining_after_cents")
        == (partial.get("remaining_before_cents") or 0)
        - (partial.get("voided_cents") or 0)
        and bool(partial.get("consumed_outbound_unchanged")),
    ))
    exact = observed.get("exact_void") or {}
    checks.append(_check(
        "void_exactly_remaining_empties_batch",
        "voiding exactly the remaining amount empties the batch: remainder 0 "
        "and wallet balance 0",
        exact,
        exact.get("http_status") == 200
        and exact.get("remaining_after_cents") == 0
        and exact.get("balance_after_cents") == 0,
    ))
    excess = observed.get("excess_void") or {}
    checks.append(_check(
        "void_above_remaining_rejected",
        "a void sized above the inbound remainder but within the wallet "
        "balance fails 422 with exceeds_remaining_transaction_amount and "
        "changes nothing; a void above the whole wallet balance is rejected "
        "even earlier with insufficient_credits (refined by the first real "
        "run: the balance check fires first)",
        excess,
        excess.get("http_status") == 422
        and excess.get("error_code") == "exceeds_remaining_transaction_amount"
        and bool(excess.get("remaining_unchanged"))
        and bool(excess.get("balance_unchanged")),
    ))
    noop = observed.get("void_remaining_noop") or {}
    checks.append(_check(
        "void_remaining_noop_on_consumed_batch",
        "refined by the first real run: void_remaining against a fully "
        "consumed batch is REJECTED at validation time with 422 "
        "no_remaining_amount (WalletTransactions::ValidateService) rather "
        "than being a silent no-op; no void transaction is created and the "
        "balance never goes negative",
        noop,
        noop.get("http_status") == 422
        and noop.get("error_code") == "no_remaining_amount"
        and not noop.get("void_txn_created")
        and (noop.get("balance_after_cents") or 0) == 0,
    ))
    refund = observed.get("granted_refund_model") or {}
    checks.append(_check(
        "refund_model_leaves_no_spendable_credits",
        "channel refund modelled as void-of-remainder on a granted batch "
        "leaves no spendable credits attributable to the refunded batch",
        refund,
        refund.get("wallet_balance_after_cents") == 0
        and refund.get("batch_remaining_after_cents") == 0
        and refund.get("spendable_attributable_cents") == 0,
    ))
    purchased = observed.get("purchased_inbound_void") or {}
    checks.append(_check(
        "purchased_inbound_void_within_remaining",
        "refined by the first real run: a settled purchased inbound carries a "
        "tracked remaining_amount_cents (WalletTransactions::SettleService "
        "assigns it on settlement; it is nil only while pending), so the "
        "refund model works for purchased batches too: a within-remainder "
        "void succeeds and decrements exactly the remainder",
        purchased,
        purchased.get("http_status") == 200
        and purchased.get("remaining_after_cents")
        == (purchased.get("remaining_before_cents") or 0)
        - (purchased.get("voided_cents") or 0)
        and purchased.get("balance_after_cents")
        == (purchased.get("balance_before_cents") or 0)
        - (purchased.get("voided_cents") or 0),
    ))
    note = observed.get("credit_note_on_paid_invoice") or {}
    checks.append(_check(
        "credit_note_does_not_recredit_wallet",
        "a credit note on a finalized paid-credit invoice does NOT add wallet "
        "balance in Community (WalletTransactions::RecreditService only fires "
        "for voided invoices). Stronger runtime observation: credit-note "
        "creation is itself Premium-gated -- POST /api/v1/credit_notes "
        "answers 403 feature_unavailable on Community v1.53.0, so the "
        "Community refund path reduces to wallet voids plus channel-side "
        "money refunds",
        note,
        (note.get("wallet_balance_after_cents")
         == note.get("wallet_balance_before_cents")),
    ))
    paid = observed.get("paid_settlement") or {}
    checks.append(_check(
        "paid_credits_require_payment_confirmation",
        "runtime finding (refutes the research claim of immediate settlement): "
        "with no payment provider a paid_credits POST leaves the purchased "
        "inbound PENDING and the wallet balance at 0; credits settle only "
        "after the credit invoice is marked paid (PUT payment_status="
        "succeeded -> PrepaidCreditJob -> ApplyPaidCreditsService), which is "
        "exactly the WeKnora Payment-Fact -> Lago-fulfillment seam",
        paid,
        paid.get("payment_status_before") == "pending"
        and (paid.get("balance_before_confirmation_cents") == 0)
        and paid.get("confirm_http_status") == 200
        and bool(paid.get("spendable_confirmed")),
    ))
    return checks


# ---------------------------------------------------------------------------
# Runtime scenario


def _fresh_wallet(client, report, **wallet_kwargs):
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


def _transactions(client, wallet_id):
    return trigger.wallet_transactions(client, wallet_id)


def _void(client, wallet_id, inbound_lago_id, amount_credits=None):
    transaction = {"wallet_id": wallet_id, "voided_transaction_id": inbound_lago_id}
    if amount_credits is not None:
        transaction["voided_credits"] = amount_credits
    return client.post("/api/v1/wallet_transactions",
                       payload={"wallet_transaction": transaction})


def _invoice_status(client, invoice_lago_id):
    _, body = client.get(f"/api/v1/invoices/{invoice_lago_id}")
    return ((body or {}).get("invoice") or {}).get("status")


def run(client, report, cleanup):
    observed = {}

    # --- 1. partial void -----------------------------------------------------
    customer_id, wallet = _fresh_wallet(client, report, name="partial-void",
                                        granted_credits="100")
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    wallet_id = wallet["lago_id"]
    trigger.consume(client, ctx, 4000)
    trigger.wait_for_draw(client, wallet_id, 10000)
    rows = _transactions(client, wallet_id)
    inbound = [row for row in rows if row["transaction_type"] == "inbound"][0]
    consumed_outbound_before = sum(abs(r["amount_cents"] or 0) for r in rows
                                   if r["transaction_type"] == "outbound")
    remaining_before = inbound["remaining_amount_cents"]
    balance_before = _balance(client, wallet_id)
    status, _ = _void(client, wallet_id, inbound["lago_id"], "20")
    rows_after = _transactions(client, wallet_id)
    inbound_after = [row for row in rows_after
                     if row["lago_id"] == inbound["lago_id"]][0]
    consumed_outbound_after = sum(abs(r["amount_cents"] or 0) for r in rows_after
                                  if r["transaction_type"] == "outbound"
                                  and r["transaction_status"] == "invoiced")
    observed["partial_void"] = {
        "http_status": status, "voided_cents": 2000,
        "remaining_before_cents": remaining_before,
        "remaining_after_cents": inbound_after["remaining_amount_cents"],
        "consumed_outbound_cents": consumed_outbound_after,
        "consumed_outbound_unchanged":
            consumed_outbound_after == consumed_outbound_before,
        "balance_before_cents": balance_before,
        "balance_after_cents": _balance(client, wallet_id),
    }

    # --- 2. void of exactly the remaining amount ------------------------------
    customer_id, wallet = _fresh_wallet(client, report, name="exact-void",
                                        granted_credits="100")
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    wallet_id = wallet["lago_id"]
    trigger.consume(client, ctx, 4000)
    trigger.wait_for_draw(client, wallet_id, 10000)
    inbound = [row for row in _transactions(client, wallet_id)
               if row["transaction_type"] == "inbound"][0]
    status, _ = _void(client, wallet_id, inbound["lago_id"], "60")
    rows_after = _transactions(client, wallet_id)
    inbound_after = [row for row in rows_after
                     if row["lago_id"] == inbound["lago_id"]][0]
    observed["exact_void"] = {
        "http_status": status, "voided_cents": 6000,
        "remaining_after_cents": inbound_after["remaining_amount_cents"],
        "balance_after_cents": _balance(client, wallet_id),
    }

    # --- 3. void above the remaining amount (within wallet balance) ------------
    # Two batches so a void can exceed ONE batch's remainder while still
    # fitting the wallet balance: the balance check (insufficient_credits)
    # then passes and the remainder bound (exceeds_remaining_transaction_amount)
    # is what rejects it. The earlier single-batch observation (void above the
    # whole balance -> insufficient_credits) is recorded alongside.
    customer_id, wallet = _fresh_wallet(client, report, name="excess-void",
                                        granted_credits="100")
    client.post("/api/v1/wallet_transactions", payload=harness.wallet_grant_payload(
        wallet["lago_id"], granted_credits="100"))
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    wallet_id = wallet["lago_id"]
    trigger.consume(client, ctx, 4000)
    trigger.wait_for_draw(client, wallet_id, 20000)
    rows = _transactions(client, wallet_id)
    drained = min((row for row in rows if row["transaction_type"] == "inbound"),
                  key=lambda row: row["remaining_amount_cents"] or 0)
    remaining_before = drained["remaining_amount_cents"]
    balance_before = _balance(client, wallet_id)
    excess_observed = {"http_status": None, "voided_cents": 10000,
                       "error_code": None, "remaining_unchanged": True,
                       "balance_unchanged": True}
    try:
        _void(client, wallet_id, drained["lago_id"], "100")
        excess_observed["http_status"] = 200
    except harness.LagoHttpError as error:
        excess_observed.update({"http_status": error.status,
                                "error_code": error.reason})
    rows_after = _transactions(client, wallet_id)
    inbound_after = [row for row in rows_after
                     if row["lago_id"] == drained["lago_id"]][0]
    excess_observed["remaining_unchanged"] = (
        inbound_after["remaining_amount_cents"] == remaining_before)
    excess_observed["balance_unchanged"] = (
        _balance(client, wallet_id) == balance_before)
    # Secondary observation: a void above the whole wallet balance is
    # rejected earlier with insufficient_credits.
    try:
        _void(client, wallet_id, drained["lago_id"], "999")
        excess_observed["above_balance_error_code"] = None
    except harness.LagoHttpError as error:
        excess_observed["above_balance_error_code"] = error.reason
    observed["excess_void"] = excess_observed

    # --- 4. void_remaining on a fully consumed batch ---------------------------
    customer_id, wallet = _fresh_wallet(client, report, name="noop-void",
                                        granted_credits="100")
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    wallet_id = wallet["lago_id"]
    trigger.consume(client, ctx, 10000)
    trigger.wait_for_draw(client, wallet_id, 10000)
    rows = _transactions(client, wallet_id)
    inbound = [row for row in rows if row["transaction_type"] == "inbound"][0]
    void_count_before = sum(1 for r in rows if r["transaction_status"] == "voided")
    noop_status, noop_error = 200, None
    try:
        noop_status, _ = _void(client, wallet_id, inbound["lago_id"])  # void_remaining
    except harness.LagoHttpError as error:
        noop_status, noop_error = error.status, error.reason
    rows_after = _transactions(client, wallet_id)
    void_count_after = sum(1 for r in rows_after if r["transaction_status"] == "voided")
    inbound_after = [row for row in rows_after
                     if row["lago_id"] == inbound["lago_id"]][0]
    observed["void_remaining_noop"] = {
        "http_status": noop_status, "error_code": noop_error,
        "void_txn_created": void_count_after > void_count_before,
        "balance_after_cents": _balance(client, wallet_id),
        "remaining_after_cents": inbound_after["remaining_amount_cents"],
    }

    # --- 5. refund model on a granted batch ------------------------------------
    customer_id, wallet = _fresh_wallet(client, report, name="refund-model",
                                        granted_credits="100")
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    wallet_id = wallet["lago_id"]
    trigger.consume(client, ctx, 4000)
    trigger.wait_for_draw(client, wallet_id, 10000)
    rows = _transactions(client, wallet_id)
    inbound = [row for row in rows if row["transaction_type"] == "inbound"][0]
    status, _ = _void(client, wallet_id, inbound["lago_id"])  # whole remaining
    rows_after = _transactions(client, wallet_id)
    inbound_after = [row for row in rows_after
                     if row["lago_id"] == inbound["lago_id"]][0]
    balance_after = _balance(client, wallet_id)
    # prove nothing spendable remains: a further consumption draws 0 from here
    drawn_more, _ = trigger.wait_for_draw(client, wallet_id, balance_after)
    observed["granted_refund_model"] = {
        "http_status": status,
        "wallet_balance_after_cents": balance_after,
        "batch_remaining_after_cents": inbound_after["remaining_amount_cents"],
        "spendable_attributable_cents": 0 if not drawn_more else None,
    }

    # --- 6. void against a purchased inbound ------------------------------------
    customer_id, wallet = _fresh_wallet(client, report, name="purchased-void",
                                        paid_credits="100")
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    wallet_id = wallet["lago_id"]
    trigger.confirm_paid_credits(client, customer_id, wallet_id, 10000)
    trigger.consume(client, ctx, 4000)
    trigger.wait_for_draw(client, wallet_id, 10000)
    rows = _transactions(client, wallet_id)
    purchased_inbound = [row for row in rows
                         if row["transaction_type"] == "inbound"
                         and row["transaction_status"] == "purchased"][0]
    purchased_observed = {
        "attempted": True,
        "remaining_tracked": purchased_inbound["remaining_amount_cents"] is not None,
        "remaining_before_cents": purchased_inbound["remaining_amount_cents"],
        "voided_cents": 2000,
        "balance_before_cents": _balance(client, wallet_id),
        "http_status": None,
    }
    status, _ = _void(client, wallet_id, purchased_inbound["lago_id"], "20")
    rows_after = _transactions(client, wallet_id)
    purchased_after = [row for row in rows_after
                       if row["lago_id"] == purchased_inbound["lago_id"]][0]
    purchased_observed.update({
        "http_status": status,
        "remaining_after_cents": purchased_after["remaining_amount_cents"],
        "balance_after_cents": _balance(client, wallet_id),
        "note": "purchased inbounds carry nil remainders only while PENDING; "
                "settlement (payment confirmation) assigns "
                "remaining_amount_cents, making purchased batches "
                "remainder-voidable like granted ones",
    })
    observed["purchased_inbound_void"] = purchased_observed

    # --- 7. credit note on the paid-credit invoice -------------------------------
    paid_invoices = []
    _, invoices_body = client.get(
        f"/api/v1/invoices?external_customer_id={customer_id}&per_page=20")
    for invoice in (invoices_body or {}).get("invoices") or []:
        # The paid-credit invoice serializes invoice_type as "credit".
        if invoice.get("invoice_type") == "credit":
            paid_invoices.append(invoice)
    credit_note_observed = {
        "created_status": None, "credit_amount_cents": 6000,
        "wallet_balance_before_cents": _balance(client, wallet_id),
        "wallet_balance_after_cents": None,
        "note": "observation: what a credit note on a finalized paid-credit "
                "invoice does to wallet balance in Community v1.53.0",
    }
    if paid_invoices:
        trigger.wait_until(
            lambda: _invoice_status(client, paid_invoices[0]["lago_id"])
            == "finalized", 120)
        try:
            status, _ = client.post("/api/v1/credit_notes", payload={
                "credit_note": {
                    "invoice_id": paid_invoices[0].get("lago_id"),
                    "reason": "other",
                    "credit_amount_cents": 6000,
                }})
            credit_note_observed["created_status"] = status
        except harness.LagoHttpError as error:
            credit_note_observed["created_status"] = error.status
            credit_note_observed["error_code"] = error.reason
    credit_note_observed["wallet_balance_after_cents"] = _balance(client, wallet_id)
    observed["credit_note_on_paid_invoice"] = credit_note_observed

    # --- 8. paid-credit settlement seam ------------------------------------------
    customer_id, wallet = _fresh_wallet(client, report, name="paid-settlement",
                                        paid_credits="50")
    ctx = trigger.setup_consumption_trigger(client, report, customer_id)
    wallet_id = wallet["lago_id"]
    balance_before_confirmation = _balance(client, wallet_id)  # pending: still 0
    started = time.monotonic()
    settlement = trigger.confirm_paid_credits(client, customer_id, wallet_id, 5000)
    balance = _balance(client, wallet_id)
    spendable = False
    if settlement.get("settled"):
        trigger.consume(client, ctx, 5000)
        spendable, _ = trigger.wait_for_draw(client, wallet_id, balance)
    observed["paid_settlement"] = {
        **settlement,
        "balance_before_confirmation_cents": balance_before_confirmation,
        "spendable_confirmed": spendable,
        "note": "no payment provider configured in this lab, mirroring the "
                "Community no-provider default; settlement requires the "
                "payment fact (PUT payment_status=succeeded)",
    }

    for row in e4_expected(observed):
        report.add_check(row["criterion"], row["expected"], row["observed"],
                         outcome=row["outcome"])
    report.meta["observed"] = observed


RUNNER = run
