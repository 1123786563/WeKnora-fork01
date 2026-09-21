"""Offline pre-registered expected-outcome tests for experiment E4
(void and refund withdraw only the unconsumed remainder).

``e4_expected(observed)`` fixes, before any real run, from the pinned source
(``WalletTransactions::VoidService``): partial void bounded by
``remaining_amount_cents`` (``exceeds_remaining_transaction_amount``),
whole-remaining void sized under the customer lock, void_remaining a no-op on
a drained batch, purchased inbounds carrying nil remainders (no refund status
exists in v1.53.0), and paid credits settling immediately in Community.
"""

import unittest

if __package__ in (None, ""):
    import sys
    from pathlib import Path

    sys.path.insert(0, str(Path(__file__).resolve().parent))

from experiments import e4_withdraw
from experiments.e4_withdraw import e4_expected, outcome_of


def base_observed(**overrides):
    """The all-good E4 baseline (fresh batch per sub-scenario)."""
    observed = {
        "partial_void": {
            "http_status": 200, "voided_cents": 2000,
            "remaining_before_cents": 6000, "remaining_after_cents": 4000,
            "consumed_outbound_cents": 4000, "consumed_outbound_unchanged": True,
            "balance_before_cents": 6000, "balance_after_cents": 4000,
        },
        "exact_void": {
            "http_status": 200, "voided_cents": 6000,
            "remaining_after_cents": 0, "balance_after_cents": 0,
        },
        "excess_void": {
            "http_status": 422, "voided_cents": 10000,
            "error_code": "exceeds_remaining_transaction_amount",
            "remaining_unchanged": True, "balance_unchanged": True,
            "above_balance_error_code": "insufficient_credits",
        },
        "void_remaining_noop": {
            "http_status": 422, "error_code": "no_remaining_amount",
            "void_txn_created": False,
            "balance_after_cents": 0, "remaining_after_cents": 0,
        },
        "granted_refund_model": {
            "http_status": 200, "wallet_balance_after_cents": 0,
            "batch_remaining_after_cents": 0, "spendable_attributable_cents": 0,
        },
        "purchased_inbound_void": {
            "attempted": True, "http_status": 200,
            "remaining_tracked": True,
            "remaining_before_cents": 6000, "voided_cents": 2000,
            "remaining_after_cents": 4000,
            "balance_before_cents": 6000, "balance_after_cents": 4000,
        },
        "credit_note_on_paid_invoice": {
            "created_status": 200, "credit_amount_cents": 6000,
            "wallet_balance_before_cents": 6000,
            "wallet_balance_after_cents": 6000,
        },
        "paid_settlement": {
            "invoice_id": "inv-1", "payment_status_before": "pending",
            "confirm_http_status": 200, "settled": True,
            "balance_cents": 5000, "settled_within_seconds": 3.0,
            "balance_before_confirmation_cents": 0,
            "spendable_confirmed": True,
        },
    }
    observed.update(overrides)
    return observed


class VoidBoundsTests(unittest.TestCase):
    def test_partial_void_decrements_only_the_remainder(self):
        checks = e4_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "partial_void_decrements_only_remaining"), "pass")

    def test_partial_void_touching_consumed_history_is_a_failure(self):
        observed = base_observed()
        observed["partial_void"]["consumed_outbound_unchanged"] = False
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "partial_void_decrements_only_remaining"), "fail")

    def test_void_of_exactly_the_remainder_empties_the_batch(self):
        checks = e4_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "void_exactly_remaining_empties_batch"), "pass")

    def test_void_above_remainder_is_rejected_and_changes_nothing(self):
        checks = e4_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "void_above_remaining_rejected"), "pass")

    def test_excess_void_succeeding_is_a_failure(self):
        observed = base_observed()
        observed["excess_void"] = {
            "http_status": 200, "voided_cents": 10000, "error_code": None,
            "remaining_unchanged": False, "balance_unchanged": False,
        }
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "void_above_remaining_rejected"), "fail")

    def test_balance_guard_firing_first_is_a_failure_for_this_check(self):
        # A void above the whole balance exits at insufficient_credits before
        # the remainder bound can be observed -- the check demands the
        # two-batch shape that isolates exceeds_remaining_transaction_amount.
        observed = base_observed()
        observed["excess_void"]["error_code"] = "insufficient_credits"
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "void_above_remaining_rejected"), "fail")

    def test_wrong_rejection_code_is_a_failure(self):
        observed = base_observed()
        observed["excess_void"]["error_code"] = "validation_errors"
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "void_above_remaining_rejected"), "fail")

    def test_void_remaining_noop_on_drained_batch(self):
        checks = e4_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "void_remaining_noop_on_consumed_batch"), "pass")

    def test_void_remaining_rejected_with_wrong_code_is_a_failure(self):
        observed = base_observed()
        observed["void_remaining_noop"]["error_code"] = "validation_errors"
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "void_remaining_noop_on_consumed_batch"), "fail")

    def test_void_remaining_creating_negative_balance_is_a_failure(self):
        observed = base_observed()
        observed["void_remaining_noop"].update({
            "http_status": 200, "error_code": None,
            "void_txn_created": True, "balance_after_cents": -2000})
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "void_remaining_noop_on_consumed_batch"), "fail")


