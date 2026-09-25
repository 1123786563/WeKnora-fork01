"""Pure-function unit tests for the T10 probe payloads and predicates.

These never touch a live stack: they pin the exact wire shapes the #82
implementation depends on (F3/F5/F8) so a regression fails here before it
can fail on the pinned runtime.
"""

from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import fixtures

_LAB_DIR = Path(__file__).resolve().parent
_PA_DIR = _LAB_DIR.parent / "payment-activation"


class GatedSubscriptionPayloadTest(unittest.TestCase):
    def test_carries_payment_rule_with_timeout_zero(self):
        payload = fixtures.gated_subscription_payload("cust-1", "plan-1", "sub-1")
        sub = payload["subscription"]
        self.assertEqual(sub["external_customer_id"], "cust-1")
        self.assertEqual(sub["plan_code"], "plan-1")
        self.assertEqual(sub["external_id"], "sub-1")
        self.assertEqual(
            sub["activation_rules"],
            [{"type": "payment", "timeout_hours": 0}],
            "the gate MUST carry timeout_hours 0 (F8: the authority never auto-cancels)",
        )

    def test_rule_shape_survives_custom_args(self):
        payload = fixtures.gated_subscription_payload("c", "p", "s")
        self.assertEqual(payload["subscription"]["external_id"], "s")


class RetryBodyTest(unittest.TestCase):
    def test_retry_body_is_exactly_empty(self):
        self.assertEqual(
            fixtures.retry_body(), {},
            "the trigger link under test resolves the payment method on the "
            "authority side; the body must not name one",
        )


class SecondPmPresentTest(unittest.TestCase):
    def test_two_methods_answer_true_immediately(self):
        self.assertTrue(fixtures.second_pm_present([{"id": "pm_old"}, {"id": "pm_new"}]))

    def test_three_methods_answer_true(self):
        self.assertTrue(fixtures.second_pm_present([{"id": 1}, {"id": 2}, {"id": 3}]))

    def test_one_method_answers_false(self):
        self.assertFalse(fixtures.second_pm_present([{"id": "pm_old"}]))

    def test_non_list_answers_false(self):
        self.assertFalse(fixtures.second_pm_present(None))
        self.assertFalse(fixtures.second_pm_present({}))


class PaymentRowHelpersTest(unittest.TestCase):
    ROWS = [
        {"lago_id": "pay_1", "status": "processing",
         "provider_payment_id": "pi_1", "invoice_ids": ["inv_g1"]},
        {"lago_id": "pay_2", "status": "succeeded",
         "provider_payment_id": "pi_2", "invoice_ids": ["inv_g2"]},
        {"lago_id": "pay_3", "status": "failed", "invoice_ids": []},
    ]

    def test_unsettled_filters_by_status(self):
        got = [p["lago_id"] for p in fixtures.unsettled_payments(self.ROWS)]
        self.assertEqual(got, ["pay_1", "pay_3"])

    def test_succeeded_counter(self):
        self.assertEqual(fixtures.succeeded_payments(self.ROWS), [self.ROWS[1]])
        self.assertEqual(fixtures.succeeded_payments([]), [])
        self.assertEqual(fixtures.succeeded_payments(None), [])

    def test_first_unsettled_requires_invoice_ids(self):
        row = fixtures.first_unsettled_payment(self.ROWS)
        self.assertEqual(row["lago_id"], "pay_1")
        # A row without invoice_ids is never eligible (F5 breakage shape).
        self.assertIsNone(fixtures.first_unsettled_payment([self.ROWS[2]]))
        self.assertIsNone(fixtures.first_unsettled_payment([]))


class StripeIntentHelpersTest(unittest.TestCase):
    INTENTS = [
        {"id": "pi_ok", "status": "requires_action",
         "metadata": {"lago_invoice_id": "0f0e-uuid", "lago_customer_id": "cus-uuid"}},
        {"id": "pi_done", "status": "succeeded",
         "metadata": {"lago_invoice_id": "other"}},
        {"id": "pi_dead", "status": "canceled", "metadata": {}},
    ]

    def test_unsettled_filters_terminal_statuses(self):
        got = [i["id"] for i in fixtures.unsettled_intents(self.INTENTS)]
        self.assertEqual(got, ["pi_ok"])

    def test_unsettled_handles_none(self):
        self.assertEqual(fixtures.unsettled_intents(None), [])
        self.assertEqual(fixtures.unsettled_intents([]), [])

    def test_intent_invoice_id_reads_metadata(self):
        self.assertEqual(fixtures.intent_invoice_id(self.INTENTS[0]), "0f0e-uuid")

    def test_intent_invoice_id_requires_nonempty_string(self):
        self.assertIsNone(fixtures.intent_invoice_id({"metadata": {"lago_invoice_id": ""}}))
        self.assertIsNone(fixtures.intent_invoice_id({"metadata": {}}))
        self.assertIsNone(fixtures.intent_invoice_id(None))


