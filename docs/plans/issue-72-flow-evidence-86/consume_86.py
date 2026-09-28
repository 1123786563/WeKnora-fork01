#!/usr/bin/env python3
"""Issue #86 flow verification: drive real Lago wallet consumption.

Consumption trigger follows the t03 wallet-semantics lab method
(deploy/lago-lab/wallet-semantics/experiments/): billable metric + plan +
subscription, then POST /api/v1/events. Lago aggregates the events into an
invoice which is paid from the customer's wallets in strict
`priority ASC, created_at ASC` order — the runtime consumption path #86's
priority encoding + authority rebalance exist to steer.

Credentials are read from the server-side .env only (never committed).
"""
import json
import os
import sys
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path

BASE = os.environ.get("LAGO_BASE", "http://127.0.0.1:48889")
CUSTOMER = os.environ.get("LAGO_CUSTOMER", "weknora-tenant-10000")


def api_key():
    env_path = Path(__file__).resolve().parents[3] / ".env"
    for line in env_path.read_text(encoding="utf-8").splitlines():
        if line.startswith("WEKNORA_COMMERCIAL_PLATFORM_API_KEY="):
            return line.strip().split("=", 1)[1]
    raise SystemExit("api key not found in .env")


KEY = api_key()


def request(method, path, payload=None):
    url = f"{BASE}{path}"
    body = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    headers["Authorization"] = f"Bearer {KEY}"
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            raw = resp.read()
            return resp.status, json.loads(raw) if raw else None
    except urllib.error.HTTPError as err:
        raw = err.read()
        return err.code, json.loads(raw) if raw else None


def wallets():
    _, body = request("GET", f"/api/v1/customers/{CUSTOMER}/wallets?per_page=20")
    return {w["name"]: w for w in body.get("wallets", [])}


def balance_snapshot():
    return {name: w.get("balance_cents") for name, w in wallets().items()}


def wait_until(predicate, timeout_seconds, interval=3.0):
    started = time.monotonic()
    while True:
        if predicate():
            return True, time.monotonic() - started
        if time.monotonic() - started >= timeout_seconds:
            return False, time.monotonic() - started
        time.sleep(interval)


def main():
    tag = uuid.uuid4().hex[:8]
    metric_code = f"weknora-86-metric-{tag}"
    plan_code = f"weknora-86-plan-{tag}"
    sub_ext = f"weknora-86-sub-{tag}"

    # 1. billable metric (sum aggregation over field `units`) — t03 shape
    status, body = request("POST", "/api/v1/billable_metrics", payload={
        "billable_metric": {
            "code": metric_code,
            "name": f"WeKnora 86 consume {tag}",
            "description": "Issue 86 flow consumption metric",
            "aggregation_type": "sum_agg",
            "field_name": "units",
        }})
    if status not in (200, 201):
        print("metric create failed:", status, body); sys.exit(2)
    metric_id = body["billable_metric"]["lago_id"]

    # 2. plan: standard charge, pay-in-advance per unit, 1 unit = 1.00 CNY
    # (t03 runtime findings: plan-level pay_in_advance REQUIRED; charge must
    #  reference billable_metric_id; invoiceable is Premium-gated, default ok)
    status, body = request("POST", "/api/v1/plans", payload={
        "plan": {
            "code": plan_code,
            "name": f"WeKnora 86 plan {tag}",
            "interval": "weekly",
            "pay_in_advance": False,
            "amount_cents": 0,
            "amount_currency": "CNY",
            "charges": [{
                "billable_metric_id": metric_id,
                "charge_model": "standard",
                "pay_in_advance": True,
                "invoiceable": True,
                "properties": {"amount": "1.00"},
            }],
        }})
    if status not in (200, 201):
        print("plan create failed:", status, json.dumps(body)[:500]); sys.exit(2)

    # 3. subscription for the flow tenant
    status, body = request("POST", "/api/v1/subscriptions", payload={
        "subscription": {
            "external_customer_id": CUSTOMER,
            "plan_code": plan_code,
            "external_id": sub_ext,
            "name": f"weknora-86 consumption trigger {tag}",
        }})
    if status not in (200, 201):
        print("subscription create failed:", status, json.dumps(body)[:500]); sys.exit(2)
    print(f"trigger ready: metric={metric_code} plan={plan_code} sub={sub_ext} "
          f"status={body['subscription'].get('status')}")

    before = balance_snapshot()
    print("balances before:", before)

    # 4. fire one consumption event: 2 units => 2.00 CNY invoice (crosses the
    # 1.00 monthly batch, remainder 1.00 must land on the earliest-expiry topup)
    txn_id = f"weknora-86-evt-{uuid.uuid4()}"
    status, body = request("POST", "/api/v1/events", payload={
        "event": {
            "transaction_id": txn_id,
            "external_customer_id": CUSTOMER,
            "external_subscription_id": sub_ext,
            "code": metric_code,
            "properties": {"units": 2},
        }})
    if status not in (200, 201):
        print("event post failed:", status, json.dumps(body)[:500]); sys.exit(2)
    print(f"event fired: {txn_id} (2 units = 2.00)")

    total_before = sum(before.values())
    ok, elapsed = wait_until(
        lambda: sum(balance_snapshot().values()) == total_before - 200, 300)
    after = balance_snapshot()
    print(f"consumption settled: {ok} after {elapsed:.0f}s; balances after: {after}")

    deltas = {k: before[k] - after[k] for k in before}
    print("deltas:", deltas)

    result = {
        "metric": metric_code, "plan": plan_code, "subscription": sub_ext,
        "event": txn_id, "consume_cents": 150,
        "before": before, "after": after, "deltas": deltas,
        "settled": ok, "settle_seconds": round(elapsed),
    }
    out = os.path.join(os.path.dirname(__file__), "consume-cny.json")
    with open(out, "w", encoding="utf-8") as fh:
        json.dump(result, fh, indent=2)
    print("saved:", out)

    # expected: monthly wallet (earliest expiry, priority 1) drained first
    # (100c), then earliest-expiry topup C (priority 2) covers 100c; D untouched.
    m = [k for k in deltas if k.endswith("-2026-09")][0]
    c = [k for k in deltas if "topup-c" in k][0]
    d = [k for k in deltas if "topup-d" in k][0]
    verdict = (after[m] == 0 and deltas[m] == 100
               and deltas[c] == 100 and after[c] == 400
               and deltas[d] == 0 and after[d] == 500)
    print("ORDER VERDICT:", "PASS (monthly first, then earliest-expiry topup)"
          if verdict else "FAIL")
    sys.exit(0 if verdict else 1)


if __name__ == "__main__":
    main()
