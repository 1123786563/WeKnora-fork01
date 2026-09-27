"""Probe phases for the #82 settle-trigger contract lab (t11, R-4 dual-track).

Each phase drives real Lago/Stripe calls through the payment-activation
lab's clients (imported read-only) and returns a sanitized report dict::

    {"phase": str, "run_id": str, "expected": str, "observed": dict,
     "status": "pass" | "fail" | "blocked-env", "evidence": dict,
     "contract_notes": [str]}

The probe verifies the FIVE links the #82 implementation (D2') depends on
(the α dual-track: channel collects + WeKnora drives the Stripe Provider
gated settle charge -> Lago built-in webhook finalize -> active):

- Setup (``phase_gated_3ds``): provider + 3DS-card customer + gated create ->
  incomplete with the stuck gating PaymentIntent (F7).
- P-A (``phase_settle_probe``): the stuck gating PI is locatable on the
  PROVIDER side only -- ``GET /v1/payment_intents?customer=`` answers an
  unsettled intent (status requires_payment_method / requires_action) whose
  metadata carries ``lago_invoice_id`` (F5). The OBSERVED status word is
  recorded (t10's only observed shape was requires_payment_method).
- P-B/C (``phase_settle_trigger`` steps 1-2): attach the settle pm + set it
  as the customer default, then ``POST /v1/payment_intents/{id}`` (update
  payment_method) + ``POST /v1/payment_intents/{id}/confirm`` answers
  status=="succeeded" (off-session synchronous charge) -- THE UNVERIFIED
  LINK of D2' step (iv).
- P-D (step 3): the built-in receive chain accepts a REAL
  payment_intent.succeeded event (body = the PI object read back from the
  Stripe API; signature = the provider's real webhook_secret from the Lago
  DB) at ``POST /webhooks/stripe/{org_id}`` -- HTTP 200 and the
  inbound_webhooks row lands (F9/F10; transport-leg stand-in per D8).
- P-E (steps 4-5): the webhook chain finalizes the authority -- subscription
  active + invoice finalized/numbered/payment_status succeeded + EXACTLY ONE
  succeeded payment + entitlements 200; re-delivering the SAME event is a
  no-op (four objects byte-identical, F2).

Verdict rules follow the T02/T10 labs: ``pass`` only on full authoritative
final state; ``blocked-env`` for environment gaps; ``fail`` means the pinned
runtime violated the expected contract. Polling is always bounded; a timeout
is a ``fail`` with the last observed state attached.
"""

from __future__ import annotations

import importlib.util
import json
import re
import subprocess
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime, timezone
from pathlib import Path

# (A-06 / F17) The payment-activation lab's clients, loaded READ-ONLY under
# an explicit alias -- the same discipline this directory's fixtures.py
# already uses (pa_fixtures). No sys.path.insert(0, _PA_DIR) happens here:
# mutating the import search path INVERTED the module binding whenever this
# file was imported by anything other than run_lab.py (REPL/pytest/a future
# tool), making the bare name ``fixtures`` resolve to payment-activation's
# same-named module. With the alias, the bare ``import fixtures`` below
# always resolves through the normal search order to THIS directory's
# module, and clients stays the explicitly-aliased pa implementation.
_PA_DIR = Path(__file__).resolve().parent.parent / "payment-activation"
_clients_spec = importlib.util.spec_from_file_location(
    "pa_clients", _PA_DIR / "clients.py")
clients = importlib.util.module_from_spec(_clients_spec)
_clients_spec.loader.exec_module(clients)

import fixtures  # noqa: E402  (THIS directory's payloads, bound by name)

_REPO_DIR = Path(__file__).resolve().parents[3]
_COMPOSE_FILE = _REPO_DIR / "deploy" / "lago" / "compose.yaml"

# Stripe test-mode payment-method tokens (public, throwaway). The gate pm is
# the 3DS-challenge card: its off-session charge stalls, which is the stable
# stuck-payment window the probe needs (F7). The settle pm settles
# immediately; overridable via LAB_SETTLE_PM for the Task 10 topology.
GATED_PM = "pm_card_threeDSecure2Required"
SETTLE_PM = "pm_card_visa"
STRIPE_PROVIDER_NAME = "WeKnora T11 Stripe Test"

PASS, FAIL, BLOCKED = "pass", "fail", "blocked-env"

# (OCR r2 / safety constraint) provider_code is interpolated into rails
# snippets below; a strict charset whitelist gates EVERY interpolation site
# (anything outside [A-Za-z0-9_-] — quotes, whitespace, semicolons — would
# break or inject the Ruby fragment).
_PROVIDER_CODE_SHAPE = re.compile(r"[A-Za-z0-9_-]+")


def _valid_provider_code(code):
    return bool(code) and _PROVIDER_CODE_SHAPE.fullmatch(code) is not None


def _utc_now():
    return datetime.now(timezone.utc).isoformat()


def _ok(status):
    return isinstance(status, int) and 200 <= status < 300


