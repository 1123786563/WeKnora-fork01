"""Behavioral tests for the deterministic Pricing Group fixture.

The fixture must reproduce, with integer/Decimal arithmetic only, the
pricing semantics of Lago Community v1.53.0 (getlago/lago-api @ 591ae90):
``standard`` = units x BigDecimal(amount) and ``package`` =
ceil((units - free_units) / package_size) x BigDecimal(amount), where the
charge ``amount`` is a decimal *string* on the wire.
"""

import json
import unittest
import uuid
from datetime import datetime, timedelta, timezone
from decimal import Decimal

import fixture


def _run_id():
    return f"{fixture.RUN_ID_PREFIX}{uuid.uuid4()}"


class BuildRunTests(unittest.TestCase):
    def test_rejects_run_ids_without_the_isolation_prefix(self):
        with self.assertRaises(ValueError):
            fixture.build_run("someone-elses-run", task_count=2)

    def test_reference_task_expected_amount_is_exact_integer_cents(self):
        run = fixture.build_run("weknora-t04-deadbeef", task_count=2)
        exp = fixture.expected_usage(run.tasks[0])
        self.assertEqual(exp.charge("model-units").amount_cents, 7000)
        self.assertEqual(exp.charge("tool-calls").amount_cents, 4500)
        self.assertEqual(exp.total_amount_cents, 11500)

    def test_reference_task_units_and_events_count(self):
        run = fixture.build_run("weknora-t04-deadbeef", task_count=2)
        exp = fixture.expected_usage(run.tasks[0])
        self.assertEqual(exp.charge("model-units").units, Decimal("1000"))
        self.assertEqual(exp.charge("model-units").events_count, 10)
        self.assertEqual(exp.charge("tool-calls").units, Decimal("250"))
        self.assertEqual(exp.charge("tool-calls").events_count, 5)

    def test_expected_amounts_are_plain_ints_never_floats(self):
        run = fixture.build_run(_run_id(), task_count=3)
        for task in run.tasks:
            exp = fixture.expected_usage(task)
            for charge in exp.charges:
                self.assertIsInstance(charge.amount_cents, int)
                self.assertIsInstance(charge.events_count, int)
            self.assertIsInstance(exp.total_amount_cents, int)

    def test_all_external_ids_carry_the_prefix_and_are_unique_within_a_run(self):
        run = fixture.build_run(_run_id(), task_count=5)
        ids = [run.run_id]
        ids += [metric.code for metric in run.metrics]
        ids += [run.plan.code]
        for task in run.tasks:
            ids.append(task.subscription_external_id)
            ids += [event.transaction_id for event in task.events]
        ids.append(fixture.negative_control_event(run.run_id).transaction_id)
        for value in ids:
            self.assertTrue(
                value.startswith(fixture.RUN_ID_PREFIX),
                f"id without prefix: {value}",
            )
        self.assertEqual(len(ids), len(set(ids)))

    def test_load_tasks_have_twenty_events_and_deterministic_amounts(self):
        run_id = _run_id()
        first = fixture.build_run(run_id, task_count=4)
        second = fixture.build_run(run_id, task_count=4)
        for task, again in zip(first.tasks[1:], second.tasks[1:]):
            self.assertEqual(len(task.events), 20)
            self.assertEqual(
                fixture.expected_usage(task), fixture.expected_usage(again)
            )
        # load task k=10 sits exactly on a package bundle boundary (100 calls)
        with_k10 = fixture.build_run(_run_id(), task_count=11)
        task_k10 = with_k10.tasks[10]
        exp = fixture.expected_usage(task_k10)
        self.assertEqual(exp.charge("tool-calls").units, Decimal("100"))
        self.assertEqual(exp.charge("tool-calls").amount_cents, 1500)

    def test_charge_amount_constants_are_derived_not_duplicated(self):
        # The decimal string sent to Lago and the local integer expectation
        # must come from the same constant, so they cannot drift apart.
        self.assertEqual(fixture.STANDARD_AMOUNT_DECIMAL, str(fixture.STANDARD_AMOUNT_CENTS))
        self.assertEqual(fixture.PACKAGE_AMOUNT_DECIMAL, str(fixture.PACKAGE_AMOUNT_CENTS))
        run = fixture.build_run(_run_id(), task_count=2)
        standard = run.plan.charges[0]
        package = run.plan.charges[1]
        self.assertEqual(standard.charge_model, "standard")
        self.assertEqual(standard.amount_decimal, fixture.STANDARD_AMOUNT_DECIMAL)
        self.assertEqual(package.charge_model, "package")
        self.assertEqual(package.amount_decimal, fixture.PACKAGE_AMOUNT_DECIMAL)
        self.assertEqual(package.package_size, fixture.PACKAGE_SIZE)
        self.assertEqual(package.free_units, fixture.FREE_UNITS)
        self.assertEqual(run.plan.currency, "CNY")
        self.assertEqual(run.plan.interval, "monthly")
        self.assertEqual(run.plan.amount_cents, 0)


