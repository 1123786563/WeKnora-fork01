"""Behavioral tests for the isolated real Lago Customer contract probe.

The probe talks to an in-process ``http.server`` fixture that stands in for
the pinned Lago API: the fixture records every request (method, path,
Authorization header, parsed body) and can inject faults, so success,
failure, malformed-response, auth-redaction, and cleanup paths are all
exercised without Docker or real network dependencies.
"""

import io
import json
import os
import re
import tempfile
import threading
import unittest
from contextlib import redirect_stdout
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from unittest import mock

from deploy.lago.contract_probe import main, run_probe


DEPLOY_DIR = Path(__file__).resolve().parent
LAGO_SH = DEPLOY_DIR / "lago.sh"
API_KEY = "secret-for-test-only"
UUID_SUFFIX = r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"


class RecordingLagoHandler(BaseHTTPRequestHandler):
    """Minimal Lago API stand-in that records requests and applies faults."""

    server_version = "RecordingLago/1"

    def log_message(self, format, *args):
        pass  # keep unittest output pristine

    def _record(self, body=None):
        self.server.requests.append({
            "method": self.command,
            "path": self.path,
            "authorization": self.headers.get("Authorization"),
            "body": body,
        })

    def _after_create_fault(self):
        return bool(self.server.fail_after_create and self.server.created_customers)

    def _send_json(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _send_raw(self, status, raw):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        self._record()
        if self.path != "/health":
            self._send_json(404, {"error": "not found"})
        elif self._after_create_fault():
            self._send_json(500, {"error": "boom-after-create"})
        else:
            self._send_json(self.server.health_status, {"status": "ok"})

    def do_POST(self):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        try:
            body = json.loads(raw.decode("utf-8"))
        except ValueError:
            body = None
        self._record(body)
        if self.path != "/api/v1/customers":
            self._send_json(404, {"error": "not found"})
            return
        customer = body.get("customer") if isinstance(body, dict) else None
        self.server.created_customers.append(customer)
        if self.server.create_raw_body is not None:
            self._send_raw(self.server.create_status, self.server.create_raw_body)
        elif self.server.create_customer_response is not None:
            self._send_json(self.server.create_status, self.server.create_customer_response)
        else:
            echoed = customer.get("external_id") if isinstance(customer, dict) else None
            self._send_json(self.server.create_status, {"customer": {"external_id": echoed}})

    def do_DELETE(self):
        self._record()
        match = re.fullmatch(r"/api/v1/customers/([^/?]+)", self.path)
        if match is None:
            self._send_json(404, {"error": "not found"})
            return
        self.server.deleted_external_ids.append(match.group(1))
        if self._after_create_fault():
            self._send_json(500, {"error": "boom-after-create"})
        else:
            self._send_json(self.server.delete_status, {"customer": {"external_id": match.group(1)}})


class RecordingLagoServer(ThreadingHTTPServer):
    """Configurable in-process Lago origin for probe tests."""

    daemon_threads = True

    def __init__(self):
        super().__init__(("127.0.0.1", 0), RecordingLagoHandler)
        self.requests = []
        self.created_customers = []
        self.deleted_external_ids = []
        self.health_status = 200
        self.create_status = 200
        self.create_customer_response = None  # None -> echo the requested external_id
        self.create_raw_body = None           # bytes -> sent verbatim (malformed-body tests)
        self.delete_status = 200
        self.fail_after_create = False        # fail every request after the first create

    @property
    def url(self):
        return f"http://127.0.0.1:{self.server_address[1]}"


class ContractProbeTestCase(unittest.TestCase):
    def setUp(self):
        self.server = RecordingLagoServer()
        thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)


class HappyPathTests(ContractProbeTestCase):
    def test_pass_run_creates_and_deletes_one_unique_synthetic_customer(self):
        result = run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(result.status, "pass")
        creates = [r for r in self.server.requests if r["method"] == "POST"]
        self.assertEqual([r["path"] for r in creates], ["/api/v1/customers"])
        customer = creates[0]["body"]["customer"]
        self.assertRegex(customer["external_id"], rf"^weknora-t01-probe-{UUID_SUFFIX}$")
        self.assertTrue(customer["name"].startswith("WeKnora Contract Probe"))
        self.assertEqual(self.server.deleted_external_ids, [customer["external_id"]])

        report = json.loads(result.serialized())
        self.assertEqual(report["status"], "pass")
        self.assertEqual(report["release"]["release"], "v1.53.0")  # from the lock, not live API text
        self.assertEqual(report["customer"]["created_external_id"], customer["external_id"])
        self.assertTrue(report["started_at"])
        self.assertTrue(report["finished_at"])

    def test_each_run_requests_a_fresh_external_id(self):
        first = run_probe(self.server.url, api_key=API_KEY)
        second = run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(first.status, "pass")
        self.assertEqual(second.status, "pass")
        requested = [customer["external_id"] for customer in self.server.created_customers]
        self.assertEqual(len(requested), 2)
        self.assertNotEqual(requested[0], requested[1])


