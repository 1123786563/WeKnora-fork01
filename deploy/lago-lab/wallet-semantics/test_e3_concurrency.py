"""Offline pre-registered expected-outcome tests for experiment E3
(concurrency and retry).

``e3_expected(observed)`` fixes, before any real run: no API idempotency for
wallet transactions (a replayed grant mints a second batch; metadata-keyed
lookup recovers the duplicates), events ARE idempotent by transaction_id,
and concurrent consumption / void-vs-consume / expiry-during-run never draw
more than the granted amount or leave a negative remainder.
"""

import unittest

if __package__ in (None, ""):
    import sys
    from pathlib import Path

    sys.path.insert(0, str(Path(__file__).resolve().parent))

from experiments import e3_concurrency
from experiments.e3_concurrency import e3_expected, outcome_of


def base_observed(**overrides):
    """The all-good E3 baseline: every pre-registered expectation holds
    (including the pre-registered negative: duplicate grants DO mint)."""
    observed = {
        "duplicate_grant": {
            "payload_replays": 2, "settled_inbounds": 2,
            "granted_each_cents": 5000, "balance_cents": 10000,
            "metadata_lookup_count": 2,
        },
        "event_idempotency": {
            "first_status": 200, "second_status": 422,
            "second_error_code": "transaction_id_already_exists",
            "second_created_event": False,
        },
        "concurrent_consumption": {
            "threads": 6, "event_cents_each": 5000, "granted_cents": 10000,
            "events_accepted": 6, "total_drawn_cents": 10000,
            "final_balance_cents": 0, "min_remaining_cents": 0,
        },
        "void_vs_consume": {
            "granted_cents": 10000, "drawn_cents": 10000, "voided_cents": 0,
            "final_balance_cents": 0, "balance_never_negative": True,
        },
        "expired_during": {
            "granted_cents": 10000, "drawn_before_expiry_cents": 4000,
            "drawn_after_expiry_cents": 4000, "total_drawn_cents": 8000,
            "remaining_cents": 2000, "wallet_status_at_end": "active",
        },
    }
    observed.update(overrides)
    return observed


class DuplicateGrantTests(unittest.TestCase):
    def test_replayed_grant_mints_a_second_settled_batch(self):
        checks = e3_expected(base_observed())
        # The pre-registered expectation is the negative itself: no API
        # idempotency, so a replay DOUBLES the balance; coordination owns it.
        self.assertEqual(outcome_of(checks, "duplicate_grant_mints_second_batch"),
                         "pass")

    def test_replayed_grant_rejected_would_contradict_the_pre_registration(self):
        observed = base_observed()
        observed["duplicate_grant"] = {
            "payload_replays": 2, "settled_inbounds": 1,
            "granted_each_cents": 5000, "balance_cents": 5000,
            "metadata_lookup_count": 1,
        }
        checks = e3_expected(observed)
        self.assertEqual(outcome_of(checks, "duplicate_grant_mints_second_batch"),
                         "fail")

    def test_metadata_keyed_lookup_recovers_both_duplicates(self):
        checks = e3_expected(base_observed())
        self.assertEqual(outcome_of(checks, "metadata_lookup_recovers_duplicates"),
                         "pass")

    def test_events_are_idempotent_by_transaction_id(self):
        checks = e3_expected(base_observed())
        self.assertEqual(outcome_of(checks, "events_idempotent_by_transaction_id"),
                         "pass")

    def test_duplicate_event_accepted_twice_is_a_failure(self):
        observed = base_observed()
        observed["event_idempotency"] = {
            "first_status": 200, "second_status": 200,
            "second_error_code": None, "second_created_event": True,
        }
        checks = e3_expected(observed)
        self.assertEqual(outcome_of(checks, "events_idempotent_by_transaction_id"),
                         "fail")


class ConcurrentConsumptionTests(unittest.TestCase):
    def test_parallel_draws_never_exceed_the_granted_amount(self):
        checks = e3_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "concurrent_consumption_within_balance"), "pass")

    def test_overdrawn_total_is_a_failure(self):
        observed = base_observed()
        observed["concurrent_consumption"]["total_drawn_cents"] = 12000
        checks = e3_expected(observed)
        self.assertEqual(
            outcome_of(checks, "concurrent_consumption_within_balance"), "fail")

    def test_negative_remaining_is_a_failure(self):
        observed = base_observed()
        observed["concurrent_consumption"]["min_remaining_cents"] = -100
        observed["concurrent_consumption"]["final_balance_cents"] = -100
        checks = e3_expected(observed)
        self.assertEqual(
            outcome_of(checks, "concurrent_consumption_within_balance"), "fail")

    def test_a_rejected_event_makes_the_run_incomplete(self):
        observed = base_observed()
        observed["concurrent_consumption"]["events_accepted"] = 5
        checks = e3_expected(observed)
        self.assertEqual(
            outcome_of(checks, "concurrent_consumption_within_balance"), "fail")


class VoidVsConsumeTests(unittest.TestCase):
    def test_void_and_consume_cannot_both_take_the_same_remainder(self):
        checks = e3_expected(base_observed())
        self.assertEqual(outcome_of(checks, "void_vs_consume_exclusive"), "pass")

    def test_combined_draw_plus_void_over_granted_is_a_failure(self):
        observed = base_observed()
        observed["void_vs_consume"] = {
            "granted_cents": 10000, "drawn_cents": 8000, "voided_cents": 4000,
            "final_balance_cents": -2000, "balance_never_negative": False,
        }
        checks = e3_expected(observed)
        self.assertEqual(outcome_of(checks, "void_vs_consume_exclusive"), "fail")

    def test_void_wins_and_consume_draws_nothing_is_a_pass(self):
        observed = base_observed()
        observed["void_vs_consume"] = {
            "granted_cents": 10000, "drawn_cents": 0, "voided_cents": 10000,
            "final_balance_cents": 0, "balance_never_negative": True,
        }
        checks = e3_expected(observed)
        self.assertEqual(outcome_of(checks, "void_vs_consume_exclusive"), "pass")


class ExpiredDuringRunTests(unittest.TestCase):
    def test_expiry_mid_run_never_double_draws(self):
        checks = e3_expected(base_observed())
        self.assertEqual(outcome_of(checks, "expired_batch_never_double_drawn"),
                         "pass")

    def test_total_pre_plus_post_draw_over_granted_is_a_failure(self):
        observed = base_observed()
        observed["expired_during"]["total_drawn_cents"] = 12000
        observed["expired_during"]["remaining_cents"] = -2000
        checks = e3_expected(observed)
        self.assertEqual(outcome_of(checks, "expired_batch_never_double_drawn"),
                         "fail")


class ModuleContractTests(unittest.TestCase):
    def test_module_exports_runner_and_slug(self):
        self.assertEqual(e3_concurrency.SLUG, "e3-concurrency")
        self.assertTrue(callable(e3_concurrency.RUNNER))


if __name__ == "__main__":
    unittest.main()
