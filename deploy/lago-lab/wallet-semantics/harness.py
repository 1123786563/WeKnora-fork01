#!/usr/bin/env python3
"""Reusable experiment harness for the T03 Lago wallet-semantics evidence lab.

Stdlib only. Provides:

- ``LagoClient``: a proxy-less JSON HTTP client for the pinned Lago API that
  classifies transport failures (``LagoTransportError`` -> blocked-env) apart
  from contract answers (``LagoHttpError`` -> fail) and never leaks the API
  key or raw response bodies into errors.
- ``ExperimentReport``: the sanitized, JSON-serializable outcome of one
  experiment run (status ``pass | fail | blocked-env``, check rows, created
  object ids, cleanup results). Every registered secret is redacted from all
  serialized text.
- ``run_experiment``: the orchestrator -- health gate, experiment body,
  per-object cleanup from ``finally``, and one sanitized JSON evidence file.

The API key is caller-environment input only: it is passed in, registered for
redaction, sent only to the lab origin, and never written anywhere.
"""

import json
import os
import uuid
from datetime import datetime, timezone
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import quote
from urllib.request import ProxyHandler, Request, build_opener

LAB_DIR = Path(__file__).resolve().parent
LOCK_PATH = LAB_DIR.parent.parent / "lago" / "images.lock.json"
DEFAULT_EVIDENCE_DIR = LAB_DIR / "evidence"
DEFAULT_API_PORT = "48893"
RUN_ID_PREFIX = "weknora-t03-"
REQUEST_TIMEOUT_SECONDS = 60
MAX_TEXT_LENGTH = 400  # observations are summaries, never response dumps
OUTCOMES = ("pass", "fail", "blocked-env")

# The lab origin is always loopback: requests, and above all the Bearer
# credential, must never be routed through a configured proxy.
_OPENER = build_opener(ProxyHandler({}))


def _utc_now():
    return datetime.now(timezone.utc).isoformat()


# ---------------------------------------------------------------------------
# Errors


class LagoTransportError(Exception):
    """The lab origin could not be reached (unreachable/timeout) -> blocked-env."""


class LagoHttpError(Exception):
    """The pinned API answered with a non-2xx contract response -> fail."""

    def __init__(self, status, reason):
        super().__init__(f"HTTP {status}: {reason}")
        self.status = status
        self.reason = reason


def sanitize_text(value, secrets=()):
    """Redact secrets and truncate: observations are summaries, never dumps."""
    text = value if isinstance(value, str) else repr(value)
    for secret in secrets:
        if secret:
            text = text.replace(secret, "***REDACTED***")
    if len(text) > MAX_TEXT_LENGTH:
        text = text[:MAX_TEXT_LENGTH] + f"...[truncated {len(text) - MAX_TEXT_LENGTH} chars]"
    return text


def _sanitize(value, secrets):
    if isinstance(value, str):
        return sanitize_text(value, secrets)
    if isinstance(value, dict):
        return {str(key): _sanitize(item, secrets) for key, item in value.items()}
    if isinstance(value, (list, tuple)):
        return [_sanitize(item, secrets) for item in value]
    return value


def check_from(criterion, expected, observed=None, observed_error=None, secrets=()):
    """Build one sanitized check row {criterion, expected, observed, outcome}."""
    if observed_error is not None:
        observed = sanitize_text(observed_error, secrets)
        outcome = "fail"
    else:
        observed = _sanitize(observed, secrets)
        outcome = "pass"
    return {"criterion": criterion, "expected": expected, "observed": observed,
            "outcome": outcome}


def outcome_of(checks, criterion):
    """Return the outcome of one check row (None if the criterion is absent)."""
    for row in checks:
        if row.get("criterion") == criterion:
            return row.get("outcome")
    return None


# ---------------------------------------------------------------------------
# Synthetic identities and fixture payload builders (decimal-string credits)


def new_run_id():
    return f"{RUN_ID_PREFIX}{uuid.uuid4()}"


