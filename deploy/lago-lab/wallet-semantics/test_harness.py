"""Behavioral tests for the T03 wallet-semantics experiment harness.

No Docker and no real network: every HTTP test talks to an in-process
``http.server`` fixture standing in for the pinned Lago API. These tests pin
the harness contract the experiments rely on -- sanitized reports, honest
pass/fail/blocked-env classification, weknora-t03- prefixed synthetic ids,
per-run cleanup from ``finally``, and the fixture payload builders.
"""

import json
import os
import re
import stat
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from unittest import mock

if __package__ in (None, ""):
    import sys

    sys.path.insert(0, str(Path(__file__).resolve().parent))

import harness

LAB_DIR = Path(__file__).resolve().parent
REPO_ROOT = LAB_DIR.parent.parent.parent
LOCK_PATH = REPO_ROOT / "deploy" / "lago" / "images.lock.json"
API_KEY = "secret-for-test-only"
UUID_SUFFIX = r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"


class RecordingLagoHandler(BaseHTTPRequestHandler):
    """Minimal Lago API stand-in: records requests, applies fault settings."""

    server_version = "RecordingLago/1"

    def log_message(self, format, *args):
        pass  # keep unittest output pristine

    def _record(self, body=None):
        self.server.requests.append(
            {
                "method": self.command,
                "path": self.path,
                "authorization": self.headers.get("Authorization"),
                "body": body,
            }
        )

    def _read_body(self):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        try:
            return json.loads(raw.decode("utf-8"))
        except ValueError:
            return None

    def _send(self, status, payload):
        if isinstance(payload, (dict, list)):
            body = json.dumps(payload).encode("utf-8")
        else:
            body = str(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        self._record()
        if self.path == "/health":
            self._send(self.server.health_status, {"status": "ok"})
            return
        if self.path.startswith("/api/v1/customers/"):
            self._send(self.server.get_status, {"customer": {"external_id": "weknora-t03-x"}})
            return
        self._send(404, {"error": "not found"})

    def do_POST(self):
        body = self._read_body()
        self._record(body)
        if self.server.api_status >= 400:
            # Mirrors the v1.53.0 validation-error shape: the specific
            # contract code lives inside error_details.
            self._send(
                self.server.api_status,
                {"status": 422, "error": "Unprocessable Entity",
                 "code": "validation_errors",
                 "error_details": {"customer": [self.server.api_error_code]}},
            )
            return
        self._send(200, {"ok": True, "received": {"echo": "SENTINEL-RAW-BODY"}})

    def do_DELETE(self):
        self._record()
        self.server.deleted_paths.append(self.path)
        self._send(200, {"customer": {"external_id": "weknora-t03-x"}})


class RecordingLagoServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self):
        super().__init__(("127.0.0.1", 0), RecordingLagoHandler)
        self.requests = []
        self.deleted_paths = []
        self.health_status = 200
        self.get_status = 200
        self.api_status = 200
        self.api_error_code = "validation_error"

    @property
    def url(self):
        return f"http://127.0.0.1:{self.server_address[1]}"


class ServerTestCase(unittest.TestCase):
    def setUp(self):
        self.server = RecordingLagoServer()
        thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)


class ReportSanitizationTests(unittest.TestCase):
    def test_report_never_contains_api_key_or_raw_response_dump(self):
        report = harness.ExperimentReport(run_id="weknora-t03-00000000-0000-0000-0000-000000000000",
                                          api_key=API_KEY)
        report.checks.append(
            harness.check_from(criterion="probe", expected="echo", observed_error=ValueError(
                "Bearer secret-for-test-only rejected; body was HEAD-SENTINEL "
                + "x" * 5000 + " TAIL-SENTINEL"))
        )
        serialized = report.serialized()
        self.assertNotIn(API_KEY, serialized)
        self.assertNotIn("TAIL-SENTINEL", serialized)  # dumps are truncated, never verbatim
        self.assertLess(len(serialized), 20000)

    def test_check_from_sanitizes_registered_secrets_inside_observed_text(self):
        report = harness.ExperimentReport(run_id="weknora-t03-run", api_key=API_KEY)
        check = harness.check_from(
            criterion="c", expected="e",
            observed={"note": f"server said: {API_KEY} is invalid"}, secrets=(API_KEY,),
        )
        self.assertNotIn(API_KEY, json.dumps(check))
        self.assertIn("***REDACTED***", check["observed"]["note"])

    def test_status_starts_pass_and_transitions_to_fail_then_sticky(self):
        report = harness.ExperimentReport(run_id="weknora-t03-run", api_key="")
        self.assertEqual(report.status, "pass")
        report.mark_blocked_env("stack down")
        self.assertEqual(report.status, "blocked-env")
        report.mark_fail("contract error")
        self.assertEqual(report.status, "fail")
        report.mark_blocked_env("late env issue")
        self.assertEqual(report.status, "fail")  # fail is sticky, never masked

    def test_created_object_ids_are_recorded_and_serialized(self):
        report = harness.ExperimentReport(run_id="weknora-t03-run", api_key="")
        report.record_created("customer", external_id="weknora-t03-c1")
        report.record_created("wallet", lago_id="123-abc")
        data = json.loads(report.serialized())
        self.assertEqual(data["created_object_ids"], [
            {"kind": "customer", "lago_id": None, "external_id": "weknora-t03-c1"},
            {"kind": "wallet", "lago_id": "123-abc", "external_id": None},
        ])


