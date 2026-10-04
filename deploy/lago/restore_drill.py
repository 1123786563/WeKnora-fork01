#!/usr/bin/env python3
"""#104 [Lago 32] staging restore + commercial reconciliation drill.

Drives the full loss -> restore -> reconcile exercise against the local
pinned Lago Community v1.53.0 stack (operator-owned, deploy/lago/README.md):

  1. seed drill-scoped objects (customer + wallet, idempotent upserts)
  2. snapshot the seven commercial classes org-wide: Customer /
     Subscription / Wallet / Invoice / Payment via the Lago API, pending
     work via the redis keyspace, Billing Projections via the WeKnora
     commercial tables
  3. lago.sh backup (timed)
  4. destroy + restore (timed -> RTO; spec: RTO <= 60 min)
  5. re-snapshot and compare each class against the pre-backup state
  6. replay the seeded ensure (same external ids) and verify no duplicate
     side effects (pending objects never double-fire)
  7. Billing Projections: full pg_dump of the WeKnora dev database,
     restored into a scratch database, commercial table row counts compared
     (a selective commercial_* dump cannot restore: the tables hold foreign
     keys into tenants and the rest of the schema)

RPO is recorded as a method property, not measured here: locally it is the
operator's backup cadence; the production design (WAL archiving, RPO <= 5
min) lives in docs/upstream-parity/lago-production-observability.md.

The API key comes from the caller environment (LAGO_DRILL_API_KEY or
LAGO_INTEGRATION_API_KEY), falling back to LAGO_ORG_API_KEY in
deploy/lago/.env -- same operator-local convention as the other probes.
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
from datetime import datetime, timezone
from pathlib import Path

LAGO_DIR = Path(__file__).resolve().parent
RTO_BUDGET_SECONDS = 3600  # spec L182: RTO no greater than sixty minutes
RPO_BUDGET_SECONDS = 300  # spec L182: RPO no greater than five minutes
DRILL_TENANT = "780104"
DRILL_EXT = f"weknora-tenant-{DRILL_TENANT}"
SCRATCH_DB = "wk_t32_drill_scratch"

# Fingerprint fields per authority class: stable across a byte-identical
# restore, meaningful for reconciliation (identity + state + money).
CLASS_FIELDS = {
    "customer": ("lago_id", "external_id", "name", "created_at"),
    "subscription": ("lago_id", "external_id", "status", "plan_code", "lago_customer_id"),
    "wallet": ("lago_id", "lago_customer_id", "currency", "status", "balance", "consumed_credits"),
    "invoice": ("lago_id", "lago_customer_id", "status", "currency", "fees_amount_cents"),
    "payment": ("lago_id", "amount_cents", "currency", "status"),
}
CLASS_RESOURCES = (
    ("customer", "customers"),
    ("subscription", "subscriptions"),
    ("wallet", "wallets"),
    ("invoice", "invoices"),
    ("payment", "payments"),
)


def env_value(key: str, default: str = "") -> str:
    env = LAGO_DIR / ".env"
    if env.exists():
        for line in env.read_text(encoding="utf-8").splitlines():
            if line.startswith(f"{key}="):
                return line.partition("=")[2].strip().strip('"')
    return default


def run(cmd: list[str], *, check: bool = True, stdin_file: Path | None = None) -> subprocess.CompletedProcess:
    stdin = stdin_file.open("rb") if stdin_file else None
    try:
        return subprocess.run(cmd, check=check, capture_output=True, stdin=stdin)
    finally:
        if stdin:
            stdin.close()


def api_strict(base: str, key: str, method: str, path: str, body: dict | None = None) -> dict | list:
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        base + "/api/v1" + path,
        data=data,
        method=method,
        headers={"Authorization": "Bearer " + key, "Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=60) as resp:
        return json.load(resp)


def list_all(base: str, key: str, resource: str) -> list[dict]:
    """Follow Lago's meta.next_page pagination to exhaust a collection."""
    items: list[dict] = []
    page = 1
    while True:
        doc = api_strict(base, key, "GET", f"/{resource}?page={page}&per_page=100")
        items.extend(doc.get(resource, []))
        nxt = (doc.get("meta") or {}).get("next_page")
        if not nxt:
            return items
        page = int(nxt)


