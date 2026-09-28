#!/usr/bin/env python3
"""Issue #86 concurrent-consumption leg (stdout only; the operator tees the
output into the evidence directory).

Fires N parallel consumption events against the Lago authority and asserts
the wallets are drawn exactly once per unit with no double-draw, no negative
balance, and the draw order follows the expiry rank (earliest-expiry wallet
drained first). Also replays one identical event transaction_id to assert
event idempotency (no extra draw).

Egress policy: local test-stack allow-list only (scheme-validated, no
redirects followed). The API key arrives via the LAGO_API_KEY env variable.
"""
import json
import os
import sys
import threading
import time
import urllib.error
import uuid
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler

ALLOWED_TARGETS = {"127.0.0.1"}  # the local Lago test stack, nothing else
CUSTOMER = os.environ.get("LAGO_CUSTOMER", "weknora-tenant-10000")


class _NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


OPENER = build_opener(_NoRedirect)


def base_url():
    raw = os.environ.get("LAGO_BASE", "http://127.0.0.1:48889")
    parts = urlsplit(raw)
    if parts.scheme not in ("http", "https"):
        raise SystemExit("scheme not allowed: " + parts.scheme)
    if parts.hostname not in ALLOWED_TARGETS:
        raise SystemExit("target host not in test-stack allow-list: "
                         + parts.hostname)
    return raw


def call(method, path, payload=None):
    body = None
    headers = {"Accept": "application/json",
               "Authorization": "Bearer " + os.environ["LAGO_API_KEY"]}
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = Request(base_url() + path, data=body, headers=headers, method=method)
    with OPENER.open(req, timeout=30) as resp:
        raw = resp.read()
        return resp.status, json.loads(raw) if raw else None


def wallet_list():
    _, body = call("GET", "/api/v1/customers/" + CUSTOMER + "/wallets?per_page=20")
    return [w for w in body.get("wallets", [])]


def balances():
    return {w["name"]: w.get("balance_cents")
            for w in wallet_list() if w.get("status") == "active"}


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
    metric_code = "weknora-86-metric-" + tag
    plan_code = "weknora-86-plan-" + tag
    sub_ext = "weknora-86-sub-" + tag

    _, body = call("POST", "/api/v1/billable_metrics", payload={
        "billable_metric": {"code": metric_code,
                            "name": "WeKnora 86 concurrent " + tag,
                            "aggregation_type": "sum_agg",
                            "field_name": "units"}})
    metric_id = body["billable_metric"]["lago_id"]
    call("POST", "/api/v1/plans", payload={
        "plan": {"code": plan_code, "name": "WeKnora 86 concurrent " + tag,
                 "interval": "weekly", "pay_in_advance": False,
                 "amount_cents": 0, "amount_currency": "CNY",
                 "charges": [{"billable_metric_id": metric_id,
                              "charge_model": "standard",
                              "pay_in_advance": True, "invoiceable": True,
                              "properties": {"amount": "1.00"}}]}})
    call("POST", "/api/v1/subscriptions", payload={
        "subscription": {"external_id": sub_ext,
                         "external_customer_id": CUSTOMER,
                         "plan_code": plan_code}})
    print("trigger ready (concurrent leg): sub=" + sub_ext)

    before = balances()
    total_before = sum(before.values())
    print("balances before:", before, "total:", total_before)

    n_events = 5
    txn_ids = ["weknora-86-con-" + uuid.uuid4().hex for _ in range(n_events)]
    results = {}

    def fire(txn):
        try:
            status, _ = call("POST", "/api/v1/events", payload={
                "event": {"transaction_id": txn,
                          "external_customer_id": CUSTOMER,
                          "external_subscription_id": sub_ext,
                          "code": metric_code,
                          "properties": {"units": 1}}})
            results[txn] = "posted" if status in (200, 201) else "http" + str(status)
        except Exception as exc:  # noqa: BLE001
            results[txn] = "error:" + type(exc).__name__

    threads = [threading.Thread(target=fire, args=(t,)) for t in txn_ids]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    print("event posts:", results)
    if any(v != "posted" for v in results.values()):
        print("FAIL: some events failed to post")
        sys.exit(2)

    draw_cents = n_events * 100
    ok, elapsed = wait_until(
        lambda: sum(balances().values()) == total_before - draw_cents, 300)
    after = balances()
    total_after = sum(after.values())
    print("draw settled: %s after %.0fs; after: %s total: %s"
          % (ok, elapsed, after, total_after))

    # replay one identical transaction_id — Lago rejects duplicate events
    # with 422 (idempotency by refusal) and never draws anything extra
    replay_status = None
    try:
        replay_status, _ = call("POST", "/api/v1/events", payload={
            "event": {"transaction_id": txn_ids[0],
                      "external_customer_id": CUSTOMER,
                      "external_subscription_id": sub_ext,
                      "code": metric_code,
                      "properties": {"units": 1}}})
    except urllib.error.HTTPError as err:  # noqa: BLE001
        replay_status = err.code
    time.sleep(10)
    replay_after = balances()
    replay_total = sum(replay_after.values())
    print("replay status:", replay_status,
          "after replay:", replay_after, "total:", replay_total)

    expiry_of = {w["name"]: w.get("expiration_at") for w in wallet_list()}
    names = sorted(after, key=lambda n: expiry_of.get(n) or "")
    early, late = names[0], names[1]
    spill = draw_cents - (before[early] - after[early])

    facts = {"txn_ids": txn_ids, "before": before, "after": after,
             "replay_after": replay_after, "draw_cents": draw_cents}

    checks = [
        ("total drawn exactly once per event (no double draw)",
         total_before - total_after == draw_cents,
         "%d - %d == %d" % (total_before, total_after, draw_cents)),
        ("no wallet negative",
         all(v >= 0 for v in after.values()), str(after)),
        ("draw order follows expiry rank (earliest drained first)",
         after[early] == 0 and spill >= 0
         and after[late] == before[late] - spill,
         "early=%s %d->%d, late=%s %d->%d, spill=%d"
         % (early, before[early], after[early],
            late, before[late], after[late], spill)),
        ("identical event replay draws nothing extra",
         replay_total == total_after,
         "%d == %d" % (replay_total, total_after)),
    ]
    for name, ok_, ev in checks:
        print("[%s] %s" % ("PASS" if ok_ else "FAIL", name))
        print("       %s" % ev)
    print("FACTS_JSON " + json.dumps(facts))
    verdict = all(c[1] for c in checks)
    print("CONCURRENT CONSUMPTION " + ("PASS" if verdict else "FAIL"))
    sys.exit(0 if verdict else 1)


if __name__ == "__main__":
    main()
