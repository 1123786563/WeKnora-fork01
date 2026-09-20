"""Offline pre-registered expected-outcome tests for experiment E1 (expiry).

``e1_expected(observed)`` turns one runtime observation into check rows with
pass/fail outcomes fixed BEFORE any real run, from the pinned-source research
basis: lazy wallet-level termination via the hourly clock (TerminateWalletsJob
at *:45), no carry-over on termination, interval rules topping up the same
persistent wallet, and the six-active-wallets-per-customer cap.
"""

import unittest

if __package__ in (None, ""):
    import sys
    from pathlib import Path

    sys.path.insert(0, str(Path(__file__).resolve().parent))

from experiments import e1_expiry
from experiments.e1_expiry import e1_expected, outcome_of


def base_observed(**overrides):
    """The all-good E1 baseline: every pre-registered expectation holds."""
    observed = {
        "monthly": {
            "code": "weknora-t03-wallet-month", "status_before": "active",
            "status_after": "terminated", "expiration_at": "2026-09-21T01:03:00Z",
            "granted_cents": 10000, "balance_before_boundary_cents": 6000,
            "balance_after_termination_cents": 6000,
        },
        "race_window": {
            "wallet_status_at_consumption": "active",
            "expiration_at_passed": True,
            "consumed_after_expiry_passed": True,
            "drawn_cents": 4000,
        },
        "termination": {"terminated": True, "elapsed_seconds": 1800,
                        "poll_window_seconds": 4500},
        "post_boundary": {
            "invoice_amount_cents": 5000, "monthly_drawn_cents": 0,
            "topup_drawn_cents": 5000, "topup_status": "active",
        },
        "resurrection": {
            "attempted": True, "http_status": 404,
            "error_code": "wallet_not_found", "rejected": True,
            "balance_after_cents": 6000,
        },
        "interval": {
            "wallet_status": "active", "has_expiration": False,
            "balance_before_cents": 3000, "balance_after_cents": 3000,
            "rule_create": {"attempted": True, "http_status": 500,
                            "error_code": "Internal Server Error",
                            "rule_created": False},
        },
        "cap": {
            "active_created": 6, "limit_status": 422,
            "limit_error_code": "wallet_limit_reached",
            "after_terminate_status": 200,
        },
    }
    observed.update(overrides)
    return observed


class ExpiryTerminationTests(unittest.TestCase):
    def test_expired_but_unterminated_wallet_is_reported_as_a_race_window(self):
        observed = base_observed()
        checks = e1_expected(observed)
        # The pre-registered expectation: lazy termination leaves an expired
        # wallet consumable until the clock runs -- that violates the
        # invariant and MUST surface as a failing check.
        self.assertEqual(
            outcome_of(checks, "expired_wallet_not_consumable"), "fail")

    def test_no_consumption_from_expired_wallet_passes_the_invariant(self):
        observed = base_observed()
        observed["race_window"] = {
            "wallet_status_at_consumption": "active",
            "expiration_at_passed": True,
            "consumed_after_expiry_passed": False, "drawn_cents": 0,
        }
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "expired_wallet_not_consumable"), "pass")

    def test_monthly_wallet_terminates_within_the_clock_window(self):
        checks = e1_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "monthly_wallet_terminates_on_expiry"), "pass")

    def test_clock_never_terminating_is_a_failure(self):
        observed = base_observed()
        observed["termination"] = {"terminated": False, "elapsed_seconds": None,
                                   "poll_window_seconds": 4500}
        observed["monthly"]["status_after"] = "active"
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "monthly_wallet_terminates_on_expiry"), "fail")


class NoCarryOverTests(unittest.TestCase):
    def test_monthly_remainder_not_consumable_after_termination(self):
        checks = e1_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "monthly_remainder_not_consumable_after_termination"),
            "pass")

    def test_consumption_touching_terminated_remainder_fails(self):
        observed = base_observed()
        observed["post_boundary"]["monthly_drawn_cents"] = 4000
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "monthly_remainder_not_consumable_after_termination"),
            "fail")

    def test_terminated_wallet_rejects_new_grants(self):
        checks = e1_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "terminated_wallet_rejects_new_grant"), "pass")

    def test_new_grant_resurrecting_a_terminated_wallet_fails(self):
        observed = base_observed()
        observed["resurrection"] = {
            "attempted": True, "http_status": 200, "error_code": None,
            "rejected": False, "balance_after_cents": 11000,
        }
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "terminated_wallet_rejects_new_grant"), "fail")

    def test_topup_batch_survives_the_monthly_boundary(self):
        checks = e1_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "topup_batch_survives_monthly_boundary"), "pass")

    def test_topup_not_consumable_after_boundary_fails(self):
        observed = base_observed()
        observed["post_boundary"]["topup_drawn_cents"] = 0
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "topup_batch_survives_monthly_boundary"), "fail")


class IntervalRuleTests(unittest.TestCase):
    def test_interval_wallet_keeps_leftover_as_mapping_negative(self):
        checks = e1_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "interval_rule_wallet_keeps_leftover"), "pass")

    def test_interval_wallet_losing_leftover_fails(self):
        observed = base_observed()
        observed["interval"]["balance_after_cents"] = 0
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "interval_rule_wallet_keeps_leftover"), "fail")

    def test_interval_rule_creation_500_is_reported_as_community_finding(self):
        # Pre-registered after the first real run: Community v1.53.0 cannot
        # create wallet recurring rules (Premium-gated service returns nil and
        # the caller crashes) -- the observation confirms the pre-registration.
        checks = e1_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "interval_rule_create_fails_in_community"), "pass")

    def test_interval_rule_creation_succeeding_is_a_failure(self):
        # The pre-registration says Community cannot create the rule; a 200
        # would CONTRADICT the recorded expectation and must surface as fail.
        observed = base_observed()
        observed["interval"]["rule_create"] = {
            "attempted": True, "http_status": 200, "error_code": None,
            "rule_created": True}
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "interval_rule_create_fails_in_community"), "fail")


class CapProbeTests(unittest.TestCase):
    def test_six_active_wallet_limit_enforced(self):
        checks = e1_expected(base_observed())
        self.assertEqual(
            outcome_of(checks, "cap_six_active_wallets_per_customer"), "pass")

    def test_seventh_wallet_succeeding_is_a_failure(self):
        observed = base_observed()
        observed["cap"] = {"active_created": 7, "limit_status": 200,
                           "limit_error_code": None,
                           "after_terminate_status": 200}
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "cap_six_active_wallets_per_customer"), "fail")

    def test_wrong_error_code_is_a_failure(self):
        observed = base_observed()
        observed["cap"]["limit_error_code"] = "validation_error"
        checks = e1_expected(observed)
        self.assertEqual(
            outcome_of(checks, "cap_six_active_wallets_per_customer"), "fail")

    def test_termination_frees_a_cap_slot(self):
        checks = e1_expected(base_observed())
        self.assertEqual(outcome_of(checks, "cap_freed_by_termination"), "pass")

    def test_slot_not_freed_is_a_failure(self):
        observed = base_observed()
        observed["cap"]["after_terminate_status"] = 422
        checks = e1_expected(observed)
        self.assertEqual(outcome_of(checks, "cap_freed_by_termination"), "fail")


class ModuleContractTests(unittest.TestCase):
    def test_module_exports_runner_and_slug(self):
        self.assertEqual(e1_expiry.SLUG, "e1-expiry")
        self.assertTrue(callable(e1_expiry.RUNNER))


if __name__ == "__main__":
    unittest.main()
