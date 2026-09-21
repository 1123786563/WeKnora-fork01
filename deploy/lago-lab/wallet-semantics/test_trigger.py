"""Offline tests for the shared consumption-trigger payloads and parsers.

The consumption trigger (confirmed against the pinned lago-api source at
591ae9005110346f1c6034ec72ea9046625668cf: ``POST /api/v1/events`` with a
``pay_in_advance`` + ``invoiceable`` charge runs
``Invoices::CreatePayInAdvanceChargeService`` which calls
``Credits::AppliedPrepaidCreditsService``) is pure payload-building plus
response parsing, so it is fully testable offline.
"""

import unittest
from decimal import Decimal

if __package__ in (None, ""):
    import sys
    from pathlib import Path

    sys.path.insert(0, str(Path(__file__).resolve().parent))

from experiments import trigger


class MoneyConversionTests(unittest.TestCase):
    def test_credits_to_cents_and_back_without_floats(self):
        self.assertEqual(trigger.credits_to_cents("100"), 10000)
        self.assertEqual(trigger.credits_to_cents("100.5"), 10050)
        self.assertEqual(trigger.credits_to_cents("0.01"), 1)
        self.assertEqual(trigger.cents_to_credits(10050), "100.5")
        self.assertEqual(trigger.cents_to_credits(21000), "210")
        with self.assertRaises(ValueError):
            trigger.credits_to_cents("1.005")  # sub-cent precision is rejected
        with self.assertRaises(ValueError):
            trigger.credits_to_cents("nan")

    def test_event_units_map_whole_dollar_cents_to_units(self):
        self.assertEqual(trigger.event_units_for_cents(21000), 210)
        self.assertEqual(trigger.event_units_for_cents(5000), 50)
        with self.assertRaises(ValueError):
            trigger.event_units_for_cents(50)  # not a whole 1.00-unit multiple


class PayloadBuilderTests(unittest.TestCase):
    def test_metric_is_sum_agg_over_units_field(self):
        payload = trigger.metric_payload(code="weknora-t03-m-x", name="consume")
        metric = payload["billable_metric"]
        self.assertEqual(metric["code"], "weknora-t03-m-x")
        self.assertEqual(metric["aggregation_type"], "sum_agg")
        self.assertEqual(metric["field_name"], "units")

    def test_plan_carries_pay_in_advance_invoiceable_standard_charge(self):
        payload = trigger.plan_payload(
            code="weknora-t03-plan-x", name="lab plan",
            metric_lago_id="b3b1c0de-0000-0000-0000-000000000000",
        )
        plan = payload["plan"]
        self.assertEqual(plan["code"], "weknora-t03-plan-x")
        self.assertEqual(plan["interval"], "weekly")
        self.assertIs(plan["pay_in_advance"], False)  # required plan-level boolean
        charge = plan["charges"][0]
        # v1.53.0 resolves embedded charges by lago_id, not by code.
        self.assertEqual(charge["billable_metric_id"],
                         "b3b1c0de-0000-0000-0000-000000000000")
        self.assertTrue(charge["pay_in_advance"])
        self.assertTrue(charge["invoiceable"])
        self.assertEqual(charge["charge_model"], "standard")
        self.assertEqual(charge["properties"]["amount"], "1.00")

    def test_event_payload_carries_unique_transaction_id_and_units(self):
        payload = trigger.event_payload(
            transaction_id="txn-1", external_customer_id="weknora-t03-c-1",
            metric_code="weknora-t03-m-x", cents=21000,
            external_subscription_id="weknora-t03-sub-1",
        )
        event = payload["event"]
        self.assertEqual(event["transaction_id"], "txn-1")
        self.assertEqual(event["external_customer_id"], "weknora-t03-c-1")
        self.assertEqual(event["code"], "weknora-t03-m-x")
        self.assertEqual(event["properties"]["units"], 210)
        # Required: without it the pay-in-advance path resolves no subscription.
        self.assertEqual(event["external_subscription_id"], "weknora-t03-sub-1")


class ResponseParserTests(unittest.TestCase):
    def test_parse_wallet_response_exposes_integer_balance_cents(self):
        wallet = trigger.parse_wallet_response({
            "lago_id": "w-1", "code": "weknora-t03-wallet-a", "status": "active",
            "priority": 2, "balance": "50.5", "expiration_at": "2026-09-21T00:00:00Z",
            "granted_credits": "100",
        })
        self.assertEqual(wallet["lago_id"], "w-1")
        self.assertEqual(wallet["balance_cents"], 5050)
        self.assertEqual(wallet["status"], "active")
        self.assertEqual(wallet["priority"], 2)
        self.assertEqual(wallet["expiration_at"], "2026-09-21T00:00:00Z")

    def test_parse_wallet_response_tolerates_missing_optional_fields(self):
        wallet = trigger.parse_wallet_response({"lago_id": "w-2", "balance": "0"})
        self.assertIsNone(wallet["status"])
        self.assertIsNone(wallet["priority"])
        self.assertIsNone(wallet["expiration_at"])
        self.assertEqual(wallet["balance_cents"], 0)

    def test_parse_transactions_preserves_remaining_amount_cents(self):
        rows = trigger.parse_transactions_response({"wallet_transactions": [
            {"lago_id": "t-1", "transaction_type": "inbound", "transaction_status": "granted",
             "status": "settled", "amount": "100", "remaining_amount_cents": 4000,
             "created_at": "2026-09-21T01:00:00Z", "metadata": [{"key": "weknora_t03_batch", "value": "b1"}]},
            {"lago_id": "t-2", "transaction_type": "outbound", "transaction_status": "invoiced",
             "status": "settled", "amount": "-40", "remaining_amount_cents": None,
             "created_at": "2026-09-21T02:00:00Z", "metadata": []},
        ]})
        self.assertEqual(rows[0]["lago_id"], "t-1")
        self.assertEqual(rows[0]["remaining_amount_cents"], 4000)
        self.assertEqual(rows[1]["amount_cents"], -4000)
        self.assertEqual(rows[1]["transaction_status"], "invoiced")

    def test_drawn_from_wallet_is_the_positive_balance_delta(self):
        self.assertEqual(trigger.drawn_from_wallet(10000, 6000), 4000)
        self.assertEqual(trigger.drawn_from_wallet(10000, 10000), 0)


if __name__ == "__main__":
    unittest.main()