class IntentCancelableTest(unittest.TestCase):
    def test_requires_action_is_cancelable(self):
        self.assertTrue(fixtures.intent_cancelable("requires_action"))

    def test_processing_is_cancelable(self):
        self.assertTrue(fixtures.intent_cancelable("processing"))

    def test_terminal_states_are_not(self):
        for status in ("succeeded", "canceled", "failed"):
            self.assertFalse(fixtures.intent_cancelable(status), status)
        self.assertFalse(fixtures.intent_cancelable(None))


class FinalizedInvoiceTest(unittest.TestCase):
    def test_picks_finalized_subscription_invoice(self):
        rows = [
            {"lago_id": "inv_x", "status": "open", "invoice_type": "subscription"},
            {"lago_id": "inv_1", "status": "finalized", "invoice_type": "subscription",
             "number": "WK-001", "payment_status": "succeeded"},
        ]
        self.assertEqual(fixtures.finalized_invoice(rows)["lago_id"], "inv_1")

    def test_tolerates_absent_invoice_type(self):
        rows = [{"lago_id": "inv_1", "status": "finalized"}]
        self.assertEqual(fixtures.finalized_invoice(rows)["lago_id"], "inv_1")

    def test_none_when_only_open(self):
        rows = [{"lago_id": "inv_x", "status": "open"}]
        self.assertIsNone(fixtures.finalized_invoice(rows))
        self.assertIsNone(fixtures.finalized_invoice([]))
        self.assertIsNone(fixtures.finalized_invoice(None))


class ReexportTest(unittest.TestCase):
    def test_pa_fixtures_shapes_reexported(self):
        payload = fixtures.plan_payload("p-1")
        self.assertIn("plan", payload)
        cust = fixtures.customer_payload("c-1", "n", "pcus-1")
        self.assertEqual(
            cust["customer"]["billing_configuration"]["provider_customer_id"], "pcus-1")
        self.assertIn("addStripePaymentProvider", fixtures.ADD_STRIPE_PROVIDER_QUERY)


# F1 regression (review round 1): importing ``phases`` outside the run_lab.py
# import order used to bind the bare name ``fixtures`` to the payment-
# activation lab's module (which lacks every probe helper), so all phases
# died with AttributeError at runtime. The binding must never depend on
# sys.path order. Verified in a subprocess with this directory first on
# sys.path and cwd outside the repo — the exact order under which phases'
# own ``sys.path.insert(0, _PA_DIR)`` pushed the WRONG fixtures.py to the
# front of the search path.
_PROBE_HELPERS = (
    "gated_subscription_payload", "retry_body", "second_pm_present",
    "intent_cancelable", "finalized_invoice", "succeeded_payments",
    "unsettled_intents", "intent_invoice_id",
)


class StandaloneImportTest(unittest.TestCase):
    def _probe(self, setup_lines: str) -> str:
        lab_dir = str(_LAB_DIR)
        code = (
            "import sys\n"
            f"{setup_lines}\n"
            "import phases\n"
            "assert phases.fixtures.__file__ is not None\n"
            f"helpers = {list(_PROBE_HELPERS)!r}\n"
            "missing = [h for h in helpers if not hasattr(phases.fixtures, h)]\n"
            "assert not missing, missing\n"
            "print(phases.fixtures.__file__)\n"
            "print(phases.clients.__file__)\n"
        )
        result = subprocess.run(
            [sys.executable, "-c", code], capture_output=True, text=True,
            cwd=tempfile.gettempdir(),
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout

    def test_import_with_this_dir_first_binds_probe_fixtures(self):
        out = self._probe(f"sys.path.insert(0, {str(_LAB_DIR)!r})")
        self.assertTrue(
            out.splitlines()[0].startswith(str(_LAB_DIR)),
            f"fixtures must resolve to THIS probe's module, got {out!r}")

    def test_run_lab_import_order_binds_probe_fixtures_and_shared_clients(self):
        # run_lab.py: sys.path.insert(0, LAB_DIR) then append(_PA_DIR); bare
        # `import clients` must stay the SAME module object for both run_lab
        # and phases (no double-load of the HTTP clients).
        setup = (
            f"sys.path.insert(0, {str(_LAB_DIR)!r})\n"
            f"sys.path.append({str(_PA_DIR)!r})\n"
            "import clients\n"
            "same = clients is sys.modules['clients']\n"
        )
        out = self._probe(setup)
        self.assertTrue(
            out.splitlines()[0].startswith(str(_LAB_DIR)),
            f"fixtures must resolve to THIS probe's module, got {out!r}")
        self.assertTrue(
            out.splitlines()[1].startswith(str(_PA_DIR)),
            f"clients must resolve to the payment-activation module, got {out!r}")


if __name__ == "__main__":
    unittest.main()