class AuthRedactionTests(ContractProbeTestCase):
    def test_bearer_credentials_go_only_to_lago_api_endpoints_on_the_configured_origin(self):
        run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(
            [r["method"] for r in self.server.requests], ["GET", "POST", "DELETE"]
        )
        for entry in self.server.requests:
            if entry["method"] in ("POST", "DELETE"):
                self.assertEqual(entry["authorization"], f"Bearer {API_KEY}")
            else:  # the /health GET needs no credentials
                self.assertIsNone(entry["authorization"])

    def test_report_contains_neither_api_key_nor_response_bodies(self):
        self.server.create_customer_response = {
            "customer": {
                "external_id": "weknora-t01-probe-echo",
                "lago_id": "SENTINEL-BODY-PAYLOAD",
            }
        }
        result = run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(result.status, "pass")
        self.assertNotIn(API_KEY, result.serialized())
        self.assertNotIn("SENTINEL-BODY-PAYLOAD", result.serialized())

    def test_missing_api_key_is_blocked_env_and_sends_no_requests(self):
        result = run_probe(self.server.url, api_key="")

        self.assertEqual(result.status, "blocked-env")
        self.assertEqual(self.server.requests, [])
        self.assertIn("LAGO_API_KEY", result.serialized())


class FailureTests(ContractProbeTestCase):
    def test_unhealthy_api_fails_before_creating_anything(self):
        self.server.health_status = 503
        result = run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(result.status, "fail")
        report = json.loads(result.serialized())
        self.assertEqual(report["health"], {"outcome": "unhealthy", "http_status": 503})
        self.assertEqual(report["create"]["outcome"], "skipped")
        self.assertEqual(report["cleanup"]["outcome"], "not-attempted")
        self.assertEqual(self.server.created_customers, [])
        self.assertEqual(self.server.deleted_external_ids, [])

    def test_non_2xx_create_is_rejected_and_nothing_is_cleaned_up(self):
        self.server.create_status = 422
        result = run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(result.status, "fail")
        report = json.loads(result.serialized())
        self.assertEqual(report["create"], {"outcome": "rejected", "http_status": 422})
        self.assertEqual(report["cleanup"], {"outcome": "not-attempted", "http_status": None})
        self.assertEqual(self.server.deleted_external_ids, [])

    def test_unparseable_create_response_is_malformed(self):
        self.server.create_raw_body = b"{not json"
        result = run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(result.status, "fail")
        report = json.loads(result.serialized())
        self.assertEqual(report["create"]["outcome"], "malformed")
        self.assertEqual(self.server.deleted_external_ids, [])

    def test_create_response_without_customer_external_id_is_malformed(self):
        self.server.create_customer_response = {"customer": {"lago_id": "no-external-id"}}
        result = run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(result.status, "fail")
        report = json.loads(result.serialized())
        self.assertEqual(report["create"], {"outcome": "malformed", "http_status": 200})
        self.assertEqual(self.server.deleted_external_ids, [])

    def test_probe_deletes_created_customer_after_later_failure(self):
        # Required case from the task brief (translated to unittest style):
        # a failure after a successful create must still run the DELETE.
        self.server.create_customer_response = {"customer": {"external_id": "weknora-t01-probe-a1"}}
        self.server.fail_after_create = True
        result = run_probe(self.server.url, api_key="secret-for-test-only")
        self.assertEqual(result.status, "fail")
        self.assertEqual(self.server.deleted_external_ids, ["weknora-t01-probe-a1"])
        self.assertNotIn("secret-for-test-only", result.serialized())

    def test_cleanup_failure_is_reported_separately_from_create_failure(self):
        self.server.fail_after_create = True
        result = run_probe(self.server.url, api_key=API_KEY)

        self.assertEqual(result.status, "fail")
        report = json.loads(result.serialized())
        self.assertEqual(report["health"], {"outcome": "ok", "http_status": 200})
        self.assertEqual(report["create"]["outcome"], "created")
        self.assertEqual(report["cleanup"], {"outcome": "failed", "http_status": 500})
        self.assertIn("cleanup", report["error"])


class CliTests(ContractProbeTestCase):
    def _patched_env(self):
        return {
            "LAGO_API_KEY": API_KEY,
            "LAGO_API_PORT": str(self.server.server_address[1]),
        }

    def test_main_writes_sanitized_report_only_to_the_explicit_output_path(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            output_path = Path(temp_dir) / "probe-report.json"
            with mock.patch.dict(os.environ, self._patched_env()):
                with redirect_stdout(io.StringIO()):
                    exit_code = main(["--output", str(output_path)])

            self.assertEqual(exit_code, 0)
            written = output_path.read_text(encoding="utf-8")
            report = json.loads(written)
            self.assertEqual(report["status"], "pass")
            self.assertNotIn(API_KEY, written)
            self.assertEqual(os.listdir(temp_dir), ["probe-report.json"])

    def test_main_exit_code_is_nonzero_when_the_probe_fails(self):
        self.server.create_status = 500
        with mock.patch.dict(os.environ, self._patched_env()):
            with redirect_stdout(io.StringIO()):
                exit_code = main([])
        self.assertEqual(exit_code, 1)

    def test_main_without_api_key_exits_blocked_env(self):
        env = {"LAGO_API_PORT": str(self.server.server_address[1])}
        with mock.patch.dict(os.environ, env):
            os.environ.pop("LAGO_API_KEY", None)
            with redirect_stdout(io.StringIO()):
                exit_code = main([])
        self.assertEqual(exit_code, 2)


class LagoShWiringTests(unittest.TestCase):
    def test_contract_probe_subcommand_execs_the_probe_and_never_handles_the_api_key(self):
        text = LAGO_SH.read_text(encoding="utf-8")
        self.assertIn('exec python3 "$LAGO_DIR/contract_probe.py" "$@"', text)
        # The API key is operator input and must flow only through the
        # environment: lago.sh itself must never read, store, or echo it.
        self.assertNotIn("LAGO_API_KEY", text)


if __name__ == "__main__":
    unittest.main()
