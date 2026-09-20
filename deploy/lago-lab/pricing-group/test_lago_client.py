"""Behavioral tests for the Lago lab client against an in-process fake API.

The fake is an ``http.server`` that records every request (method, path,
Authorization header, parsed body) and serves configurable responses, so
delivery, read-back, duplicate classification, exact reconciliation, and
secret redaction are all exercised without Docker or a real key.
"""

import json
import os
import unittest
from decimal import Decimal
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread
from unittest import mock
from urllib.parse import parse_qs, urlparse

import fixture
from lago_client import (
    BlockedEnvError,
    classify_send,
    dumps_sanitized,
    get_current_usage,
    get_event,
    list_events,
    poll_until_reconciled,
    reconcile,
    send_batch,
    send_event,
)


API_KEY = "secret-for-test-only"


def _expected_reference_usage():
    run = fixture.build_run("weknora-t04-clienttest", task_count=2)
    return run, fixture.expected_usage(run.tasks[0])


def _observed_usage(run, exp, *, total_override=None, model_units_override=None,
                    tool_units_override=None, model_events_override=None,
                    drop_tool=False, extra_charge=False, currency=None,
                    taxes=None, per_charge_override=None):
    charges = []
    model_code = run.metrics[0].code
    tool_code = run.metrics[1].code
    model = exp.charge("model-units")
    tool = exp.charge("tool-calls")
    charges.append({
        "units": str(model_units_override if model_units_override is not None else model.units),
        "events_count": model_events_override if model_events_override is not None else model.events_count,
        "amount_cents": per_charge_override if per_charge_override is not None else model.amount_cents,
        "charge": {"charge_model": "standard"},
        "billable_metric": {"code": model_code},
    })
    if not drop_tool:
        charges.append({
            "units": str(tool_units_override if tool_units_override is not None else tool.units),
            "events_count": tool.events_count,
            "amount_cents": tool.amount_cents,
            "charge": {"charge_model": "package"},
            "billable_metric": {"code": tool_code},
        })
    if extra_charge:
        charges.append({
            "units": "1", "events_count": 1, "amount_cents": 9,
            "charge": {"charge_model": "standard"},
            "billable_metric": {"code": f"{run.run_id}-unexpected"},
        })
    total = total_override if total_override is not None else exp.total_amount_cents
    return {
        "from_datetime": "2026-09-21T00:00:00+00:00",
        "to_datetime": "2026-10-21T00:00:00+00:00",
        "currency": currency if currency is not None else "CNY",
        "amount_cents": total,
        "total_amount_cents": total,
        "taxes_amount_cents": taxes if taxes is not None else 0,
        "charges_usage": charges,
    }


class FakeLagoHandler(BaseHTTPRequestHandler):
    server_version = "FakeLago/1"

    def log_message(self, format, *args):
        pass

    def _record(self, body=None):
        self.server.requests.append({
            "method": self.command,
            "path": self.path,
            "authorization": self.headers.get("Authorization"),
            "content_type": self.headers.get("Content-Type"),
            "body": body,
        })

    def _respond(self, status, payload):
        raw = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def _read_body(self):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        try:
            return json.loads(raw.decode("utf-8"))
        except ValueError:
            return None

    def do_POST(self):
        body = self._read_body()
        self._record(body)
        if self.path == "/api/v1/events":
            status, payload = self.server.event_response
            if payload is None:
                payload = {"event": body.get("event", {})}
            self._respond(status, payload)
        elif self.path == "/api/v1/events/batch":
            status, payload = self.server.batch_response
            if payload is None:
                payload = {"events": body.get("events", [])}
            self._respond(status, payload)
        else:
            self._respond(404, {"error": "not found"})

    def do_GET(self):
        self._record()
        parsed = urlparse(self.path)
        if parsed.path.startswith("/api/v1/events/"):
            transaction_id = parsed.path.rsplit("/", 1)[1]
            stored = self.server.event_store.get(transaction_id, {"transaction_id": transaction_id})
            self._respond(self.server.get_event_status, {"event": stored})
        elif parsed.path == "/api/v1/events":
            self._respond(200, {"events": self.server.listed_events})
        elif parsed.path.endswith("/current_usage"):
            responses = self.server.usage_responses
            if responses:
                status, payload = responses.pop(0)
            else:
                status, payload = self.server.usage_response
            self._respond(status, payload)
        else:
            self._respond(404, {"error": "not found"})


class FakeLagoServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self):
        super().__init__(("127.0.0.1", 0), FakeLagoHandler)
        self.requests = []
        self.event_response = (200, None)
        self.batch_response = (200, None)
        self.get_event_status = 200
        self.event_store = {}
        self.listed_events = []
        self.usage_response = (200, {"customer_usage": {}})
        self.usage_responses = []  # queue consumed before usage_response