def new_customer_id():
    return f"{RUN_ID_PREFIX}cust-{uuid.uuid4()}"


def new_wallet_code():
    return f"{RUN_ID_PREFIX}wallet-{uuid.uuid4()}"


def new_batch_key():
    return f"{RUN_ID_PREFIX}batch-{uuid.uuid4()}"


def batch_metadata(batch_key):
    """Wallet-create metadata (object form; wallets accept a dict)."""
    return {"weknora_t03_batch": batch_key}


def batch_metadata_list(batch_key):
    """Wallet-transaction metadata (array form; the transaction API rejects
    a dict with metadata: ["invalid_type"] on v1.53.0)."""
    return [{"key": "weknora_t03_batch", "value": batch_key}]


def customer_payload(external_id, name="WeKnora T03 Lab", currency="USD"):
    return {"customer": {"external_id": external_id, "name": name, "currency": currency}}


def wallet_payload(external_customer_id, name, granted_credits=None, paid_credits=None,
                   rate_amount="1", currency="USD", expiration_at=None, priority=None,
                   recurring_transaction_rules=None, batch_key=None, transaction_batch_key=None):
    """Wallet create payload; credits are decimal strings, never floats."""
    wallet = {
        "external_customer_id": external_customer_id,
        "code": new_wallet_code(),
        "name": name,
        "currency": currency,
        "rate_amount": rate_amount,
    }
    if granted_credits is not None:
        wallet["granted_credits"] = granted_credits
    if paid_credits is not None:
        wallet["paid_credits"] = paid_credits
    if expiration_at is not None:
        wallet["expiration_at"] = expiration_at
    if priority is not None:
        wallet["priority"] = priority
    if recurring_transaction_rules is not None:
        wallet["recurring_transaction_rules"] = recurring_transaction_rules
    if batch_key is not None:
        wallet["metadata"] = batch_metadata(batch_key)
    if transaction_batch_key is not None:
        wallet["transaction_metadata"] = [
            {"key": "weknora_t03_batch", "value": transaction_batch_key},
        ]
    return {"wallet": wallet}


def wallet_grant_payload(wallet_lago_id, granted_credits=None, paid_credits=None,
                         batch_key=None, name=None):
    transaction = {"wallet_id": wallet_lago_id}
    if granted_credits is not None:
        transaction["granted_credits"] = granted_credits
    if paid_credits is not None:
        transaction["paid_credits"] = paid_credits
    if name is not None:
        transaction["name"] = name
    if batch_key is not None:
        transaction["metadata"] = batch_metadata_list(batch_key)
    return {"wallet_transaction": transaction}


def one_off_invoice_payload(external_customer_id, fee_name, unit_amount_cents,
                            units="1", currency="USD"):
    """One-off invoice carrying a single custom fee (the consumption trigger)."""
    return {
        "invoice": {
            "external_customer_id": external_customer_id,
            "currency": currency,
            "fees": [{
                "item": {
                    "name": fee_name,
                    "type": "custom",
                    "unit_amount_cents": unit_amount_cents,
                },
                "units": units,
            }],
        }
    }


# ---------------------------------------------------------------------------
# HTTP client