class RunContext:
    """One probe run: identity, clients, shared state, and poll knobs."""

    def __init__(self, lago_url, api_key, run_id=None, prefix=None,
                 stripe_key=None, graphql_jwt=None,
                 stripe_base_url="https://api.stripe.com",
                 compose_project="weknora-lago-t11", lab_env=None,
                 settle_pm=None,
                 poll_interval=2.0, poll_timeout=180.0,
                 trigger_poll_timeout=120.0, pm_poll_timeout=60.0,
                 request_timeout=30.0):
        self.run_id = run_id or str(uuid.uuid4())
        self.prefix = prefix or f"weknora-t11-{self.run_id[:8]}"
        self.lago_url = lago_url.rstrip("/")
        self.api_key = api_key
        self.stripe_key = stripe_key
        self.graphql_jwt = graphql_jwt
        self.lago = clients.LagoRestClient(lago_url, api_key, timeout=request_timeout)
        self.stripe = (
            clients.StripeTestClient(stripe_key, base_url=stripe_base_url,
                                     timeout=request_timeout)
            if stripe_key else None
        )
        self.compose_project = compose_project
        self.lab_env = Path(lab_env) if lab_env else None
        self.settle_pm = settle_pm or SETTLE_PM
        self.poll_interval = poll_interval
        self.poll_timeout = poll_timeout
        self.trigger_poll_timeout = trigger_poll_timeout
        self.pm_poll_timeout = pm_poll_timeout
        self.state = {}
        self._notes = []
        self._graphql_organization_id = None

    # -- identity helpers ---------------------------------------------------
    def customer_external_id(self):
        return f"{self.prefix}-cust-a"

    def subscription_external_id(self):
        return f"{self.prefix}-sub-a"

    @property
    def plan_code(self):
        return f"{self.prefix}-plan"

    @property
    def feature_code(self):
        return f"{self.prefix}-feature"

    # -- secrets + notes ----------------------------------------------------
    def secret_values(self):
        return tuple(v for v in (self.api_key, self.stripe_key, self.graphql_jwt) if v)

    def note(self, text):
        self._notes.append(text)

    def drain_notes(self):
        notes = list(self._notes)
        self._notes = []
        return notes

    def _resolve_graphql_organization_id(self):
        """Lago v1.53.0 scopes GraphQL mutations by organization: the API
        reads it from the x-lago-organization header (RequiredOrganization
        raises 'Missing organization id' without it). Resolve the operator
        organization's lago_id once via REST and cache it; None when
        unresolved (header omitted)."""
        if self._graphql_organization_id is None:
            try:
                status, body = self.lago.request("GET", "/api/v1/organizations")
                org = ((body or {}).get("organization") or {})
                if _ok(status) and org.get("lago_id"):
                    self._graphql_organization_id = org["lago_id"]
            except (OSError, clients.OffOriginRedirect) as error:
                self.note(
                    "x-lago-organization resolution failed "
                    f"({error.__class__.__name__}); header omitted — GraphQL "
                    "mutations may be rejected with 'Missing organization id'"
                )
        return self._graphql_organization_id

    def graphql(self, query, variables):
        """One GraphQL call with the operator JWT. Returns (status, body)."""
        payload = json.dumps({"query": query, "variables": variables}).encode("utf-8")
        headers = {
            "Content-Type": "application/json",
            "Accept": "application/json",
            "Authorization": f"Bearer {self.graphql_jwt}" if self.graphql_jwt else "",
        }
        organization_id = self._resolve_graphql_organization_id()
        if organization_id:
            headers["x-lago-organization"] = organization_id
        request = urllib.request.Request(
            f"{self.lago_url}/graphql",
            data=payload,
            headers=headers,
            method="POST",
        )
        # (OCR r2) the GraphQL egress uses the shared ORIGIN-BOUND proxyless
        # opener (clients._proxyless_opener) — the same redirect discipline
        # the REST clients already enforce, instead of a bare proxy bypass.
        from urllib.parse import urlsplit
        opener = clients._proxyless_opener(urlsplit(self.lago_url)[:2])
        with opener.open(request, timeout=self.lago._timeout) as response:
            status = response.status
            raw = response.read()
        try:
            return status, json.loads(raw.decode("utf-8")) if raw else None
        except ValueError:
            return status, {"_raw": raw.decode("utf-8", errors="replace")}

    def poll(self, fn, done, description, timeout=None):
        """Bounded polling. Returns (finished, last_result, attempts)."""
        deadline = time.monotonic() + (timeout or self.poll_timeout)
        attempts = 0
        last = None
        while True:
            attempts += 1
            last = fn()
            if done(last):
                return True, last, attempts
            if time.monotonic() >= deadline:
                self.note(f"poll exhausted: {description} (attempts={attempts})")
                return False, last, attempts
            time.sleep(self.poll_interval)

    # -- shared API accessors -----------------------------------------------
    def payments_for(self, external_customer_id):
        """GET /api/v1/payments?external_customer_id= (F5: no visibility
        filter -- non-succeeded rows answer). Returns (status, rows)."""
        status, body = self.lago.get(
            f"/api/v1/payments?external_customer_id={external_customer_id}")
        rows = body.get("payments", []) if isinstance(body, dict) else []
        return status, rows

    def invoices_for(self, external_customer_id):
        status, body = self.lago.get(
            f"/api/v1/invoices?external_customer_id={external_customer_id}")
        rows = body.get("invoices", []) if isinstance(body, dict) else []
        return status, rows

    def payment_methods_for(self, external_customer_id):
        status, body = self.lago.get(
            f"/api/v1/customers/{external_customer_id}/payment_methods")
        rows = body.get("payment_methods", []) if isinstance(body, dict) else []
        return status, rows

    # -- lab stack DB (read-only psql via docker compose exec; P-D only) ----
    def lago_db_query(self, sql):
        """One scalar psql query inside the lab stack's db service.

        Runs ``docker compose -f deploy/lago/compose.yaml --env-file lab.env
        -p <project> exec -T db psql -U lago -tAc <sql>`` and returns the
        first output line (or None). Secrets read this way (the provider
        webhook_secret) stay in memory only -- never reported raw.
        """
        command = ["docker", "compose", "-f", str(_COMPOSE_FILE)]
        if self.lab_env and self.lab_env.exists():
            command += ["--env-file", str(self.lab_env)]
        command += ["-p", self.compose_project,
                    "exec", "-T", "db", "psql", "-U", "lago", "-tAc", sql]
        # (A-07 / F52) A docker/psql subprocess failure is an ENVIRONMENT
        # gap (docker down, container name drift, subprocess timeout --
        # TimeoutExpired is a SubprocessError, NOT an OSError), not a
        # contract verdict: answer None and let the caller's own
        # before/after logic decide instead of escaping into the harness's
        # unexpected_error bucket.
        try:
            result = subprocess.run(command, capture_output=True, text=True,
                                    timeout=30)
        except (OSError, subprocess.SubprocessError):
            return None
        output = (result.stdout or "").strip().splitlines()
        return output[0].strip() if output else None

    # -- lab api container (rails runner; model-layer reads/writes only) ----
    def _rails_runner(self, code):
        """Run one Ruby snippet inside the lab api container (docker compose
        exec). Returns stdout text or None. NEVER writes SQL directly -- the
        model layer owns the encrypted/jsonb columns."""
        command = ["docker", "compose", "-f", str(_COMPOSE_FILE)]
        if self.lab_env and self.lab_env.exists():
            command += ["--env-file", str(self.lab_env)]
        command += ["-p", self.compose_project,
                    "exec", "-T", "api", "bin/rails", "runner", code]
        try:
            result = subprocess.run(command, capture_output=True, text=True,
                                    timeout=180)
        except (OSError, subprocess.SubprocessError):
            return None
        return (result.stdout or "").strip() or None

    def _runner_sentinel(self, code):
        """rails runner output with sentinel extraction.

        The api container prints a "Sidekiq Pro is not installed" banner on
        stdout BEFORE the snippet's own output — a bare read would mistake
        the banner for the value (observed t11 run 3: the banner was
        returned as the webhook secret and Lago answered 500 ArgumentError).
        The snippet's print is wrapped in START/END sentinels; only the
        between-text is returned (None when absent).
        """
        wrapped = ("print \"__WKNORA_T11_START__\"; " + code +
                   "; print \"__WKNORA_T11_END__\"")
        raw = self._rails_runner(wrapped) or ""
        if "__WKNORA_T11_START__" not in raw or "__WKNORA_T11_END__" not in raw:
            return None
        return raw.split("__WKNORA_T11_START__", 1)[1].split("__WKNORA_T11_END__", 1)[0] or None

    def provider_webhook_secret(self, provider_code):
        """The provider's webhook_secret (BaseProvider settings accessor).

        OBSERVED BOUNDARY (t11 run 1): Stripe REFUSES to register a loopback
        webhook URL ("Invalid URL: URL must be publicly accessible") —
        RegisterWebhookJob errors and NO secret ever lands in settings, so
        the plan's F10 assumption (psql-readable webhook_secret column) does
        not hold on a local stack (the column lives in the settings jsonb,
        and it is EMPTY). Fallback, transport-leg stand-in per D8: mint a
        REAL Stripe-generated secret through the SAME API
        RegisterWebhookService uses (public-format placeholder URL — the
        endpoint never receives pushes; the harness delivers), store it
        through the model layer exactly where RegisterWebhookService would,
        and read it back. Signature verification and the whole receive
        chain stay 100% Lago built-in.
        """
        # (OCR r2) the interpolation gate: a non-conforming provider_code
        # must fail closed BEFORE any rails snippet is built.
        if not _valid_provider_code(provider_code):
            self.note(f"provider_code {provider_code!r} failed the charset "
                      "whitelist — refusing to interpolate into the rails snippet")
            return None
        value = self._runner_sentinel(
            "print PaymentProviders::StripeProvider.where(deleted_at: nil)"
            f".find_by(code: '{provider_code}').try(:webhook_secret).to_s")
        if value:
            return value
        if not self.stripe:
            return None
        # Mint a Stripe-generated secret for the local stack.
        status, body = self.stripe._form("POST", "/v1/webhook_endpoints", {
            "url": f"https://lago-t11-local.invalid/webhooks/stripe/{self.run_id[:8]}",
            "enabled_events[]": "payment_intent.succeeded",
            "description": f"weknora t11 local settle probe {self.run_id[:8]} "
                           "(secret-minting stand-in; harness-delivered)",
        })
        if not _ok(status) or not isinstance(body, dict) or not body.get("secret"):
            self.note(f"Stripe webhook_endpoint create failed (HTTP {status}); "
                      "cannot mint a provider secret for the local stack")
            return None
        secret = body["secret"]
        self.state["stripe_webhook_endpoint_id"] = body.get("id")
        # (OCR r2) the secret rides STDIN into the container — never the
        # docker exec argv (-e WSECRET=… is visible in the local process
        # list for the exec's lifetime; stdin is not).
        written = self._compose_exec_stdin(
            secret,
            "PaymentProviders::StripeProvider.where(deleted_at: nil)"
            f".find_by(code: '{provider_code}')"
            ".update!(webhook_secret: STDIN.read)")
        if not written:
            self.note("model-layer webhook_secret store failed")
            return None
        readback = self._runner_sentinel(
            "print PaymentProviders::StripeProvider.where(deleted_at: nil)"
            f".find_by(code: '{provider_code}').try(:webhook_secret).to_s")
        return readback or None

    def _compose_exec_stdin(self, secret, code):
        """docker compose exec running a rails snippet with the secret piped
        via STDIN (the argv never carries it)."""
        command = ["docker", "compose", "-f", str(_COMPOSE_FILE)]
        if self.lab_env and self.lab_env.exists():
            command += ["--env-file", str(self.lab_env)]
        command += ["-p", self.compose_project, "exec", "-T",
                    "api", "bin/rails", "runner", code]
        try:
            result = subprocess.run(command, input=secret, capture_output=True,
                                    text=True, timeout=180)
        except (OSError, subprocess.SubprocessError):
            return False
        return result.returncode == 0

    def organization_webhook_scope(self):
        """The organization id the webhook URL scopes to
        (POST /webhooks/stripe/:organization_id, F9)."""
        return self.lago_db_query("select id from organizations order by created_at limit 1")

    def deliver_stripe_webhook(self, org_id, secret, event, provider_code=None):
        """Deliver a signed webhook event to the BUILT-IN receive route
        (transport-leg stand-in per D8: real PI object body + real-secret
        HMAC signature; the receiving/processing chain is 100% Lago).

        Returns (http_status, response_text, delivered_payload_sha).
        """
        payload = json.dumps(event, separators=(",", ":")).encode("utf-8")
        signature = fixtures.sign_stripe_event(secret, payload)
        url = f"{self.lago_url}/webhooks/stripe/{org_id}"
        if provider_code:
            from urllib.parse import quote
            url += f"?code={quote(provider_code, safe='')}"
        request = urllib.request.Request(
            url,
            data=payload,
            headers={
                "Content-Type": "application/json",
                "Stripe-Signature": signature,
            },
            method="POST",
        )
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        import hashlib
        payload_sha = hashlib.sha256(payload).hexdigest()
        try:
            with opener.open(request, timeout=self.lago._timeout) as response:
                return response.status, response.read().decode("utf-8", "replace"), payload_sha
        except urllib.error.HTTPError as error:
            return error.code, error.read().decode("utf-8", "replace"), payload_sha