class SyntheticIdentityTests(unittest.TestCase):
    def test_new_run_id_is_weknora_t03_prefixed_uuid_per_run(self):
        first = harness.new_run_id()
        second = harness.new_run_id()
        self.assertRegex(first, rf"^weknora-t03-{UUID_SUFFIX}$")
        self.assertNotEqual(first, second)

    def test_fixture_builders_emit_weknora_t03_external_ids_and_batch_keys(self):
        batch = harness.new_batch_key()
        self.assertRegex(batch, rf"^weknora-t03-batch-{UUID_SUFFIX}$")

        customer = harness.customer_payload(external_id=harness.new_customer_id())
        self.assertRegex(customer["customer"]["external_id"], rf"^weknora-t03-cust-{UUID_SUFFIX}$")
        self.assertEqual(customer["customer"]["name"], "WeKnora T03 Lab")

        wallet = harness.wallet_payload(
            external_customer_id=customer["customer"]["external_id"],
            name="batch", granted_credits="100.0", batch_key=batch,
        )
        self.assertRegex(wallet["wallet"]["code"], rf"^weknora-t03-wallet-{UUID_SUFFIX}$")
        self.assertEqual(wallet["wallet"]["metadata"]["weknora_t03_batch"], batch)
        self.assertEqual(wallet["wallet"]["rate_amount"], "1")
        self.assertIsInstance(wallet["wallet"]["granted_credits"], str)

        grant = harness.wallet_grant_payload(wallet_lago_id="lago-1", granted_credits="10.5",
                                             batch_key=batch)
        self.assertEqual(grant["wallet_transaction"]["wallet_id"], "lago-1")
        self.assertEqual(grant["wallet_transaction"]["granted_credits"], "10.5")
        # Transaction metadata uses the array form (dict is rejected with
        # metadata: ["invalid_type"] on v1.53.0); wallet metadata uses dict.
        self.assertEqual(grant["wallet_transaction"]["metadata"],
                         [{"key": "weknora_t03_batch", "value": batch}])
        self.assertEqual(wallet["wallet"]["metadata"],
                         {"weknora_t03_batch": batch})

        invoice = harness.one_off_invoice_payload(
            external_customer_id=customer["customer"]["external_id"],
            fee_name="consume", unit_amount_cents=500,
        )
        fee = invoice["invoice"]["fees"][0]
        self.assertEqual(fee["item"]["name"], "consume")
        self.assertEqual(fee["item"]["unit_amount_cents"], 500)
        self.assertEqual(fee["units"], "1")


class ClientAuthTests(ServerTestCase):
    def test_bearer_credentials_go_only_to_the_lab_origin_and_never_to_health(self):
        client = harness.LagoClient(self.server.url, API_KEY)
        client.health()
        client.get("/api/v1/customers/weknora-t03-x")
        health_calls = [r for r in self.server.requests if r["path"] == "/health"]
        api_calls = [r for r in self.server.requests if r["path"] != "/health"]
        self.assertEqual(len(health_calls), 1)
        self.assertIsNone(health_calls[0]["authorization"])
        for entry in api_calls:
            self.assertEqual(entry["authorization"], f"Bearer {API_KEY}")
            self.assertTrue(entry["path"].startswith("/api/v1/"))

    def test_client_ignores_proxy_environment(self):
        dead_proxy = "http://127.0.0.1:9/"
        env = {name: dead_proxy for name in
               ("http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY",
                "all_proxy", "ALL_PROXY")}
        with mock.patch.dict(os.environ, env):
            client = harness.LagoClient(self.server.url, API_KEY)
            client.health()
        self.assertEqual(len(self.server.requests), 1)

    def test_non_2xx_contract_answer_raises_lago_http_error_with_sanitized_reason(self):
        self.server.api_status = 422
        self.server.api_error_code = "wallet_limit_reached"
        client = harness.LagoClient(self.server.url, API_KEY)
        with self.assertRaises(harness.LagoHttpError) as raised:
            client.post("/api/v1/wallets", payload={"wallet": {}})
        self.assertEqual(raised.exception.status, 422)
        # The specific contract code is dug out of error_details, not the
        # generic top-level "validation_errors".
        self.assertIn("wallet_limit_reached", str(raised.exception))
        self.assertNotIn("SENTINEL-RAW-BODY", str(raised.exception))

    def test_connection_refused_raises_lago_transport_error(self):
        # Bind then close a socket to get a guaranteed-refused local port.
        import socket
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            port = probe.getsockname()[1]
        client = harness.LagoClient(f"http://127.0.0.1:{port}", API_KEY)
        with self.assertRaises(harness.LagoTransportError):
            client.health()