class ClientTestCase(unittest.TestCase):
    def setUp(self):
        self.server = FakeLagoServer()
        thread = Thread(target=self.server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(self.server.shutdown)
        self.addCleanup(self.server.server_close)
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"
        self.run, self.expected = _expected_reference_usage()
        self.task = self.run.tasks[0]

    def _event_payload(self):
        return {
            "transaction_id": "weknora-t04-clienttest-task-0-model-0",
            "external_subscription_id": self.task.subscription_external_id,
            "code": self.run.metrics[0].code,
            "timestamp": "2026-09-21T12:00:00+00:00",
            "properties": {"units": 100},
        }


class DeliveryTests(ClientTestCase):
    def test_send_event_posts_bearer_json_with_transaction_id_to_events(self):
        result = send_event(self.base, API_KEY, self._event_payload())
        self.assertEqual(result.status, 200)
        request = self.server.requests[-1]
        self.assertEqual(request["method"], "POST")
        self.assertEqual(request["path"], "/api/v1/events")
        self.assertEqual(request["authorization"], f"Bearer {API_KEY}")
        self.assertEqual(request["content_type"], "application/json")
        self.assertIn("transaction_id", request["body"]["event"])
        self.assertNotIn("external_id", request["body"]["event"])

    def test_send_batch_wraps_events_and_refuses_over_one_hundred(self):
        events = [
            {"transaction_id": f"weknora-t04-clienttest-b-{i}", "code": "c", "properties": {"units": 1}}
            for i in range(150)
        ]
        with self.assertRaises(ValueError):
            send_batch(self.base, API_KEY, events)
        self.assertEqual(self.server.requests, [])
        ok = send_batch(self.base, API_KEY, events[:100])
        self.assertEqual(ok.status, 200)
        request = self.server.requests[-1]
        self.assertEqual(request["path"], "/api/v1/events/batch")
        self.assertEqual(len(request["body"]["events"]), 100)

    def test_requests_reach_the_configured_origin_even_with_proxy_env(self):
        dead = "http://127.0.0.1:9/"
        env = {name: dead for name in
               ("http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY", "all_proxy", "ALL_PROXY")}
        with mock.patch.dict(os.environ, env):
            send_event(self.base, API_KEY, self._event_payload())
            get_event(self.base, API_KEY, "t")
        self.assertEqual(len(self.server.requests), 2)

    def test_missing_api_key_blocks_before_any_request(self):
        payload = {"transaction_id": "t", "code": "c", "properties": {}}
        for call in (
            lambda: send_event(self.base, "", payload),
            lambda: send_batch(self.base, "", [payload]),
            lambda: get_event(self.base, "", "t"),
            lambda: list_events(self.base, "", "sub", "code"),
            lambda: get_current_usage(self.base, "", "cust", "sub"),
        ):
            with self.assertRaises(BlockedEnvError):
                call()
        self.assertEqual(self.server.requests, [])


class ReadBackTests(ClientTestCase):
    def test_get_event_reads_the_event_resource_by_transaction_id(self):
        result = get_event(self.base, API_KEY, "weknora-t04-clienttest-xyz")
        self.assertEqual(result.status, 200)
        self.assertEqual(result.body["event"]["transaction_id"], "weknora-t04-clienttest-xyz")
        request = self.server.requests[-1]
        self.assertEqual(request["method"], "GET")
        self.assertEqual(request["path"], "/api/v1/events/weknora-t04-clienttest-xyz")

    def test_list_events_filters_by_subscription_and_code(self):
        list_events(self.base, API_KEY, "weknora-t04-clienttest-task-0", "weknora-t04-code")
        request = self.server.requests[-1]
        self.assertEqual(request["method"], "GET")
        parsed = urlparse(request["path"])
        self.assertEqual(parsed.path, "/api/v1/events")
        query = parse_qs(parsed.query)
        self.assertEqual(query["external_subscription_id"], ["weknora-t04-clienttest-task-0"])
        self.assertEqual(query["code"], ["weknora-t04-code"])

    def test_get_current_usage_uses_the_customer_scoped_v1_endpoint(self):
        self.server.usage_response = (200, {"customer_usage": _observed_usage(self.run, self.expected)})
        result = get_current_usage(
            self.base, API_KEY, "weknora-t04-clienttest", "weknora-t04-clienttest-task-0"
        )
        self.assertEqual(result.status, 200)
        request = self.server.requests[-1]
        parsed = urlparse(request["path"])
        self.assertEqual(
            parsed.path, "/api/v1/customers/weknora-t04-clienttest/current_usage"
        )
        self.assertEqual(
            parse_qs(parsed.query)["external_subscription_id"],
            ["weknora-t04-clienttest-task-0"],
        )


class ClassificationTests(unittest.TestCase):
    def test_2xx_is_accepted(self):
        self.assertEqual(classify_send(200, {"event": {}}), "accepted")

    def test_value_already_exist_422_is_duplicate_transaction_id(self):
        body = {
            "status": 422,
            "code": "validation_errors",
            "error_details": {"transaction_id": ["value_already_exist"]},
        }
        self.assertEqual(classify_send(422, body), "duplicate_transaction_id")
        self.assertNotEqual(classify_send(422, body), "accepted")

    def test_other_422_is_validation_error(self):
        body = {"code": "validation_errors", "error_details": {"code": ["invalid"]}}
        self.assertEqual(classify_send(422, body), "validation_error")

    def test_server_error_is_unexpected(self):
        self.assertEqual(classify_send(500, {"error": "boom"}), "unexpected")


class ReconcileTests(ClientTestCase):
    def test_exact_observation_reconciles(self):
        observed = _observed_usage(self.run, self.expected)
        report = reconcile(observed, self.expected)
        self.assertTrue(report.ok)
        self.assertEqual(report.diffs, ())
        self.assertEqual(report.total_amount_cents, self.expected.total_amount_cents)

    def test_one_cent_mismatch_fails_reconciliation(self):
        observed = _observed_usage(self.run, self.expected, total_override=11501)
        report = reconcile(observed, self.expected)
        self.assertFalse(report.ok)
        self.assertIn("amount_cents", report.diffs)
        self.assertTrue(any("11501" in detail and "11500" in detail for detail in report.details))

    def test_units_string_mismatch_fails(self):
        observed = _observed_usage(self.run, self.expected, model_units_override=Decimal("999"))
        report = reconcile(observed, self.expected)
        self.assertFalse(report.ok)
        self.assertIn("units", report.diffs)

    def test_events_count_mismatch_fails(self):
        observed = _observed_usage(self.run, self.expected, model_events_override=11)
        report = reconcile(observed, self.expected)
        self.assertFalse(report.ok)
        self.assertIn("events_count", report.diffs)

    def test_missing_charge_fails(self):
        observed = _observed_usage(self.run, self.expected, drop_tool=True)
        report = reconcile(observed, self.expected)
        self.assertFalse(report.ok)
        self.assertIn("charges_usage", report.diffs)

    def test_extra_charge_fails(self):
        observed = _observed_usage(self.run, self.expected, extra_charge=True)
        report = reconcile(observed, self.expected)
        self.assertFalse(report.ok)
        self.assertIn("charges_usage", report.diffs)

    def test_per_charge_amount_mismatch_fails(self):
        observed = _observed_usage(self.run, self.expected, per_charge_override=6999)
        report = reconcile(observed, self.expected)
        self.assertFalse(report.ok)
        self.assertIn("amount_cents", report.diffs)

    def test_wrong_currency_fails(self):
        observed = _observed_usage(self.run, self.expected, currency="EUR")
        report = reconcile(observed, self.expected)
        self.assertFalse(report.ok)
        self.assertIn("currency", report.diffs)

    def test_nonzero_taxes_fail(self):
        observed = _observed_usage(self.run, self.expected, taxes=7)
        report = reconcile(observed, self.expected)
        self.assertFalse(report.ok)
        self.assertIn("taxes_amount_cents", report.diffs)


class PollTests(ClientTestCase):
    def test_poll_returns_on_first_exact_reading(self):
        stale = _observed_usage(self.run, self.expected, total_override=0, drop_tool=True)
        exact = _observed_usage(self.run, self.expected)
        self.server.usage_responses = [
            (200, {"customer_usage": stale}),
            (200, {"customer_usage": exact}),
        ]
        outcome = poll_until_reconciled(
            self.base, API_KEY, self.run.run_id, self.task.subscription_external_id,
            self.expected, interval=0, timeout=5, sleep=lambda _s: None,
        )
        self.assertTrue(outcome.reconciled)
        self.assertEqual(outcome.attempts, 2)
        self.assertIsNotNone(outcome.first_exact_at)
        self.assertEqual(len(outcome.timeline), 2)
        self.assertFalse(outcome.timeline[0]["ok"])
        self.assertTrue(outcome.timeline[-1]["ok"])

    def test_poll_records_unresolved_after_timeout(self):
        stale = _observed_usage(self.run, self.expected, total_override=1)
        self.server.usage_response = (200, {"customer_usage": stale})
        outcome = poll_until_reconciled(
            self.base, API_KEY, self.run.run_id, self.task.subscription_external_id,
            self.expected, interval=0, timeout=0.05, sleep=lambda _s: None,
        )
        self.assertFalse(outcome.reconciled)
        self.assertIsNone(outcome.first_exact_at)
        self.assertGreaterEqual(outcome.attempts, 1)
        self.assertFalse(outcome.final_report.ok)


class RedactionTests(ClientTestCase):
    def test_serialized_output_never_contains_the_api_key(self):
        observed = _observed_usage(self.run, self.expected, total_override=11501)
        report = reconcile(observed, self.expected)
        bundle = {
            "key_echo": API_KEY,
            "auth_header": f"Bearer {API_KEY}",
            "report": report.to_dict(),
        }
        text = dumps_sanitized(bundle, API_KEY)
        self.assertNotIn(API_KEY, text)
        parsed = json.loads(text)
        self.assertNotIn(API_KEY, json.dumps(parsed))
        self.assertFalse(parsed["report"]["ok"])


if __name__ == "__main__":
    unittest.main()