def _report(ctx, phase, expected, observed, status, evidence=None):
    report = {
        "phase": phase,
        "run_id": ctx.run_id,
        "expected": expected,
        "observed": observed,
        "status": status,
        "evidence": evidence or {},
        "contract_notes": ctx.drain_notes(),
        "recorded_at": _utc_now(),
    }
    return clients.sanitize(report, extra_secrets=ctx.secret_values())


def _blocked(ctx, phase, reason, expected=""):
    return _report(ctx, phase, expected, {"blocked_reason": reason}, BLOCKED)


def phase_gated_3ds(ctx):
    """F7 re-proof: provider + customer A on the 3DS card + gated create ->
    incomplete with the imported default pm."""
    expected = (
        "gated create (activation_rules payment/timeout_hours=0) is immediately "
        "incomplete; the customer's default (3DS) payment method is imported to "
        "Lago before the create"
    )
    if not ctx.stripe_key:
        return _blocked(ctx, "gated_3ds",
                        "missing Stripe test key (set STRIPE_SECRET_KEY, sk_test_/rk_test_ only)",
                        expected)
    if not ctx.graphql_jwt:
        return _blocked(ctx, "gated_3ds",
                        "missing operator GraphQL JWT (loginUser)", expected)

    # 1. GraphQL provider registration (user JWT, not Premium-gated).
    try:
        gstatus, gbody = ctx.graphql(
            fixtures.ADD_STRIPE_PROVIDER_QUERY,
            fixtures.add_stripe_provider_variables(
                f"{ctx.prefix}-stripe", STRIPE_PROVIDER_NAME, ctx.stripe_key),
        )
    except (OSError, clients.LabError) as error:
        return _blocked(ctx, "gated_3ds",
                        f"Lago API unreachable ({error.__class__.__name__})", expected)
    provider = ((gbody or {}).get("data") or {}).get("addStripePaymentProvider") \
        if isinstance(gbody, dict) else None
    if not _ok(gstatus) or not provider:
        observed = {"blocked_reason": "AddStripePaymentProvider failed",
                    "graphql_status": gstatus,
                    "graphql_errors": (gbody or {}).get("errors") if isinstance(gbody, dict) else None}
        return _report(ctx, "gated_3ds", expected, observed, FAIL)
    ctx.state["provider_code"] = provider.get("code")

    try:
        # 2. Plan (the gated create needs a plan).
        ps, _pb = ctx.lago.post("/api/v1/plans", fixtures.plan_payload(
            ctx.plan_code, name="WeKnora T11 probe plan"))
        if not _ok(ps):
            return _report(ctx, "gated_3ds", expected,
                           {"error": f"plan create rejected (HTTP {ps})"}, FAIL)
        ctx.state["plan_code"] = ctx.plan_code

        # 3. Stripe customer A with the 3DS challenge card as default pm.
        # (A-11 / F51) The provider customer id is registered in ctx.state
        # THE MOMENT it exists -- every earlier failure path (attach raises
        # LabError, the Lago create below is rejected, a transport error in
        # between) still leaves phase_cleanup able to delete the cus_…
        # instead of leaking it (the cleanup contract: every lab object
        # created this run is deleted).
        ext = ctx.customer_external_id()
        stripe_customer = ctx.stripe.create_customer(
            description=f"{ext} (3DS challenge card)",
            metadata={"run": ctx.run_id, "lab": "t11"},
        )
        stripe_customer_id = stripe_customer["id"]
        ctx.state["stripe_customer_id"] = stripe_customer_id
        attached_pm = ctx.stripe.attach_payment_method(GATED_PM, stripe_customer_id)
        ctx.stripe.set_default_payment_method(stripe_customer_id, attached_pm["id"])

        # 4. Lago customer linked to the provider.
        cs, _cb = ctx.lago.post("/api/v1/customers", fixtures.customer_payload(
            ext, "WeKnora T11 customer A (3DS card)", stripe_customer_id,
            payment_provider_code=provider.get("code")))
        if not _ok(cs):
            return _report(ctx, "gated_3ds", expected,
                           {"error": f"Lago customer create rejected (HTTP {cs})"}, FAIL)
        ctx.state["customer"] = {
            "external_id": ext, "stripe_customer_id": stripe_customer_id}

        # 5. Bounded poll: the default pm is imported (the gated create
        #    refuses with no_default_payment_method until it lands, F11).
        def fetch_pms():
            return ctx.payment_methods_for(ext)

        ready, last, attempts = ctx.poll(
            fetch_pms, lambda r: _ok(r[0]) and len(r[1]) > 0,
            "default payment method import for customer A", ctx.pm_poll_timeout)
        if not ready:
            return _report(ctx, "gated_3ds", expected,
                           {"error": "no default payment method imported",
                            "last_status": last[0]}, FAIL)

        # 6. The gated create.
        sub_ext = ctx.subscription_external_id()
        ss, sb = ctx.lago.post("/api/v1/subscriptions", fixtures.gated_subscription_payload(
            ext, ctx.state["plan_code"], sub_ext))
        sub_status = (sb or {}).get("subscription", {}).get("status") \
            if isinstance(sb, dict) else None
        ctx.state["subscription_external_id"] = sub_ext
        if not _ok(ss) or sub_status != "incomplete":
            return _report(ctx, "gated_3ds", expected,
                           {"error": "gated create rejected or not incomplete",
                            "create_status": ss, "subscription_status": sub_status},
                           FAIL, evidence={"create_response": sb})
    except (OSError, clients.LabError) as error:
        return _blocked(ctx, "gated_3ds",
                        f"transport error ({error.__class__.__name__})", expected)

    return _report(ctx, "gated_3ds", expected, {
        "subscription_external_id": ctx.state["subscription_external_id"],
        "subscription_status": "incomplete",
        "payment_methods_imported": len(last[1]),
        "poll_attempts": attempts,
    }, PASS)


