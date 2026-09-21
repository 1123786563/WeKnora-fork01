"""Behavioral tests for the T02 experiment phase library.

Every phase is driven against an in-process stateful fake of the pinned Lago
v1.53.0 API plus a fake Stripe: the fakes replay the contract shapes verified
against the pinned source (gated subscription stays incomplete with an open
pending invoice; provider success settles it exactly once; duplicates are
rejected; decline cancels with payment_failed; manual payments are
premium-forbidden). Verdicts are asserted on full final state, not on error
codes alone, and every report must survive sanitize() with zero secret
occurrences.
"""

import json
import sys
import threading
import unittest
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

LAB_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(LAB_DIR))

import phases  # noqa: E402
from phases import (  # noqa: E402
    phase_activate,
    phase_cleanup,
    phase_decline_control,
    phase_duplicates,
    phase_gate,
    phase_manual,
    phase_provider_setup,
    phase_retries,
    phase_setup,
)

API_KEY = "api-key-canary-000"
STRIPE_KEY = "sk_test_" + "canary000000000000000000"
JWT = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ3ZWtub3JhIn0.jt-canary-signature"


class FakeStripeHandler(BaseHTTPRequestHandler):
    server_version = "FakeStripe/1"

    def log_message(self, format, *args):
        pass

    def _form(self):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        return urllib.parse.parse_qs(raw.decode("utf-8"))

    def _send(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        parts = self.path.strip("/").split("/")
        form = self._form()
        if parts[:2] == ["v1", "customers"]:
            self.server.seq += 1
            self.server.customers.append(f"cus_test_{self.server.seq}")
            self._send(200, {"id": f"cus_test_{self.server.seq}", "object": "customer"})
        elif len(parts) == 4 and parts[:2] == ["v1", "payment_methods"] and parts[3] == "attach":
            self.server.attaches.append((parts[2], form.get("customer", [""])[0]))
            self._send(200, {"id": parts[2], "customer": form.get("customer", [""])[0]})
        else:
            self._send(404, {"error": {"message": "no route"}})

    def do_DELETE(self):
        parts = self.path.strip("/").split("/")
        if len(parts) == 3 and parts[:2] == ["v1", "customers"]:
            self._send(200, {"id": parts[2], "deleted": True})
        else:
            self._send(404, {"error": {"message": "no route"}})


class FakeStripe(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self):
        super().__init__(("127.0.0.1", 0), FakeStripeHandler)
        self.seq = 0
        self.customers = []
        self.attaches = []

    @property
    def url(self):
        host, port = self.server_address
        return f"http://{host}:{port}"


class FakeLagoHandler(BaseHTTPRequestHandler):
    """Stateful fake of the pinned Lago v1.53.0 REST + GraphQL surface."""

    server_version = "FakeLago/1"

    def log_message(self, format, *args):
        pass

    # -- plumbing -----------------------------------------------------------
    def _send(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _body(self):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        if not raw:
            return None
        return json.loads(raw.decode("utf-8"))

    def _query(self):
        parsed = urllib.parse.urlsplit(self.path)
        return parsed.path, urllib.parse.parse_qs(parsed.query)

    # -- GET ----------------------------------------------------------------
    def do_GET(self):
        s = self.server
        path, query = self._query()
        if path == "/health":
            self._send(200, {"status": "ok"})
            return
        if path.startswith("/api/v1/customers/") and path.endswith("/payment_methods"):
            ext = path.split("/")[4]
            customer = s.customers.get(ext)
            if customer is None:
                self._send(404, {"status": 404, "error": "Not Found", "code": "customer_not_found"})
                return
            customer["methods_polls"] += 1
            if customer["methods_polls"] <= s.methods_delay_polls:
                self._send(200, {"payment_methods": []})
            else:
                self._send(200, {"payment_methods": [
                    {"lago_id": 9001, "customer_lago_id": 1,
                     "payment_method_type": "card",
                     "provider_payment_method_id": "pm_test_default",
                     "default": True, "status": "active"},
                ]})
            return
        if path.startswith("/api/v1/subscriptions/") and path.endswith("/entitlements"):
            ext = path.split("/")[4]
            sub = s.subscriptions.get(ext)
            if sub is None or (sub["status"] != "active" and not s.entitlements_200_while_incomplete):
                self._send(404, {"status": 404, "error": "Not Found", "code": "subscription_not_found"})
            else:
                self._send(200, {"entitlements": [
                    {"code": s.plan_feature_code, "name": s.plan_feature_code,
                     "privileges": [], "overrides": {}},
                ]})
            return
        if path.startswith("/api/v1/subscriptions/"):
            ext = path.split("/")[4].split("?")[0]
            wanted = query.get("status", ["active"])[0]
            sub = s.subscriptions.get(ext)
            if sub is None:
                self._send(404, {"status": 404, "error": "Not Found", "code": "subscription_not_found"})
            else:
                self._advance(sub)  # polling itself drives async settlement
                if sub["status"] != wanted:
                    self._send(404, {"status": 404, "error": "Not Found", "code": "subscription_not_found"})
                else:
                    self._send(200, {"subscription": self._subscription_json(sub)})
            return
        if path == "/api/v1/subscriptions":
            ext = query.get("external_id", [None])[0]
            statuses = query.get("status[]", query.get("status", ["active"]))
            matches = [
                sub for sub in s.subscriptions.values()
                if sub["external_id"] == ext and sub["status"] in statuses
            ]
            self._send(200, {
                "subscriptions": [self._subscription_json(m) for m in matches],
                "meta": {"total_count": len(matches)},
            })
            return
        if path == "/api/v1/invoices":
            ext = query.get("external_customer_id", [None])[0]
            self._maybe_reveal_invoices(ext)
            invoices = [
                inv for inv in s.invoices.values()
                if inv["external_customer_id"] == ext and inv["revealed"]
            ]
            self._send(200, {"invoices": [dict(inv) for inv in invoices]})
            return
        if path == "/api/v1/payments":
            ext = query.get("external_customer_id", [None])[0]
            payments = [p for p in s.payments if p["external_customer_id"] == ext]
            if s.mutate_after_duplicates:
                payments = payments + [dict(payments[-1], id=777777)] if payments else payments
            self._send(200, {"payments": payments})
            return
        self._send(404, {"status": 404, "error": "Not Found", "code": "not_found"})

    # -- POST ---------------------------------------------------------------
    def do_POST(self):
        s = self.server
        path, _query = self._query()
        body = self._body()
        if path == "/graphql":
            auth = self.headers.get("Authorization", "")
            query = (body or {}).get("query", "")
            if "loginUser" in query:
                password = ((body or {}).get("variables", {})
                            .get("input", {}).get("password", ""))
                if password == s.operator_password:
                    self._send(200, {"data": {"loginUser": {"token": s.jwt}}})
                else:
                    self._send(200, {"errors": [{"message": "Invalid email or password"}]})
                return
            if auth != f"Bearer {s.jwt}":
                self._send(200, {"errors": [{"message": "Not authorized"}]})
                return
            if "addStripePaymentProvider" in query:
                s.provider_registered = True
                self._send(200, {"data": {"addStripePaymentProvider": {
                    "id": "1", "code": "stripe-test", "name": "Stripe Test",
                }}})
            else:
                self._send(200, {"errors": [{"message": "unknown mutation"}]})
            return
        if path == "/api/v1/features":
            if s.fail_feature_create:
                self._send(422, {"status": 422, "error": "Unprocessable Entity",
                                 "code": "value_already_exist"})
                return
            feature = body["feature"]
            s.features[feature["code"]] = dict(feature, lago_id=len(s.features) + 1)
            self._send(200, {"feature": s.features[feature["code"]]})
            return
        if path == "/api/v1/plans":
            plan = body["plan"]
            s.plans[plan["code"]] = dict(
                plan, lago_id=len(s.plans) + 1, pay_in_advance=plan.get("pay_in_advance", True)
            )
            self._send(200, {"plan": s.plans[plan["code"]]})
            return
        if path.startswith("/api/v1/plans/") and path.endswith("/entitlements"):
            plan_code = path.split("/")[4]
            if plan_code not in s.plans:
                self._send(404, {"status": 404, "error": "Not Found", "code": "plan_not_found"})
                return
            entitlements = body.get("entitlements") or {}
            if isinstance(entitlements, list):  # legacy/wrong shape: runtime 500s
                self._send(500, {"status": 500, "error": "Internal Server Error"})
                return
            for feature_code in entitlements:
                s.plan_entitlements.setdefault(plan_code, []).append(feature_code)
                s.plan_feature_code = feature_code
            self._send(200, {"entitlements": [
                {"code": code, "name": code, "privileges": []}
                for code in s.plan_entitlements[plan_code]
            ]})
            return
        if path == "/api/v1/customers":
            customer = body["customer"]
            ext = customer["external_id"]
            if ext in s.customers:
                s.customers[ext].update(customer)
            else:
                s.customers[ext] = dict(customer, methods_polls=0)
            self._send(200, {"customer": dict(s.customers[ext], lago_id=len(s.customers))})
            return
        if path == "/api/v1/subscriptions":
            sub = body["subscription"]
            ext = sub["external_id"]
            existing = s.subscriptions.get(ext)
            if existing is not None:
                if s.duplicate_sub_accepted:
                    clone = dict(existing, lago_id=s.seq("sub"), polls=0)
                    s.subscriptions[ext + "-dup"] = clone
                    self._send(200, {"subscription": self._subscription_json(clone)})
                elif existing["status"] == "incomplete":
                    self._send(422, {"status": 422, "error": "Unprocessable Entity",
                                     "code": "subscription_incomplete"})
                else:
                    self._send(422, {"status": 422, "error": "Unprocessable Entity",
                                     "code": "value_already_exist"})
                return
            tag = ext.rsplit("-", 1)[-1]
            record = {
                "lago_id": s.seq("sub"), "external_id": ext,
                "external_customer_id": sub["external_customer_id"],
                "plan_code": sub["plan_code"],
                "status": "incomplete", "cancellation_reason": None,
                "mode": s.mode_for_customer.get(tag, "gate"), "polls": 0,
            }
            s.subscriptions[ext] = record
            invoice_id = s.seq("inv")
            s.invoices[invoice_id] = {
                "lago_id": invoice_id,
                "external_customer_id": sub["external_customer_id"],
                "subscription_external_id": ext,
                "invoice_type": "subscription",
                "status": "open", "payment_status": "pending", "number": None,
                "total_amount_cents": s.plan_amount_cents,
                "total_paid_amount_cents": 0,
                "revealed_polls": 0, "revealed": False,
                "pending_payment_added": False,
            }
            self._send(200, {"subscription": self._subscription_json(record)})
            return
        if path == "/api/v1/payments":
            if s.manual_allowed:
                invoice_id = body["payment"]["invoice_id"]
                invoice = s.invoices[invoice_id]
                s.payments.append({
                    "id": s.seq("pay"), "external_customer_id": invoice["external_customer_id"],
                    "invoice_lago_id": invoice_id, "status": "succeeded",
                    "amount_cents": body["payment"]["amount_cents"],
                    "provider_payment_id": None, "payable_id": invoice_id,
                    "payment_type": "manual",
                })
                invoice["total_paid_amount_cents"] = body["payment"]["amount_cents"]
                invoice["payment_status"] = "succeeded"
                sub = s.subscriptions[invoice["subscription_external_id"]]
                sub["status"] = "active"
                self._send(200, {"payment": {"lago_id": s.seq("pay")}})
            else:
                self._send(403, {"status": 403, "error": "Forbidden", "code": "forbidden",
                                 "message": "Manual payment recording requires a premium license"})
            return
        if path.startswith("/api/v1/invoices/") and path.endswith("/retry_payment"):
            invoice_id = int(path.split("/")[4])
            invoice = s.invoices.get(invoice_id)
            if invoice is None:
                self._send(404, {"status": 404, "error": "Not Found", "code": "invoice_not_found"})
            elif invoice["payment_status"] == "succeeded":
                self._send(422, {"status": 422, "error": "Unprocessable Entity",
                                 "code": "not_allowed",
                                 "error_message": "Invalid status"})
            else:
                self._send(200, {"invoice": {"lago_id": invoice_id,
                                             "payment_status": invoice["payment_status"]}})
            return
        self._send(404, {"status": 404, "error": "Not Found", "code": "not_found"})

    # -- DELETE -------------------------------------------------------------
    def do_DELETE(self):
        s = self.server
        path, _query = self._query()
        if path.startswith("/api/v1/subscriptions/"):
            ext = path.split("/")[4]
            if ext not in s.subscriptions:
                self._send(404, {"status": 404, "error": "Not Found", "code": "subscription_not_found"})
                return
            if s.fail_first_terminate and not s.terminates_failed:
                s.terminates_failed = True
                self._send(500, {"status": 500, "error": "Internal Server Error"})
                return
            del s.subscriptions[ext]
            self._send(200, {"subscription": {"external_id": ext, "status": "terminated"}})
            return
        if path.startswith("/api/v1/customers/"):
            ext = path.split("/")[4]
            if ext not in s.customers:
                self._send(404, {"status": 404, "error": "Not Found", "code": "customer_not_found"})
                return
            del s.customers[ext]
            self._send(200, {"customer": {"external_id": ext}})
            return
        if path.startswith("/api/v1/plans/"):
            code = path.split("/")[4]
            s.plans.pop(code, None)
            s.plan_entitlements.pop(code, None)
            self._send(200, {"plan": {"code": code}})
            return
        if path.startswith("/api/v1/features/"):
            code = path.split("/")[4]
            s.features.pop(code, None)
            self._send(200, {"feature": {"code": code}})
            return
        self._send(404, {"status": 404, "error": "Not Found", "code": "not_found"})

    # -- state machine ------------------------------------------------------
    def _subscription_json(self, sub):
        return {
            "lago_id": sub["lago_id"], "external_id": sub["external_id"],
            "external_customer_id": sub["external_customer_id"],
            "status": sub["status"], "cancellation_reason": sub["cancellation_reason"],
            "plan_code": sub["plan_code"],
        }

    def _advance(self, sub):
        """Apply a mode transition on each poll once the invoice is visible."""
        s = self.server
        self._maybe_reveal_invoices(sub["external_customer_id"])
        invoice = next(
            (inv for inv in s.invoices.values()
             if inv["subscription_external_id"] == sub["external_id"]), None
        )
        if invoice is None or not invoice["revealed"]:
            return
        sub["polls"] += 1
        threshold = {"activate": s.activate_flip_polls,
                     "decline": s.decline_flip_polls}.get(sub["mode"])
        if sub["mode"] == "gate":
            if s.gate_fails_after_charge and sub["polls"] >= s.activate_flip_polls:
                self._fail_payment(sub, invoice)
            return
        if threshold is None or sub["polls"] < threshold:
            return
        if sub["mode"] == "activate" and not s.never_activate:
            self._succeed_payment(sub, invoice)
        elif sub["mode"] == "decline" and not s.never_cancel:
            self._fail_payment(sub, invoice)

    def _succeed_payment(self, sub, invoice):
        s = self.server
        if invoice["payment_status"] == "succeeded":
            return
        payments = [p for p in s.payments if p["invoice_lago_id"] == invoice["lago_id"]]
        base = payments[0] if payments else {
            "id": s.seq("pay"), "external_customer_id": invoice["external_customer_id"],
            "invoice_lago_id": invoice["lago_id"], "amount_cents": invoice["total_amount_cents"],
            "payable_id": invoice["lago_id"], "payment_type": "stripe",
        }
        count = 2 if s.duplicate_payment_injection else 1
        for i in range(count):
            s.payments.append(dict(base, id=s.seq("pay"), status="succeeded",
                                   provider_payment_id=f"pi_test_{sub['lago_id']}_{i}",
                                   amount_cents=invoice["total_amount_cents"]))
            invoice["total_paid_amount_cents"] += invoice["total_amount_cents"]
        invoice["status"] = "finalized"
        invoice["number"] = f"INV-{invoice['lago_id']:04d}"
        invoice["payment_status"] = "succeeded"
        sub["status"] = "active"

    def _fail_payment(self, sub, invoice):
        s = self.server
        if sub["status"] == "canceled":
            return
        s.payments.append({
            "id": s.seq("pay"), "external_customer_id": invoice["external_customer_id"],
            "invoice_lago_id": invoice["lago_id"], "status": "failed",
            "amount_cents": invoice["total_amount_cents"],
            "provider_payment_id": f"pi_test_{sub['lago_id']}_f", "payable_id": invoice["lago_id"],
        })
        invoice["status"] = "closed"
        invoice["payment_status"] = "failed"
        sub["status"] = "canceled"
        sub["cancellation_reason"] = "payment_failed"

    def _maybe_reveal_invoices(self, ext):
        s = self.server
        for invoice in s.invoices.values():
            if invoice["external_customer_id"] != ext or invoice["revealed"]:
                continue
            invoice["revealed_polls"] += 1
            if invoice["revealed_polls"] > s.invoice_appear_polls:
                invoice["revealed"] = True
                if not invoice["pending_payment_added"]:
                    invoice["pending_payment_added"] = True
                    s.payments.append({
                        "id": s.seq("pay"),
                        "external_customer_id": ext,
                        "invoice_lago_id": invoice["lago_id"], "status": "pending",
                        "amount_cents": invoice["total_amount_cents"],
                        "provider_payment_id": None, "payable_id": invoice["lago_id"],
                        "payment_type": "stripe",
                    })


class FakeLago(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self):
        super().__init__(("127.0.0.1", 0), FakeLagoHandler)
        self.jwt = JWT
        self.operator_password = "operator-password-canary"
        self.seq_counters = {"sub": 0, "inv": 0, "pay": 0}
        self.features = {}
        self.plans = {}
        self.plan_entitlements = {}
        self.plan_feature_code = None
        self.customers = {}
        self.subscriptions = {}
        self.invoices = {}
        self.payments = []
        self.provider_registered = False
        self.plan_amount_cents = 5000
        self.mode_for_customer = {"a": "gate", "b": "activate", "c": "decline"}
        # knobs
        self.methods_delay_polls = 1
        self.invoice_appear_polls = 1
        self.activate_flip_polls = 2
        self.decline_flip_polls = 2
        self.never_activate = False
        self.never_cancel = False
        self.duplicate_payment_injection = False
        self.manual_allowed = False
        self.duplicate_sub_accepted = False
        self.fail_first_terminate = False
        self.terminates_failed = False
        self.entitlements_200_while_incomplete = False
        self.gate_fails_after_charge = False
        self.mutate_after_duplicates = False
        self.fail_feature_create = False

    def seq(self, kind):
        self.seq_counters[kind] += 1
        return self.seq_counters[kind]

    @property
    def url(self):
        host, port = self.server_address
        return f"http://{host}:{port}"


class stack:
    """Context manager: fake Lago + fake Stripe + fast-polling RunContext."""

    def __enter__(self):
        self.lago = FakeLago()
        self.stripe = FakeStripe()
        for server in (self.lago, self.stripe):
            threading.Thread(target=server.serve_forever, daemon=True).start()
        self.ctx = self._make_ctx()
        return self

    def _make_ctx(self, **overrides):
        kwargs = dict(
            lago_url=self.lago.url,
            api_key=API_KEY,
            stripe_key=STRIPE_KEY,
            graphql_jwt=JWT,
            stripe_base_url=self.stripe.url,
            poll_interval=0.01,
            poll_timeout=1.0,
            stability_rounds=2,
            stability_delay=0.01,
            request_timeout=5.0,
        )
        kwargs.update(overrides)
        return phases.RunContext(**kwargs)

    def __exit__(self, *exc):
        for server in (self.lago, self.stripe):
            server.shutdown()
            server.server_close()
        return False


def seeded(env):
    """Run setup + provider phases and assert both passed."""
    setup_report = phase_setup(env.ctx)
    assert setup_report["status"] == "pass", setup_report
    provider_report = phase_provider_setup(env.ctx)
    assert provider_report["status"] == "pass", provider_report
    return env.ctx


def settled_gate_and_activation(ctx):
    gate = phase_gate(ctx)
    activation = phase_activate(ctx)
    assert gate["status"] == "pass", gate
    assert activation["status"] == "pass", activation
    return gate, activation


class TestSetupPhase(unittest.TestCase):
    def test_creates_feature_plan_and_entitlement(self):
        with stack() as env:
            report = phase_setup(env.ctx)
        self.assertEqual(report["status"], "pass", report)
        observed = report["observed"]
        self.assertEqual(observed["feature_create_status"], 200)
        self.assertEqual(observed["plan_create_status"], 200)
        self.assertEqual(observed["entitlement_attach_status"], 200)
        self.assertTrue(observed["plan_pay_in_advance"])
        self.assertEqual(observed["plan_interval"], "monthly")
        self.assertGreater(observed["plan_amount_cents"], 0)

    def test_fails_when_feature_create_rejected(self):
        with stack() as env:
            env.lago.fail_feature_create = True
            report = phase_setup(env.ctx)
        self.assertEqual(report["status"], "fail", report)
        self.assertEqual(report["observed"]["feature_create_status"], 422)


class TestProviderSetupPhase(unittest.TestCase):
    def test_blocked_env_without_stripe_key(self):
        with stack() as env:
            env.ctx = env._make_ctx(stripe_key=None)
            setup = phase_setup(env.ctx)
            provider = phase_provider_setup(env.ctx)
            gate = phase_gate(env.ctx)
            manual = phase_manual(env.ctx)
            cleanup = phase_cleanup(env.ctx)
        self.assertEqual(setup["status"], "pass")
        self.assertEqual(provider["status"], "blocked-env")
        self.assertIn("Stripe", provider["observed"]["blocked_reason"])
        self.assertEqual(gate["status"], "blocked-env")
        self.assertEqual(manual["status"], "blocked-env")
        self.assertEqual(cleanup["status"], "pass")

    def test_blocked_env_when_jwt_rejected(self):
        with stack() as env:
            env.ctx = env._make_ctx(graphql_jwt="eyJwrong.wrong.wrongsig")
            report = phase_provider_setup(env.ctx)
        self.assertEqual(report["status"], "blocked-env", report)

    def test_blocked_env_when_stripe_unreachable(self):
        with stack() as env:
            env.ctx = env._make_ctx(stripe_base_url="http://127.0.0.1:9")
            report = phase_provider_setup(env.ctx)
        self.assertEqual(report["status"], "blocked-env", report)
        self.assertIn("unreachable", report["observed"]["blocked_reason"])

    def test_customers_get_payment_methods_after_polling(self):
        with stack() as env:
            ctx = seeded(env)
            provider_report = ctx.state.get("provider_report")
            customers = ctx.state["customers"]
            self.assertIsNotNone(provider_report)
        for tag in ("a", "b", "c"):
            self.assertIn(tag, customers)
            self.assertTrue(customers[tag]["external_id"].startswith("weknora-t02-"))
            self.assertTrue(customers[tag]["stripe_customer_id"].startswith("cus_test_"))
            self.assertTrue(customers[tag]["payment_methods_ready"])


class TestGatePhase(unittest.TestCase):
    def test_requires_incomplete_and_entitlement_404(self):
        with stack() as env:
            ctx = seeded(env)
            report = phase_gate(ctx)
        self.assertEqual(report["status"], "pass", report)
        observed = report["observed"]
        self.assertEqual(observed["subscription_status"], "incomplete")
        self.assertEqual(observed["entitlements_status"], 404)
        self.assertEqual(observed["invoice_status"], "open")
        self.assertEqual(observed["invoice_payment_status"], "pending")
        self.assertIsNone(observed["invoice_number"])
        self.assertLessEqual(observed["payments_non_succeeded_count"], 1)
        self.assertTrue(observed["stable_window"]["still_incomplete"])
        self.assertTrue(observed["stable_window"]["still_pending"])

    def test_fails_on_entitlements_deviation_with_contract_note(self):
        with stack() as env:
            ctx = seeded(env)
            env.lago.entitlements_200_while_incomplete = True
            report = phase_gate(ctx)
        self.assertEqual(report["status"], "fail", report)
        self.assertEqual(report["observed"]["entitlements_status"], 200)
        self.assertTrue(any("entitlement" in note for note in report["contract_notes"]))

    def test_falls_back_to_pre_charge_window_when_charge_fails(self):
        with stack() as env:
            ctx = seeded(env)
            env.lago.gate_fails_after_charge = True
            report = phase_gate(ctx)
        self.assertEqual(report["status"], "pass", report)
        self.assertEqual(report["observed"]["subscription_status"], "incomplete")
        notes = " ".join(report["contract_notes"])
        self.assertIn("canceled", notes)


class TestManualPhase(unittest.TestCase):
    def test_records_forbidden_without_mutating_state(self):
        with stack() as env:
            ctx = seeded(env)
            phase_gate(ctx)
            report = phase_manual(ctx)
        self.assertEqual(report["status"], "pass", report)
        observed = report["observed"]
        self.assertEqual(observed["http_status"], 403)
        self.assertTrue(observed["state_unchanged"])
        self.assertEqual(observed["invoice_status_after"], "open")
        self.assertEqual(observed["subscription_status_after"], "incomplete")
        self.assertIn("invoice_id", json.dumps(observed.get("param_contract", {})))

    def test_fails_when_runtime_allows_manual_payment(self):
        with stack() as env:
            ctx = seeded(env)
            phase_gate(ctx)
            env.lago.manual_allowed = True
            report = phase_manual(ctx)
        self.assertEqual(report["status"], "fail", report)
        self.assertEqual(report["observed"]["http_status"], 200)
        self.assertFalse(report["observed"]["state_unchanged"])


class TestActivatePhase(unittest.TestCase):
    def test_settles_exactly_once(self):
        with stack() as env:
            ctx = seeded(env)
            report = phase_activate(ctx)
        self.assertEqual(report["status"], "pass", report)
        observed = report["observed"]
        self.assertEqual(observed["subscription_status"], "active")
        self.assertEqual(observed["entitlements_status"], 200)
        self.assertEqual(observed["invoice_status"], "finalized")
        self.assertTrue(observed["invoice_number"])
        self.assertEqual(observed["invoice_payment_status"], "succeeded")
        self.assertEqual(observed["payments_succeeded_count"], 1)
        self.assertTrue(observed["provider_payment_id"])
        self.assertEqual(
            observed["total_paid_amount_cents"],
            observed["invoice_total_amount_cents"],
        )

    def test_fails_without_exactly_one_succeeded_payment(self):
        with stack() as env:
            ctx = seeded(env)
            env.lago.duplicate_payment_injection = True
            report = phase_activate(ctx)
        self.assertEqual(report["status"], "fail", report)
        self.assertEqual(report["observed"]["payments_succeeded_count"], 2)

    def test_poll_timeout_fails_with_last_observed_state(self):
        with stack() as env:
            ctx = seeded(env)
            env.lago.never_activate = True
            report = phase_activate(ctx)
        self.assertEqual(report["status"], "fail", report)
        observed = report["observed"]
        self.assertEqual(observed["last_observed_incomplete"], "incomplete")
        self.assertTrue(observed["poll_exhausted"])


class TestDuplicatesPhase(unittest.TestCase):
    def test_asserts_final_state_after_duplicate_attempts(self):
        with stack() as env:
            ctx = seeded(env)
            settled_gate_and_activation(ctx)
            report = phase_duplicates(ctx)
        self.assertEqual(report["status"], "pass", report)
        observed = report["observed"]
        self.assertEqual(observed["re_post"]["http_status"], 422)
        self.assertEqual(observed["re_post"]["code"], "value_already_exist")
        self.assertNotEqual(observed["retry_payment"]["http_status"], 200)
        self.assertEqual(observed["manual_re_post"]["http_status"], 403)
        final = observed["final"]
        self.assertEqual(final["subscription_status"], "active")
        self.assertEqual(final["payments_succeeded_count"], 1)
        self.assertEqual(final["invoice_status"], "finalized")
        self.assertEqual(final["invoice_payment_status"], "succeeded")
        self.assertEqual(final["total_paid_amount_cents"], final["invoice_total_amount_cents"])

    def test_fails_when_duplicate_registration_accepted(self):
        with stack() as env:
            ctx = seeded(env)
            settled_gate_and_activation(ctx)
            env.lago.duplicate_sub_accepted = True
            report = phase_duplicates(ctx)
        self.assertEqual(report["status"], "fail", report)

    def test_fails_on_state_drift_despite_correct_error_codes(self):
        with stack() as env:
            ctx = seeded(env)
            settled_gate_and_activation(ctx)
            env.lago.mutate_after_duplicates = True
            report = phase_duplicates(ctx)
        self.assertEqual(report["status"], "fail", report)
        self.assertEqual(report["observed"]["re_post"]["http_status"], 422)
        self.assertEqual(report["observed"]["final"]["payments_succeeded_count"], 2)


class TestRetriesPhase(unittest.TestCase):
    def test_recovers_same_identity_and_keeps_gate_pending(self):
        with stack() as env:
            ctx = seeded(env)
            settled_gate_and_activation(ctx)
            report = phase_retries(ctx)
        self.assertEqual(report["status"], "pass", report)
        observed = report["observed"]
        recovery = observed["response_loss_recovery"]
        self.assertTrue(recovery["same_lago_id"])
        self.assertEqual(recovery["subscription_status"], "active")
        self.assertEqual(recovery["subscription_count"], 1)
        gate_retry = observed["pending_gate_retry"]
        self.assertEqual(gate_retry["invoice_payment_status_after"], "pending")
        self.assertEqual(
            gate_retry["payments_count_after"], gate_retry["payments_count_before"]
        )
        self.assertEqual(gate_retry["succeeded_count_after"], 0)

    def test_fails_when_second_commercial_object_appears(self):
        with stack() as env:
            ctx = seeded(env)
            settled_gate_and_activation(ctx)
            env.lago.duplicate_sub_accepted = True
            report = phase_retries(ctx)
        self.assertEqual(report["status"], "fail", report)
        self.assertEqual(report["observed"]["response_loss_recovery"]["subscription_count"], 2)


class TestDeclineControlPhase(unittest.TestCase):
    def test_requires_canceled_payment_failed(self):
        with stack() as env:
            ctx = seeded(env)
            report = phase_decline_control(ctx)
        self.assertEqual(report["status"], "pass", report)
        observed = report["observed"]
        self.assertEqual(observed["subscription_status"], "canceled")
        self.assertEqual(observed["cancellation_reason"], "payment_failed")
        self.assertEqual(observed["invoice_status"], "closed")
        self.assertEqual(observed["entitlements_status"], 404)

    def test_timeout_fails_with_last_observed_state(self):
        with stack() as env:
            ctx = seeded(env)
            env.lago.never_cancel = True
            report = phase_decline_control(ctx)
        self.assertEqual(report["status"], "fail", report)
        self.assertEqual(report["observed"]["last_observed_incomplete"], "incomplete")
        self.assertTrue(report["observed"]["poll_exhausted"])


class TestCleanupPhase(unittest.TestCase):
    def test_reports_every_object_and_passes_on_full_happy_path(self):
        with stack() as env:
            ctx = seeded(env)
            settled_gate_and_activation(ctx)
            phase_decline_control(ctx)
            report = phase_cleanup(ctx)
        self.assertEqual(report["status"], "pass", report)
        outcomes = {item["object"]: item["outcome"] for item in report["observed"]["objects"]}
        self.assertEqual(outcomes.get("subscription:a"), "deleted")
        self.assertEqual(outcomes.get("subscription:b"), "deleted")
        self.assertEqual(outcomes.get("subscription:c"), "deleted")
        self.assertEqual(outcomes.get("customer:a"), "deleted")
        self.assertEqual(outcomes.get("customer:b"), "deleted")
        self.assertEqual(outcomes.get("customer:c"), "deleted")
        self.assertEqual(outcomes.get("plan"), "deleted")
        self.assertEqual(outcomes.get("feature"), "deleted")
        stripe_deleted = [o for o in report["observed"]["objects"] if o["object"] == "stripe_customer"]
        self.assertEqual(len(stripe_deleted), 3)

    def test_reports_distinct_failure_and_continues(self):
        with stack() as env:
            ctx = seeded(env)
            settled_gate_and_activation(ctx)
            env.lago.fail_first_terminate = True
            report = phase_cleanup(ctx)
        self.assertEqual(report["status"], "fail", report)
        outcomes = {item["object"]: item["outcome"] for item in report["observed"]["objects"]}
        self.assertEqual(outcomes.get("subscription:a"), "failed")
        self.assertEqual(outcomes.get("subscription:b"), "deleted")
        self.assertEqual(outcomes.get("plan"), "deleted")

    def test_skips_never_created_objects(self):
        with stack() as env:
            report = phase_cleanup(env.ctx)
        self.assertEqual(report["status"], "pass", report)
        self.assertEqual(report["observed"]["objects"], [])


class TestRunnerContract(unittest.TestCase):
    def test_cleanup_runs_after_midphase_exception(self):
        with stack() as env:
            ctx = seeded(env)
            phase_gate(ctx)
            original = phases.phase_activate

            def exploding(inner_ctx):
                raise RuntimeError("simulated mid-phase crash")

            phases.phase_activate = exploding
            cleanup_report = None
            try:
                with self.assertRaises(RuntimeError):
                    try:
                        phases.phase_activate(ctx)
                    finally:
                        cleanup_report = phase_cleanup(ctx)
            finally:
                phases.phase_activate = original
        self.assertIsNotNone(cleanup_report)
        self.assertEqual(cleanup_report["status"], "pass", cleanup_report)

    def test_every_report_is_sanitized(self):
        canaries = (API_KEY, STRIPE_KEY, JWT)
        with stack() as env:
            ctx = seeded(env)
            reports = [
                phase_setup(ctx),
                phase_provider_setup(ctx),
                phase_gate(ctx),
                phase_manual(ctx),
                phase_activate(ctx),
                phase_duplicates(ctx),
                phase_retries(ctx),
                phase_decline_control(ctx),
                phase_cleanup(ctx),
            ]
        for report in reports:
            text = json.dumps(report)
            for canary in canaries:
                self.assertNotIn(canary, text, f"leak in phase {report['phase']}")


if __name__ == "__main__":
    unittest.main()
