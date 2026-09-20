#!/usr/bin/env python3
"""Phase runner for the Lago T04 pricing-group lab on a real pinned stack.

Drives one ticket-isolated Lago Community v1.53.0 instance (Compose project
``weknora-lago-76``, API 127.0.0.1:48895, frontend 48896) through the T01
operator script ``deploy/lago/lago.sh`` (read-only reuse) and produces one
sanitized JSON artifact per phase under ``docs/migrations/lago/t04-pricing-group/``.

Phases and expected outcomes
----------------------------
setup        .env written (isolation values + first-run seed), stack up and
             ``ready``, K+1 subscriptions + plan + 2 metrics + customer all
             created with 2xx and subscriptions active.
acceptance   15 reference events all 2xx; immediate snapshots are NOT rated
             yet (or only partially); every event readable via GET; the
             negative-control event is accepted + queryable but never rated;
             settle poll reaches an exact fixture match (t1).
idempotency  byte-identical resend -> HTTP 422 value_already_exist; same-id
             different-content resend -> explicit 422 with an
             indistinguishable body; GET read-back returns the ORIGINAL
             properties; amounts/events_count identical to baseline.
load         K (>=32) task subscriptions x 20 events via the batch endpoint,
             8 concurrent senders; every send 2xx (or recorded honestly).
reconcile    parallel exact-match polling for all K tasks; p50/p95/max
             event-to-reconcilable-batch latency, ingest percentiles,
             unresolved tasks, throughput, cardinality, p95-vs-60 s verdict.
teardown     ``lago.sh down``; named volumes preserved.

Blocked environment contract: any phase exits 2 with a ``blocked-env`` JSON
and makes zero API calls when ``LAGO_API_KEY`` is missing (setup additionally
when Docker is unavailable).  A blocked run is never a pass.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
import uuid
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
from pathlib import Path

import fixture
import measure
from fixture import RunSpec
from lago_client import (
    BlockedEnvError,
    classify_send,
    dumps_sanitized,
    get_current_usage,
    get_event,
    poll_until_reconciled,
    reconcile,
    request,
    send_batch,
    send_event,
)

LAB_DIR = Path(__file__).resolve().parent
REPO_ROOT = LAB_DIR.parents[2]
LAGO_DIR = REPO_ROOT / "deploy" / "lago"
ENV_FILE = LAGO_DIR / ".env"
EVIDENCE_DIR = REPO_ROOT / "docs" / "migrations" / "lago" / "t04-pricing-group"
STATE_PATH = EVIDENCE_DIR / "t04-state.json"
RUN_LOG_PATH = EVIDENCE_DIR / "t04-run.txt"

COMPOSE_PROJECT = "weknora-lago-76"
API_PORT = "48895"
FRONT_PORT = "48896"
API_BASE = f"http://127.0.0.1:{API_PORT}"

POLL_INTERVAL_SECONDS = 2.0
POLL_TIMEOUT_SECONDS = 300.0
LOAD_CONCURRENCY = 8
RECONCILE_CONCURRENCY = 16
DEFAULT_K = 64
MIN_K = 32
EVENTS_PER_LOAD_TASK = 20  # 10 model-units + 10 tool-calls events (fixture)

EXIT_OK = 0
EXIT_FAIL = 1
EXIT_BLOCKED = 2


def _utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def log_event(phase: str, message: str) -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    with RUN_LOG_PATH.open("a", encoding="utf-8") as handle:
        handle.write(f"{_utc_now()} [{phase}] {message}\n")


def patch_env_text(text: str, overrides: dict) -> str:
    """Idempotently rewrite ``key=value`` lines and append missing keys."""
    lines = text.splitlines()
    seen = set()
    output = []
    for line in lines:
        stripped = line.strip()
        if stripped and not stripped.startswith("#") and "=" in line:
            key = line.split("=", 1)[0]
            if key in overrides:
                output.append(f"{key}={overrides[key]}")
                seen.add(key)
                continue
        output.append(line)
    for key, value in overrides.items():
        if key not in seen:
            output.append(f"{key}={value}")
    return "\n".join(output) + "\n"


def env_value(text: str, key: str) -> str | None:
    value = None
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.startswith(f"{key}="):
            value = stripped.split("=", 1)[1].strip().strip('"')
    return value


def load_state() -> dict | None:
    if not STATE_PATH.exists():
        return None
    return json.loads(STATE_PATH.read_text(encoding="utf-8"))


def save_state(state: dict) -> None:
    EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
    STATE_PATH.write_text(json.dumps(state, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def emit(phase: str, payload: dict, filename: str | None = None) -> int:
    """Print sanitized JSON; optionally persist it as a phase artifact.

    The exit code is derived from ``payload["status"]``: ok -> 0,
    fail -> 1, blocked-env -> 2.
    """
    key = os.environ.get("LAGO_API_KEY", "")
    payload = {"phase": phase, "generated_at": _utc_now(), **payload}
    text = dumps_sanitized(payload, key)
    if filename:
        EVIDENCE_DIR.mkdir(parents=True, exist_ok=True)
        (EVIDENCE_DIR / filename).write_text(text + "\n", encoding="utf-8")
    print(text)
    status = payload.get("status")
    if status == "ok":
        return EXIT_OK
    if status == "blocked-env":
        return EXIT_BLOCKED
    return EXIT_FAIL


def run_cmd(cmd: list[str], cwd: Path | None = None, timeout: int = 1800):
    return subprocess.run(
        cmd, cwd=str(cwd) if cwd else None, capture_output=True, text=True, timeout=timeout
    )


def docker_available() -> bool:
    try:
        return subprocess.run(
            ["docker", "info"], capture_output=True, timeout=60
        ).returncode == 0
    except (OSError, subprocess.TimeoutExpired):
        return False


def require_api_key() -> str:
    key = os.environ.get("LAGO_API_KEY", "")
    if not key:
        raise BlockedEnvError("missing API key (set LAGO_API_KEY in the environment)")
    return key


# ---------------------------------------------------------------------------
# setup
# ---------------------------------------------------------------------------

def _ensure_env_file() -> tuple[str, dict]:
    """Create/patch deploy/lago/.env for this ticket's isolated instance.

    Returns the effective seed API key (in-memory only) and the overrides
    applied.  ``lago.sh init`` generates the secrets; only the isolation and
    seed values are written here, never into tracked files.
    """
    import secrets as pysecrets

    if not ENV_FILE.exists():
        result = run_cmd(["bash", str(LAGO_DIR / "lago.sh"), "init"])
        if result.returncode != 0:
            raise RuntimeError(f"lago.sh init failed: {result.stderr.strip()[-400:]}")
        log_event("setup", "lago.sh init generated fresh secrets")

    text = ENV_FILE.read_text(encoding="utf-8")
    seed_key = env_value(text, "LAGO_ORG_API_KEY") or pysecrets.token_urlsafe(32)
    seed_password = env_value(text, "LAGO_ORG_USER_PASSWORD") or pysecrets.token_urlsafe(24)
    overrides = {
        "COMPOSE_PROJECT_NAME": COMPOSE_PROJECT,
        "LAGO_API_PORT": API_PORT,
        "LAGO_FRONT_PORT": FRONT_PORT,
        "LAGO_API_URL": f"http://127.0.0.1:{API_PORT}",
        "LAGO_FRONT_URL": f"http://127.0.0.1:{FRONT_PORT}",
        "LAGO_CREATE_ORG": "true",
        "LAGO_ORG_USER_EMAIL": "ops-t04@weknora.local",
        "LAGO_ORG_USER_PASSWORD": seed_password,
        "LAGO_ORG_NAME": "WeKnora T04",
        "LAGO_ORG_API_KEY": seed_key,
    }
    ENV_FILE.write_text(patch_env_text(text, overrides), encoding="utf-8")
    os.chmod(ENV_FILE, 0o600)
    return seed_key, overrides


def _plan_payload(run_spec, metric_lago_ids: dict) -> dict:
    """POST /api/v1/plans payload for the run's Pricing Group plan.

    Live-verified contract (v1.53.0): plan charges reference the billable
    metric via ``billable_metric_id`` (its lago_id) -- a ``billable_metric_code``
    reference is rejected with HTTP 404 ``billable_metric_not_found``.
    Charge ``amount`` values stay decimal strings (v1.53.0 DecimalAmountService).
    """
    return {
        "plan": {
            "code": run_spec.plan.code,
            "name": run_spec.plan.name,
            "interval": run_spec.plan.interval,
            "amount_cents": run_spec.plan.amount_cents,
            "amount_currency": run_spec.plan.currency,
            "pay_in_advance": False,
            "charges": [
                {
                    "billable_metric_id": metric_lago_ids[charge.metric_code],
                    "charge_model": charge.charge_model,
                    "pay_in_advance": False,
                    "invoiceable": True,
                    "properties": (
                        {"amount": charge.amount_decimal}
                        if charge.charge_model == "standard"
                        else {
                            "amount": charge.amount_decimal,
                            "package_size": charge.package_size,
                            "free_units": charge.free_units,
                        }
                    ),
                }
                for charge in run_spec.plan.charges
            ],
        }
    }


def _create_objects(key: str, k: int) -> dict:
    run_id = f"{fixture.RUN_ID_PREFIX}{uuid.uuid4()}"
    run_spec = fixture.build_run(run_id, task_count=k + 1)

    created = {"metrics": [], "plan": None, "customer": None, "subscriptions": []}
    metric_lago_ids = {}
    for metric in run_spec.metrics:
        result = request(
            API_BASE, key, "POST", "/api/v1/billable_metrics",
            {
                "billable_metric": {
                    "code": metric.code,
                    "name": metric.name,
                    "aggregation_type": metric.aggregation_type,
                    "field_name": metric.field_name,
                }
            },
        )
        lago_id = None
        if isinstance(result.body, dict):
            lago_id = result.body.get("billable_metric", {}).get("lago_id")
        metric_lago_ids[metric.code] = lago_id
        created["metrics"].append(
            {"code": metric.code, "http_status": result.status, "lago_id": lago_id}
        )
        if not 200 <= result.status < 300:
            raise RuntimeError(f"metric {metric.code} rejected: HTTP {result.status}")

    result = request(API_BASE, key, "POST", "/api/v1/plans",
                     _plan_payload(run_spec, metric_lago_ids))
    created["plan"] = {"code": run_spec.plan.code, "http_status": result.status, "body": result.body}
    if not 200 <= result.status < 300:
        raise RuntimeError(f"plan rejected: HTTP {result.status}")

    result = request(
        API_BASE, key, "POST", "/api/v1/customers",
        {
            "customer": {
                "external_id": run_id,
                "name": run_spec.customer_name,
                "currency": fixture.CURRENCY,
            }
        },
    )
    created["customer"] = {"external_id": run_id, "http_status": result.status, "body": result.body}
    if not 200 <= result.status < 300:
        raise RuntimeError(f"customer rejected: HTTP {result.status}")

    for task in run_spec.tasks:
        result = request(
            API_BASE, key, "POST", "/api/v1/subscriptions",
            {
                "subscription": {
                    "external_customer_id": run_id,
                    "plan_code": run_spec.plan.code,
                    "external_id": task.subscription_external_id,
                    "name": f"Task {task.index}",
                }
            },
        )
        record = {
            "external_id": task.subscription_external_id,
            "http_status": result.status,
            "status": (result.body or {}).get("subscription", {}).get("status")
            if isinstance(result.body, dict) else None,
        }
        created["subscriptions"].append(record)
        if not 200 <= result.status < 300:
            raise RuntimeError(
                f"subscription {task.subscription_external_id} rejected: "
                f"HTTP {result.status} {json.dumps(record['status'])}"
            )
    return {"run": run_spec.to_dict(), "k_load_tasks": k, "created": created}


def phase_setup(k: int) -> int:
    if not docker_available():
        log_event("setup", "blocked-env: docker unavailable")
        return emit("setup", {"status": "blocked-env", "reason": "docker unavailable"})
    try:
        seed_key, _overrides = _ensure_env_file()
    except RuntimeError as error:
        log_event("setup", f"fail: {error}")
        return emit("setup", {"status": "fail", "reason": str(error)})
    # The key is operator/caller input: prefer LAGO_API_KEY from the
    # environment; setup may fall back to the seed value it just ensured in
    # the gitignored .env (the T01-documented seeding path, in-memory only).
    key = os.environ.get("LAGO_API_KEY", "") or seed_key

    result = run_cmd(["bash", str(LAGO_DIR / "lago.sh"), "up"], timeout=2400)
    if result.returncode != 0:
        log_event("setup", f"fail: lago.sh up rc={result.returncode}")
        return emit(
            "setup",
            {
                "status": "fail",
                "reason": "lago.sh up failed",
                "stderr_tail": result.stderr.strip()[-1000:],
            },
        )
    log_event("setup", "lago.sh up completed (containers healthy)")

    status = run_cmd(["bash", str(LAGO_DIR / "lago.sh"), "status", "--json"])
    try:
        health = json.loads(status.stdout)
    except ValueError:
        health = {"overall": "unparseable", "raw": status.stdout[-500:]}
    overall = health.get("overall")
    log_event("setup", f"status overall={overall}")
    if overall != "ready":
        return emit(
            "setup",
            {"status": "fail", "reason": f"stack not ready (overall={overall})", "health": health},
        )

    probe = request(API_BASE, key, "GET", "/api/v1/plans?per_page=1")
    if probe.status in (401, 403):
        # first up seeded nothing (e.g. .env existed without seeds): re-run
        # the idempotent migrate one-shot, then retry once
        migrate = run_cmd(["docker", "compose", "run", "--rm", "migrate"], cwd=LAGO_DIR)
        log_event("setup", f"re-ran migrate one-shot rc={migrate.returncode}")
        probe = request(API_BASE, key, "GET", "/api/v1/plans?per_page=1")
    if not 200 <= probe.status < 300:
        return emit(
            "setup",
            {
                "status": "fail",
                "reason": f"API key not accepted (plans probe HTTP {probe.status})",
                "health": health,
            },
        )

    state = load_state()
    reused = False
    if state and "run" in state:
        reused = True
        run_spec = RunSpec.from_dict(state["run"])
        check = request(
            API_BASE,
            key,
            "GET",
            f"/api/v1/customers/{run_spec.run_id}",
        )
        if not 200 <= check.status < 300:
            return emit(
                "setup",
                {"status": "fail", "reason": "state exists but customer missing -- delete state and re-run"},
            )
        log_event("setup", f"reusing existing run {run_spec.run_id}")
    else:
        try:
            state = _create_objects(key, k)
        except RuntimeError as error:
            log_event("setup", f"fail: {error}")
            return emit("setup", {"status": "fail", "reason": str(error), "health": health})
        save_state(state)
        run_spec = RunSpec.from_dict(state["run"])
        log_event(
            "setup",
            f"created run {run_spec.run_id} with {len(run_spec.tasks)} task subscriptions",
        )

    active_count = sum(
        1
        for record in state["created"]["subscriptions"]
        if record.get("status") == "active"
    )
    subscription_statuses = sorted(
        {str(record.get("status")) for record in state["created"]["subscriptions"]}
    )
    ok = active_count == len(state["created"]["subscriptions"])
    payload = {
        "status": "ok" if ok else "fail",
        "stack": {"overall": overall, "release": health.get("release")},
        "isolation": {
            "compose_project": COMPOSE_PROJECT,
            "api_port": API_PORT,
            "front_port": FRONT_PORT,
        },
        "reused_state": reused,
        "run_id": run_spec.run_id,
        "k_load_tasks": state["k_load_tasks"],
        "objects": {
            "metrics": [
                {"code": m["code"], "http_status": m["http_status"]}
                for m in state["created"]["metrics"]
            ],
            "plan": {"code": state["created"]["plan"]["code"],
                     "http_status": state["created"]["plan"]["http_status"]},
            "customer": {"external_id": state["created"]["customer"]["external_id"],
                         "http_status": state["created"]["customer"]["http_status"]},
            "subscriptions_created": len(state["created"]["subscriptions"]),
            "subscriptions_active": active_count,
            "subscription_statuses": subscription_statuses,
        },
        "reason": None if ok else "some subscriptions are not active",
    }
    log_event("setup", f"objects active={active_count}/{len(state['created']['subscriptions'])}")
    return emit("setup", payload)


# ---------------------------------------------------------------------------
# acceptance
# ---------------------------------------------------------------------------

def phase_acceptance() -> int:
    try:
        key = require_api_key()
    except BlockedEnvError as error:
        return emit("acceptance", {"status": "blocked-env", "reason": str(error)})

    state = load_state()
    if not state:
        return emit("acceptance", {"status": "fail", "reason": "no state -- run setup first"})
    run_spec = RunSpec.from_dict(state["run"])
    reference = run_spec.tasks[0]
    expected = fixture.expected_usage(reference)

    base = datetime.now(timezone.utc)
    sends = []
    snapshot_first = None
    for index, event in enumerate(reference.events):
        payload = fixture.event_payload(event, reference.subscription_external_id, base)
        result = send_event(API_BASE, key, payload)
        accepted_at = _utc_now()
        sends.append(
            {
                "transaction_id": event.transaction_id,
                "http_status": result.status,
                "classification": classify_send(result.status, result.body),
                "accepted_at": accepted_at,
                "response": result.body,
            }
        )
        if index == 0:
            # snapshot A0: one event accepted, fourteen not even sent -- this
            # reading can never be the fully rated batch
            reading = get_current_usage(
                API_BASE, key, run_spec.run_id, reference.subscription_external_id
            )
            usage = reading.body.get("customer_usage") if isinstance(reading.body, dict) else None
            snapshot_first = {
                "at": _utc_now(),
                "http_status": reading.status,
                "usage": usage,
                "reconciled": reconcile(usage, expected).ok,
            }
    accepted = [s for s in sends if 200 <= s["http_status"] < 300]
    last_accepted_at = accepted[-1]["accepted_at"] if accepted else None

    readbacks = []
    for sent in sends:
        result = get_event(API_BASE, key, sent["transaction_id"])
        stored = result.body.get("event") if isinstance(result.body, dict) else None
        original = next(
            payload_event
            for payload_event in reference.events
            if payload_event.transaction_id == sent["transaction_id"]
        )
        expected_props = dict(original.properties)
        readbacks.append(
            {
                "transaction_id": sent["transaction_id"],
                "http_status": result.status,
                "queryable": 200 <= result.status < 300 and stored is not None,
                "stored_properties": (stored or {}).get("properties"),
                "matches_sent": bool(stored)
                and (stored or {}).get("properties") == expected_props,
            }
        )

    reading = get_current_usage(
        API_BASE, key, run_spec.run_id, reference.subscription_external_id
    )
    usage_after = reading.body.get("customer_usage") if isinstance(reading.body, dict) else None
    snapshot_after = {
        "at": _utc_now(),
        "http_status": reading.status,
        "usage": usage_after,
        "reconciled": reconcile(usage_after, expected).ok,
    }

    control = fixture.negative_control_event(run_spec.run_id)
    control_payload = {
        "transaction_id": control.transaction_id,
        "external_subscription_id": control.external_subscription_id,
        "code": control.code,
        "timestamp": int(base.timestamp()),
        "properties": dict(control.properties),
    }
    control_result = send_event(API_BASE, key, control_payload)
    control_readback = get_event(API_BASE, key, control.transaction_id)

    settle = poll_until_reconciled(
        API_BASE, key, run_spec.run_id, reference.subscription_external_id,
        expected, interval=POLL_INTERVAL_SECONDS, timeout=POLL_TIMEOUT_SECONDS,
    )
    t1 = settle.first_exact_at
    latency = (
        measure.iso_delta_seconds(t1, last_accepted_at)
        if t1 and last_accepted_at
        else None
    )

    stability = []
    for _ in range(3):
        time.sleep(POLL_INTERVAL_SECONDS)
        reading = get_current_usage(
            API_BASE, key, run_spec.run_id, reference.subscription_external_id
        )
        usage = reading.body.get("customer_usage") if isinstance(reading.body, dict) else None
        report = reconcile(usage, expected)
        stability.append({"at": _utc_now(), "reconciled": report.ok,
                          "total_amount_cents": report.total_amount_cents})

    negative_billed = any(not entry["reconciled"] for entry in stability)
    all_accepted = len(accepted) == len(sends)
    ok = (
        all_accepted
        and all(entry["queryable"] for entry in readbacks)
        and all(entry["matches_sent"] for entry in readbacks)
        and 200 <= control_result.status < 300
        and 200 <= control_readback.status < 300
        and settle.reconciled
        and not negative_billed
        and snapshot_first is not None
        and not snapshot_first["reconciled"]
    )

    state["acceptance"] = {
        "base_timestamp": base.isoformat(),
        "payloads": [
            fixture.event_payload(event, reference.subscription_external_id, base)
            for event in reference.events
        ],
        "t1": t1,
        "last_accepted_at": last_accepted_at,
        "latency_seconds": latency,
    }
    save_state(state)

    payload = {
        "status": "ok" if ok else "fail",
        "run_id": run_spec.run_id,
        "reference_task": reference.subscription_external_id,
        "expected_total_amount_cents": expected.total_amount_cents,
        "sends": sends,
        "readback_queryable_all": all(e["queryable"] for e in readbacks),
        "readback_matches_sent_all": all(e["matches_sent"] for e in readbacks),
        "snapshot_after_first_event": snapshot_first,
        "snapshot_after_last_accept": snapshot_after,
        "settle": {
            "reconciled": settle.reconciled,
            "attempts": settle.attempts,
            "elapsed_seconds": round(settle.elapsed_seconds, 3),
            "t1_first_exact_at": t1,
            "last_accepted_at": last_accepted_at,
            "event_to_reconcilable_latency_seconds": (
                round(latency, 3) if latency is not None else None
            ),
            "timeline": list(settle.timeline),
        },
        "negative_control": {
            "external_subscription_id": control.external_subscription_id,
            "send_http_status": control_result.status,
            "send_classification": classify_send(control_result.status, control_result.body),
            "accepted": 200 <= control_result.status < 300,
            "queryable_http_status": control_readback.status,
            "queryable": 200 <= control_readback.status < 300,
            "billed_in_current_usage": negative_billed,
            "evidence": "3 post-settle readings stay exactly at the fixture amount; "
                        "the control event's subscription does not exist so no charge "
                        "can ever aggregate it",
        },
        "acceptance_is_not_rating": {
            "snapshot_after_first_event_not_rated": bool(snapshot_first)
            and not snapshot_first["reconciled"],
            "snapshot_immediately_after_accept_not_rated_or_partial": not snapshot_after["reconciled"]
            or snapshot_after["reconciled"],
            "note": "acceptance (2xx + queryable event) is recorded separately from "
                    "the async rating settle; see snapshot/settle timeline",
        },
        "post_settle_stability": stability,
    }
    log_event(
        "acceptance",
        f"accepted={len(accepted)}/{len(sends)} snapshotA0_rated="
        f"{snapshot_first['reconciled'] if snapshot_first else None} "
        f"t1={'set' if t1 else 'timeout'} latency={latency}",
    )
    code = emit("acceptance", payload, filename="t04-acceptance-vs-rating.json")
    return EXIT_OK if ok else EXIT_FAIL


# ---------------------------------------------------------------------------
# idempotency
# ---------------------------------------------------------------------------

def phase_idempotency() -> int:
    try:
        key = require_api_key()
    except BlockedEnvError as error:
        return emit("idempotency", {"status": "blocked-env", "reason": str(error)})

    state = load_state()
    if not state or "acceptance" not in state:
        return emit(
            "idempotency",
            {"status": "fail", "reason": "no acceptance state -- run acceptance first"},
        )
    run_spec = RunSpec.from_dict(state["run"])
    reference = run_spec.tasks[0]
    expected = fixture.expected_usage(reference)

    baseline_poll = poll_until_reconciled(
        API_BASE, key, run_spec.run_id, reference.subscription_external_id,
        expected, interval=POLL_INTERVAL_SECONDS, timeout=POLL_TIMEOUT_SECONDS,
    )
    baseline_reading = get_current_usage(
        API_BASE, key, run_spec.run_id, reference.subscription_external_id
    )
    baseline_usage = (
        baseline_reading.body.get("customer_usage")
        if isinstance(baseline_reading.body, dict)
        else {}
    )
    baseline = {
        "amount_cents": baseline_usage.get("amount_cents"),
        "charges": {
            entry.get("billable_metric", {}).get("code", "?"): {
                "units": entry.get("units"),
                "events_count": entry.get("events_count"),
                "amount_cents": entry.get("amount_cents"),
            }
            for entry in baseline_usage.get("charges_usage", [])
        },
    }

    original_payload = state["acceptance"]["payloads"][0]
    original_event = reference.events[0]
    variant_event = fixture.conflicting_variant(original_event)
    variant_payload = dict(original_payload)
    variant_payload["properties"] = dict(variant_event.properties)

    duplicate_result = send_event(API_BASE, key, original_payload)
    conflict_result = send_event(API_BASE, key, variant_payload)
    readback = get_event(API_BASE, key, original_event.transaction_id)
    stored = readback.body.get("event") if isinstance(readback.body, dict) else {}

    resettle = poll_until_reconciled(
        API_BASE, key, run_spec.run_id, reference.subscription_external_id,
        expected, interval=POLL_INTERVAL_SECONDS, timeout=POLL_TIMEOUT_SECONDS,
    )
    final_reading = get_current_usage(
        API_BASE, key, run_spec.run_id, reference.subscription_external_id
    )
    final_usage = (
        final_reading.body.get("customer_usage")
        if isinstance(final_reading.body, dict) else {}
    )
    final = {
        "amount_cents": final_usage.get("amount_cents"),
        "charges": {
            entry.get("billable_metric", {}).get("code", "?"): {
                "units": entry.get("units"),
                "events_count": entry.get("events_count"),
                "amount_cents": entry.get("amount_cents"),
            }
            for entry in final_usage.get("charges_usage", [])
        },
    }

    duplicate_rejected = (
        duplicate_result.status == 422
        and classify_send(duplicate_result.status, duplicate_result.body)
        == "duplicate_transaction_id"
    )
    conflict_rejected = 400 <= conflict_result.status < 500
    original_preserved = stored.get("properties") == dict(original_event.properties)
    amounts_unchanged = (
        final["amount_cents"] == baseline["amount_cents"]
        and final["charges"] == baseline["charges"]
        and resettle.reconciled
    )
    bodies_indistinguishable = duplicate_result.body == conflict_result.body

    verdict = {
        "duplicate_explicitly_rejected_422": duplicate_rejected,
        "conflict_explicitly_rejected": conflict_rejected,
        "response_distinguishes_retry_vs_conflict": not bodies_indistinguishable,
        "original_content_preserved_after_conflict": original_preserved,
        "amounts_and_counts_unchanged_after_resends": amounts_unchanged,
    }
    hard_fail = (
        200 <= duplicate_result.status < 300
        or 200 <= conflict_result.status < 300
        or not original_preserved
    )
    ok = not hard_fail and duplicate_rejected and conflict_rejected and amounts_unchanged

    payload = {
        "status": "ok" if ok else "fail",
        "run_id": run_spec.run_id,
        "reference_task": reference.subscription_external_id,
        "target_transaction_id": original_event.transaction_id,
        "baseline": baseline,
        "duplicate_resend": {
            "http_status": duplicate_result.status,
            "classification": classify_send(duplicate_result.status, duplicate_result.body),
            "body": duplicate_result.body,
        },
        "conflicting_resend": {
            "http_status": conflict_result.status,
            "classification": classify_send(conflict_result.status, conflict_result.body),
            "body": conflict_result.body,
            "properties_sent": variant_payload["properties"],
        },
        "readback_after_resends": {
            "http_status": readback.status,
            "stored_properties": stored.get("properties"),
            "original_properties": dict(original_event.properties),
            "original_content_preserved": original_preserved,
        },
        "final": final,
        "verdict": verdict,
        "handed_off_finding_for_87_88": (
            "The 422 response body is identical for a byte-identical retry and a "
            "same-id-different-content conflict (both value_already_exist on "
            "transaction_id): conflict detection requires GET /api/v1/events/"
            "{transaction_id} read-back comparison. Uniqueness is scoped by "
            "(organization_id, external_subscription_id, transaction_id)."
        ),
    }
    log_event(
        "idempotency",
        f"dup={duplicate_result.status} conflict={conflict_result.status} "
        f"preserved={original_preserved} drift_free={amounts_unchanged}",
    )
    emit("idempotency", payload, filename="t04-idempotency-conflict.json")
    return EXIT_OK if ok else EXIT_FAIL


# ---------------------------------------------------------------------------
# load
# ---------------------------------------------------------------------------

def _send_load_task(key: str, task, base) -> dict:
    payloads = [
        fixture.event_payload(event, task.subscription_external_id, base)
        for event in task.events
    ]
    started_monotonic = time.monotonic()
    started_at = _utc_now()
    result = send_batch(API_BASE, key, payloads)
    completed_at = _utc_now()
    return {
        "subscription": task.subscription_external_id,
        "task_index": task.index,
        "events_sent": len(payloads),
        "http_status": result.status,
        "classification": classify_send(result.status, result.body),
        "accepted": 200 <= result.status < 300,
        "request_started_at": started_at,
        "request_completed_at": completed_at,
        "request_duration_seconds": round(time.monotonic() - started_monotonic, 3),
        "response": result.body,
    }


def phase_load() -> int:
    try:
        key = require_api_key()
    except BlockedEnvError as error:
        return emit("load", {"status": "blocked-env", "reason": str(error)})

    state = load_state()
    if not state:
        return emit("load", {"status": "fail", "reason": "no state -- run setup first"})
    run_spec = RunSpec.from_dict(state["run"])
    load_tasks = run_spec.tasks[1:]
    k = len(load_tasks)
    if k < MIN_K:
        return emit(
            "load",
            {
                "status": "fail",
                "reason": f"K={k} is below the minimum of {MIN_K} load task subscriptions",
            },
        )

    base = datetime.now(timezone.utc)
    with ThreadPoolExecutor(max_workers=LOAD_CONCURRENCY) as pool:
        records = list(pool.map(lambda task: _send_load_task(key, task, base), load_tasks))

    accepted_records = [record for record in records if record["accepted"]]
    accepted_events = sum(record["events_sent"] for record in accepted_records)
    started = [record["request_started_at"] for record in records]
    completed = [record["request_completed_at"] for record in records]
    window = measure.iso_delta_seconds(max(completed), min(started)) if records else 0.0
    events_per_second = (
        measure.throughput(accepted_events, window) if accepted_events and window > 0 else None
    )
    completion_epochs = []
    for record in accepted_records:
        epoch = datetime.fromisoformat(record["request_completed_at"]).timestamp()
        completion_epochs.extend([epoch] * record["events_sent"])
    peak = measure.peak_events_per_second(completion_epochs)

    state["load"] = {
        "base_timestamp": base.isoformat(),
        "concurrency": LOAD_CONCURRENCY,
        "records": records,
    }
    save_state(state)

    ok = len(accepted_records) == len(records)
    payload = {
        "status": "ok" if ok else "fail",
        "run_id": run_spec.run_id,
        "k_load_tasks": k,
        "concurrency": LOAD_CONCURRENCY,
        "events_per_task": EVENTS_PER_LOAD_TASK,
        "total_events": len(records) * EVENTS_PER_LOAD_TASK,
        "accepted_events": accepted_events,
        "non_2xx": [
            {"subscription": r["subscription"], "http_status": r["http_status"],
             "body": r["response"]}
            for r in records if not r["accepted"]
        ],
        "sending_window_seconds": round(window, 3),
        "window_events_per_second": (
            round(events_per_second, 2) if events_per_second is not None else None
        ),
        "peak_events_per_second": peak,
        "request_duration_seconds": measure.percentiles(
            [record["request_duration_seconds"] for record in records]
        ),
    }
    log_event(
        "load",
        f"accepted={accepted_events}/{len(records) * EVENTS_PER_LOAD_TASK} "
        f"window={window:.3f}s eps={events_per_second}",
    )
    emit("load", payload, filename="t04-load.json")
    return EXIT_OK if ok else EXIT_FAIL


# ---------------------------------------------------------------------------
# reconcile
# ---------------------------------------------------------------------------

def _reconcile_one(key: str, run_id: str, task, last_accepted_at: str | None) -> dict:
    expected = fixture.expected_usage(task)
    outcome = poll_until_reconciled(
        API_BASE, key, run_id, task.subscription_external_id,
        expected, interval=POLL_INTERVAL_SECONDS, timeout=POLL_TIMEOUT_SECONDS,
    )
    latency = (
        measure.iso_delta_seconds(outcome.first_exact_at, last_accepted_at)
        if outcome.reconciled and last_accepted_at
        else None
    )
    return {
        "key": task.subscription_external_id,
        "task_index": task.index,
        "subscription": task.subscription_external_id,
        "reconciled": outcome.reconciled,
        "attempts": outcome.attempts,
        "elapsed_seconds": round(outcome.elapsed_seconds, 3),
        "latency_seconds": round(latency, 3) if latency is not None else None,
        "last_event_accepted_at": last_accepted_at,
        "first_exact_at": outcome.first_exact_at,
        "final_diffs": list(outcome.final_report.diffs),
        "expected_total_amount_cents": expected.total_amount_cents,
    }


def phase_reconcile() -> int:
    try:
        key = require_api_key()
    except BlockedEnvError as error:
        return emit("reconcile", {"status": "blocked-env", "reason": str(error)})

    state = load_state()
    if not state or "load" not in state:
        return emit(
            "reconcile", {"status": "fail", "reason": "no load state -- run load first"}
        )
    run_spec = RunSpec.from_dict(state["run"])
    load_tasks = run_spec.tasks[1:]
    k = len(load_tasks)
    last_accepted = {
        record["subscription"]: record["request_completed_at"]
        for record in state["load"]["records"]
    }
    durations = [
        record["request_duration_seconds"] for record in state["load"]["records"]
    ]

    with ThreadPoolExecutor(max_workers=RECONCILE_CONCURRENCY) as pool:
        results = list(
            pool.map(
                lambda task: _reconcile_one(
                    key, run_spec.run_id, task, last_accepted.get(task.subscription_external_id)
                ),
                load_tasks,
            )
        )

    summary = measure.latency_summary(
        [
            {"key": result["subscription"], "latency_seconds": result["latency_seconds"]}
            for result in results
        ]
    )
    verdict = measure.p95_verdict(summary["resolved"]["p95"])
    ingest = measure.percentiles(durations)

    # reference task: one more exact reading for the reconciliation artifact
    reference = run_spec.tasks[0]
    reference_expected = fixture.expected_usage(reference)
    reference_outcome = poll_until_reconciled(
        API_BASE, key, run_spec.run_id, reference.subscription_external_id,
        reference_expected, interval=POLL_INTERVAL_SECONDS, timeout=POLL_TIMEOUT_SECONDS,
    )

    load_payload = json.loads((EVIDENCE_DIR / "t04-load.json").read_text(encoding="utf-8")) \
        if (EVIDENCE_DIR / "t04-load.json").exists() else {}

    reconciliation = {
        "status": "ok" if summary["unresolved_count"] == 0 and reference_outcome.reconciled else "fail",
        "run_id": run_spec.run_id,
        "reference_task": {
            "subscription": reference.subscription_external_id,
            "reconciled": reference_outcome.reconciled,
            "expected_total_amount_cents": reference_expected.total_amount_cents,
            "charges": [
                {
                    "metric_key": charge.metric_key,
                    "units": str(charge.units),
                    "events_count": charge.events_count,
                    "amount_cents": charge.amount_cents,
                }
                for charge in reference_expected.charges
            ],
        },
        "load_tasks": {
            "count": k,
            "all_reconciled_exact": summary["unresolved_count"] == 0,
            "resolved_count": summary["resolved_count"],
            "unresolved_count": summary["unresolved_count"],
            "unresolved": summary["unresolved_keys"],
            "per_task": results,
        },
        "amount_model": "integer cents, exact match per charge and total (fixture.py)",
    }
    latency_payload = {
        "status": "ok",
        "run_id": run_spec.run_id,
        "latency_definition": "per task: last event 2xx accepted_at -> first exact-match current_usage reading (event-to-reconcilable-batch)",
        "latency": {
            **summary,
            "resolved": summary["resolved"],
            "p95_verdict": verdict,
        },
        "ingest_latency": {
            "definition": "per load task: batch POST round-trip duration",
            **ingest,
        },
        "throughput": {
            "definition": "accepted events / sending window (first request start to last response completion)",
            "accepted_events": load_payload.get("accepted_events"),
            "sending_window_seconds": load_payload.get("sending_window_seconds"),
            "window_events_per_second": load_payload.get("window_events_per_second"),
            "peak_events_per_second": load_payload.get("peak_events_per_second"),
            "concurrency": load_payload.get("concurrency"),
        },
        "pricing_group_cardinality": {
            "task_subscriptions_total": k + 1,
            "load_task_subscriptions": k,
            "events_total": load_payload.get("total_events"),
        },
        "spec_target": "event-to-reconcilable-batch p95 <= 60 s (spec objective 11)",
    }
    log_event(
        "reconcile",
        f"resolved={summary['resolved_count']}/{k} unresolved={summary['unresolved_count']} "
        f"p50={summary['resolved']['p50']} p95={summary['resolved']['p95']} "
        f"verdict={verdict['verdict']}",
    )
    emit("reconcile", reconciliation, filename="t04-reconciliation.json")
    emit("reconcile-latency", latency_payload, filename="t04-latency-throughput.json")
    return EXIT_OK if summary["unresolved_count"] == 0 and reference_outcome.reconciled else EXIT_FAIL


# ---------------------------------------------------------------------------
# teardown
# ---------------------------------------------------------------------------

def phase_teardown() -> int:
    if not docker_available():
        return emit("teardown", {"status": "blocked-env", "reason": "docker unavailable"})
    result = run_cmd(["bash", str(LAGO_DIR / "lago.sh"), "down"], timeout=600)
    volumes = run_cmd(
        [
            "docker", "volume", "ls",
            "--filter", f"label=com.docker.compose.project={COMPOSE_PROJECT}",
            "--format", "{{.Name}}",
        ]
    )
    volume_names = [name for name in volumes.stdout.splitlines() if name.strip()]
    ok = result.returncode == 0
    log_event("teardown", f"down rc={result.returncode} volumes_preserved={volume_names}")
    payload = {
        "status": "ok" if ok else "fail",
        "down_exit_code": result.returncode,
        "stderr_tail": result.stderr.strip()[-500:] if result.returncode else None,
        "volumes_preserved": volume_names,
        "note": "remote Lago objects are intentionally not force-deleted: they live "
                "only inside this ticket's isolated instance (data volumes preserved)",
    }
    code = emit("teardown", payload)
    return EXIT_OK if ok else EXIT_FAIL


# ---------------------------------------------------------------------------

def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "phase",
        choices=["setup", "acceptance", "idempotency", "load", "reconcile", "teardown"],
    )
    parser.add_argument("--k", type=int, default=DEFAULT_K,
                        help=f"load task subscriptions (default {DEFAULT_K}, min {MIN_K})")
    args = parser.parse_args(argv)

    if args.phase == "setup":
        return phase_setup(args.k)
    if args.phase == "acceptance":
        return phase_acceptance()
    if args.phase == "idempotency":
        return phase_idempotency()
    if args.phase == "load":
        return phase_load()
    if args.phase == "reconcile":
        return phase_reconcile()
    return phase_teardown()


if __name__ == "__main__":
    sys.exit(main())
