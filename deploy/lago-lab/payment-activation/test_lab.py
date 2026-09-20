"""Behavioral tests for the T02 lab foundation: env generation, redaction-safe
clients, and the lab.sh lifecycle scoping.

Everything here runs offline: HTTP behavior is exercised against in-process
``http.server`` fixtures, lab.sh's ``init`` only touches a temp file, and the
compose argv invariant is asserted statically over the lab.sh source (the one
``docker compose`` call site must carry the lab project, env file, and pinned
compose file).
"""

import json
import os
import re
import stat
import subprocess
import sys
import tempfile
import threading
import unittest
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

LAB_DIR = Path(__file__).resolve().parent
REPO_DIR = LAB_DIR.parents[2]
sys.path.insert(0, str(LAB_DIR))

import clients  # noqa: E402  (flat import from the lab directory)

LAB_SH = LAB_DIR / "lab.sh"
LAB_ENV_EXAMPLE = LAB_DIR / "lab.env.example"
LAB_GITIGNORE = LAB_DIR / ".gitignore"

API_KEY = "lago-api-key-canary-000"
PASSWORD = "operator-password-canary-000"
JWT_CANARY = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwInQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c"
STRIPE_TEST_KEY = "sk_test_" + "51Canary00000000000000000000"


def run_lab_sh(args, env_file, extra_env=None):
    """Run lab.sh against an isolated env file, never the real lab.env."""
    env = dict(os.environ)
    env["LAB_ENV_FILE"] = str(env_file)
    if extra_env:
        env.update(extra_env)
    return subprocess.run(
        [str(LAB_SH), *args], capture_output=True, text=True, env=env, check=False
    )


class RecordingHandler(BaseHTTPRequestHandler):
    """Configurable stand-in origin that records every request it receives."""

    server_version = "RecordingLab/1"

    def log_message(self, format, *args):
        pass

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
        if not raw:
            return None
        content_type = self.headers.get("Content-Type", "")
        text = raw.decode("utf-8", errors="replace")
        if "json" in content_type:
            try:
                return json.loads(text)
            except ValueError:
                return {"_raw": text}
        return {"_form": urllib.parse.parse_qs(text)}

    def _send_json(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _route(self, body):
        route = self.server.routes.get((self.command, self.path))
        if route is None:
            self._send_json(404, {"error": "no route"})
            return
        if "location" in route:
            self.send_response(route.get("status", 302))
            self.send_header("Location", route["location"])
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        self._send_json(route.get("status", 200), route.get("json", {}))

    def do_GET(self):
        self._record()
        self._route(None)

    def do_POST(self):
        self._record(self._read_body())
        self._route(self.server.requests[-1]["body"])

    def do_DELETE(self):
        self._record()
        self._route(None)


class RecordingServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self):
        super().__init__(("127.0.0.1", 0), RecordingHandler)
        self.requests = []
        self.routes = {}

    @property
    def url(self):
        host, port = self.server_address
        return f"http://{host}:{port}"

    def add(self, method, path, status=200, payload=None, location=None):
        route = {"status": status, "json": payload}
        if location is not None:
            route = {"status": status, "location": location}
        self.routes[(method, path)] = route


class server_fixture:
    """Context manager for a recording server."""

    def __enter__(self):
        self.server = RecordingServer()
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        return self.server

    def __exit__(self, *exc):
        self.server.shutdown()
        self.server.server_close()
        return False


def parse_env_file(path):
    values = {}
    for line in Path(path).read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        values[key] = value.strip().strip('"')
    return values


