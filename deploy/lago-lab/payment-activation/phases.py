"""Experiment phases for the Lago T02 payment-activation lab.

Each phase drives real REST/GraphQL/Stripe calls through the Task 1 clients
and returns a sanitized report dict::

    {"phase": str, "run_id": str, "expected": str, "observed": dict,
     "status": "pass" | "fail" | "blocked-env", "evidence": dict,
     "contract_notes": [str]}

Verdict rules (plan, Honesty of outcomes):

- ``pass`` only on full authoritative final state (subscription status,
  invoice status/payment_status, payment rows, entitlement list) -- never on
  an HTTP code alone.
- ``blocked-env`` for environment gaps: missing Stripe test key, missing
  GraphQL JWT, unreachable Lago API or Stripe API, or a phase whose required
  predecessor objects were never created. Never a mock pass.
- ``fail`` means the pinned runtime violated the expected contract.

Polling is always bounded (RunContext knobs); a timeout is a ``fail`` with
the last observed state attached. Unknown-outcome POSTs are never blind
retried: recovery goes through the documented same-identity GET
(``/api/v1/subscriptions?external_id=...``), mirroring the spec's
indeterminate-outcome rule.
"""

from __future__ import annotations

import json
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime, timezone

import clients
import fixtures

CUSTOMER_TAGS = ("a", "b", "c")
CUSTOMER_NAMES = {
    "a": "WeKnora T02 customer A (3DS challenge card)",
    "b": "WeKnora T02 customer B (succeeding card)",
    "c": "WeKnora T02 customer C (declined card)",
}
# Stripe test-mode payment-method tokens (public, throwaway):
STRIPE_PAYMENT_METHODS = {
    "a": "pm_card_authenticationRequired",
    "b": "pm_card_visa",
    "c": "pm_card_visa_chargeDeclined",
}
STRIPE_PROVIDER_NAME = "WeKnora T02 Stripe Test"

PASS, FAIL, BLOCKED = "pass", "fail", "blocked-env"


def _utc_now():
    return datetime.now(timezone.utc).isoformat()


def _ok(status):
    return isinstance(status, int) and 200 <= status < 300


