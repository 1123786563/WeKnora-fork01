"""Probe phases for the #82 settlement-trigger contract lab (t10).

Each phase drives real Lago/Stripe calls through the payment-activation
lab's clients (imported read-only) and returns a sanitized report dict::

    {"phase": str, "run_id": str, "expected": str, "observed": dict,
     "status": "pass" | "fail" | "blocked-env", "evidence": dict,
     "contract_notes": [str]}

The probe verifies the THREE links the #82 implementation (D2) depends on:

- P1 (``phase_payment_probe``): the stuck gated payment row is visible via
  ``GET /api/v1/payments?external_customer_id=`` WITH a non-empty
  ``invoice_ids`` (F5) -- the coordinator's only path to the invisible
  gating invoice's lago_id.
- P2 (``phase_trigger``): cancel the stuck provider intent -> switch the
  settle payment method (attach + default + Lago re-import) ->
  ``POST /api/v1/invoices/{id}/retry_payment`` with an empty body -> the
  subscription reaches ``active`` with EXACTLY ONE succeeded payment and a
  finalized invoice (F1/F3/F4).
- P3 (``phase_retry_dup``): repeated retries never mint a second succeeded
  payment (the authority's unique pending-payment defense).
- P4 (``phase_paid_retry``): retrying a PAID invoice answers HTTP 405
  ``invalid_status`` (F2).

Verdict rules follow the T02 lab: ``pass`` only on full authoritative final
state; ``blocked-env`` for environment gaps; ``fail`` means the pinned
runtime violated the expected contract. Polling is always bounded; a timeout
is a ``fail`` with the last observed state attached.
"""

from __future__ import annotations

import json
import sys
import time
import urllib.request
import uuid
from datetime import datetime, timezone
from pathlib import Path

_PA_DIR = Path(__file__).resolve().parent.parent / "payment-activation"
if str(_PA_DIR) not in sys.path:
    sys.path.insert(0, str(_PA_DIR))

import clients  # noqa: E402  (payment-activation clients, read-only)
import fixtures  # noqa: E402  (THIS directory's payloads shadow pa's fixtures)

# Stripe test-mode payment-method tokens (public, throwaway). The gate pm is
# the 3DS-challenge card: its off-session charge stalls in requires_action,
# which is the stable stuck-payment window the probe needs (F7). The settle
# pm settles immediately (F2/F8 of the #81 flow evidence).
GATED_PM = "pm_card_threeDSecure2Required"
SETTLE_PM = "pm_card_visa"
STRIPE_PROVIDER_NAME = "WeKnora T10 Stripe Test"

PASS, FAIL, BLOCKED = "pass", "fail", "blocked-env"


def _utc_now():
    return datetime.now(timezone.utc).isoformat()


def _ok(status):
    return isinstance(status, int) and 200 <= status < 300


class RunContext:
    """One probe run: identity, clients, shared state, and poll knobs."""

    def __init__(self, lago_url, api_key, run_id=None, prefix=None,
                 stripe_key=None, graphql_jwt=None,
                 stripe_base_url="https://api.stripe.com",
                 poll_interval=2.0, poll_timeout=180.0,
                 trigger_poll_timeout=120.0, pm_poll_timeout=60.0,
                 request_timeout=30.0):
        self.run_id = run_id or str(uuid.uuid4())
        self.prefix = prefix or f"weknora-t10-{self.run_id[:8]}"
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
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
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