class TestGenerateLabEnv(unittest.TestCase):
    def test_pins_isolation_values(self):
        env = clients.generate_lab_env(rsa_private_key="dummy-rsa-key")
        self.assertEqual(env["COMPOSE_PROJECT_NAME"], "weknora-lago-74")
        self.assertEqual(env["LAGO_API_PORT"], "48891")
        self.assertEqual(env["LAGO_FRONT_PORT"], "48892")
        self.assertEqual(env["LAGO_API_URL"], "http://127.0.0.1:48891")
        self.assertEqual(env["LAGO_FRONT_URL"], "http://127.0.0.1:48892")

    def test_seed_values_present_for_operator_onboarding(self):
        env = clients.generate_lab_env(rsa_private_key="dummy-rsa-key")
        self.assertEqual(env["LAGO_CREATE_ORG"], "true")
        self.assertIn("@", env["LAGO_ORG_USER_EMAIL"])
        self.assertTrue(env["LAGO_ORG_USER_PASSWORD"])
        self.assertTrue(env["LAGO_ORG_NAME"])
        self.assertTrue(env["LAGO_ORG_API_KEY"])

    def test_generated_secrets_differ_from_lago_sample_defaults(self):
        for _ in range(2):  # randomness cannot accidentally equal a sample twice
            env = clients.generate_lab_env(rsa_private_key="dummy-rsa-key")
            for key, sample in clients.SAMPLE_DEFAULTS.items():
                self.assertNotEqual(
                    env[key], sample, f"{key} must never equal the sample default"
                )
            self.assertTrue(env["LAGO_RSA_PRIVATE_KEY"])
            self.assertTrue(env["LAGO_ORG_API_KEY"])
            self.assertTrue(env["LAGO_ORG_USER_PASSWORD"])

    def test_two_generations_differ(self):
        a = clients.generate_lab_env(rsa_private_key="dummy-rsa-key")
        b = clients.generate_lab_env(rsa_private_key="dummy-rsa-key")
        for key in ("POSTGRES_PASSWORD", "SECRET_KEY_BASE", "LAGO_ORG_API_KEY"):
            self.assertNotEqual(a[key], b[key])

    def test_write_refuses_overwrite_and_uses_mode_600(self):
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "lab.env"
            env = clients.generate_lab_env(rsa_private_key="dummy-rsa-key")
            clients.write_lab_env(target, env)
            self.assertTrue(target.exists())
            self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o600)
            with self.assertRaises(clients.LabError):
                clients.write_lab_env(target, env)


class TestStripeClient(unittest.TestCase):
    def test_refuses_live_keys(self):
        for bad in ("sk_live_abc", "rk_live_abc", "", None, "whsec_abc", "pk_test_abc"):
            with self.assertRaises(clients.RefusedLiveKey):
                clients.StripeTestClient(api_key=bad)

    def test_accepts_test_keys_and_hides_key_in_repr(self):
        client = clients.StripeTestClient(
            api_key=STRIPE_TEST_KEY, base_url="http://127.0.0.1:1"
        )
        self.assertNotIn(STRIPE_TEST_KEY, repr(client))
        self.assertNotIn(STRIPE_TEST_KEY, str(client))

    def test_stripe_calls_carry_bearer_and_form_bodies(self):
        with server_fixture() as server:
            server.add("POST", "/v1/customers", 200, {"id": "cus_test_123"})
            server.add(
                "POST", "/v1/payment_methods/pm_1/attach", 200, {"id": "pm_1"}
            )
            server.add(
                "POST",
                "/v1/customers/cus_test_123",
                200,
                {"id": "cus_test_123"},
            )
            server.add("DELETE", "/v1/customers/cus_test_123", 200, {"deleted": True})
            client = clients.StripeTestClient(
                api_key=STRIPE_TEST_KEY, base_url=server.url
            )
            customer = client.create_customer(description="weknora-t02-x")
            self.assertEqual(customer["id"], "cus_test_123")
            client.attach_payment_method("pm_1", "cus_test_123")
            client.set_default_payment_method("cus_test_123", "pm_1")
            self.assertTrue(client.delete_customer("cus_test_123"))
            for request in server.requests:
                self.assertEqual(
                    request["authorization"], f"Bearer {STRIPE_TEST_KEY}"
                )
            attach = [
                r for r in server.requests if r["path"].endswith("/pm_1/attach")
            ][0]
            self.assertIn("cus_test_123", attach["body"]["_form"]["customer"][0])
            default = [
                r
                for r in server.requests
                if r["path"] == "/v1/customers/cus_test_123" and r["method"] == "POST"
            ][-1]
            self.assertIn(
                "pm_1",
                default["body"]["_form"]["invoice_settings[default_payment_method]"][0],
            )