class LagoClient:
    def __init__(self, base_url, api_key, timeout=REQUEST_TIMEOUT_SECONDS):
        self.base_url = str(base_url).rstrip("/")
        self.api_key = api_key
        self.timeout = timeout

    def request(self, method, path, payload=None):
        """One JSON call. Returns (status, parsed_body_or_None).

        Raises LagoHttpError for non-2xx (sanitized reason, no body dump) and
        LagoTransportError when the origin cannot be reached at all.
        """
        url = f"{self.base_url}{path}"
        headers = {"Accept": "application/json"}
        body = None
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if self.api_key and path.startswith("/api/"):
            headers["Authorization"] = f"Bearer {self.api_key}"
        request = Request(url, data=body, headers=headers, method=method)
        try:
            with _OPENER.open(request, timeout=self.timeout) as response:
                status = response.status
                raw = response.read()
        except HTTPError as error:
            raise LagoHttpError(error.code, self._error_reason(error)) from None
        except OSError as error:  # URLError, connection refused, timeouts
            raise LagoTransportError(sanitize_text(error, (self.api_key,))) from None
        if not raw:
            return status, None
        try:
            return status, json.loads(raw.decode("utf-8"))
        except ValueError:
            return status, None

    @staticmethod
    def _error_reason(error):
        """Short, sanitized reason from an error body -- never a raw dump.

        v1.53.0 renders validation failures as
        ``{"code": "validation_errors", "error_details": {field: [code, ..]}}``,
        so the specific contract code (e.g. ``wallet_limit_reached``) is dug
        out of ``error_details`` first; the top-level code is the fallback.
        """
        reason = ""
        try:
            raw = error.read()
            parsed = json.loads(raw.decode("utf-8"))
            if isinstance(parsed, dict):
                details = parsed.get("error_details")
                if isinstance(details, dict):
                    for value in details.values():
                        if isinstance(value, list) and value and value[0]:
                            reason = str(value[0])
                            break
                        if isinstance(value, str) and value:
                            reason = value
                            break
                if not reason:
                    reason = str(
                        parsed.get("code") or parsed.get("error")
                        or parsed.get("status") or parsed.get("message") or ""
                    )
        except (OSError, ValueError):
            pass
        return sanitize_text(reason or "no error body") if reason else "no error body"

    def get(self, path):
        return self.request("GET", path)

    def post(self, path, payload=None):
        return self.request("POST", path, payload=payload)

    def put(self, path, payload=None):
        return self.request("PUT", path, payload=payload)

    def delete(self, path):
        return self.request("DELETE", path)

    def health(self):
        """GET /health (unauthenticated); returns the HTTP status."""
        return self.request("GET", "/health")[0]


def load_release_identity(lock_path=LOCK_PATH):
    with Path(lock_path).open(encoding="utf-8") as lock_file:
        lock = json.load(lock_file)
    return {"release": lock["release"], "images": lock["images"]}


# ---------------------------------------------------------------------------
# Report model


class ExperimentReport:
    """Sanitized, JSON-serializable outcome of one experiment run."""

    def __init__(self, run_id, api_key="", meta=None):
        self.run_id = run_id
        self._secrets = tuple(s for s in (api_key,) if s)
        self._status = "pass"
        self._error = None
        self.checks = []
        self.created_object_ids = []
        self.cleanup = []
        self.meta = dict(meta or {})
        self.started_at = _utc_now()
        self.finished_at = None

    @property
    def status(self):
        if self._status != "pass":
            return self._status
        outcomes = {row.get("outcome") for row in self.checks}
        if "fail" in outcomes:
            return "fail"
        if outcomes and "blocked-env" in outcomes:
            return "blocked-env"
        return "pass"

    @property
    def error(self):
        return self._error

    def register_secret(self, value):
        if value:
            self._secrets = tuple(set(self._secrets) | {value})

    def mark_fail(self, reason):
        if self._status != "fail":
            self._status = "fail"
            self._error = sanitize_text(reason, self._secrets)

    def mark_blocked_env(self, reason):
        if self._status == "pass":
            self._status = "blocked-env"
            self._error = sanitize_text(reason, self._secrets)

    def add_check(self, criterion, expected, observed=None, outcome="pass",
                  observed_error=None):
        if observed_error is not None:
            observed = sanitize_text(observed_error, self._secrets)
            outcome = "fail"
        if outcome not in OUTCOMES:
            raise ValueError(f"invalid outcome {outcome!r}")
        row = {"criterion": criterion, "expected": expected,
               "observed": _sanitize(observed, self._secrets), "outcome": outcome}
        self.checks.append(row)
        return row

    def record_created(self, kind, lago_id=None, external_id=None):
        entry = {"kind": kind, "lago_id": lago_id, "external_id": external_id}
        self.created_object_ids.append(entry)
        return entry

    def to_dict(self):
        return {
            "run_id": self.run_id,
            "status": self.status,
            "error": self._error,
            "started_at": self.started_at,
            "finished_at": self.finished_at,
            **_sanitize(self.meta, self._secrets),
            "checks": _sanitize(self.checks, self._secrets),
            "created_object_ids": _sanitize(self.created_object_ids, self._secrets),
            "cleanup": _sanitize(self.cleanup, self._secrets),
        }

    def serialized(self):
        return json.dumps(self.to_dict(), sort_keys=True)

    def write_to(self, path):
        self.finished_at = self.finished_at or _utc_now()
        target = Path(path)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(self.serialized() + "\n", encoding="utf-8")


