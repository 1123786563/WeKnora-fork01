#!/usr/bin/env python3
"""Report a Lago Compose health snapshot for local operators."""

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path
from urllib.error import URLError
from urllib.request import urlopen


DEPLOY_DIR = Path(__file__).resolve().parent
LOCK_PATH = DEPLOY_DIR / "images.lock.json"
CORE_SERVICES = ("api", "api-worker", "db", "redis")


def classify_service(row):
    """Convert one Compose ps row into a stable, small service record."""
    if not isinstance(row, dict):
        return {"service": "unknown", "state": "unknown", "image": None, "image_id": None}

    service = row.get("Service") or row.get("Name") or "unknown"
    compose_state = str(row.get("State", "")).strip().lower()
    health = str(row.get("Health", "")).strip().lower()

    if compose_state and compose_state != "running":
        state = compose_state
    elif compose_state != "running":
        state = "unknown"
    elif health in {"healthy", "unhealthy", "starting"}:
        state = health
    elif not health:
        state = "running_unverified"
    else:
        state = "unknown"

    return {
        "service": service,
        "state": state,
        "image": row.get("Image"),
        "image_id": row.get("ImageID"),
    }


def _service_state(row):
    return classify_service(row)["state"]


def overall_status(api_ok, services):
    """Classify availability using API, worker, database, Redis, and clock."""
    if not api_ok:
        return "unavailable"

    states = {name: _service_state(services.get(name)) for name in CORE_SERVICES}
    if any(state == "unknown" for state in states.values()):
        return "unavailable"

    clock_state = _service_state(services.get("api-clock"))
    if clock_state == "unknown":
        return "degraded"
    if any(state != "healthy" for state in states.values()) or clock_state != "running_unverified":
        return "degraded"
    return "ready"


def load_release_identity(lock_path=LOCK_PATH):
    """Load the release and immutable image references from the lock file."""
    with Path(lock_path).open(encoding="utf-8") as lock_file:
        lock = json.load(lock_file)
    return {"release": lock["release"], "images": lock["images"]}


def _compose_rows(output):
    """Parse Compose JSON array output and JSON-lines output."""
    output = output.strip()
    if not output:
        return []
    try:
        parsed = json.loads(output)
        return parsed if isinstance(parsed, list) else [parsed]
    except json.JSONDecodeError:
        return [json.loads(line) for line in output.splitlines() if line.strip()]


def _api_ok(url):
    try:
        with urlopen(url, timeout=3) as response:
            return 200 <= response.status < 300
    except (OSError, URLError, ValueError):
        return False


def build_snapshot(rows, api_ok, release_identity):
    by_service = {row.get("Service"): row for row in rows if isinstance(row, dict) and row.get("Service")}
    services = {name: classify_service(by_service.get(name)) for name in (
        "api", "api-worker", "api-clock", "db", "redis", "front", "pdf"
    )}
    return {
        "release": release_identity["release"],
        "images": release_identity["images"],
        "api": {"state": "healthy" if api_ok else "unavailable"},
        "services": services,
        "overall": overall_status(api_ok, by_service),
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--json", action="store_true", help="print a JSON health snapshot")
    args = parser.parse_args(argv)
    if not args.json:
        parser.error("--json is required")

    try:
        result = subprocess.run(
            ["docker", "compose", "ps", "--format", "json"],
            cwd=DEPLOY_DIR,
            check=True,
            capture_output=True,
            text=True,
        )
        rows = _compose_rows(result.stdout)
    except (OSError, subprocess.CalledProcessError, json.JSONDecodeError):
        rows = []

    port = os.environ.get("LAGO_API_PORT", "48889")
    api_ok = _api_ok(f"http://127.0.0.1:{port}/health")
    try:
        identity = load_release_identity()
    except (OSError, KeyError, json.JSONDecodeError) as error:
        print(json.dumps({"overall": "unavailable", "error": f"invalid image lock: {error}"}))
        return 1

    print(json.dumps(build_snapshot(rows, api_ok, identity), sort_keys=True))
    return 0 if api_ok else 1


if __name__ == "__main__":
    sys.exit(main())