class RefundModelTests(unittest.TestCase):
    def test_granted_refund_model_leaves_no_spendable_credits(self):
        checks = e4_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "refund_model_leaves_no_spendable_credits"), "pass")

    def test_spendable_credits_remaining_after_refund_is_a_failure(self):
        observed = base_observed()
        observed["granted_refund_model"].update({
            "wallet_balance_after_cents": 3000,
            "batch_remaining_after_cents": 3000,
            "spendable_attributable_cents": 3000})
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "refund_model_leaves_no_spendable_credits"), "fail")

    def test_purchased_inbound_void_within_remaining(self):
        # Refined by the first real run: settlement assigns tracked remainders
        # to purchased inbounds too, so the refund model (remainder-only void)
        # works for purchased batches exactly as for granted ones.
        checks = e4_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "purchased_inbound_void_within_remaining"), "pass")

    def test_purchased_void_overdrawing_the_remainder_is_a_failure(self):
        observed = base_observed()
        observed["purchased_inbound_void"]["remaining_after_cents"] = -1000
        observed["purchased_inbound_void"]["balance_after_cents"] = -1000
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "purchased_inbound_void_within_remaining"), "fail")

    def test_purchased_void_rejected_is_a_failure_for_the_refined_model(self):
        observed = base_observed()
        observed["purchased_inbound_void"]["http_status"] = 422
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "purchased_inbound_void_within_remaining"), "fail")

    def test_credit_note_on_paid_invoice_does_not_recredit_the_wallet(self):
        checks = e4_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "credit_note_does_not_recredit_wallet"), "pass")

    def test_credit_note_recrediting_wallet_is_a_failure(self):
        observed = base_observed()
        observed["credit_note_on_paid_invoice"]["wallet_balance_after_cents"] = 12000
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "credit_note_does_not_recredit_wallet"), "fail")


class PaidSettlementTests(unittest.TestCase):
    def test_paid_credits_require_payment_confirmation_before_spendability(self):
        checks = e4_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "paid_credits_require_payment_confirmation"), "pass")

    def test_paid_credits_settling_without_confirmation_is_a_failure(self):
        # The corrected pre-registration (after the first real run refuted
        # "immediately spendable"): credits pending before the payment fact
        # and spendable after it. Balance before confirmation > 0 would mean
        # they settled without the payment fact -- contradicting it.
        observed = base_observed()
        observed["paid_settlement"]["balance_before_confirmation_cents"] = 5000
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "paid_credits_require_payment_confirmation"), "fail")

    def test_unspendable_after_confirmation_is_a_failure(self):
        observed = base_observed()
        observed["paid_settlement"]["spendable_confirmed"] = False
        checks = e4_expected(observed)
        self.assertEqual(
            outcome_of(checks, "paid_credits_require_payment_confirmation"), "fail")


class ModuleContractTests(unittest.TestCase):
    def test_module_exports_runner_and_slug(self):
        self.assertEqual(e4_withdraw.SLUG, "e4-withdraw")
        self.assertTrue(callable(e4_withdraw.RUNNER))


if __name__ == "__main__":
    unittest.main()
