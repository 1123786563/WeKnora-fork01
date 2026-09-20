"""Lago v1.53.0 lab client: delivery, read-back, classification, reconciliation.

Coded against the pinned-release contract facts (getlago/lago-api @ 591ae90,
verified 2026-09-21 -- see docs/plans/2026-09-21-lago-t04-pricing-group.md):

* event idempotency field is ``transaction_id`` (POST /api/v1/events body
  member), the response on duplicates is HTTP 422 with
  ``error_details.transaction_id == ["value_already_exist"]`` and it does NOT
  distinguish retry-vs-conflict (conflict detection needs read-back);
* batch delivery is POST /api/v1/events/batch with at most 100 events;
* events are queryable via GET /api/v1/events/{transaction_id} and
  GET /api/v1/events?external_subscription_id=&code=;
* current usage is customer-scoped in v1.53.0:
  GET /api/v1/customers/{external_customer_id}/current_usage?external_subscription_id=...
  and every ``charges_usage[].units`` is a decimal *string*;
* money comparisons are exact integer cents; units compare as Decimal.

The API key is caller-environment input only: it is sent as a Bearer
credential to the configured origin and never appears in any serialized
output (``dumps_sanitized`` redacts it).  An empty key raises
``BlockedEnvError`` before any network traffic.
"""

from __future__ import annotations

import json
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation
from urllib.error import HTTPError
from urllib.parse import quote, urlencode
from urllib.request import ProxyHandler, Request, build_opener

from fixture import ExpectedUsage

# The Lago origin is always local; requests and the Bearer credential must
# never be routed through a system- or env-configured proxy.
_OPENER = build_opener(ProxyHandler({}))

REQUEST_TIMEOUT_SECONDS = 30
BATCH_LIMIT = 100
REDACTED = "[REDACTED]"


class BlockedEnvError(RuntimeError):
    """Raised when the caller environment cannot exercise the contract."""


@dataclass(frozen=True)
class CallResult:
    status: int
    body: dict | None
    error: str | None = None

    def to_dict(self) -> dict:
        return {"status": self.status, "body": self.body, "error": self.error}


def _utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def _require_key(api_key: str) -> str:
    if not api_key:
        raise BlockedEnvError("missing API key (set LAGO_API_KEY in the environment)")
    return api_key