class TestLagoRestClient(unittest.TestCase):
    def test_sends_bearer_to_configured_origin(self):
        with server_fixture() as server:
            server.add("GET", "/api/v1/customers", 200, {"customers": []})
            client = clients.LagoRestClient(base_url=server.url, api_key=API_KEY)
            status, body = client.get("/api/v1/customers")
            self.assertEqual(status, 200)
            self.assertEqual(body, {"customers": []})
            self.assertEqual(
                server.requests[0]["authorization"], f"Bearer {API_KEY}"
            )

    def test_never_sends_bearer_to_other_origin(self):
        with server_fixture() as origin, server_fixture() as other:
            other.add("GET", "/api/v1/echo", 200, {"ok": True})
            client = clients.LagoRestClient(base_url=origin.url, api_key=API_KEY)
            status, _ = client.get(f"{other.url}/api/v1/echo")
            self.assertEqual(status, 200)
            self.assertIsNone(other.requests[0]["authorization"])

    def test_refuses_off_origin_redirect_without_forwarding(self):
        with server_fixture() as origin, server_fixture() as attacker:
            origin.add(
                "GET",
                "/api/v1/redirect-me",
                302,
                location=f"{attacker.url}/api/v1/steal",
            )
            attacker.add("GET", "/api/v1/steal", 200, {"ok": True})
            client = clients.LagoRestClient(base_url=origin.url, api_key=API_KEY)
            with self.assertRaises(clients.OffOriginRedirect):
                client.get("/api/v1/redirect-me")
            self.assertEqual(attacker.requests, [])
            self.assertEqual(
                origin.requests[0]["authorization"], f"Bearer {API_KEY}"
            )

    def test_follows_same_origin_redirect(self):
        with server_fixture() as server:
            server.add(
                "GET", "/api/v1/old", 302, location=f"{server.url}/api/v1/new"
            )
            server.add("GET", "/api/v1/new", 200, {"moved": True})
            client = clients.LagoRestClient(base_url=server.url, api_key=API_KEY)
            status, body = client.get("/api/v1/old")
            self.assertEqual(status, 200)
            self.assertEqual(body, {"moved": True})

    def test_http_error_returns_status_and_sanitized_body(self):
        with server_fixture() as server:
            server.add(
                "POST", "/api/v1/payments", 403, {"status": 403, "error": "forbidden"}
            )
            client = clients.LagoRestClient(base_url=server.url, api_key=API_KEY)
            status, body = client.post("/api/v1/payments", {"payment": {}})
            self.assertEqual(status, 403)
            self.assertEqual(body["error"], "forbidden")

    def test_repr_and_errors_never_contain_api_key(self):
        with server_fixture() as server:
            client = clients.LagoRestClient(base_url=server.url, api_key=API_KEY)
            self.assertNotIn(API_KEY, repr(client))
            server.add("GET", "/boom", 500, {"error": "boom"})
            try:
                client.get("/boom")
                self.fail("expected an exception")
            except Exception as error:  # noqa: BLE001
                self.assertNotIn(API_KEY, str(error))


class TestGraphqlLogin(unittest.TestCase):
    def test_extracts_jwt_from_login_user(self):
        with server_fixture() as server:
            server.add(
                "POST",
                "/graphql",
                200,
                {"data": {"loginUser": {"token": JWT_CANARY}}},
            )
            token = clients.lago_graphql_login(
                server.url, "ops@weknora.local", PASSWORD
            )
            self.assertEqual(token, JWT_CANARY)
            sent = server.requests[0]["body"]
            self.assertEqual(sent["variables"]["input"]["password"], PASSWORD)
            self.assertIn("loginUser", sent["query"])

    def test_failure_raises_without_echoing_credentials(self):
        with server_fixture() as server:
            server.add(
                "POST",
                "/graphql",
                200,
                {"errors": [{"message": "Invalid email or password"}]},
            )
            with self.assertRaises(clients.LoginError) as caught:
                clients.lago_graphql_login(server.url, "ops@weknora.local", PASSWORD)
            message = str(caught.exception)
            self.assertNotIn(PASSWORD, message)
            self.assertNotIn(JWT_CANARY, message)

    def test_http_error_raises_login_error_without_credentials(self):
        with server_fixture() as server:
            server.add("POST", "/graphql", 500, {"error": "internal"})
            with self.assertRaises(clients.LoginError) as caught:
                clients.lago_graphql_login(server.url, "ops@weknora.local", PASSWORD)
            self.assertNotIn(PASSWORD, str(caught.exception))


