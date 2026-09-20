"""Offline pre-registered expected-outcome tests for experiment E2 (order).

``e2_expected(observed)`` fixes, before any real run, the pinned-source
expectations: wallets are consumed in ``order(:priority, :created_at)``
regardless of expiry rank, and within one wallet inbound batches consume by
``priority, granted-before-purchased, created_at`` with a never-negative,
monotonically non-increasing ``remaining_amount_cents``.
"""

import unittest

if __package__ in (None, ""):
    import sys
    from pathlib import Path

    sys.path.insert(0, str(Path(__file__).resolve().parent))

from experiments import e2_order
from experiments.e2_order import e2_expected, outcome_of


def base_observed(**overrides):
    """The all-good E2 baseline: priority A(1) < B(2) < C(3) is followed
    exactly although the creation order was C, B, A and expiry rank disagrees
    with creation order; the tie-break wallet drains granted G1, granted G2,
    purchased P1 (granted-before-purchased beats created_at)."""
    observed = {
        "creation_order": ["weknora-t03-wallet-c", "weknora-t03-wallet-b",
                           "weknora-t03-wallet-a"],
        "priority_map": {"weknora-t03-wallet-a": 1, "weknora-t03-wallet-b": 2,
                         "weknora-t03-wallet-c": 3},
        "expiry_rank_earliest_first": ["weknora-t03-wallet-a", "weknora-t03-wallet-b",
                                       "weknora-t03-wallet-c"],
        "invoice_amount_cents": 21000,
        "draw_sequence": [
            {"code": "weknora-t03-wallet-a", "priority": 1, "drawn_cents": 5000},
            {"code": "weknora-t03-wallet-b", "priority": 2, "drawn_cents": 6000},
            {"code": "weknora-t03-wallet-c", "priority": 3, "drawn_cents": 10000},
        ],
        "same_wallet_batches": [
            {"batch": "G1", "transaction_status": "granted", "created_at": "2026-09-21T01:00:01Z"},
            {"batch": "G2", "transaction_status": "granted", "created_at": "2026-09-21T01:00:03Z"},
            {"batch": "P1", "transaction_status": "purchased", "created_at": "2026-09-21T01:00:02Z"},
        ],
        "tie_break_drain_sequence": ["G1", "G2", "P1"],
        "drawdown": {"all_non_increasing": True, "min_remaining_cents": 0},
    }
    observed.update(overrides)
    return observed


class WalletOrderTests(unittest.TestCase):
    def test_consumption_follows_priority_over_creation_and_expiry_rank(self):
        checks = e2_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "consumption_follows_priority_over_creation_and_expiry_rank"),
            "pass")

    def test_creation_order_consumption_is_a_failure(self):
        observed = base_observed()
        observed["draw_sequence"] = [
            {"code": "weknora-t03-wallet-c", "priority": 3, "drawn_cents": 10000},
            {"code": "weknora-t03-wallet-b", "priority": 2, "drawn_cents": 6000},
            {"code": "weknora-t03-wallet-a", "priority": 1, "drawn_cents": 5000},
        ]
        checks = e2_expected(observed)
        self.assertEqual(
            outcome_of(checks, "consumption_follows_priority_over_creation_and_expiry_rank"),
            "fail")

    def test_partial_draw_signature_distinguishes_orders(self):
        # A different last-wallet signature means a different order was taken.
        observed = base_observed()
        observed["draw_sequence"] = [
            {"code": "weknora-t03-wallet-a", "priority": 1, "drawn_cents": 5000},
            {"code": "weknora-t03-wallet-c", "priority": 3, "drawn_cents": 10000},
            {"code": "weknora-t03-wallet-b", "priority": 2, "drawn_cents": 6000},
        ]
        checks = e2_expected(observed)
        self.assertEqual(
            outcome_of(checks, "consumption_follows_priority_over_creation_and_expiry_rank"),
            "fail")


class WithinWalletTieBreakTests(unittest.TestCase):
    def test_granted_before_purchased_at_equal_priority(self):
        checks = e2_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "granted_before_purchased_at_equal_priority"), "pass")

    def test_created_at_order_draining_purchased_first_is_a_failure(self):
        observed = base_observed()
        observed["tie_break_drain_sequence"] = ["G1", "P1", "G2"]
        checks = e2_expected(observed)
        self.assertEqual(
            outcome_of(checks, "granted_before_purchased_at_equal_priority"), "fail")

    def test_created_at_tiebreak_within_granted_batches(self):
        checks = e2_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "created_at_tiebreak_within_granted"), "pass")

    def test_later_granted_batch_drained_before_earlier_is_a_failure(self):
        observed = base_observed()
        observed["tie_break_drain_sequence"] = ["G2", "G1", "P1"]
        checks = e2_expected(observed)
        self.assertEqual(
            outcome_of(checks, "created_at_tiebreak_within_granted"), "fail")


class DrawdownTests(unittest.TestCase):
    def test_remaining_never_negative(self):
        checks = e2_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "remaining_amount_never_negative"), "pass")

    def test_negative_remaining_is_a_failure(self):
        observed = base_observed()
        observed["drawdown"]["min_remaining_cents"] = -100
        checks = e2_expected(observed)
        self.assertEqual(
            outcome_of(checks, "remaining_amount_never_negative"), "fail")

    def test_drawdown_monotonic_non_increasing(self):
        checks = e2_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "drawdown_monotonic_non_increasing"), "pass")

    def test_increasing_remaining_is_a_failure(self):
        observed = base_observed()
        observed["drawdown"]["all_non_increasing"] = False
        checks = e2_expected(observed)
        self.assertEqual(
            outcome_of(checks, "drawdown_monotonic_non_increasing"), "fail")


class ModuleContractTests(unittest.TestCase):
    def test_module_exports_runner_and_slug(self):
        self.assertEqual(e2_order.SLUG, "e2-order")
        self.assertTrue(callable(e2_order.RUNNER))


if __name__ == "__main__":
    unittest.main()
