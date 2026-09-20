#!/usr/bin/env python3
"""Probe the real Lago API Customer contract: create one synthetic customer,
always delete it again, and emit a sanitized JSON verdict.

The report never contains the API key or raw response bodies. It records the
release identity from deploy/lago/images.lock.json (not live API text), the
synthetic customer id, per-step outcomes with HTTP statuses, and timestamps.
Exit codes: 0 pass, 1 fail, 2 blocked-env (missing API key).
"""

import argparse
import json
import os
import sys
import uuid
from datetime import datetime, timezone
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import quote
from urllib.request import ProxyHandler, Request, build_opener

try:  # Importable both as a package member and as a plain script.
    from deploy.lago.health import load_release_identity
except ImportError:  # script mode: sys.path[0] is deploy/lago
    from health import load_release_identity


DEPLOY_DIR = Path(__file__).resolve().parent
LOCK_PATH = DEPLOY_DIR / "images.lock.json"
DEFAULT_API_PORT = "48889"
PROBE_ID_PREFIX = "weknora-t01-probe-"
REQUEST_TIMEOUT_SECONDS = 10
EXIT_CODES = {"pass": 0, "fail": 1, "blocked-env": 2}

# The probe origin is always local (127.0.0.1): requests, and above all the
# Bearer credential, must never be routed through a system- or env-configured
# proxy, so every call goes through this proxy-less opener.
_OPENER = build_opener(ProxyHandler({}))


def _utc_now():
    return datetime.now(timezone.utc).isoformat()


class _StepFailure(Exception):
    """A probe step failed with a short, sanitized reason for the report."""

    def __init__(self, step, outcome, http_status, reason):
        super().__init__(reason)
        self.step = step
        self.outcome = outcome
        self.http_status = http_status
        self.reason = reason