class RunExperimentTests(ServerTestCase):
    def _experiment(self, client, report, cleanup):
        report.record_created("customer", external_id="weknora-t03-c1")
        report.add_check(criterion="unit", expected="one", observed="one", outcome="pass")

    def test_missing_api_key_is_blocked_env_with_zero_api_calls(self):
        with tempfile.TemporaryDirectory() as tmp:
            report = harness.run_experiment(
                "e-test", "etest-missing-key", self.server.url, "",
                experiment_main=self._experiment, evidence_dir=tmp,
            )
            self.assertEqual(report.status, "blocked-env")
            self.assertEqual(self.server.requests, [])
            data = json.loads(report.serialized())
            self.assertIn("LAGO_API_KEY", data["error"])
            self.assertEqual(data["checks"], [])
            self.assertTrue((Path(tmp) / "etest-missing-key.json").exists())

    def test_unreachable_stack_is_blocked_env_not_fail(self):
        import socket
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            port = probe.getsockname()[1]
        with tempfile.TemporaryDirectory() as tmp:
            report = harness.run_experiment(
                "e-test", "etest-down", f"http://127.0.0.1:{port}", API_KEY,
                experiment_main=self._experiment, evidence_dir=tmp,
            )
            self.assertEqual(report.status, "blocked-env")
            data = json.loads(report.serialized())
            self.assertIn("unreachable", data["error"])
            self.assertEqual(data["checks"], [])  # pre-condition failed: no checks asserted

    def test_contract_http_error_is_fail_not_blocked_env(self):
        self.server.api_status = 422
        def failing(client, report, cleanup):
            client.post("/api/v1/wallets", payload={"wallet": {}})
        with tempfile.TemporaryDirectory() as tmp:
            report = harness.run_experiment(
                "e-test", "etest-422", self.server.url, API_KEY,
                experiment_main=failing, evidence_dir=tmp,
            )
            self.assertEqual(report.status, "fail")
            data = json.loads(report.serialized())
            self.assertIn("422", data["error"])

    def test_cleanup_runs_from_finally_after_midrun_failure(self):
        def failing(client, report, cleanup):
            report.record_created("customer", external_id="weknora-t03-c1")
            client.post("/api/v1/wallets", payload={"wallet": {}})
            raise AssertionError("never reached")
        self.server.api_status = 500
        with tempfile.TemporaryDirectory() as tmp:
            report = harness.run_experiment(
                "e-test", "etest-cleanup", self.server.url, API_KEY,
                experiment_main=failing, evidence_dir=tmp,
            )
            self.assertEqual(report.status, "fail")
            self.assertEqual(len(self.server.deleted_paths), 1)
            data = json.loads(report.serialized())
            self.assertEqual(data["cleanup"][0]["outcome"], "deleted")
            self.assertEqual(data["cleanup"][0]["kind"], "customer")

    def test_report_records_release_identity_from_the_image_lock(self):
        with tempfile.TemporaryDirectory() as tmp:
            report = harness.run_experiment(
                "e-test", "etest-release", self.server.url, API_KEY,
                experiment_main=self._experiment, evidence_dir=tmp,
            )
            data = json.loads(report.serialized())
            self.assertEqual(data["release"]["release"], "v1.53.0")  # lock truth, not API text
            self.assertIn("getlago/api", json.dumps(data["release"]["images"]))

    def test_evidence_file_is_sanitized_json_matching_the_report(self):
        with tempfile.TemporaryDirectory() as tmp:
            def leaking(client, report, cleanup):
                report.add_check(criterion="k", expected="v",
                                 observed={"auth": f"Bearer {API_KEY}"}, outcome="pass")
            report = harness.run_experiment(
                "e-test", "etest-secret", self.server.url, API_KEY,
                experiment_main=leaking, evidence_dir=tmp,
            )
            text = (Path(tmp) / "etest-secret.json").read_text(encoding="utf-8")
            self.assertNotIn(API_KEY, text)
            self.assertEqual(json.loads(text)["status"], report.status)


if __name__ == "__main__":
    unittest.main()