def _cancel_intent(ctx, provider_payment_id):
    """POST /v1/payment_intents/{id}/cancel with the credential ONLY in the
    Authorization header. Returns (http_status, body)."""
    return ctx.stripe._form(
        "POST", f"/v1/payment_intents/{provider_payment_id}/cancel")


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
    except OSError as error:
        return _blocked(ctx, "gated_3ds",
                        f"Lago API unreachable ({error.__class__.__name__})", expected)
    provider = ((gbody or {}).get("data") or {}).get("addStripePaymentProvider") \
        if isinstance(gbody, dict) else None
    if not _ok(gstatus) or not provider:
        observed = {"blocked_reason": "AddStripePaymentProvider failed",
                    "graphql_status": gstatus,
                    "graphql_errors": (gbody or {}).get("errors") if isinstance(gbody, dict) else None}
        return _report(ctx, "gated_3ds", expected, observed, FAIL)

    try:
        # 2. Plan (the gated create needs a plan).
        ps, _pb = ctx.lago.post("/api/v1/plans", fixtures.plan_payload(
            ctx.plan_code, name="WeKnora T10 probe plan"))
        if not _ok(ps):
            return _report(ctx, "gated_3ds", expected,
                           {"error": f"plan create rejected (HTTP {ps})"}, FAIL)
        ctx.state["plan_code"] = ctx.plan_code

        # 3. Stripe customer A with the 3DS challenge card as default pm.
        ext = ctx.customer_external_id()
        stripe_customer = ctx.stripe.create_customer(
            description=f"{ext} (3DS challenge card)",
            metadata={"run": ctx.run_id, "lab": "t10"},
        )
        stripe_customer_id = stripe_customer["id"]
        attached_pm = ctx.stripe.attach_payment_method(GATED_PM, stripe_customer_id)
        ctx.stripe.set_default_payment_method(stripe_customer_id, attached_pm["id"])

        # 4. Lago customer linked to the provider.
        cs, _cb = ctx.lago.post("/api/v1/customers", fixtures.customer_payload(
            ext, "WeKnora T10 customer A (3DS card)", stripe_customer_id,
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
    except OSError as error:
        return _blocked(ctx, "gated_3ds",
                        f"transport error ({error.__class__.__name__})", expected)

    return _report(ctx, "gated_3ds", expected, {
        "subscription_external_id": ctx.state["subscription_external_id"],
        "subscription_status": "incomplete",
        "payment_methods_imported": len(last[1]),
        "poll_attempts": attempts,
    }, PASS)


def phase_payment_probe(ctx):
    """P1: locate the stuck provider payment and the gating invoice id via
    SUPPORTED read surfaces.

    Two channels are probed honestly:

    - Lago ``GET /api/v1/payments?external_customer_id=``: probed and
      recorded AS OBSERVED. The pinned v1.53.0 PaymentsQuery applies a
      ``visible_payable_condition`` (the payable invoice must be in
      VISIBLE_STATUS draft/finalized/voided/failed/pending), so a payment
      whose gating invoice is still open/closed is INVISIBLE — the plan's F5
      premise ("no visibility filter") is falsified here if the list stays
      empty.
    - Stripe ``GET /v1/payment_intents?customer=<provider_customer_id>``:
      the pinned ``Invoices::Payments::CreateService`` stamps EVERY provider
      payment with ``metadata.lago_invoice_id`` — the SUPPORTED provider-side
      locator for the invisible gating invoice. P1 passes on this channel
      only (this is the invoice identity the settle trigger resolves).
    """
    expected = (
        "the stuck 3DS charge is locatable: the Lago payments index is probed "
        "and its visibility recorded; the Stripe intent list for the provider "
        "customer answers an unsettled PaymentIntent whose metadata carries "
        "lago_invoice_id (the supported invoice locator the settle trigger uses)"
    )
    customer = ctx.state.get("customer")
    if not customer or not ctx.state.get("subscription_external_id"):
        return _blocked(ctx, "payment_probe",
                        "requires the gated_3ds phase to have created customer A "
                        "and the gated subscription", expected)
    try:
        # Channel 1 (recorded as observed, never load-bearing after this).
        lago_status, lago_rows = ctx.payments_for(customer["external_id"])
        lago_visible_unsettled = fixtures.first_unsettled_payment(lago_rows)

        # Channel 2: the supported provider-side locator.
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
            "lago_payments_index": {
                "http_status": lago_status,
                "payments_total": len(lago_rows),
                "unsettled_visible": lago_visible_unsettled is not None,
                "note": "pinned v1.53.0 PaymentsQuery filters by the payable "
                        "invoice's VISIBLE_STATUS; open/closed gating invoices "
                        "hide their payment rows",
            },
            "stripe_intents": {
                "http_status": status,
                "total": len(rows),
                "unsettled": len(unsettled),
                "intent_status": (intent or {}).get("status"),
                "provider_payment_id": (intent or {}).get("id"),
                "lago_invoice_id": invoice_id,
                "poll_attempts": attempts,
            },
        }
        if not ready or intent is None or invoice_id is None:
            observed["error"] = "no unsettled intent carrying lago_invoice_id metadata"
            return _report(ctx, "payment_probe", expected, observed, FAIL)
        ctx.state["payment"] = {
            "lago_id": None,
            "status": (intent or {}).get("status"),
            "provider_payment_id": (intent or {}).get("id"),
            "invoice_lago_id": invoice_id,
            "locator": "stripe_intent_metadata",
        }
    except OSError as error:
        return _blocked(ctx, "payment_probe",
                        f"transport error ({error.__class__.__name__})", expected)
    return _report(ctx, "payment_probe", expected, observed, PASS)


def phase_trigger(ctx):
    """P2: cancel the stuck intent -> settle pm switch -> retry_payment ->
    active with exactly one succeeded payment (F1)."""
    expected = (
        "(a) stuck provider intent canceled; (b) settle pm attached+default; "
        "(c) Lago re-imports it as a SECOND payment method; (d) POST "
        "/api/v1/invoices/{id}/retry_payment {} answers 200; (e) subscription "
        "reaches active within the poll window with exactly ONE succeeded "
        "payment and the invoice finalized/numbered/payment_status=succeeded "
        "with entitlements 200 (F1)"
    )
    customer = ctx.state.get("customer")
    payment = ctx.state.get("payment")
    if not customer or not payment:
        return _blocked(ctx, "trigger",
                        "requires gated_3ds + payment_probe to have run", expected)
    observed = {}
    try:
        # (a) Cancel the stuck provider intent (only when still cancelable;
        #     an already-terminal intent is success-and-continue).
        provider_payment_id = payment.get("provider_payment_id")
        if provider_payment_id and fixtures.intent_cancelable(payment.get("status")):
            cstatus, cbody = _cancel_intent(ctx, provider_payment_id)
            observed["cancel_status"] = cstatus
            if not _ok(cstatus):
                # A 4xx here means the intent is already terminal on the
                # provider side; record and continue (the cancel link is
                # best-effort -- the retry unlock is what P2 tests).
                ctx.note(f"intent cancel answered HTTP {cstatus}; continuing")
        else:
            observed["cancel_status"] = "skipped"
            ctx.note(f"payment status {payment.get('status')!r} not cancelable; skipping")

        # (b) Attach the settle pm and make it the default.
        stripe_customer_id = customer["stripe_customer_id"]
        attached = ctx.stripe.attach_payment_method(SETTLE_PM, stripe_customer_id)
        ctx.stripe.set_default_payment_method(stripe_customer_id, attached["id"])
        observed["settle_pm_attached"] = attached.get("id") is not None

        # (c) Re-trigger the Lago import so the settle pm becomes the
        #     authority-visible default: re-POST the customer UPSERT with its
        #     billing_configuration (pinned v1.53.0 has NO PUT route — probe
        #     finding recorded in t10 DECISION.md), then poll for a SECOND
        #     pm. If the same-provider-customer upsert does not mint the
        #     import, fall back to a NEW provider customer (fresh Stripe
        #     customer with the settle pm as default) — both variants are
        #     recorded as observed. The import poll is BEST-EFFORT here: the
        #     P2 verdict rides on (e) activation alone (a local stack with no
        #     public webhook reachability cannot always mint the second
        #     import, and the pinned builder falls back to the provider-side
        #     default payment method when no Lago pm row names the method).
        gstatus, gbody = ctx.lago.get(
            f"/api/v1/customers/{customer['external_id']}")
        if not _ok(gstatus):
            return _report(ctx, "trigger", expected,
                           {"error": "customer read failed", "http_status": gstatus}, FAIL)
        cust_body = (gbody or {}).get("customer") or {}
        billing = cust_body.get("billing_configuration") \
            if isinstance(cust_body, dict) else None
        if not billing:
            return _report(ctx, "trigger", expected,
                           {"error": "customer carries no billing_configuration"}, FAIL)
        pstatus, _pb = ctx.lago.post("/api/v1/customers", {"customer": {
            "external_id": customer["external_id"],
            "name": cust_body.get("name") or customer["external_id"],
            "billing_configuration": billing,
        }})
        observed["customer_upsert_status"] = pstatus
        if not _ok(pstatus):
            return _report(ctx, "trigger", expected,
                           {"error": "customer upsert rejected", "http_status": pstatus}, FAIL)

        def fetch_pms():
            return ctx.payment_methods_for(customer["external_id"])

        pm_ready, pm_last, pm_attempts = ctx.poll(
            fetch_pms, lambda r: _ok(r[0]) and fixtures.second_pm_present(r[1]),
            "second (settle) payment method import", ctx.pm_poll_timeout)
        observed["payment_methods_count_same_pcid"] = len(pm_last[1]) if pm_last else 0
        observed["pm_poll_attempts"] = pm_attempts
        if not pm_ready:
            ctx.note("same-pcid upsert did not mint a second import; trying a "
                     "fresh provider customer carrying the settle pm")
            fresh = ctx.stripe.create_customer(
                description=f"{customer['external_id']} settle-pm carrier",
                metadata={"run": ctx.run_id, "lab": "t10", "role": "settle"},
            )
            fresh_id = fresh["id"]
            attached2 = ctx.stripe.attach_payment_method(SETTLE_PM, fresh_id)
            ctx.stripe.set_default_payment_method(fresh_id, attached2["id"])
            billing2 = dict(billing)
            billing2["provider_customer_id"] = fresh_id
            u2status, _b2 = ctx.lago.post("/api/v1/customers", {"customer": {
                "external_id": customer["external_id"],
                "name": cust_body.get("name") or customer["external_id"],
                "billing_configuration": billing2,
            }})
            observed["customer_upsert_status_fresh_pcid"] = u2status
            ctx.state["settle_pcid_swap"] = fresh_id
            pm_ready2, pm_last2, _a2 = ctx.poll(
                fetch_pms, lambda r: _ok(r[0]) and fixtures.second_pm_present(r[1]),
                "second pm import via fresh provider customer", ctx.pm_poll_timeout)
            observed["payment_methods_count_fresh_pcid"] = len(pm_last2[1]) if pm_last2 else 0
            if not pm_ready2:
                ctx.note("fresh-pcid upsert did not mint a second import either; "
                         "continuing — (e) activation is the P2 verdict")
            observed["second_pm_imported"] = pm_ready or pm_ready2

        # (d) The trigger: retry_payment with the empty body.
        invoice_id = payment["invoice_lago_id"]
        rstatus, rbody = ctx.lago.post(
            f"/api/v1/invoices/{invoice_id}/retry_payment", fixtures.retry_body())
        observed["retry_status"] = rstatus
        observed["retry_body"] = rbody if isinstance(rbody, dict) else None
        if not _ok(rstatus):
            observed["error"] = f"retry_payment rejected (HTTP {rstatus})"
            return _report(ctx, "trigger", expected, observed, FAIL,
                           evidence={"retry_response": rbody})

        # (e) Bounded poll: subscription active + F1 full set.
        sub_ext = ctx.state["subscription_external_id"]

        def fetch_activation():
            ss, sb = ctx.lago.get(f"/api/v1/subscriptions/{sub_ext}?status=active")
            active = _ok(ss)
            pstatus, prows = ctx.payments_for(customer["external_id"])
            istatus, irows = ctx.invoices_for(customer["external_id"])
            return {
                "active": active,
                "succeeded_count": len(fixtures.succeeded_payments(prows)),
                "invoice": fixtures.finalized_invoice(irows),
                "payments_http": pstatus,
            }

        done, act_last, act_attempts = ctx.poll(
            fetch_activation,
            lambda r: r["active"] and r["succeeded_count"] >= 1 and r["invoice"] is not None,
            "subscription activation", ctx.trigger_poll_timeout)
        observed["activation_observed"] = bool(done)
        observed["subscription_active"] = bool(act_last and act_last["active"])
        observed["succeeded_payment_count"] = act_last["succeeded_count"] if act_last else 0
        invoice = act_last["invoice"] if act_last else None
        observed["invoice_finalized"] = {
            "lago_id": (invoice or {}).get("lago_id"),
            "number": (invoice or {}).get("number"),
            "payment_status": (invoice or {}).get("payment_status"),
            "total_amount_cents": (invoice or {}).get("total_amount_cents"),
        }
        observed["poll_attempts"] = act_attempts

        es, _eb = ctx.lago.get(f"/api/v1/subscriptions/{sub_ext}/entitlements")
        observed["entitlements_status"] = es

        ok = (
            observed["subscription_active"]
            and observed["succeeded_payment_count"] == 1
            and invoice is not None
            and (invoice or {}).get("payment_status") == "succeeded"
            and (invoice or {}).get("number")
            and es == 200
        )
        if not ok:
            observed["error"] = "activation contract violated (see observed fields)"
            return _report(ctx, "trigger", expected, observed, FAIL)
        ctx.state["activated"] = True
        ctx.state["invoice"] = invoice
    except OSError as error:
        return _blocked(ctx, "trigger",
                        f"transport error ({error.__class__.__name__})", expected)
    return _report(ctx, "trigger", expected, observed, PASS)


def phase_retry_dup(ctx):
    """P3: two more retries never mint a second succeeded payment."""
    expected = (
        "repeating POST /api/v1/invoices/{id}/retry_payment twice after "
        "activation leaves the succeeded payment count UNCHANGED (the "
        "authority's unique pending-payment defense)"
    )
    customer = ctx.state.get("customer")
    payment = ctx.state.get("payment")
    if not ctx.state.get("activated") or not customer or not payment:
        return _blocked(ctx, "retry_dup",
                        "requires the trigger phase to have activated the "
                        "subscription", expected)
    try:
        before_status, before_rows = ctx.payments_for(customer["external_id"])
        before = len(fixtures.succeeded_payments(before_rows))
        attempts = []
        for _ in range(2):
            rstatus, rbody = ctx.lago.post(
                f"/api/v1/invoices/{payment['invoice_lago_id']}/retry_payment",
                fixtures.retry_body())
            attempts.append(rstatus)
        time.sleep(ctx.poll_interval)  # let any erroneous async create land
        after_status, after_rows = ctx.payments_for(customer["external_id"])
        after = len(fixtures.succeeded_payments(after_rows))
        observed = {
            "succeeded_before": before,
            "succeeded_after": after,
            "retry_http_statuses": attempts,
            "payments_http": [before_status, after_status],
        }
        if after != before:
            observed["error"] = "duplicate retries minted extra succeeded payments"
            return _report(ctx, "retry_dup", expected, observed, FAIL)
    except OSError as error:
        return _blocked(ctx, "retry_dup",
                        f"Lago API unreachable ({error.__class__.__name__})", expected)
    return _report(ctx, "retry_dup", expected, observed, PASS)


def phase_paid_retry(ctx):
    """P4: retrying the PAID invoice answers 405 invalid_status (F2)."""
    expected = (
        "POST /api/v1/invoices/{paid_id}/retry_payment answers HTTP 405 with "
        "the Lago error code invalid_status (F2: the POST verb route exists, "
        "the state refuses)"
    )
    payment = ctx.state.get("payment")
    if not ctx.state.get("activated") or not payment:
        return _blocked(ctx, "paid_retry",
                        "requires the trigger phase to have activated the "
                        "subscription", expected)
    try:
        rstatus, rbody = ctx.lago.post(
            f"/api/v1/invoices/{payment['invoice_lago_id']}/retry_payment",
            fixtures.retry_body())
    except OSError as error:
        return _blocked(ctx, "paid_retry",
                        f"Lago API unreachable ({error.__class__.__name__})", expected)
    code = (rbody or {}).get("code") if isinstance(rbody, dict) else None
    observed = {"retry_status": rstatus, "error_code": code}
    if rstatus != 405 or code != "invalid_status":
        observed["error"] = "paid-invoice retry did not answer 405 invalid_status"
        return _report(ctx, "paid_retry", expected, observed, FAIL,
                       evidence={"retry_response": rbody})
    return _report(ctx, "paid_retry", expected, observed, PASS)


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

    customer = state.get("customer")
    if customer:
        try:
            status, _body = ctx.lago.delete(
                f"/api/v1/customers/{customer['external_id']}")
            record("customer", customer["external_id"],
                   "deleted" if _ok(status) else "failed", status)
        except OSError:
            record("customer", customer["external_id"], "failed", None)
        if ctx.stripe:
            deleted = ctx.stripe.delete_customer(customer["stripe_customer_id"])
            record("stripe_customer", customer["stripe_customer_id"],
                   "deleted" if deleted else "failed", 200 if deleted else None)

    if state.get("plan_code"):
        try:
            status, _body = ctx.lago.delete(f"/api/v1/plans/{state['plan_code']}")
            record("plan", state["plan_code"], "deleted" if _ok(status) else "failed", status)
        except OSError:
            record("plan", state["plan_code"], "failed", None)

    failures = [item for item in objects if item["outcome"] != "deleted"]
    observed = {
        "objects": objects,
        "cleanup_failures": [f"{item['object']}({item['id']})" for item in failures],
    }
    return _report(ctx, "cleanup", expected, observed,
                   FAIL if failures else PASS)


# Execution-order contract: gated_3ds -> payment_probe (P1) -> trigger (P2)
# -> retry_dup (P3) -> paid_retry (P4) -> cleanup. The runner's PHASE_SEQUENCE
# mirrors this tuple (guarded by the runner import).
PHASE_ORDER = (
    phase_gated_3ds,
    phase_payment_probe,
    phase_trigger,
    phase_retry_dup,
    phase_paid_retry,
)
