#!/usr/bin/env python3
"""Focused runner tests for source-owned helper-start acknowledgment."""
import importlib.util
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("run_v2.py")
spec = importlib.util.spec_from_file_location("run_v2_source_test", SCRIPT)
run_v2 = importlib.util.module_from_spec(spec)
spec.loader.exec_module(run_v2)


class HelperStartBoundaryTests(unittest.TestCase):
    def receipt(self, argv=None, exit_code=0, timed_out=False):
        return {
            "argv": argv or ["docker", "start", "e0-helper"],
            "exit_code": exit_code,
            "timed_out": timed_out,
            "container_id": "a" * 64,
            "peer_ip": "172.20.0.9",
        }

    def test_successful_start_is_logged_before_both_matching_source_acks(self):
        order = []
        commands = []
        records = []

        def start(name, op_id, ordinal):
            order.append(("start", name, op_id, ordinal))
            return self.receipt()

        def send(source, request):
            order.append(("ack", source))
            commands.append((source, request))
            return {
                "ok": True,
                "source": source,
                "phase_id": "positive-controls",
                "boundary": "helper_start",
                "operation_id": request["operation_id"],
                "controller_ordinal": request["controller_ordinal"],
                "stream_cursor": 7 if source == "direct" else 4,
            }

        def record(row):
            order.append(("record", row["operation_id"]))
            records.append(row)

        def dispatch():
            order.append(("probe", "after-both-acks"))
            return {"exit_code": 0}

        result = run_v2.start_helper_and_ack(
            "positive-controls",
            "e0-helper",
            operation_id="run:docker_start:17",
            controller_ordinal=17,
            start_operation=start,
            send_source=send,
            record_operation=record,
            dispatch=dispatch,
        )

        self.assertEqual([item[0] for item in order], ["start", "record", "ack", "ack", "probe"])
        self.assertEqual([source for source, _ in commands], ["direct", "adapter"])
        for _, request in commands:
            self.assertEqual(request, {
                "op": "helper_start",
                "phase_id": "positive-controls",
                "operation_id": "run:docker_start:17",
                "controller_ordinal": 17,
            })
        self.assertEqual(len(records), 1)
        self.assertEqual(records[0]["operation"], "docker_start")
        self.assertEqual(records[0]["argv"], ["docker", "start", "e0-helper"])
        self.assertEqual(result["source_cursors"], {"direct": 7, "adapter": 4})
        self.assertEqual(result["probe_result"], {"exit_code": 0})

    def test_failed_or_mismatched_host_start_never_sends_source_ack(self):
        calls = []

        def start(name, op_id, ordinal):
            return self.receipt(exit_code=1)

        with self.assertRaisesRegex(RuntimeError, "helper start did not succeed"):
            run_v2.start_helper_and_ack(
                "phase-failed", "e0-helper", operation_id="start-failed", controller_ordinal=2,
                start_operation=start, send_source=lambda *_: calls.append("ack"),
                record_operation=lambda _: None,
            )
        self.assertEqual(calls, [])

        def wrong_argv(name, op_id, ordinal):
            return self.receipt(argv=["docker", "run", "--rm", "some-other-helper"])

        with self.assertRaisesRegex(RuntimeError, "does not match the registered helper"):
            run_v2.start_helper_and_ack(
                "phase-wrong", "e0-helper", operation_id="start-wrong", controller_ordinal=3,
                start_operation=wrong_argv, send_source=lambda *_: calls.append("ack"),
                record_operation=lambda _: None,
            )
        self.assertEqual(calls, [])

    def test_one_sided_or_forged_ack_fails_before_helper_traffic_can_start(self):
        calls = []
        dispatched = []

        def start(name, op_id, ordinal):
            return self.receipt()

        def one_sided(source, request):
            calls.append(source)
            if source == "adapter":
                return {"ok": False, "source": source}
            return {
                "ok": True, "source": source, "phase_id": request["phase_id"],
                "boundary": "helper_start", "operation_id": request["operation_id"],
                "controller_ordinal": request["controller_ordinal"], "stream_cursor": 9,
            }

        with self.assertRaisesRegex(RuntimeError, "adapter helper-start acknowledgment"):
            run_v2.start_helper_and_ack(
                "phase-one-sided", "e0-helper", operation_id="start-one-sided", controller_ordinal=4,
                start_operation=start, send_source=one_sided, record_operation=lambda _: None,
                dispatch=lambda: dispatched.append("probe"),
            )
        self.assertEqual(calls, ["direct", "adapter"])
        self.assertEqual(dispatched, [])

        def forged(source, request):
            return {
                "ok": True, "source": source, "phase_id": request["phase_id"],
                "boundary": "helper_start", "operation_id": "forged-start",
                "controller_ordinal": request["controller_ordinal"], "stream_cursor": 10,
            }

        with self.assertRaisesRegex(RuntimeError, "direct helper-start acknowledgment identity mismatch"):
            run_v2.start_helper_and_ack(
                "phase-forged", "e0-helper", operation_id="start-real", controller_ordinal=5,
                start_operation=start, send_source=forged, record_operation=lambda _: None,
                dispatch=lambda: dispatched.append("probe"),
            )
        self.assertEqual(dispatched, [])

    def test_source_evidence_volumes_are_distinct_and_helper_gets_no_recorder_mount(self):
        direct = run_v2.source_evidence_mount("direct", "run-direct-evidence")
        adapter = run_v2.source_evidence_mount("adapter", "run-adapter-evidence")
        helper = run_v2.helper_container_argv("e0-helper", "e0-external", run_v2.IMAGE)

        self.assertEqual(direct, ["--mount", "type=volume,source=run-direct-evidence,target=/evidence"])
        self.assertEqual(adapter, ["--mount", "type=volume,source=run-adapter-evidence,target=/evidence"])
        self.assertNotEqual(direct[1], adapter[1])
        self.assertNotIn("--mount", helper)
        self.assertNotIn("-v", helper)

if __name__ == "__main__":
    unittest.main(verbosity=2)