def _json_call(method, url, api_key=None, payload=None, parse=True):
    """Issue one JSON HTTP call; return (http_status, parsed_body_or_None).

    Raises HTTPError for non-2xx statuses, OSError (incl. URLError) for
    transport failures, and ValueError for unparseable response bodies.
    """
    headers = {"Accept": "application/json"}
    body = None
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    request = Request(url, data=body, headers=headers, method=method)
    with _OPENER.open(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
        status = response.status
        raw = response.read()
    if not parse or not raw:
        return status, None
    return status, json.loads(raw.decode("utf-8"))


def _check_health(base_url):
    """GET /health on the configured origin; return the HTTP status."""
    try:
        status, _ = _json_call("GET", f"{base_url}/health", parse=False)
    except HTTPError as error:
        raise _StepFailure(
            "health", "unhealthy", error.code, f"health check returned HTTP {error.code}"
        ) from None
    except OSError:
        raise _StepFailure("health", "error", None, "health check request failed") from None
    return status


def _create_customer(base_url, api_key, external_id, name):
    """POST /api/v1/customers; return (http_status, echoed external_id)."""
    payload = {"customer": {"external_id": external_id, "name": name}}
    try:
        status, parsed = _json_call(
            "POST", f"{base_url}/api/v1/customers", api_key=api_key, payload=payload
        )
    except HTTPError as error:
        raise _StepFailure(
            "create", "rejected", error.code, f"create returned HTTP {error.code}"
        ) from None
    except OSError:
        raise _StepFailure("create", "error", None, "create request failed") from None
    except ValueError:
        raise _StepFailure(
            "create", "malformed", None, "create response was not valid JSON"
        ) from None
    customer = parsed.get("customer") if isinstance(parsed, dict) else None
    echoed = customer.get("external_id") if isinstance(customer, dict) else None
    if not isinstance(echoed, str) or not echoed:
        raise _StepFailure(
            "create", "malformed", status, "create response missing customer.external_id"
        ) from None
    return status, echoed


def _delete_customer(base_url, api_key, external_id):
    """DELETE the customer by external id; classify the outcome, never raise.

    A 404 is a provable miss — a clean end state ("absent"), not a failure —
    which is what makes the lenient cleanup after a failed create safe: the
    requested id is deleted whether or not the server persisted it.
    """
    url = f"{base_url}/api/v1/customers/{quote(external_id, safe='')}"
    try:
        status, _ = _json_call("DELETE", url, api_key=api_key, parse=False)
    except HTTPError as error:
        if error.code == 404:
            return {"outcome": "absent", "http_status": 404}
        return {"outcome": "failed", "http_status": error.code}
    except OSError:
        return {"outcome": "failed", "http_status": None}
    return {"outcome": "deleted", "http_status": status}


class ProbeResult:
    """Sanitized, JSON-serializable outcome of one probe run."""

    def __init__(self, report):
        self._report = report

    @property
    def status(self):
        return self._report["status"]

    def serialized(self):
        return json.dumps(self._report, sort_keys=True)


def _synthetic_customer():
    """A unique, non-personal synthetic customer identity for one probe run."""
    external_id = f"{PROBE_ID_PREFIX}{uuid.uuid4()}"
    name = f"WeKnora Contract Probe {external_id[-12:]}"
    return external_id, name


def run_probe(base_url, api_key, release_identity=None, lock_path=LOCK_PATH):
    """Run the health/create/delete probe; return a sanitized ProbeResult."""
    started_at = _utc_now()
    base_url = str(base_url).rstrip("/")
    external_id, name = _synthetic_customer()

    report = {
        "probe": "lago-customer-contract",
        "status": "pass",
        "base_url": base_url,
        "release": None,
        "customer": {
            "requested_external_id": external_id,
            "name": name,
            "created_external_id": None,
        },
        "health": {"outcome": "skipped", "http_status": None},
        "create": {"outcome": "skipped", "http_status": None},
        "cleanup": {"outcome": "not-attempted", "http_status": None},
        "error": None,
        "started_at": started_at,
        "finished_at": None,
    }

    def finish():
        report["finished_at"] = _utc_now()
        return ProbeResult(report)

    try:
        identity = release_identity or load_release_identity(lock_path)
    except (OSError, KeyError, ValueError):
        report["status"] = "fail"
        report["error"] = "invalid image lock"
        return finish()
    report["release"] = {"release": identity["release"], "images": identity["images"]}

    if not api_key:
        report["status"] = "blocked-env"
        report["error"] = "missing API key (set LAGO_API_KEY in the environment)"
        return finish()

    created_external_id = None
    create_attempted = False
    failure = None
    try:
        report["health"] = {"outcome": "ok", "http_status": _check_health(base_url)}
        create_attempted = True
        create_status, created_external_id = _create_customer(
            base_url, api_key, external_id, name
        )
        report["create"] = {"outcome": "created", "http_status": create_status}
    except _StepFailure as step_failure:
        failure = step_failure
        report[step_failure.step] = {
            "outcome": step_failure.outcome,
            "http_status": step_failure.http_status,
        }
        report["error"] = step_failure.reason
    finally:
        # Cleanup always runs once a customer was created, whatever else
        # failed. #73 carryover: after a create step that was ATTEMPTED but
        # failed (rejected/error/malformed), the server may still have
        # persisted the synthetic customer (persisted-but-response-lost), so
        # the probe lenient-deletes the REQUESTED id too — a 404 is the
        # provable nothing-was-created case and counts as a clean end state.
        # A health failure (create never attempted) stays delete-free.
        if created_external_id is not None:
            report["customer"]["created_external_id"] = created_external_id
            report["cleanup"] = _delete_customer(base_url, api_key, created_external_id)
        elif create_attempted:
            report["cleanup"] = _delete_customer(base_url, api_key, external_id)

    if failure is not None:
        report["status"] = "fail"
    elif report["cleanup"]["outcome"] == "failed":
        report["status"] = "fail"
        cleanup_status = report["cleanup"]["http_status"]
        report["error"] = (
            f"cleanup failed (HTTP {cleanup_status})" if cleanup_status is not None
            else "cleanup request failed"
        )
    return finish()


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output", metavar="PATH", help="write the sanitized JSON report to this path"
    )
    args = parser.parse_args(argv)

    port = os.environ.get("LAGO_API_PORT", DEFAULT_API_PORT)
    base_url = f"http://127.0.0.1:{port}"
    result = run_probe(base_url, os.environ.get("LAGO_API_KEY", ""))
    text = result.serialized()
    if args.output:
        Path(args.output).write_text(text + "\n", encoding="utf-8")
    print(text)
    return EXIT_CODES[result.status]


if __name__ == "__main__":
    sys.exit(main())