class PackageCeilBoundaryTests(unittest.TestCase):
    def _tool_task(self, calls_values):
        run = fixture.build_run("weknora-t04-ceilprobe", task_count=1)
        events = tuple(
            fixture.UsageEvent(
                transaction_id=f"weknora-t04-ceilprobe-tool-{i}",
                code=run.metrics[1].code,
                offset_seconds=i,
                properties={"calls": value},
            )
            for i, value in enumerate(calls_values)
        )
        return fixture.Task(
            index=0,
            subscription_external_id="weknora-t04-ceilprobe-task-0",
            events=events,
        )

    def test_exactly_one_bundle_worth_of_units_is_one_bundle(self):
        exp = fixture.expected_usage(self._tool_task([100]))
        self.assertEqual(exp.charge("tool-calls").amount_cents, 1500)

    def test_one_unit_over_a_bundle_rounds_up_to_two_bundles(self):
        exp = fixture.expected_usage(self._tool_task([101]))
        self.assertEqual(exp.charge("tool-calls").amount_cents, 3000)

    def test_zero_units_is_zero_bundles(self):
        exp = fixture.expected_usage(self._tool_task([0, 0]))
        self.assertEqual(exp.charge("tool-calls").amount_cents, 0)


class SerializationTests(unittest.TestCase):
    def test_serialized_run_spec_contains_no_float_literals(self):
        run = fixture.build_run(_run_id(), task_count=3)
        data = json.loads(json.dumps(run.to_dict()))

        def walk(node):
            self.assertNotIsInstance(node, float)
            if isinstance(node, dict):
                for value in node.values():
                    walk(value)
            elif isinstance(node, list):
                for value in node:
                    walk(value)

        walk(data)

    def test_run_spec_round_trips_through_json(self):
        run = fixture.build_run(_run_id(), task_count=3)
        restored = fixture.RunSpec.from_dict(json.loads(json.dumps(run.to_dict())))
        self.assertEqual(restored, run)


class EventMaterializationTests(unittest.TestCase):
    def test_event_payloads_carry_transaction_id_and_integer_properties(self):
        run = fixture.build_run("weknora-t04-tsprobe", task_count=2)
        task = run.tasks[0]
        base = datetime(2026, 9, 21, 12, 0, 0, tzinfo=timezone.utc)
        payloads = [
            fixture.event_payload(event, task.subscription_external_id, base)
            for event in task.events
        ]
        self.assertEqual(len(payloads), 15)
        for event, payload in zip(task.events, payloads):
            self.assertEqual(payload["transaction_id"], event.transaction_id)
            self.assertEqual(payload["external_subscription_id"], task.subscription_external_id)
            self.assertEqual(payload["code"], event.code)
            for value in payload["properties"].values():
                self.assertIsInstance(value, int)

    def test_timestamps_are_monotonic_and_inside_the_current_period(self):
        run = fixture.build_run("weknora-t04-tsprobe", task_count=2)
        task = run.tasks[1]  # a 20-event load task exercises the widest span
        base = datetime(2026, 9, 21, 12, 0, 0, tzinfo=timezone.utc)
        stamps = [
            datetime.fromisoformat(
                fixture.event_payload(event, task.subscription_external_id, base)["timestamp"]
            )
            for event in task.events
        ]
        self.assertEqual(stamps, sorted(stamps))
        self.assertTrue(all(base <= stamp <= base + timedelta(seconds=60) for stamp in stamps))


class NegativeControlAndConflictTests(unittest.TestCase):
    def test_negative_control_event_uses_a_legal_metric_but_unknown_subscription(self):
        run = fixture.build_run("weknora-t04-negprobe", task_count=2)
        event = fixture.negative_control_event(run.run_id)
        known = {task.subscription_external_id for task in run.tasks}
        self.assertNotIn(event.external_subscription_id, known)
        self.assertTrue(event.external_subscription_id.startswith(fixture.RUN_ID_PREFIX))
        self.assertEqual(event.code, run.metrics[0].code)  # a legal metric code
        self.assertTrue(event.transaction_id.startswith(fixture.RUN_ID_PREFIX))

    def test_conflicting_variant_keeps_identity_but_changes_integer_content(self):
        run = fixture.build_run("weknora-t04-confprobe", task_count=2)
        for event in run.tasks[0].events:
            variant = fixture.conflicting_variant(event)
            self.assertEqual(variant.transaction_id, event.transaction_id)
            self.assertEqual(variant.code, event.code)
            self.assertNotEqual(variant.properties, event.properties)
            for key, value in variant.properties.items():
                self.assertIsInstance(value, int)
                self.assertNotEqual(value, event.properties[key])


if __name__ == "__main__":
    unittest.main()


class AggregationContractTests(unittest.TestCase):
    def test_metrics_use_the_v1_53_0_sum_agg_enum(self):
        run = fixture.build_run(_run_id(), task_count=2)
        for metric in run.metrics:
            self.assertEqual(metric.aggregation_type, "sum_agg")
            self.assertIn(metric.field_name, ("units", "calls"))