def phase_settle_probe(ctx):
    """P-A: locate the stuck gating PaymentIntent on the PROVIDER side.

    ``GET /v1/payment_intents?customer=`` must answer an unsettled intent
    (status ``requires_payment_method`` or ``requires_action`` -- t10's only
    observed shape was requires_payment_method, F5) whose metadata carries
    ``lago_invoice_id`` (the pinned CreateService stamps it on every provider
    payment). This is the ONLY supported locator for the invisible gating
    invoice (F4: the Lago payments index hides it).
    """
    expected = (
        "the stuck gating intent is locatable: GET /v1/payment_intents?customer= "
        "answers an unsettled intent (requires_payment_method|requires_action) "
        "whose metadata carries lago_invoice_id (F5 provider-side locator); the "
        "OBSERVED status word is recorded for Task 5's test alignment"
    )
    customer = ctx.state.get("customer")
    if not customer or not ctx.state.get("subscription_external_id"):
        return _blocked(ctx, "settle_probe",
                        "requires the gated_3ds phase to have created customer A "
                        "and the gated subscription", expected)
    try:
        stripe_customer_id = customer["stripe_customer_id"]

        def fetch_intents():
            status, body = ctx.stripe._form(
                "GET", f"/v1/payment_intents?customer={stripe_customer_id}&limit=20")
            rows = body.get("data", []) if isinstance(body, dict) else []
            return status, rows

        ready, last, attempts = ctx.poll(
            fetch_intents,
            lambda r: _ok(r[0]) and len(fixtures.unsettled_intents(r[1])) > 0
            and fixtures.intent_invoice_id(fixtures.unsettled_intents(r[1])[0]) is not None,
            "unsettled Stripe intent with lago_invoice_id metadata",
            ctx.pm_poll_timeout)
        status, rows = last if last else (None, [])
        unsettled = fixtures.unsettled_intents(rows)
        intent = unsettled[0] if unsettled else None
        invoice_id = fixtures.intent_invoice_id(intent)
        observed = {
            "stripe_intents": {
                "http_status": status,
                "total": len(rows),
                "unsettled": len(unsettled),
                "intent_status": (intent or {}).get("status"),
                "intent_created": (intent or {}).get("created"),
                "provider_payment_id": (intent or {}).get("id"),
                "lago_invoice_id": invoice_id,
                "poll_attempts": attempts,
            },
        }
        if not ready or intent is None or invoice_id is None:
            observed["error"] = ("no unsettled intent (requires_payment_method/"
                                 "requires_action) carrying lago_invoice_id metadata")
            return _report(ctx, "settle_probe", expected, observed, FAIL)
        ctx.state["payment"] = {
            "lago_id": None,
            "status": (intent or {}).get("status"),
            "provider_payment_id": (intent or {}).get("id"),
            "invoice_lago_id": invoice_id,
            "locator": "stripe_intent_metadata",
        }
    except (OSError, clients.LabError) as error:
        return _blocked(ctx, "settle_probe",
                        f"transport error ({error.__class__.__name__})", expected)
    return _report(ctx, "settle_probe", expected, observed, PASS)