def fingerprint(cls: str, doc: dict) -> dict:
    return {field: doc.get(field) for field in CLASS_FIELDS[cls]}


def snapshot_authority(base: str, key: str) -> dict[str, list[dict]]:
    return {
        cls: sorted((fingerprint(cls, doc) for doc in list_all(base, key, resource)), key=lambda d: str(d.get("lago_id")))
        for cls, resource in CLASS_RESOURCES
    }


REDIS_SIZE_SCRIPT = r"""
redis-cli --scan | sort | while read -r k; do
  t="$(redis-cli TYPE "$k")"
  case "$t" in
    list) c="$(redis-cli LLEN "$k")" ;;
    set)  c="$(redis-cli SCARD "$k")" ;;
    zset) c="$(redis-cli ZCARD "$k")" ;;
    hash) c="$(redis-cli HLEN "$k")" ;;
    *)    c="$(redis-cli STRLEN "$k")" ;;
  esac
  echo "$k $c"
done
"""


def snapshot_pending_work() -> dict[str, int]:
    """Sidekiq BACKLOG fingerprint: queue depths plus the retry/schedule/
    dead sets. Runtime stat counters (processed counts, per-second keys)
    are excluded on purpose -- they advance while the restored stack works,
    and pending work is about backlog, not throughput."""
    out = run(["docker", "exec", "weknora-lago-redis-1", "sh", "-c", REDIS_SIZE_SCRIPT]).stdout.decode()
    fingerprint_map: dict[str, int] = {}
    for line in out.splitlines():
        parts = line.split()
        if len(parts) != 2:
            continue
        key, size = parts
        if key.startswith("queue:") or key in ("retry", "schedule", "dead"):
            fingerprint_map[key] = int(size or 0)
    return fingerprint_map


def weknora_db() -> tuple[str, str, str]:
    container = env_value("WK_DRILL_DB_CONTAINER", "WeKnora-postgres-dev")
    user = run(["docker", "exec", container, "sh", "-c", "echo $POSTGRES_USER"]).stdout.decode().strip()
    dbname = run(["docker", "exec", container, "sh", "-c", "echo $POSTGRES_DB"]).stdout.decode().strip()
    if not user or not dbname:
        raise SystemExit(f"blocked-env: could not read POSTGRES_USER/POSTGRES_DB from {container}")
    return container, user, dbname


def weknora_commercial_tables(container: str, user: str, dbname: str) -> list[str]:
    sql = "select tablename from pg_tables where schemaname='public' and tablename like 'commercial%' order by 1"
    out = run(["docker", "exec", container, "psql", "-U", user, "-d", dbname, "-tAc", sql]).stdout.decode()
    return [line.strip() for line in out.splitlines() if line.strip()]


def snapshot_billing_projections() -> dict[str, int]:
    container, user, dbname = weknora_db()
    counts: dict[str, int] = {}
    for table in weknora_commercial_tables(container, user, dbname):
        out = run(["docker", "exec", container, "psql", "-U", user, "-d", dbname, "-tAc", f"select count(*) from {table}"])
        counts[table] = int(out.stdout.decode().strip() or 0)
    return counts


def dump_weknora_database(path: Path) -> None:
    container, user, dbname = weknora_db()
    with path.open("wb") as fh:
        subprocess.run(
            ["docker", "exec", container, "pg_dump", "-U", user, "-d", dbname, "-Fc"],
            check=True,
            stdout=fh,
        )