class TestSanitize(unittest.TestCase):
    def test_strips_known_secret_shapes(self):
        raw = {
            "authorization": f"Bearer {JWT_CANARY}",
            "stripe_key": STRIPE_TEST_KEY,
            "live_key": "rk_live_" + "51BadBadBad0000000000000000",
            "nested": {"jwt": JWT_CANARY, "note": "plain text stays"},
            "list": [f"Bearer {API_KEY}"],
        }
        clean = clients.sanitize(raw)
        text = json.dumps(clean)
        for secret in (JWT_CANARY, STRIPE_TEST_KEY, "rk_live_" + "51BadBadBad0000000000000000", API_KEY):
            self.assertNotIn(secret, text)
        self.assertIn("plain text stays", text)

    def test_strips_extra_canary_secrets(self):
        canary = "canary-random-secret-value-42"
        clean = clients.sanitize({"customer_name": canary}, extra_secrets=(canary,))
        self.assertNotIn(canary, json.dumps(clean))

    def test_handles_non_string_scalars(self):
        clean = clients.sanitize({"n": 5, "b": True, "x": None})
        self.assertEqual(clean, {"n": 5, "b": True, "x": None})


class TestLabShScoping(unittest.TestCase):
    """lab.sh must scope every docker compose call to the lab project."""

    def setUp(self):
        self.assertTrue(LAB_SH.exists(), "lab.sh must exist")
        self.text = LAB_SH.read_text(encoding="utf-8")

    def test_project_and_env_file_are_lab_scoped(self):
        self.assertIn('COMPOSE_PROJECT="weknora-lago-74"', self.text)
        self.assertIn('ENV_FILE="${LAB_ENV_FILE:-$LAB_DIR/lab.env}"', self.text)
        self.assertIn('COMPOSE_FILE="$REPO_DIR/deploy/lago/compose.yaml"', self.text)

    def test_exactly_one_docker_call_site_and_it_is_scoped(self):
        docker_lines = [
            line
            for line in self.text.splitlines()
            if re.search(r"\bdocker\b", line) and not line.strip().startswith("#")
        ]
        compose_lines = [line for line in docker_lines if "compose" in line]
        self.assertEqual(len(compose_lines), 1, docker_lines)
        line = compose_lines[0]
        for fragment in ('-f "$COMPOSE_FILE"', '--env-file "$ENV_FILE"', '-p "$COMPOSE_PROJECT"'):
            self.assertIn(fragment, line)

    def test_status_snippet_uses_same_scoping(self):
        # The status implementation must pass the compose file, env file, and
        # project name into its health snapshot helper.
        self.assertIn('"$COMPOSE_FILE"', self.text)
        self.assertIn('"$COMPOSE_PROJECT"', self.text)
        self.assertIn('"$ENV_FILE"', self.text)

    def test_init_refuses_existing_env_file(self):
        self.assertIn("refusing to overwrite", self.text)