def _read_four_objects(ctx):
    """The P-E authority snapshot: subscription / invoice / payments /
    entitlements, as comparable JSON-able rows."""
    customer = ctx.state["customer"]
    sub_ext = ctx.state["subscription_external_id"]
    ss, sb = ctx.lago.get(f"/api/v1/subscriptions/{sub_ext}?status=active")
    subscription = (sb or {}).get("subscription") if isinstance(sb, dict) else None
    istatus, irows = ctx.invoices_for(customer["external_id"])
    pstatus, prows = ctx.payments_for(customer["external_id"])
    es, _eb = ctx.lago.get(f"/api/v1/subscriptions/{sub_ext}/entitlements")
    return {
        "subscription": {
            "read_status": ss,
            "status": (subscription or {}).get("status"),
        },
        "invoice": fixtures.finalized_invoice(irows),
        "payments": {
            "http": pstatus,
            "succeeded": len(fixtures.succeeded_payments(prows)),
            "statuses": sorted(p.get("status") for p in (prows or [])
                               if isinstance(p, dict)),
        },
        "entitlements_http": es,
        "invoices_http": istatus,
    }


def _four_objects_fulfilled(snapshot):
    """The P-E predicate: active + finalized/succeeded invoice + EXACTLY ONE
    succeeded payment + entitlements 200."""
    invoice = snapshot.get("invoice") or {}
    return bool(
        snapshot.get("subscription", {}).get("status") == "active"
        and invoice.get("payment_status") == "succeeded"
        and invoice.get("number")
        and snapshot.get("payments", {}).get("succeeded") == 1
        and snapshot.get("entitlements_http") == 200
    )


