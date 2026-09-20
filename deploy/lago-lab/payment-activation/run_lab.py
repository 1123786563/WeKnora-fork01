#!/usr/bin/env python3
"""Ordered runner for the T02 payment-activation lab experiment.

Reads the lab stack's own ``lab.env`` for the Lago origin and the seeded
operator credentials (API key + login), takes the Stripe **test-mode** key
from the caller environment (``STRIPE_TEST_SECRET_KEY`` or
``STRIPE_SECRET_KEY``; live keys are refused), and executes the Task 2
phases in order:

    setup -> provider_setup -> gate -> manual -> activate -> duplicates
    -> retries -> decline_control -> cleanup (always, from finally)

Outputs one sanitized JSON report per phase plus ``t02-environment.json``
and a sanitized operator timeline ``t02-run.txt`` under ``--output-dir``.
The release identity comes from ``deploy/lago/images.lock.json`` (never from
live API text). Before finishing, every written file is scanned for the
secret values known to this run; any hit is scrubbed and reported, and the
scan must end at zero hits.

Overall verdict: ``pass`` only when every executed phase passed (including
cleanup). Exit codes: 0 pass, 1 fail, 2 blocked-env. No secret value is ever
printed, logged, or written.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime, timezone
from pathlib import Path

LAB_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(LAB_DIR))

import clients  # noqa: E402
import phases  # noqa: E402

REPO_DIR = LAB_DIR.parents[2]
LOCK_PATH = REPO_DIR / "deploy" / "lago" / "images.lock.json"
COMPOSE_FILE = REPO_DIR / "deploy" / "lago" / "compose.yaml"
COMPOSE_PROJECT = "weknora-lago-74"

EXIT_CODES = {"pass": 0, "fail": 1, "blocked-env": 2}
REDACTED = "***REDACTED***"

PHASE_FILES = {
    "setup": "t02-setup.json",
    "provider_setup": "t02-provider.json",
    "gate": "t02-gating.json",
    "manual": "t02-manual.json",
    "activate": "t02-activation.json",
    "duplicates": "t02-duplicates.json",
    "retries": "t02-retries.json",
    "decline_control": "t02-decline.json",
    "cleanup": "t02-cleanup.json",
}
ENVIRONMENT_FILE = "t02-environment.json"
TIMELINE_FILE = "t02-run.txt"


def utc_now():
    return datetime.now(timezone.utc).isoformat()


def load_env_file(path):
    values = {}
    for line in Path(path).read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        values[key.strip()] = value.strip().strip('"')
    return values


def write_json(path, payload):
    Path(path).write_text(json.dumps(payload, sort_keys=True, indent=2) + "\n",
                          encoding="utf-8")


def check_health(api_url):
    """GET /health on the lab origin; returns the HTTP status or None."""
    try:
        request = urllib.request.Request(f"{api_url.rstrip('/')}/health")
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open(request, timeout=10) as response:
            return response.status
    except urllib.error.HTTPError as error:
        return error.code
    except (OSError, ValueError):
        return None


def resolve_stripe_key(environ):
    """First non-empty test-mode key among STRIPE_TEST_SECRET_KEY / STRIPE_SECRET_KEY.

    Returns (key_or_None, source_name_or_None, refused_live_key: bool).
    """
    for name in ("STRIPE_TEST_SECRET_KEY", "STRIPE_SECRET_KEY"):
        value = (environ.get(name) or "").strip()
        if not value:
            continue
        if value.startswith(clients.STRIPE_TEST_PREFIXES):
            return value, name, False
        return None, name, True  # present but not test mode: refuse, never use
    return None, None, False


def load_release_identity():
    with LOCK_PATH.open(encoding="utf-8") as lock_file:
        lock = json.load(lock_file)
    return {"release": lock["release"], "images": lock["images"]}


def preflight_container_stripe(env_file):
    """Can the Lago API container reach api.stripe.com at all?

    Unauthenticated curl: any HTTP code proves reachability (401 expected);
    no code / curl failure means unreachable or stack down. Never sends the
    key into the container process list.
    """
    try:
        result = subprocess.run(
            ["docker", "compose", "-f", str(COMPOSE_FILE),
             "--env-file", str(env_file), "-p", COMPOSE_PROJECT,
             "exec", "-T", "api",
             "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
             "--max-time", "10", "https://api.stripe.com/v1/charges"],
            capture_output=True, text=True, timeout=45, check=False,
        )
        code = result.stdout.strip()
        return {"checked": True, "http_code": code or None,
                "reachable": bool(code) and code != "000",
                "returncode": result.returncode}
    except (OSError, subprocess.SubprocessError) as error:
        return {"checked": False, "reachable": None,
                "error": error.__class__.__name__}


class Timeline:
    def __init__(self):
        self.started_at = utc_now()
        self.lines = [f"[{self.started_at}] run start"]

    def log(self, message):
        stamp = utc_now()
        self.lines.append(f"[{stamp}] {message}")
        print(f"run_lab: {message}", flush=True)

    def text(self):
        self.lines.append(f"[{utc_now()}] run finished")
        return "\n".join(self.lines) + "\n"


def scan_and_scrub(output_dir, secrets, timeline):
    """Scan every file for known secret values; scrub hits; report."""
    scrubbed = []
    hits_total = 0
    for path in sorted(output_dir.rglob("*")):
        if not path.is_file():
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        hits = 0
        for secret in secrets:
            if secret and secret in text:
                hits += text.count(secret)
                text = text.replace(secret, REDACTED)
        if hits:
            path.write_text(text, encoding="utf-8")
            scrubbed.append({"file": path.name, "hits": hits})
            hits_total += hits
            timeline.log(f"SECRETS SCAN: scrubbed {hits} hit(s) in {path.name}")
    # verify zero remaining hits
    remaining = 0
    for path in sorted(output_dir.rglob("*")):
        if not path.is_file():
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        remaining += sum(text.count(secret) for secret in secrets if secret)
    return {"files_scanned": sum(1 for p in output_dir.rglob("*") if p.is_file()),
            "scrubbed": scrubbed, "hits_after_scrub": remaining,
            "clean": remaining == 0}


def run_experiment(args):
    timeline = Timeline()
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    env_file = Path(args.lab_env)
    if not env_file.exists():
        timeline.log(f"missing {env_file} -- run lab.sh init first")
        return EXIT_CODES["blocked-env"], timeline
    env = load_env_file(env_file)

    api_url = env.get("LAGO_API_URL") or clients.LAGO_API_URL
    api_key = env.get("LAGO_ORG_API_KEY", "")
    org_email = env.get("LAGO_ORG_USER_EMAIL", "")
    org_password = env.get("LAGO_ORG_USER_PASSWORD", "")
    stripe_key, stripe_source, refused_live = resolve_stripe_key(os.environ)
    stripe_base_url = os.environ.get("LAB_STRIPE_BASE_URL") or "https://api.stripe.com"

    run_id = str(uuid.uuid4())
    timeline.log(f"run_id={run_id} output_dir={output_dir}")

    # Preflight: pinned release identity, API health, operator login.
    try:
        identity = load_release_identity()
    except (OSError, ValueError, KeyError):
        identity = None
        timeline.log("could not load images.lock.json release identity")
    health_status = check_health(api_url)
    timeline.log(f"lago health HTTP {health_status}")
    jwt = None
    login_ok = False
    if health_status and api_key and org_email:
        try:
            jwt = clients.lago_graphql_login(api_url, org_email, org_password)
            login_ok = jwt is not None
            timeline.log("graphql loginUser: ok")
        except clients.LoginError as error:
            timeline.log(f"graphql loginUser failed: {error}")
    else:
        timeline.log("graphql login skipped (health/API key/credentials missing)")

    container_stripe = None
    if stripe_key:
        container_stripe = preflight_container_stripe(env_file)
        timeline.log(
            "stripe reachability from api container: "
            f"{container_stripe.get('http_code') if container_stripe.get('checked') else 'not checked'}"
        )

    ctx = phases.RunContext(
        lago_url=api_url,
        api_key=api_key,
        run_id=run_id,
        stripe_key=stripe_key,
        graphql_jwt=jwt,
        stripe_base_url=stripe_base_url,
        poll_interval=args.poll_interval,
        poll_timeout=args.poll_timeout,
        stability_rounds=args.stability_rounds,
        stability_delay=args.stability_delay,
    )

    order = [
        ("setup", phases.phase_setup),
        ("provider_setup", phases.phase_provider_setup),
        ("gate", phases.phase_gate),
        ("manual", phases.phase_manual),
        ("activate", phases.phase_activate),
        ("duplicates", phases.phase_duplicates),
        ("retries", phases.phase_retries),
        ("decline_control", phases.phase_decline_control),
    ]

    def run_one(name, fn):
        timeline.log(f"phase {name}: start")
        started = time.monotonic()
        try:
            report = fn(ctx)
        except Exception as error:  # unexpected: recorded honestly, never fatal here
            report = clients.sanitize(
                {
                    "phase": name, "run_id": run_id,
                    "expected": "phase completes with a classified report",
                    "observed": {"unexpected_error": error.__class__.__name__,
                                 "detail": str(error)[:200]},
                    "status": "fail", "evidence": {}, "contract_notes": [],
                    "recorded_at": utc_now(),
                },
                extra_secrets=ctx.secret_values(),
            )
            timeline.log(f"phase {name}: UNEXPECTED {error.__class__.__name__}")
        write_json(output_dir / PHASE_FILES[name], report)
        timeline.log(f"phase {name}: {report['status']} "
                     f"({time.monotonic() - started:.1f}s)")
        return report

    phase_statuses = {}
    cleanup_report = None
    try:
        for name, fn in order:
            phase_statuses[name] = run_one(name, fn)["status"]
    finally:
        cleanup_report = run_one("cleanup", phases.phase_cleanup)
        phase_statuses["cleanup"] = cleanup_report["status"]

    statuses = list(phase_statuses.values())
    if "fail" in statuses:
        overall = "fail"
    elif "blocked-env" in statuses:
        overall = "blocked-env"
    else:
        overall = "pass"

    # Environment report (sanitized by construction: booleans + names only).
    environment = {
        "run": {
            "run_id": run_id,
            "started_at": timeline.started_at,
            "finished_at": utc_now(),
            "overall": overall,
            "phase_statuses": phase_statuses,
        },
        "release": identity,
        "lago": {
            "api_url": api_url,
            "health_http_status": health_status,
            "graphql_login_ok": login_ok,
            "api_key_seeded": bool(api_key),
        },
        "stripe": {
            "key_source": stripe_source,
            "test_mode_key_present": bool(stripe_key),
            "refused_non_test_key": refused_live,
            "api_container_reachability": container_stripe,
        },
    }
    write_json(output_dir / ENVIRONMENT_FILE, environment)

    timeline.log(f"overall verdict: {overall}")
    (output_dir / TIMELINE_FILE).write_text(timeline.text(), encoding="utf-8")

    # Secrets scan over everything written (defense in depth; reports are
    # sanitized by construction). Zero hits required for promotion to docs/.
    secrets = [api_key, org_password, jwt, stripe_key]
    scan = scan_and_scrub(output_dir, secrets, timeline)
    timeline.log(
        f"secrets scan: {scan['files_scanned']} files, "
        f"{len(scan['scrubbed'])} scrubbed, hits_after_scrub={scan['hits_after_scrub']}"
    )
    if not scan["clean"]:
        overall = "fail"
    # rewrite timeline + environment with the final scan result
    environment["secrets_scan"] = scan
    write_json(output_dir / ENVIRONMENT_FILE, environment)
    (output_dir / TIMELINE_FILE).write_text(timeline.text(), encoding="utf-8")
    return EXIT_CODES[overall], timeline


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", default=str(LAB_DIR / "evidence"),
                        help="directory for the per-phase reports (default: evidence/)")
    parser.add_argument("--lab-env", default=str(LAB_DIR / "lab.env"),
                        help="path to the lab stack env file")
    parser.add_argument("--poll-interval", type=float, default=3.0)
    parser.add_argument("--poll-timeout", type=float, default=300.0)
    parser.add_argument("--stability-rounds", type=int, default=3)
    parser.add_argument("--stability-delay", type=float, default=5.0)
    args = parser.parse_args(argv)
    code, _timeline = run_experiment(args)
    return code


if __name__ == "__main__":
    sys.exit(main())