def _call(method: str, url: str, api_key: str, payload=None) -> CallResult:
    """One JSON call.  Returns the outcome; never raises for HTTP statuses."""
    headers = {"Accept": "application/json"}
    body = None
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    request = Request(url, data=body, headers=headers, method=method)
    try:
        with _OPENER.open(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            status = response.status
            raw = response.read()
    except HTTPError as error:
        raw = error.read()
        status = error.code
    except OSError as error:
        return CallResult(0, None, f"transport error: {error.__class__.__name__}")
    try:
        parsed = json.loads(raw.decode("utf-8")) if raw else None
    except ValueError:
        return CallResult(status, None, "response body was not valid JSON")
    return CallResult(status, parsed)


def _base(base_url: str) -> str:
    return str(base_url).rstrip("/")


def send_event(base_url: str, api_key: str, event_payload: dict) -> CallResult:
    """POST /api/v1/events with one {event: {...}} body member."""
    _require_key(api_key)
    return _call(
        "POST",
        f"{_base(base_url)}/api/v1/events",
        api_key,
        {"event": event_payload},
    )


def send_batch(base_url: str, api_key: str, event_payloads: list) -> CallResult:
    """POST /api/v1/events/batch with at most 100 events."""
    _require_key(api_key)
    if len(event_payloads) > BATCH_LIMIT:
        raise ValueError(f"batch endpoint accepts at most {BATCH_LIMIT} events")
    return _call(
        "POST",
        f"{_base(base_url)}/api/v1/events/batch",
        api_key,
        {"events": list(event_payloads)},
    )


def get_event(base_url: str, api_key: str, transaction_id: str) -> CallResult:
    """GET /api/v1/events/{transaction_id} -- read back the stored content."""
    _require_key(api_key)
    path = quote(str(transaction_id), safe="")
    return _call("GET", f"{_base(base_url)}/api/v1/events/{path}", api_key)


def list_events(
    base_url: str, api_key: str, external_subscription_id: str, code: str
) -> CallResult:
    """GET /api/v1/events?external_subscription_id=&code= (first page)."""
    _require_key(api_key)
    query = urlencode(
        {"external_subscription_id": external_subscription_id, "code": code}
    )
    return _call("GET", f"{_base(base_url)}/api/v1/events?{query}", api_key)


def get_current_usage(
    base_url: str, api_key: str, customer_external_id: str, external_subscription_id: str
) -> CallResult:
    """GET /api/v1/customers/{id}/current_usage?external_subscription_id=..."""
    _require_key(api_key)
    customer_path = quote(str(customer_external_id), safe="")
    query = urlencode({"external_subscription_id": external_subscription_id})
    return _call(
        "GET",
        f"{_base(base_url)}/api/v1/customers/{customer_path}/current_usage?{query}",
        api_key,
    )


def classify_send(status: int, body) -> str:
    """Classify a send outcome from status + body, without content guessing."""
    if 200 <= status < 300:
        return "accepted"
    if 400 <= status < 500:
        details = None
        if isinstance(body, dict):
            details = body.get("error_details")
        if (
            status == 422
            and isinstance(details, dict)
            and "value_already_exist" in details.get("transaction_id", [])
        ):
            return "duplicate_transaction_id"
        return "validation_error"
    return "unexpected"


@dataclass(frozen=True)
class ChargeComparison:
    metric_key: str
    ok: bool
    observed_units: str | None
    expected_units: str
    observed_events_count: int | None
    expected_events_count: int
    observed_amount_cents: int | None
    expected_amount_cents: int


@dataclass(frozen=True)
class ReconcileReport:
    ok: bool
    diffs: tuple  # short field names, e.g. ("amount_cents", "units")
    details: tuple  # human-readable one-line explanations
    charge_comparisons: tuple = ()
    total_amount_cents: int | None = None

    def to_dict(self) -> dict:
        return {
            "ok": self.ok,
            "diffs": list(self.diffs),
            "details": list(self.details),
            "charges": [comparison.__dict__ for comparison in self.charge_comparisons],
            "total_amount_cents": self.total_amount_cents,
        }


def _metric_suffix(billable_metric) -> str | None:
    if not isinstance(billable_metric, dict):
        return None
    code = billable_metric.get("code")
    if not isinstance(code, str):
        return None
    return "-".join(code.rsplit("-", 2)[-2:])  # "...-model-units" -> "model-units"


def reconcile(observed_customer_usage: dict | None, expected: ExpectedUsage) -> ReconcileReport:
    """Exact comparison of one customer_usage reading against the fixture.

    Any field mismatch (per-charge units / events_count / amount_cents,
    missing or extra charges, currency, taxes, totals) fails the report with
    a short field name in ``diffs`` and a full explanation in ``details``.
    """
    diffs = []
    details = []
    comparisons = []
    usage = observed_customer_usage if isinstance(observed_customer_usage, dict) else {}
    charges_usage = usage.get("charges_usage")
    charges_usage = charges_usage if isinstance(charges_usage, list) else []
    if not isinstance(observed_customer_usage, dict):
        diffs.append("customer_usage")
        details.append("current_usage response missing customer_usage payload")

    matched_codes = set()
    for expectation in expected.charges:
        observed_charge = None
        for candidate in charges_usage:
            if _metric_suffix(candidate.get("billable_metric")) == expectation.metric_key:
                observed_charge = candidate
                break
        if observed_charge is None:
            diffs.append("charges_usage")
            details.append(f"charge {expectation.metric_key}: missing from charges_usage")
            comparisons.append(
                ChargeComparison(
                    metric_key=expectation.metric_key,
                    ok=False,
                    observed_units=None,
                    expected_units=str(expectation.units),
                    observed_events_count=None,
                    expected_events_count=expectation.events_count,
                    observed_amount_cents=None,
                    expected_amount_cents=expectation.amount_cents,
                )
            )
            continue
        matched_codes.add(
            observed_charge.get("billable_metric", {}).get("code")
        )

        observed_units_raw = observed_charge.get("units")
        try:
            observed_units = Decimal(str(observed_units_raw))
            units_ok = observed_units == expectation.units
        except (InvalidOperation, ValueError, TypeError):
            observed_units = None
            units_ok = False
        observed_events = observed_charge.get("events_count")
        events_ok = (
            isinstance(observed_events, int)
            and observed_events == expectation.events_count
        )
        observed_amount = observed_charge.get("amount_cents")
        amount_ok = (
            isinstance(observed_amount, int)
            and observed_amount == expectation.amount_cents
        )
        if not units_ok:
            diffs.append("units")
            details.append(
                f"charge {expectation.metric_key}: units observed "
                f"{observed_units_raw!r} != expected {expectation.units}"
            )
        if not events_ok:
            diffs.append("events_count")
            details.append(
                f"charge {expectation.metric_key}: events_count observed "
                f"{observed_events!r} != expected {expectation.events_count}"
            )
        if not amount_ok:
            diffs.append("amount_cents")
            details.append(
                f"charge {expectation.metric_key}: amount_cents observed "
                f"{observed_amount!r} != expected {expectation.amount_cents}"
            )
        comparisons.append(
            ChargeComparison(
                metric_key=expectation.metric_key,
                ok=units_ok and events_ok and amount_ok,
                observed_units=str(observed_units_raw),
                expected_units=str(expectation.units),
                observed_events_count=observed_events,
                expected_events_count=expectation.events_count,
                observed_amount_cents=observed_amount,
                expected_amount_cents=expectation.amount_cents,
            )
        )

    expected_keys = {expectation.metric_key for expectation in expected.charges}
    for candidate in charges_usage:
        suffix = _metric_suffix(candidate.get("billable_metric"))
        if suffix not in expected_keys:
            diffs.append("charges_usage")
            details.append(
                f"unexpected charge in charges_usage: {candidate.get('billable_metric')}"
            )

    currency = usage.get("currency")
    if currency != "CNY":
        diffs.append("currency")
        details.append(f"currency observed {currency!r} != expected 'CNY'")

    taxes = usage.get("taxes_amount_cents")
    if taxes != 0:
        diffs.append("taxes_amount_cents")
        details.append(f"taxes_amount_cents observed {taxes!r} != expected 0")

    observed_total = usage.get("amount_cents")
    observed_total_all = usage.get("total_amount_cents")
    if not (isinstance(observed_total, int) and observed_total == expected.total_amount_cents):
        diffs.append("amount_cents")
        details.append(
            f"customer_usage.amount_cents observed {observed_total!r} != "
            f"expected {expected.total_amount_cents}"
        )
    if not (
        isinstance(observed_total_all, int)
        and observed_total_all == expected.total_amount_cents
    ):
        diffs.append("total_amount_cents")
        details.append(
            f"customer_usage.total_amount_cents observed {observed_total_all!r} != "
            f"expected {expected.total_amount_cents}"
        )

    ok = not diffs
    return ReconcileReport(
        ok=ok,
        diffs=tuple(diffs),
        details=tuple(details),
        charge_comparisons=tuple(comparisons),
        total_amount_cents=observed_total if isinstance(observed_total, int) else None,
    )


@dataclass(frozen=True)
class PollOutcome:
    reconciled: bool
    attempts: int
    elapsed_seconds: float
    first_exact_at: str | None
    final_report: ReconcileReport
    timeline: tuple = field(default_factory=tuple)

    def to_dict(self) -> dict:
        return {
            "reconciled": self.reconciled,
            "attempts": self.attempts,
            "elapsed_seconds": round(self.elapsed_seconds, 3),
            "first_exact_at": self.first_exact_at,
            "final_report": self.final_report.to_dict(),
            "timeline": list(self.timeline),
        }


def poll_until_reconciled(
    base_url: str,
    api_key: str,
    customer_external_id: str,
    external_subscription_id: str,
    expected: ExpectedUsage,
    interval: float = 2.0,
    timeout: float = 300.0,
    sleep=time.sleep,
    monotonic=time.monotonic,
) -> PollOutcome:
    """Poll current_usage every ``interval`` seconds until exact or timeout.

    A timeout is an honest ``reconciled=False`` (the caller records it as an
    unresolved finding), never a fabricated pass.
    """
    started = monotonic()
    deadline = started + timeout
    attempts = 0
    timeline = []
    report = ReconcileReport(False, ("customer_usage",), ("no reading yet",))
    first_exact_at = None
    reconciled = False
    while True:
        attempts += 1
        at = _utc_now()
        result = get_current_usage(base_url, api_key, customer_external_id, external_subscription_id)
        usage = result.body.get("customer_usage") if isinstance(result.body, dict) else None
        report = reconcile(usage, expected)
        timeline.append(
            {
                "attempt": attempts,
                "at": at,
                "http_status": result.status,
                "ok": report.ok,
                "diffs": list(report.diffs),
            }
        )
        if report.ok:
            reconciled = True
            first_exact_at = at
            break
        if monotonic() >= deadline:
            break
        sleep(interval)
    elapsed = monotonic() - started
    return PollOutcome(
        reconciled=reconciled,
        attempts=attempts,
        elapsed_seconds=elapsed,
        first_exact_at=first_exact_at,
        final_report=report,
        timeline=tuple(timeline),
    )


def dumps_sanitized(payload, api_key: str) -> str:
    """Serialize JSON with any occurrence of the API key redacted."""
    text = json.dumps(payload, sort_keys=True, default=str)
    if api_key:
        text = text.replace(api_key, REDACTED)
    return text