def restore_weknora_scratch(dump: Path) -> tuple[dict[str, int], str]:
    """Restore the full WeKnora dump into a scratch database and return the
    commercial table row counts plus the pg_restore stderr tail. The scratch
    DB is dropped afterwards; the live dev tables are never touched.

    pg_restore exit code is not fatal here: the dev image's template
    already carries the tiger/topology/paradedb schemas, so those CREATE
    SCHEMA statements conflict harmlessly -- row counts are the verdict."""
    container, user, _ = weknora_db()
    run(["docker", "exec", container, "dropdb", "-U", user, "--if-exists", SCRATCH_DB])
    run(["docker", "exec", container, "createdb", "-U", user, SCRATCH_DB])
    try:
        proc = run(
            ["docker", "exec", "-i", container, "pg_restore", "-U", user, "-d", SCRATCH_DB, "--no-owner"],
            check=False,
            stdin_file=dump,
        )
        stderr_tail = "\n".join(proc.stderr.decode().splitlines()[-3:])
        counts: dict[str, int] = {}
        for table in weknora_commercial_tables(container, user, SCRATCH_DB):
            out = run(["docker", "exec", container, "psql", "-U", user, "-d", SCRATCH_DB, "-tAc", f"select count(*) from {table}"])
            counts[table] = int(out.stdout.decode().strip() or 0)
        return counts, stderr_tail
    finally:
        run(["docker", "exec", container, "dropdb", "-U", user, "--if-exists", SCRATCH_DB])


def compare_classes(pre: dict, post: dict) -> dict:
    verdict: dict = {}
    for cls in sorted(set(pre) | set(post)):
        verdict[cls] = {
            "pre": len(pre.get(cls, [])),
            "post": len(post.get(cls, [])),
            "equal": pre.get(cls) == post.get(cls),
        }
    return verdict


def seed(base: str, key: str) -> None:
    """Drill-scoped objects: customer upsert + one wallet. Idempotent so a
    crashed drill run can simply be re-run."""
    api_strict(base, key, "POST", "/customers", {"customer": {"external_id": DRILL_EXT, "name": "restore drill t32"}})
    existing = api_strict(base, key, "GET", f"/customers/{DRILL_EXT}/wallets").get("wallets", [])
    if not existing:
        api_strict(
            base,
            key,
            "POST",
            "/wallets",
            {"wallet": {"external_customer_id": DRILL_EXT, "currency": "USD", "rate_amount": "1.0", "granted_credits": "10", "consumed_credits": "0"}},
        )


def replay_idempotency(base: str, key: str) -> dict:
    """The pending/restored objects must not double-fire side effects. The
    representative replay is the ensure-upsert the whole seam relies on:
    re-POST the same customer create and verify identity stability and no
    second object."""
    before = [c for c in list_all(base, key, "customers") if c.get("external_id") == DRILL_EXT]
    api_strict(base, key, "POST", "/customers", {"customer": {"external_id": DRILL_EXT, "name": "restore drill t32"}})
    after = [c for c in list_all(base, key, "customers") if c.get("external_id") == DRILL_EXT]
    return {
        "before": len(before),
        "after": len(after),
        "identity_stable": bool(before) and bool(after) and before[0]["lago_id"] == after[0]["lago_id"],
        "no_duplicate": len(after) == len(before) == 1,
    }


def wait_healthy(base: str, key: str, timeout: int = 600) -> bool:
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            api_strict(base, key, "GET", "/organizations")
            return True
        except (urllib.error.URLError, urllib.error.HTTPError, OSError):
            time.sleep(3)
    return False