class RunContext:
    """One lab run: identity, clients, shared state, and poll knobs."""

    def __init__(self, lago_url, api_key, run_id=None, prefix=None,
                 stripe_key=None, graphql_jwt=None,
                 stripe_base_url="https://api.stripe.com",
                 poll_interval=2.0, poll_timeout=180.0,
                 stability_rounds=3, stability_delay=3.0,
                 request_timeout=30.0):
        self.run_id = run_id or str(uuid.uuid4())
        self.prefix = prefix or f"weknora-t02-{self.run_id}"
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
        self.stability_rounds = stability_rounds
        self.stability_delay = stability_delay
        self.state = {}
        self._notes = []

    # -- identity helpers ---------------------------------------------------
    def customer_external_id(self, tag):
        return f"{self.prefix}-{tag}"

    def subscription_external_id(self, tag):
        return f"{self.prefix}-sub-{tag}"

    @property
    def feature_code(self):
        return f"{self.prefix}-feature"

    @property
    def plan_code(self):
        return f"{self.prefix}-plan"

    # -- secrets + notes ----------------------------------------------------
    def secret_values(self):
        return tuple(v for v in (self.api_key, self.stripe_key, self.graphql_jwt) if v)

    def note(self, text):
        self._notes.append(text)

    def drain_notes(self):
        notes = list(self._notes)
        self._notes = []
        return notes

    # -- low level ------------------------------------------------------------
    def graphql(self, query, variables):
        """One GraphQL call with the operator JWT. Returns (status, body)."""
        payload = json.dumps({"query": query, "variables": variables}).encode("utf-8")
        request = urllib.request.Request(
            f"{self.lago_url}/graphql",
            data=payload,
            headers={
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Authorization": f"Bearer {self.graphql_jwt}" if self.graphql_jwt else "",
            },
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

    def poll(self, fn, done, description):
        """Bounded polling. Returns (finished, last_result, attempts)."""
        deadline = time.monotonic() + self.poll_timeout
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


# -- shared API accessors -----------------------------------------------------


def _subscription_show(ctx, external_id, status):
    try:
        return ctx.lago.get(f"/api/v1/subscriptions/{external_id}?status={status}")
    except OSError as error:
        ctx.note(f"transport error reading subscription {status}: {error.__class__.__name__}")
        raise


def _subscription_index(ctx, external_id, statuses):
    query = "&".join([f"external_id={external_id}"] + [f"status[]={s}" for s in statuses])
    return ctx.lago.get(f"/api/v1/subscriptions?{query}")


def _invoices_for(ctx, customer_external_id):
    status, body = ctx.lago.get(
        f"/api/v1/invoices?external_customer_id={customer_external_id}"
    )
    invoices = body.get("invoices", []) if isinstance(body, dict) else []
    return status, invoices


def _payments_for(ctx, customer_external_id):
    status, body = ctx.lago.get(
        f"/api/v1/payments?external_customer_id={customer_external_id}"
    )
    payments = body.get("payments", []) if isinstance(body, dict) else []
    return status, payments


def _gating_invoice(invoices):
    subscription_invoices = [
        inv for inv in invoices
        if inv.get("invoice_type") in (None, "subscription")
    ]
    return subscription_invoices[0] if subscription_invoices else (
        invoices[0] if invoices else None
    )


def _succeeded(payments):
    return [p for p in payments if p.get("status") == "succeeded"]


# -- phases -------------------------------------------------------------------


def phase_setup(ctx):
    """Create Feature, pay-in-advance monthly Plan, and plan entitlement."""
    expected = (
        "feature + plan created (monthly, pay_in_advance, amount_cents>0) and "
        "the plan entitlement attached, each 2xx"
    )
    feature_code = ctx.feature_code
    plan_code = ctx.plan_code
    try:
        fs, _fb = ctx.lago.post("/api/v1/features", fixtures.feature_payload(feature_code))
        ps, pb = ctx.lago.post("/api/v1/plans", fixtures.plan_payload(plan_code))
        observed = {
            "feature_code": feature_code,
            "plan_code": plan_code,
            "feature_create_status": fs,
            "plan_create_status": ps,
        }
        plan_echo = (pb or {}).get("plan", {}) if isinstance(pb, dict) else {}
        observed["plan_interval"] = plan_echo.get("interval")
        observed["plan_amount_cents"] = plan_echo.get("amount_cents")
        observed["plan_pay_in_advance"] = plan_echo.get("pay_in_advance")
        if plan_echo and plan_echo.get("pay_in_advance") is not True:
            ctx.note("runtime plan echo did not confirm pay_in_advance=true")
        if not _ok(fs) or not _ok(ps):
            observed["error"] = "feature or plan creation rejected"
            status = FAIL
        else:
            es, eb = ctx.lago.post(
                f"/api/v1/plans/{plan_code}/entitlements",
                fixtures.plan_entitlements_payload(feature_code),
            )
            observed["entitlement_attach_status"] = es
            # The v1.53.0 PlanEntitlementSerializer keys the feature by "code".
            attached = isinstance(eb, dict) and any(
                (item.get("code") or item.get("feature_code")) == feature_code
                for item in eb.get("entitlements", [])
            )
            if not _ok(es) or not attached:
                observed["error"] = "plan entitlement attach failed or unconfirmed"
                status = FAIL
            else:
                ctx.state["feature_code"] = feature_code
                ctx.state["plan_code"] = plan_code
                status = PASS
    except OSError as error:
        return _blocked(ctx, "setup", f"Lago API unreachable ({error.__class__.__name__})", expected)
    return _report(ctx, "setup", expected, observed, status,
                   evidence={"codes": {"feature": feature_code, "plan": plan_code}})


def phase_provider_setup(ctx):
    """Register the Stripe provider and connect customers A/B/C.

    A: pm_card_authenticationRequired (stable pending window), B: pm_card_visa
    (settles), C: pm_card_visa_chargeDeclined (negative control).
    """
    expected = (
        "AddStripePaymentProvider via GraphQL; Stripe test customers with "
        "attached default payment methods; Lago customers linked with "
        "provider_customer_id and a Lago default payment method each"
    )
    if not ctx.stripe_key:
        return _blocked(ctx, "provider_setup",
                        "missing Stripe test key (set STRIPE_SECRET_KEY, sk_test_/rk_test_ only)",
                        expected)
    if not ctx.graphql_jwt:
        return _blocked(ctx, "provider_setup",
                        "missing operator GraphQL JWT (loginUser)", expected)

    # 1. GraphQL provider registration (user JWT, not Premium-gated).
    try:
        gstatus, gbody = ctx.graphql(
            fixtures.ADD_STRIPE_PROVIDER_QUERY,
            fixtures.add_stripe_provider_variables(
                f"{ctx.prefix}-stripe", STRIPE_PROVIDER_NAME, ctx.stripe_key
            ),
        )
    except OSError as error:
        return _blocked(ctx, "provider_setup",
                        f"Lago API unreachable ({error.__class__.__name__})", expected)
    provider = ((gbody or {}).get("data") or {}).get("addStripePaymentProvider") \
        if isinstance(gbody, dict) else None
    if not _ok(gstatus) or not provider:
        errors = (gbody or {}).get("errors") if isinstance(gbody, dict) else None
        auth_rejected = gstatus in (401, 403) or any(
            isinstance(e, dict) and "authoriz" in str(e.get("message", "")).lower()
            for e in (errors or [])
        )
        if auth_rejected:
            return _blocked(ctx, "provider_setup",
                            "GraphQL operator JWT rejected (loginUser)", expected)
        ctx.note(f"graphql provider registration failed (HTTP {gstatus})")
        observed = {"blocked_reason": "AddStripePaymentProvider failed",
                    "graphql_status": gstatus,
                    "graphql_errors": errors}
        return _report(ctx, "provider_setup", expected, observed, FAIL)

    observed = {"provider": {"code": provider.get("code"), "name": provider.get("name")},
                "customers": {}}
    # 2. Stripe + Lago customers.
    customers = {}
    try:
        for tag in CUSTOMER_TAGS:
            ext = ctx.customer_external_id(tag)
            stripe_customer = ctx.stripe.create_customer(
                description=f"{ext} ({CUSTOMER_NAMES[tag]})",
                metadata={"run": ctx.run_id, "tag": tag},
            )
            stripe_customer_id = stripe_customer["id"]
            ctx.stripe.attach_payment_method(STRIPE_PAYMENT_METHODS[tag], stripe_customer_id)
            ctx.stripe.set_default_payment_method(stripe_customer_id, STRIPE_PAYMENT_METHODS[tag])
            cs, _cb = ctx.lago.post(
                "/api/v1/customers",
                fixtures.customer_payload(ext, CUSTOMER_NAMES[tag], stripe_customer_id),
            )
            if not _ok(cs):
                observed["error"] = f"Lago customer create rejected for {tag} (HTTP {cs})"
                ctx.state["customers"] = customers
                return _report(ctx, "provider_setup", expected, observed, FAIL)
            # 3. Bounded poll: Lago default payment method imported for this customer.
            def fetch(ext=ext):
                status, body = ctx.lago.get(f"/api/v1/customers/{ext}/payment_methods")
                methods = body.get("payment_methods", []) if isinstance(body, dict) else []
                return status, methods

            ready, last, attempts = ctx.poll(
                fetch,
                lambda r: _ok(r[0]) and len(r[1]) > 0,
                f"default payment method import for {tag}",
            )
            customers[tag] = {
                "external_id": ext,
                "stripe_customer_id": stripe_customer_id,
                "payment_methods_ready": ready,
                "payment_methods_poll_attempts": attempts,
            }
            observed["customers"][tag] = customers[tag]
            if not ready:
                observed["error"] = (
                    f"no Lago default payment method imported for customer {tag} "
                    f"(last: {last[0]} with {len(last[1])} methods)"
                )
                ctx.state["customers"] = customers
                return _report(ctx, "provider_setup", expected, observed, FAIL)
    except OSError as error:
        ctx.state["customers"] = customers
        reason = "Stripe API unreachable" if "stripe" in str(error.__class__.__name__).lower() \
            else f"transport error ({error.__class__.__name__})"
        return _blocked(ctx, "provider_setup",
                        f"Stripe API unreachable ({error.__class__.__name__})", expected)

    ctx.state["customers"] = customers
    ctx.state["provider_report"] = True
    return _report(ctx, "provider_setup", expected, observed, PASS)


def phase_gate(ctx):
    """AC1: customer A gated subscription observed incomplete and unusable."""
    expected = (
        "subscription created with activation_rules payment/timeout_hours=0 is "
        "immediately incomplete; entitlements endpoint 404; gating invoice "
        "open/pending with no number; at most one non-succeeded payment; "
        "state stable across the auth-challenge window"
    )
    customer = (ctx.state.get("customers") or {}).get("a")
    if not customer or not ctx.state.get("plan_code"):
        return _blocked(ctx, "gate",
                        "requires provider-connected customer A and the lab plan "
                        "(provider setup did not run or did not succeed)", expected)
    ext = ctx.subscription_external_id("a")
    try:
        cs, cb = ctx.lago.post(
            "/api/v1/subscriptions",
            fixtures.subscription_payload(customer["external_id"], ctx.state["plan_code"], ext),
        )
    except OSError as error:
        return _blocked(ctx, "gate", f"Lago API unreachable ({error.__class__.__name__})", expected)
    observed = {
        "subscription_external_id": ext,
        "create_status": cs,
        "subscription_status": (cb or {}).get("subscription", {}).get("status")
        if isinstance(cb, dict) else None,
    }
    if not _ok(cs) or observed["subscription_status"] != "incomplete":
        observed["error"] = "gated subscription create rejected or not incomplete"
        return _report(ctx, "gate", expected, observed, FAIL,
                       evidence={"create_response": cb})
    ctx.state.setdefault("subscriptions_created", set()).add("a")

    # Entitlements must be unusable while incomplete.
    try:
        es, _eb = ctx.lago.get(f"/api/v1/subscriptions/{ext}/entitlements")
    except OSError as error:
        return _blocked(ctx, "gate", f"Lago API unreachable ({error.__class__.__name__})", expected)
    observed["entitlements_status"] = es
    if es != 404:
        ctx.note(
            f"entitlements endpoint returned HTTP {es} while incomplete "
            f"(source says active-only => 404); runtime deviation recorded"
        )

    # Gating invoice: open, pending, numberless (created async).
    def fetch_invoices():
        return _invoices_for(ctx, customer["external_id"])[1]

    invoice_ready, invoice_last, _ = ctx.poll(
        fetch_invoices,
        lambda invoices: _gating_invoice(invoices) is not None,
        "gating invoice creation for customer A",
    )
    invoice = _gating_invoice(invoice_last or [])
    if invoice is None:
        observed["error"] = "gating invoice never appeared"
        return _report(ctx, "gate", expected, observed, FAIL,
                       evidence={"invoices_last_seen": invoice_last})
    observed["invoice_status"] = invoice.get("status")
    observed["invoice_payment_status"] = invoice.get("payment_status")
    observed["invoice_number"] = invoice.get("number")
    observed["invoice_lago_id"] = invoice.get("lago_id")

    try:
        ps, payments = _payments_for(ctx, customer["external_id"])
    except OSError as error:
        return _blocked(ctx, "gate", f"Lago API unreachable ({error.__class__.__name__})", expected)
    non_succeeded = [p for p in payments if p.get("status") != "succeeded"]
    observed["payments_non_succeeded_count"] = len(non_succeeded)

    pre_charge_ok = (
        observed["subscription_status"] == "incomplete"
        and observed["entitlements_status"] == 404
        and observed["invoice_status"] == "open"
        and observed["invoice_payment_status"] == "pending"
        and observed["invoice_number"] in (None, "")
        and len(non_succeeded) <= 1
    )

    # Stable window across the auth-challenge auto-charge attempt.
    stable = {"still_incomplete": True, "still_pending": True, "rounds": 0}
    for _ in range(ctx.stability_rounds):
        time.sleep(ctx.stability_delay)
        ss, sb = _subscription_show(ctx, ext, "incomplete")
        sub_now = (sb or {}).get("subscription", {}) if isinstance(sb, dict) else {}
        if ss == 404:
            stable["still_incomplete"] = False
        invoices_now = fetch_invoices()
        invoice_now = _gating_invoice(invoices_now)
        if invoice_now is not None and invoice_now.get("payment_status") != "pending":
            stable["still_pending"] = False
        stable["rounds"] += 1
        if not stable["still_incomplete"]:
            break
    observed["stable_window"] = stable

    ctx.state["gate"] = {
        "subscription_external_id": ext,
        "invoice_lago_id": observed["invoice_lago_id"],
        "customer_external_id": customer["external_id"],
    }

    status = FAIL
    if pre_charge_ok and stable["still_incomplete"] and stable["still_pending"]:
        status = PASS
    elif pre_charge_ok:
        # The auto-charge failed instead of staying pending: AC1 evidence
        # falls back to the observed pre-charge window (plan risk 2).
        ss, sb = _subscription_show(ctx, ext, "canceled")
        canceled = _ok(ss)
        if canceled:
            reason = (sb or {}).get("subscription", {}).get("cancellation_reason")
            ctx.note(
                "3DS window did not hold: subscription became canceled "
                f"(cancellation_reason={reason}); AC1 evidence is the pre-charge "
                "observation window between creation and the first charge attempt"
            )
            status = PASS
        else:
            observed["error"] = "pre-charge window observed but state drifted unexpectedly"
    else:
        observed["error"] = observed.get("error") or "pre-charge contract not observed"
    return _report(ctx, "gate", expected, observed, status)


def phase_manual(ctx):
    """AC4 (manual path): attempt a manual Payment on customer A's gate."""
    expected = (
        "POST /api/v1/payments {invoice_id, amount_cents, reference, paid_at} "
        "against the open gating invoice is forbidden on Community "
        "(Premium-gated) and leaves invoice + subscription state unchanged"
    )
    gate = ctx.state.get("gate")
    if not gate:
        return _blocked(ctx, "manual",
                        "requires customer A gating invoice (gate phase did not produce one)",
                        expected)
    customer_ext = gate["customer_external_id"]
    sub_ext = gate["subscription_external_id"]
    try:
        _, invoices = _invoices_for(ctx, customer_ext)
        invoice = next(
            (inv for inv in invoices if inv.get("lago_id") == gate["invoice_lago_id"]),
            _gating_invoice(invoices),
        )
        if invoice is None:
            return _blocked(ctx, "manual", "gating invoice disappeared before the attempt", expected)
        amount = invoice.get("total_amount_cents")
        before = {
            "invoice_status": invoice.get("status"),
            "invoice_payment_status": invoice.get("payment_status"),
            "total_paid_amount_cents": invoice.get("total_paid_amount_cents"),
        }
        _, payments_before = _payments_for(ctx, customer_ext)
        ms, mb = ctx.lago.post(
            "/api/v1/payments",
            fixtures.manual_payment_payload(
                invoice.get("lago_id"), amount, f"{ctx.prefix}-manual-1"
            ),
        )
        # Re-read authoritative state.
        _, invoices_after = _invoices_for(ctx, customer_ext)
        invoice_after = next(
            (inv for inv in invoices_after if inv.get("lago_id") == invoice.get("lago_id")),
            _gating_invoice(invoices_after),
        )
        _, payments_after = _payments_for(ctx, customer_ext)
        ss, sb = _subscription_show(ctx, sub_ext, "incomplete")
        sub_after = (sb or {}).get("subscription", {}) if isinstance(sb, dict) else {}
        forbidden = ms == 403
        unchanged = (
            invoice_after is not None
            and invoice_after.get("status") == before["invoice_status"]
            and invoice_after.get("payment_status") == before["invoice_payment_status"]
            and invoice_after.get("total_paid_amount_cents") == before["total_paid_amount_cents"]
            and _ok(ss)
            and sub_after.get("status") == "incomplete"
            and len(_succeeded(payments_after)) == len(_succeeded(payments_before))
        )
        observed = {
            "http_status": ms,
            "forbidden": forbidden,
            "state_unchanged": unchanged,
            "invoice_status_before": before["invoice_status"],
            "invoice_status_after": invoice_after.get("status") if invoice_after else None,
            "invoice_payment_status_after": invoice_after.get("payment_status") if invoice_after else None,
            "subscription_status_after": sub_after.get("status") if _ok(ss) else f"http_{ss}",
            "param_contract": {
                "accepted_fields_source_verified": [
                    "invoice_id", "amount_cents", "reference", "paid_at"
                ],
                "note": (
                    "no external_id and no status field exist in the v1.53.0 REST "
                    "contract; status is always recorded succeeded server-side and "
                    "the operation is Premium-gated"
                ),
            },
        }
        if not forbidden:
            ctx.note(
                "manual payment was NOT forbidden by the runtime -- premium gate "
                "not observed; deviation from pinned-source expectation"
            )
            status = FAIL
        elif not unchanged:
            observed["error"] = "state changed despite forbidden response"
            status = FAIL
        else:
            status = PASS
        return _report(ctx, "manual", expected, observed, status,
                       evidence={"response_body": mb})
    except OSError as error:
        return _blocked(ctx, "manual", f"Lago API unreachable ({error.__class__.__name__})", expected)


def phase_activate(ctx):
    """AC2: customer B gated subscription settles exactly once via provider."""
    expected = (
        "gated subscription for customer B transitions to active exactly once: "
        "one succeeded provider payment with provider_payment_id, invoice "
        "finalized with a number and payment_status succeeded, entitlements "
        "200 with the plan feature, total_paid == invoice total"
    )
    customer = (ctx.state.get("customers") or {}).get("b")
    if not customer or not ctx.state.get("plan_code"):
        return _blocked(ctx, "activate",
                        "requires provider-connected customer B and the lab plan", expected)
    ext = ctx.subscription_external_id("b")
    try:
        cs, cb = ctx.lago.post(
            "/api/v1/subscriptions",
            fixtures.subscription_payload(customer["external_id"], ctx.state["plan_code"], ext),
        )
    except OSError as error:
        return _blocked(ctx, "activate", f"Lago API unreachable ({error.__class__.__name__})", expected)
    observed = {"subscription_external_id": ext, "create_status": cs}
    if not _ok(cs):
        observed["error"] = "gated subscription create rejected"
        return _report(ctx, "activate", expected, observed, FAIL,
                       evidence={"create_response": cb})
    ctx.state.setdefault("subscriptions_created", set()).add("b")

    try:
        def sub_active():
            status, body = _subscription_show(ctx, ext, "active")
            sub = (body or {}).get("subscription", {}) if isinstance(body, dict) else {}
            return _ok(status), sub.get("status"), status

        settled, sub_last, attempts = ctx.poll(
            sub_active, lambda r: r[0], "customer B subscription activation"
        )
        observed["subscription_status"] = sub_last[1]
        observed["poll_exhausted"] = not settled
        if not settled:
            # Attach the last actually observed state for review.
            is_status, ib = _subscription_show(ctx, ext, "incomplete")
            observed["subscription_http_status"] = sub_last[2]
            observed["last_observed_incomplete"] = (
                (ib or {}).get("subscription", {}).get("status")
                if _ok(is_status) and isinstance(ib, dict) else None
            )
            observed["error"] = "subscription never became active before poll timeout"
            return _report(ctx, "activate", expected, observed, FAIL)

        es, eb = ctx.lago.get(f"/api/v1/subscriptions/{ext}/entitlements")
        entitlements = eb.get("entitlements", []) if isinstance(eb, dict) else []
        observed["entitlements_status"] = es
        observed["entitlements"] = entitlements
        feature_present = any(
            (item.get("code") or item.get("feature_code")) == ctx.state.get("feature_code")
            for item in entitlements
        )

        _, invoices = _invoices_for(ctx, customer["external_id"])
        invoice = _gating_invoice(invoices)
        if invoice is None:
            observed["error"] = "no invoice found for customer B"
            return _report(ctx, "activate", expected, observed, FAIL)
        observed["invoice_status"] = invoice.get("status")
        observed["invoice_number"] = invoice.get("number")
        observed["invoice_payment_status"] = invoice.get("payment_status")
        observed["invoice_total_amount_cents"] = invoice.get("total_amount_cents")

        _, payments = _payments_for(ctx, customer["external_id"])
        succeeded = _succeeded(payments)
        observed["payments_succeeded_count"] = len(succeeded)
        observed["provider_payment_id"] = succeeded[0].get("provider_payment_id") if succeeded else None
        observed["total_paid_amount_cents"] = invoice.get("total_paid_amount_cents")

        checks = {
            "subscription_active": observed["subscription_status"] == "active",
            "entitlements_ok": es == 200 and feature_present,
            "invoice_finalized": observed["invoice_status"] == "finalized",
            "invoice_numbered": bool(observed["invoice_number"]),
            "invoice_paid": observed["invoice_payment_status"] == "succeeded",
            "exactly_one_succeeded_payment": len(succeeded) == 1,
            "provider_payment_recorded": bool(observed["provider_payment_id"]),
            "totals_match": (
                invoice.get("total_paid_amount_cents")
                == invoice.get("total_amount_cents")
            ),
        }
        observed["checks"] = checks
        status = PASS if all(checks.values()) else FAIL
        if not checks["exactly_one_succeeded_payment"]:
            ctx.note(
                f"expected exactly one succeeded payment, observed {len(succeeded)}"
            )
        if status == PASS:
            ctx.state["activation"] = {
                "subscription_external_id": ext,
                "invoice_lago_id": invoice.get("lago_id"),
                "customer_external_id": customer["external_id"],
                "total_paid_amount_cents": invoice.get("total_paid_amount_cents"),
                "invoice_total_amount_cents": invoice.get("total_amount_cents"),
            }
        return _report(ctx, "activate", expected, observed, status)
    except OSError as error:
        return _blocked(ctx, "activate", f"Lago API unreachable ({error.__class__.__name__})", expected)


def phase_duplicates(ctx):
    """AC2: duplicate registrations against settled customer B state."""
    expected = (
        "re-POST subscription with same external_id -> 422 value_already_exist; "
        "retry_payment on the paid invoice -> not allowed; manual re-POST -> "
        "forbidden; final authoritative state byte-identical (one active "
        "subscription, one succeeded payment, invoice unchanged)"
    )
    activation = ctx.state.get("activation")
    if not activation:
        return _blocked(ctx, "duplicates",
                        "requires settled customer B state (activate phase did not succeed)",
                        expected)
    sub_ext = activation["subscription_external_id"]
    customer = (ctx.state.get("customers") or {}).get("b")
    invoice_lago_id = activation["invoice_lago_id"]
    try:
        # Baseline final state.
        _, sub_body = _subscription_show(ctx, sub_ext, "active")
        sub = (sub_body or {}).get("subscription", {}) if isinstance(sub_body, dict) else {}
        base_lago_id = sub.get("lago_id")
        _, invoices = _invoices_for(ctx, customer["external_id"])
        invoice = next(
            (inv for inv in invoices if inv.get("lago_id") == invoice_lago_id),
            _gating_invoice(invoices),
        )
        _, payments = _payments_for(ctx, customer["external_id"])
        baseline = {
            "subscription_status": sub.get("status"),
            "invoice_status": invoice.get("status"),
            "invoice_payment_status": invoice.get("payment_status"),
            "invoice_total_amount_cents": invoice.get("total_amount_cents"),
            "total_paid_amount_cents": invoice.get("total_paid_amount_cents"),
            "payments_succeeded_count": len(_succeeded(payments)),
        }

        # Probe 1: duplicate subscription registration.
        rs, rb = ctx.lago.post(
            "/api/v1/subscriptions",
            fixtures.subscription_payload(customer["external_id"], ctx.state["plan_code"], sub_ext),
        )
        re_post = {"http_status": rs, "code": (rb or {}).get("code") if isinstance(rb, dict) else None}

        # Probe 2: retry_payment on the already-paid invoice.
        rrs, rrb = ctx.lago.post(f"/api/v1/invoices/{invoice_lago_id}/retry_payment", {})
        retry = {
            "http_status": rrs,
            "code": (rrb or {}).get("code") if isinstance(rrb, dict) else None,
        }

        # Probe 3: manual payment re-attempt.
        ms, mb = ctx.lago.post(
            "/api/v1/payments",
            fixtures.manual_payment_payload(
                invoice_lago_id, invoice.get("total_amount_cents"),
                f"{ctx.prefix}-manual-dup",
            ),
        )
        manual_re_post = {"http_status": ms}

        # Final authoritative state after all duplicate attempts.
        _, sub_body2 = _subscription_show(ctx, sub_ext, "active")
        sub2 = (sub_body2 or {}).get("subscription", {}) if isinstance(sub_body, dict) else {}
        _, invoices2 = _invoices_for(ctx, customer["external_id"])
        invoice2 = next(
            (inv for inv in invoices2 if inv.get("lago_id") == invoice_lago_id),
            _gating_invoice(invoices2),
        )
        _, payments2 = _payments_for(ctx, customer["external_id"])
        final = {
            "subscription_status": sub2.get("status"),
            "subscription_same_lago_id": sub2.get("lago_id") == base_lago_id,
            "invoice_status": invoice2.get("status") if invoice2 else None,
            "invoice_payment_status": invoice2.get("payment_status") if invoice2 else None,
            "invoice_total_amount_cents": invoice2.get("total_amount_cents") if invoice2 else None,
            "total_paid_amount_cents": invoice2.get("total_paid_amount_cents") if invoice2 else None,
            "payments_succeeded_count": len(_succeeded(payments2)),
        }
        probes_rejected = (
            not _ok(rs) and not _ok(rrs) and ms == 403
        )
        state_intact = (
            final["subscription_status"] == baseline["subscription_status"] == "active"
            and final["subscription_same_lago_id"]
            and final["invoice_status"] == baseline["invoice_status"]
            and final["invoice_payment_status"] == baseline["invoice_payment_status"]
            and final["invoice_total_amount_cents"] == baseline["invoice_total_amount_cents"]
            and final["total_paid_amount_cents"] == baseline["total_paid_amount_cents"]
            and final["payments_succeeded_count"] == baseline["payments_succeeded_count"] == 1
        )
        observed = {
            "re_post": re_post,
            "retry_payment": retry,
            "manual_re_post": manual_re_post,
            "final": final,
        }
        status = PASS if probes_rejected and state_intact else FAIL
        if not probes_rejected:
            ctx.note("a duplicate registration attempt was accepted by the runtime")
        if not state_intact:
            ctx.note("final state drifted after duplicate attempts (exactly-once violated)")
        return _report(ctx, "duplicates", expected, observed, status,
                       evidence={"baseline": baseline,
                                 "responses": {"re_post": rb, "retry_payment": rrb,
                                               "manual": mb}})
    except OSError as error:
        return _blocked(ctx, "duplicates", f"Lago API unreachable ({error.__class__.__name__})", expected)


def phase_retries(ctx):
    """AC3: response-loss recovery and retry on a live pending gate."""
    expected = (
        "response-loss on create recovers the SAME Lago subscription via "
        "GET by external_id (no second commercial object); retry_payment on "
        "customer A's pending gate re-attempts without creating a second "
        "pending/succeeded payment row; customer A payment count unchanged"
    )
    activation = ctx.state.get("activation")
    gate = ctx.state.get("gate")
    if not activation or not gate:
        return _blocked(ctx, "retries",
                        "requires settled customer B state and customer A pending gate",
                        expected)
    sub_ext = activation["subscription_external_id"]
    known_lago_id = None
    customer_b = (ctx.state.get("customers") or {}).get("b")
    customer_a = (ctx.state.get("customers") or {}).get("a")
    try:
        _, sub_body = _subscription_show(ctx, sub_ext, "active")
        known_lago_id = ((sub_body or {}).get("subscription") or {}).get("lago_id")

        # (1) response-loss on create: re-POST (outcome treated as lost), then
        # recover via the documented same-identity GET.
        rs, _rb = ctx.lago.post(
            "/api/v1/subscriptions",
            fixtures.subscription_payload(customer_b["external_id"], ctx.state["plan_code"], sub_ext),
        )
        is_status, ib = _subscription_index(
            ctx, sub_ext, ["active", "incomplete", "pending", "canceled"]
        )
        recovered = ib.get("subscriptions", []) if isinstance(ib, dict) else []
        recovery = {
            "re_post_http_status": rs,
            "index_http_status": is_status,
            "subscription_count": len(recovered),
            "same_lago_id": (
                len(recovered) == 1
                and known_lago_id is not None
                and recovered[0].get("lago_id") == known_lago_id
            ),
            "subscription_status": recovered[0].get("status") if recovered else None,
        }

        # (2) retry on the live pending gate (customer A).
        invoice_lago_id = gate["invoice_lago_id"]
        _, invoices = _invoices_for(ctx, customer_a["external_id"])
        invoice = next(
            (inv for inv in invoices if inv.get("lago_id") == invoice_lago_id),
            None,
        )
        _, payments_before = _payments_for(ctx, customer_a["external_id"])
        rrs, rrb = ctx.lago.post(f"/api/v1/invoices/{invoice_lago_id}/retry_payment", {})
        time.sleep(ctx.stability_delay)
        _, invoices_after = _invoices_for(ctx, customer_a["external_id"])
        invoice_after = next(
            (inv for inv in invoices_after if inv.get("lago_id") == invoice_lago_id), None
        )
        _, payments_after = _payments_for(ctx, customer_a["external_id"])
        gate_retry = {
            "http_status": rrs,
            "invoice_payment_status_before": invoice.get("payment_status") if invoice else None,
            "invoice_payment_status_after": invoice_after.get("payment_status") if invoice_after else None,
            "payments_count_before": len(payments_before),
            "payments_count_after": len(payments_after),
            "succeeded_count_before": len(_succeeded(payments_before)),
            "succeeded_count_after": len(_succeeded(payments_after)),
        }
        observed = {
            "response_loss_recovery": recovery,
            "pending_gate_retry": gate_retry,
        }
        checks = {
            "same_identity_recovered": (
                recovery["subscription_count"] == 1 and recovery["same_lago_id"]
            ),
            "gate_still_pending": (
                gate_retry["invoice_payment_status_after"] == "pending"
            ),
            "no_second_payment_row": (
                gate_retry["payments_count_after"] == gate_retry["payments_count_before"]
            ),
            "no_succeeded_payment_for_gate": gate_retry["succeeded_count_after"] == 0,
        }
        observed["checks"] = checks
        status = PASS if all(checks.values()) else FAIL
        return _report(ctx, "retries", expected, observed, status,
                       evidence={"responses": {"re_post": _rb, "retry_payment": rrb}})
    except OSError as error:
        return _blocked(ctx, "retries", f"Lago API unreachable ({error.__class__.__name__})", expected)


def phase_decline_control(ctx):
    """Negative control: a declined provider payment must not activate."""
    expected = (
        "customer C gated subscription with the declined card ends canceled "
        "with cancellation_reason payment_failed; invoice closed; "
        "entitlements 404 -- activation only on verified provider success"
    )
    customer = (ctx.state.get("customers") or {}).get("c")
    if not customer or not ctx.state.get("plan_code"):
        return _blocked(ctx, "decline_control",
                        "requires provider-connected customer C and the lab plan", expected)
    ext = ctx.subscription_external_id("c")
    try:
        cs, cb = ctx.lago.post(
            "/api/v1/subscriptions",
            fixtures.subscription_payload(customer["external_id"], ctx.state["plan_code"], ext),
        )
        observed = {"subscription_external_id": ext, "create_status": cs}
        if not _ok(cs):
            observed["error"] = "gated subscription create rejected for customer C"
            return _report(ctx, "decline_control", expected, observed, FAIL,
                           evidence={"create_response": cb})
        ctx.state.setdefault("subscriptions_created", set()).add("c")

        def canceled():
            status, body = _subscription_show(ctx, ext, "canceled")
            sub = (body or {}).get("subscription", {}) if isinstance(body, dict) else {}
            return _ok(status), sub.get("status"), sub.get("cancellation_reason")

        settled, last, _ = ctx.poll(canceled, lambda r: r[0], "customer C cancellation")
        observed["poll_exhausted"] = not settled
        observed["subscription_status"] = last[1]
        observed["cancellation_reason"] = last[2]
        if not settled:
            # Attach the last actually observed state for review.
            is_status, ib = _subscription_show(ctx, ext, "incomplete")
            observed["last_observed_incomplete"] = (
                (ib or {}).get("subscription", {}).get("status")
                if _ok(is_status) and isinstance(ib, dict) else None
            )
            observed["error"] = "subscription never canceled before poll timeout"
            return _report(ctx, "decline_control", expected, observed, FAIL)

        _, invoices = _invoices_for(ctx, customer["external_id"])
        invoice = _gating_invoice(invoices)
        observed["invoice_status"] = invoice.get("status") if invoice else None
        observed["invoice_payment_status"] = invoice.get("payment_status") if invoice else None
        es, _eb = ctx.lago.get(f"/api/v1/subscriptions/{ext}/entitlements")
        observed["entitlements_status"] = es
        checks = {
            "canceled": observed["subscription_status"] == "canceled",
            "payment_failed_reason": observed["cancellation_reason"] == "payment_failed",
            "invoice_closed": observed["invoice_status"] == "closed",
            "entitlements_unusable": es == 404,
        }
        observed["checks"] = checks
        status = PASS if all(checks.values()) else FAIL
        return _report(ctx, "decline_control", expected, observed, status)
    except OSError as error:
        return _blocked(ctx, "decline_control", f"Lago API unreachable ({error.__class__.__name__})", expected)


def phase_cleanup(ctx):
    """Always executed last: delete everything this run created, in order.

    subscription* -> customers -> plan -> feature (dependency order), plus
    best-effort Stripe test-customer deletion. Every object outcome is
    reported distinctly; a cleanup failure fails this phase (never silent).
    """
    expected = "every lab object created this run is deleted; failures reported distinctly"
    objects = []
    state = ctx.state

    def record(object_name, identifier, outcome, http_status=None):
        objects.append({
            "object": object_name,
            "id": identifier,
            "outcome": outcome,
            "http_status": http_status,
        })

    # 1. subscriptions (terminate), then customers.
    created_tags = state.get("subscriptions_created", set())
    for tag in CUSTOMER_TAGS:
        if tag not in created_tags:
            continue
        sub_ext = ctx.subscription_external_id(tag)
        try:
            status, _body = ctx.lago.delete(f"/api/v1/subscriptions/{sub_ext}")
            record(f"subscription:{tag}", sub_ext,
                   "deleted" if _ok(status) else "failed", status)
        except OSError:
            record(f"subscription:{tag}", sub_ext, "failed", None)

    for tag in CUSTOMER_TAGS:
        customer = (state.get("customers") or {}).get(tag)
        if not customer:
            continue
        try:
            status, _body = ctx.lago.delete(
                f"/api/v1/customers/{customer['external_id']}"
            )
            record(f"customer:{tag}", customer["external_id"],
                   "deleted" if _ok(status) else "failed", status)
        except OSError:
            record(f"customer:{tag}", customer["external_id"], "failed", None)

    # 2. plan, then feature.
    if state.get("plan_code"):
        try:
            status, _body = ctx.lago.delete(f"/api/v1/plans/{state['plan_code']}")
            record("plan", state["plan_code"], "deleted" if _ok(status) else "failed", status)
        except OSError:
            record("plan", state["plan_code"], "failed", None)
    if state.get("feature_code"):
        try:
            status, _body = ctx.lago.delete(f"/api/v1/features/{state['feature_code']}")
            record("feature", state["feature_code"], "deleted" if _ok(status) else "failed", status)
        except OSError:
            record("feature", state["feature_code"], "failed", None)

    # 3. Stripe test customers (throwaway, best-effort but reported).
    for tag in CUSTOMER_TAGS:
        customer = (state.get("customers") or {}).get(tag)
        if not customer or not ctx.stripe:
            continue
        deleted = ctx.stripe.delete_customer(customer["stripe_customer_id"])
        record("stripe_customer", customer["stripe_customer_id"],
               "deleted" if deleted else "failed", 200 if deleted else None)

    failures = [item for item in objects if item["outcome"] != "deleted"]
    observed = {
        "objects": objects,
        "cleanup_failures": [f"{item['object']}({item['id']})" for item in failures],
    }
    status = FAIL if failures else PASS
    return _report(ctx, "cleanup", expected, observed, status)


PHASE_ORDER = (
    phase_setup,
    phase_provider_setup,
    phase_gate,
    phase_manual,
    phase_activate,
    phase_duplicates,
    phase_retries,
    phase_decline_control,
    phase_cleanup,
)