def phase_settle_trigger(ctx):
    """P-B/C/D/E: the dual-track settle rail, end to end.

    (1) P-B: attach the settle pm + set it as the customer default.
    (2) P-C: POST /v1/payment_intents/{id} (payment_method=...) +
        POST /v1/payment_intents/{id}/confirm -> status=="succeeded" (THE
        unverified link of D2' step iv).
    (3) P-D: deliver a REAL payment_intent.succeeded event (body = the PI
        object read back from Stripe; signature = the provider's real
        webhook_secret) to POST /webhooks/stripe/{org_id} -> HTTP 200 and
        the inbound_webhooks row lands.
    (4) P-E: within the poll window the authority finalizes -- subscription
        active, invoice finalized/numbered/payment_status succeeded, exactly
        one succeeded payment, entitlements 200.
    (5) P-E no-op: re-delivering the SAME event leaves the four objects
        byte-identical (F2 duplicate delivery defense).
    """
    expected = (
        "(P-B) settle pm attached+default; (P-C) PI update payment_method + "
        "confirm answers status==succeeded; (P-D) POST /webhooks/stripe/{org_id} "
        "with a real-secret signature answers 200 and the inbound_webhooks row "
        "lands; (P-E) subscription active + invoice finalized/numbered/"
        "payment_status=succeeded + EXACTLY ONE succeeded payment + "
        "entitlements 200 within the poll window; re-delivery of the same "
        "event is a no-op (four objects byte-identical)"
    )
    customer = ctx.state.get("customer")
    payment = ctx.state.get("payment")
    if not customer or not payment:
        return _blocked(ctx, "settle_trigger",
                        "requires gated_3ds + settle_probe to have run", expected)
    observed = {}
    try:
        # (P-B) attach the settle pm and make it the default.
        stripe_customer_id = customer["stripe_customer_id"]
        attached = ctx.stripe.attach_payment_method(ctx.settle_pm, stripe_customer_id)
        ctx.stripe.set_default_payment_method(stripe_customer_id, attached["id"])
        observed["settle_pm_attached"] = attached.get("id") is not None

        # (P-C) the settle rail: update the stuck intent's payment method,
        # then confirm it off-session (synchronous charge).
        pi_id = payment["provider_payment_id"]
        ustatus, ubody = ctx.stripe._form(
            "POST", f"/v1/payment_intents/{pi_id}",
            {"payment_method": attached["id"]})
        observed["pi_update_status"] = ustatus
        if not _ok(ustatus):
            observed["error"] = f"PI payment_method update rejected (HTTP {ustatus})"
            return _report(ctx, "settle_trigger", expected, observed, FAIL,
                           evidence={"pi_update_response": ubody})
        cstatus, cbody = ctx.stripe._form(
            "POST", f"/v1/payment_intents/{pi_id}/confirm")
        observed["pi_confirm_status"] = cstatus
        confirmed_status = (cbody or {}).get("status") if isinstance(cbody, dict) else None
        observed["pi_status_after_confirm"] = confirmed_status
        if not _ok(cstatus) or confirmed_status != "succeeded":
            observed["error"] = (f"PI confirm did not reach succeeded "
                                 f"(HTTP {cstatus}, status={confirmed_status!r})")
            return _report(ctx, "settle_trigger", expected, observed, FAIL,
                           evidence={"pi_confirm_response": cbody})

        # (P-D) read the REAL PI object back and deliver the REAL event body
        # through the built-in webhook route with a real-secret signature.
        rstatus, rbody = ctx.stripe._form("GET", f"/v1/payment_intents/{pi_id}")
        if not _ok(rstatus) or not isinstance(rbody, dict):
            observed["error"] = f"PI read-back failed (HTTP {rstatus})"
            return _report(ctx, "settle_trigger", expected, observed, FAIL)
        provider_code = ctx.state.get("provider_code")
        secret = ctx.provider_webhook_secret(provider_code)
        org_id = ctx.organization_webhook_scope()
        if not secret or not org_id:
            return _blocked(ctx, "settle_trigger",
                            "webhook_secret/organization id not readable from the "
                            "lab DB (docker compose exec db psql unavailable)",
                            expected)
        event = fixtures.build_pi_succeeded_event(rbody)
        inbound_before = ctx.lago_db_query("select count(*) from inbound_webhooks")

        # The built-in route scopes the provider lookup by the ?code= query
        # param (WebhooksController#stripe -> FindService) — the same shape
        # RegisterWebhookService bakes into the endpoint URL.
        wstatus, wtext, payload_sha = ctx.deliver_stripe_webhook(
            org_id, secret, event, provider_code=provider_code)
        observed["webhook_status"] = wstatus
        observed["webhook_event_id"] = event["id"]
        observed["webhook_payload_sha256"] = payload_sha
        if wstatus != 200:
            observed["error"] = f"webhook delivery rejected (HTTP {wstatus})"
            return _report(ctx, "settle_trigger", expected, observed, FAIL,
                           evidence={"webhook_response": wtext[:500]})

        # inbound_webhooks row must have landed (P-D receive half).
        inbound_after = ctx.lago_db_query("select count(*) from inbound_webhooks")
        observed["inbound_webhooks_count"] = {
            "before": inbound_before, "after": inbound_after}
        if inbound_after is None or (inbound_before is not None
                                     and int(inbound_after) < int(inbound_before) + 1):
            observed["error"] = "inbound_webhooks row did not land"
            return _report(ctx, "settle_trigger", expected, observed, FAIL)

        # (P-E) bounded poll: the built-in chain finalizes the authority.
        done, last, attempts = ctx.poll(
            lambda: _read_four_objects(ctx), _four_objects_fulfilled,
            "webhook-driven activation (active + finalized + 1 payment)",
            ctx.trigger_poll_timeout)
        observed["activation_observed"] = bool(done)
        observed["poll_attempts"] = attempts
        if not done:
            observed["error"] = "webhook chain did not finalize within the window"
            observed["last_snapshot"] = last
            return _report(ctx, "settle_trigger", expected, observed, FAIL)
        snapshot_one = last
        observed["four_objects_after_settle"] = snapshot_one

        # (P-E no-op) re-deliver the SAME signed event: byte-identical state.
        wstatus2, _wtext2, _sha2 = ctx.deliver_stripe_webhook(
            org_id, secret, event, provider_code=provider_code)
        observed["webhook_redelivery_status"] = wstatus2
        time.sleep(max(ctx.poll_interval * 2, 4.0))  # let any async work land
        snapshot_two = _read_four_objects(ctx)
        observed["four_objects_after_redelivery"] = snapshot_two
        observed["redelivery_noop"] = json.dumps(snapshot_one, sort_keys=True) == \
            json.dumps(snapshot_two, sort_keys=True)
        payments_succeeded = snapshot_two.get("payments", {}).get("succeeded")
        if not observed["redelivery_noop"] or payments_succeeded != 1:
            observed["error"] = "duplicate webhook delivery changed the four objects"
            return _report(ctx, "settle_trigger", expected, observed, FAIL)
        ctx.state["activated"] = True
        ctx.state["invoice"] = snapshot_two.get("invoice")
    except (OSError, clients.LabError) as error:
        return _blocked(ctx, "settle_trigger",
                        f"transport error ({error.__class__.__name__})", expected)
    return _report(ctx, "settle_trigger", expected, observed, PASS)