# ---------------------------------------------------------------------------
# Cleanup and orchestration


class Cleanup:
    """Best-effort, customer-scoped cleanup of every recorded object."""

    def __init__(self, client, report):
        self.client = client
        self.report = report

    def run(self):
        # Reverse creation order: wallets terminate before their customer.
        for entry in reversed(self.report.created_object_ids):
            outcome = {"kind": entry["kind"], "target": entry.get("external_id")
                       or entry.get("lago_id"), "outcome": "skipped"}
            try:
                if entry["kind"] == "customer" and entry.get("external_id"):
                    status, _ = self.client.delete(
                        "/api/v1/customers/" + quote(entry["external_id"], safe=""))
                    outcome["outcome"] = "deleted" if status == 200 else f"http-{status}"
                elif entry["kind"] == "wallet" and entry.get("lago_id"):
                    # DELETE /wallets/:id terminates the wallet on v1.53.0.
                    status, _ = self.client.delete("/api/v1/wallets/" + quote(str(entry["lago_id"]), safe=""))
                    outcome["outcome"] = "terminated" if status == 200 else f"http-{status}"
            except LagoHttpError as error:
                outcome["outcome"] = f"http-{error.status}"
            except LagoTransportError:
                outcome["outcome"] = "unreachable"
            self.report.cleanup.append(outcome)


def run_experiment(name, slug, base_url, api_key, experiment_main, evidence_dir=None,
                   meta=None):
    """Run one experiment end-to-end and write its sanitized evidence file.

    ``experiment_main(client, report, cleanup)`` performs the scenario; every
    object it records via ``report.record_created`` is cleaned up from the
    ``finally`` block whatever happens later in the run.
    """
    run_id = new_run_id()
    report = ExperimentReport(run_id=run_id, api_key=api_key)
    report.meta.update({"experiment": name, "base_url": str(base_url)})
    evidence_path = Path(evidence_dir or DEFAULT_EVIDENCE_DIR) / f"{slug}.json"

    try:
        report.meta["release"] = load_release_identity()
    except (OSError, KeyError, ValueError):
        report.mark_fail("invalid image lock")
        report.write_to(evidence_path)
        return report

    if not api_key:
        report.mark_blocked_env("missing API key (set LAGO_API_KEY in the environment)")
        report.write_to(evidence_path)
        return report

    client = LagoClient(base_url, api_key)
    try:
        client.health()
    except LagoTransportError:
        report.mark_blocked_env(
            f"stack unreachable at {base_url}/health (is the lab up?)")
        report.write_to(evidence_path)
        return report
    except LagoHttpError as error:
        report.mark_fail(f"health returned HTTP {error.status}")
        report.write_to(evidence_path)
        return report

    cleanup = Cleanup(client, report)
    try:
        experiment_main(client, report, cleanup)
    except LagoTransportError as error:
        report.mark_blocked_env(f"transport failure mid-run: {error}")
    except LagoHttpError as error:
        report.mark_fail(f"contract error mid-run: HTTP {error.status} {error.reason}")
    finally:
        cleanup.run()

    report.write_to(evidence_path)
    return report


def default_base_url():
    port = os.environ.get("LAGO_API_PORT", DEFAULT_API_PORT)
    return f"http://127.0.0.1:{port}"