class TestLabShInit(unittest.TestCase):
    def test_init_writes_mode_600_env_and_refuses_overwrite(self):
        with tempfile.TemporaryDirectory() as tmp:
            env_file = Path(tmp) / "lab.env"
            result = run_lab_sh(["init"], env_file)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue(env_file.exists())
            self.assertEqual(stat.S_IMODE(env_file.stat().st_mode), 0o600)
            values = parse_env_file(env_file)
            self.assertEqual(values["COMPOSE_PROJECT_NAME"], "weknora-lago-74")
            self.assertEqual(values["LAGO_API_PORT"], "48891")
            self.assertEqual(values["LAGO_API_URL"], "http://127.0.0.1:48891")
            self.assertEqual(values["LAGO_CREATE_ORG"], "true")
            again = run_lab_sh(["init"], env_file)
            self.assertNotEqual(again.returncode, 0)
            self.assertIn("refusing to overwrite", again.stderr)

    def test_init_secrets_differ_from_sample_defaults(self):
        with tempfile.TemporaryDirectory() as tmp:
            env_file = Path(tmp) / "lab.env"
            result = run_lab_sh(["init"], env_file)
            self.assertEqual(result.returncode, 0, result.stderr)
            values = parse_env_file(env_file)
            for key, sample in clients.SAMPLE_DEFAULTS.items():
                self.assertNotEqual(values[key], sample)
            self.assertTrue(values["LAGO_RSA_PRIVATE_KEY"])

    def test_init_does_not_set_a_license(self):
        with tempfile.TemporaryDirectory() as tmp:
            env_file = Path(tmp) / "lab.env"
            run_lab_sh(["init"], env_file)
            values = parse_env_file(env_file)
            self.assertNotIn("LAGO_LICENSE", values)

    def test_init_never_touches_the_real_lab_env(self):
        # Isolated init must never create or modify the operator's real
        # lab.env (it may legitimately exist from a real `lab.sh init`).
        real_env = LAB_DIR / "lab.env"
        before = real_env.stat().st_mtime_ns if real_env.exists() else None
        with tempfile.TemporaryDirectory() as tmp:
            run_lab_sh(["init"], Path(tmp) / "lab.env")
        after = real_env.stat().st_mtime_ns if real_env.exists() else None
        self.assertEqual(before, after)


class TestLabShUpValidation(unittest.TestCase):
    def _env_with(self, tmp, **overrides):
        env = clients.generate_lab_env(rsa_private_key="dummy-rsa-not-a-real-key")
        env.update({k: str(v) for k, v in overrides.items()})
        env_file = Path(tmp) / "lab.env"
        env_file.write_text(
            "".join(f'{k}="{v}"\n' for k, v in env.items()), encoding="utf-8"
        )
        return env_file

    def test_up_rejects_sample_default_secret(self):
        with tempfile.TemporaryDirectory() as tmp:
            env_file = self._env_with(
                tmp, POSTGRES_PASSWORD=clients.SAMPLE_DEFAULTS["POSTGRES_PASSWORD"]
            )
            result = run_lab_sh(["up"], env_file)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("POSTGRES_PASSWORD", result.stderr)

    def test_up_rejects_port_url_drift(self):
        with tempfile.TemporaryDirectory() as tmp:
            env_file = self._env_with(tmp, LAGO_API_URL="http://127.0.0.1:48999")
            result = run_lab_sh(["up"], env_file)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("LAGO_API_URL", result.stderr)

    def test_up_rejects_a_license_flag(self):
        with tempfile.TemporaryDirectory() as tmp:
            env_file = self._env_with(tmp, LAGO_LICENSE="not-empty")
            result = run_lab_sh(["up"], env_file)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("LAGO_LICENSE", result.stderr)

    def test_up_requires_an_env_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            result = run_lab_sh(["up"], Path(tmp) / "missing.env")
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("init", result.stderr)


class TestCommittedFiles(unittest.TestCase):
    def test_gitignore_hides_lab_env_and_transients(self):
        self.assertTrue(LAB_GITIGNORE.exists())
        text = LAB_GITIGNORE.read_text(encoding="utf-8")
        self.assertIn("lab.env", text)
        self.assertIn("__pycache__", text)

    def test_env_example_contains_no_real_secrets(self):
        self.assertTrue(LAB_ENV_EXAMPLE.exists())
        text = LAB_ENV_EXAMPLE.read_text(encoding="utf-8")
        for _, sample in clients.SAMPLE_DEFAULTS.items():
            self.assertNotIn(sample, text)
        for key in clients.SAMPLE_DEFAULTS:
            self.assertIn(key, text)
        self.assertIn("COMPOSE_PROJECT_NAME=weknora-lago-74", text)
        self.assertIn("LAGO_API_PORT=48891", text)
        self.assertIn("LAGO_FRONT_PORT=48892", text)
        # placeholders only: no generated-looking values
        self.assertNotRegex(text, r"(sk|rk)_(test|live)_[A-Za-z0-9]{16,}")


if __name__ == "__main__":
    unittest.main()