def phase_cleanup(ctx):
    """Always executed last: delete everything this run created."""
    expected = "every lab object created this run is deleted; failures reported distinctly"
    objects = []
    state = ctx.state

    def record(object_name, identifier, outcome, http_status=None):
        objects.append({
            "object": object_name, "id": identifier,
            "outcome": outcome, "http_status": http_status,
        })

    sub_ext = state.get("subscription_external_id")
    if sub_ext:
        deleted = False
        last_status = None
        for sub_status in ("active", "incomplete", "canceled"):
            try:
                status, _body = ctx.lago.delete(
                    f"/api/v1/subscriptions/{sub_ext}?status={sub_status}")
            except OSError:
                status = None
            last_status = status
            if _ok(status):
                record("subscription", sub_ext, "deleted", status)
                deleted = True
                break
        if not deleted:
            record("subscription", sub_ext, "failed", last_status)

    # (A-11 / F51) The stripe customer id is read with a FALLBACK: an early
    # failure between the provider create and the Lago create left only
    # state["stripe_customer_id"] (no "customer" mapping) -- the cus_… must
    # still be deleted, never leaked.
    customer = state.get("customer")
    stripe_customer_id = state.get("stripe_customer_id") or \
        (customer or {}).get("stripe_customer_id")
    if customer:
        try:
            status, _body = ctx.lago.delete(
                f"/api/v1/customers/{customer['external_id']}")
            record("customer", customer["external_id"],
                   "deleted" if _ok(status) else "failed", status)
        except OSError:
            record("customer", customer["external_id"], "failed", None)
    if stripe_customer_id and ctx.stripe:
        deleted = ctx.stripe.delete_customer(stripe_customer_id)
        record("stripe_customer", stripe_customer_id,
               "deleted" if deleted else "failed", 200 if deleted else None)

    if state.get("plan_code"):
        try:
            status, _body = ctx.lago.delete(f"/api/v1/plans/{state['plan_code']}")
            record("plan", state["plan_code"], "deleted" if _ok(status) else "failed", status)
        except OSError:
            record("plan", state["plan_code"], "failed", None)

    endpoint_id = state.get("stripe_webhook_endpoint_id")
    if endpoint_id and ctx.stripe:
        try:
            status, _body = ctx.stripe._form(
                "DELETE", f"/v1/webhook_endpoints/{endpoint_id}")
            record("stripe_webhook_endpoint", endpoint_id,
                   "deleted" if _ok(status) else "failed", status)
        except OSError:
            record("stripe_webhook_endpoint", endpoint_id, "failed", None)

    # (OCR r2) The lab's OWN payment provider (created through GraphQL in
    # phase_gated_3ds) is part of "every lab object created this run": the
    # pinned v1.53 GraphQL surface exposes no provider destroy mutation we
    # could drive from here, so the residue is RECORDED explicitly (never
    # silently leaked) — repeated runs accumulate providers under their
    # unique run prefixes, and each carries a webhook secret; operators
    # prune them with the recorded code list.
    provider_code = state.get("provider_code")
    if provider_code:
        record("payment_provider", provider_code, "residue-recorded", None)

    failures = [item for item in objects
                if item["outcome"] not in ("deleted", "residue-recorded")]
    observed = {
        "objects": objects,
        "cleanup_failures": [f"{item['object']}({item['id']})" for item in failures],
    }
    return _report(ctx, "cleanup", expected, observed,
                   FAIL if failures else PASS)


# Execution-order contract: gated_3ds (setup) -> settle_probe (P-A) ->
# settle_trigger (P-B/C/D/E) -> cleanup. The runner's PHASE_SEQUENCE is
# DERIVED from this tuple (single source of truth); the alignment is
# guarded by test_phase_order_contract_matches_runner_order in
# test_phases.py (the payment-activation lab's discipline).
PHASE_ORDER = (
    phase_gated_3ds,
    phase_settle_probe,
    phase_settle_trigger,
)