def backup_dir_of(output: str) -> str:
    for line in output.splitlines():
        if line.startswith("backup written to "):
            return line.partition("backup written to ")[2].strip()
    raise SystemExit("lago.sh backup did not report its output directory")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--base-url", default=None)
    parser.add_argument("--api-key", default=None)
    parser.add_argument("--out", default=None, help="write the JSON verdict here too")
    parser.add_argument("--keep-backup", action="store_true", help="keep the timestamped backup dir after the drill")
    args = parser.parse_args()

    key = args.api_key or os.environ.get("LAGO_DRILL_API_KEY") or os.environ.get("LAGO_INTEGRATION_API_KEY") or env_value("LAGO_ORG_API_KEY")
    if not key:
        print("blocked-env: no API key (LAGO_DRILL_API_KEY / LAGO_INTEGRATION_API_KEY env or LAGO_ORG_API_KEY in deploy/lago/.env)", file=sys.stderr)
        return 2
    base = args.base_url or f"http://127.0.0.1:{env_value('LAGO_API_PORT', '48889')}"

    report: dict = {"drill": "lago-restore-t32", "started_at": datetime.now(timezone.utc).isoformat(), "base_url": base, "tenant": DRILL_TENANT}

    seed(base, key)
    pre_authority = snapshot_authority(base, key)
    pre_pending = snapshot_pending_work()
    pre_projections = snapshot_billing_projections()
    dump_path = LAGO_DIR / "backups" / "drill-weknora-full.dump"
    dump_path.parent.mkdir(parents=True, exist_ok=True)
    dump_weknora_database(dump_path)

    t0 = time.monotonic()
    backup_out = run(["bash", str(LAGO_DIR / "lago.sh"), "backup"]).stdout.decode()
    backup_seconds = round(time.monotonic() - t0, 1)
    backup_dir = backup_dir_of(backup_out)
    dump_size = (Path(backup_dir) / "lago.dump").stat().st_size
    if dump_size < 1024:
        report.update({"verdict": "fail", "reason": f"backup dump suspiciously small ({dump_size} bytes)"})
        print(json.dumps(report, indent=2))
        return 1

    t1 = time.monotonic()
    run(["bash", str(LAGO_DIR / "lago.sh"), "restore", backup_dir])
    rto_seconds = round(time.monotonic() - t1, 1)
    if not wait_healthy(base, key):
        report.update({"verdict": "fail", "reason": "stack did not answer after restore"})
        if args.out:
            Path(args.out).write_text(json.dumps(report, indent=2) + "\n")
        print(json.dumps(report, indent=2))
        return 1

    post_authority = snapshot_authority(base, key)
    post_pending = snapshot_pending_work()
    post_projections = snapshot_billing_projections()

    classes = compare_classes(pre_authority, post_authority)
    classes["pending_work"] = {"pre": len(pre_pending), "post": len(post_pending), "equal": pre_pending == post_pending}
    classes["billing_projections"] = {"pre": len(pre_projections), "post": len(post_projections), "equal": pre_projections == post_projections}

    scratch_counts, scratch_stderr = restore_weknora_scratch(dump_path)
    projections_scratch_equal = scratch_counts == pre_projections
    replay = replay_idempotency(base, key)

    report.update(
        {
            "backup_dir": backup_dir,
            "backup_seconds": backup_seconds,
            "rto_seconds": rto_seconds,
            "rto_budget_seconds": RTO_BUDGET_SECONDS,
            "rto_within_budget": rto_seconds <= RTO_BUDGET_SECONDS,
            "rpo": {
                "budget_seconds": RPO_BUDGET_SECONDS,
                "method": "local: operator backup cadence (lago.sh backup); production design: WAL archiving per docs/upstream-parity/lago-production-observability.md",
                "evidence": "method default, not measured by this drill",
            },
            "classes": classes,
            "billing_projections_scratch_restore": {
                "tables": len(pre_projections),
                "rows_total": sum(pre_projections.values()),
                "counts_equal": projections_scratch_equal,
                "pg_restore_stderr_tail": scratch_stderr,
            },
            "replay_idempotency": replay,
            "finished_at": datetime.now(timezone.utc).isoformat(),
        }
    )
    report["verdict"] = (
        "pass"
        if all(c["equal"] for c in classes.values())
        and projections_scratch_equal
        and replay["no_duplicate"]
        and replay["identity_stable"]
        and report["rto_within_budget"]
        else "fail"
    )

    blob = json.dumps(report, indent=2)
    print(blob)
    if args.out:
        Path(args.out).parent.mkdir(parents=True, exist_ok=True)
        Path(args.out).write_text(blob + "\n")
    dump_path.unlink(missing_ok=True)
    # The backup stays on disk unless the drill passed -- a failed drill
    # leaves the recovery path available.
    if not args.keep_backup and report["verdict"] == "pass":
        run(["rm", "-rf", backup_dir], check=False)
    return 0 if report["verdict"] == "pass" else 1


if __name__ == "__main__":
    sys.exit(main())
