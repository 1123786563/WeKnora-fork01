#!/usr/bin/env python3
"""OM-03 / OM-04 runtime experiments against local OpenMeter v1.0.0-beta.232.

Real request/response evidence only. Same no-proxy urllib opener pattern as
scripts/saas/contract_inventory.py::_probe. Each section is fault-isolated:
a section crash records its traceback instead of losing earlier artifacts.
Run-2 note: a first execution crashed mid-run after creating customer key
om0304-20260912-cust1 server-side (id 01M29V7MWK3KRK6WZVX51PJ1KM) without
saving artifacts; run-2 uses key om0304-20260912-cust1r2 for a clean race.
"""
from __future__ import annotations

import concurrent.futures
import json
import threading
import traceback
import urllib.error
import urllib.request
from datetime import datetime, timezone

BASE = "http://127.0.0.1:48888"
NS = "om0304-20260912"
OUT_DIR = "artifacts/saas-contract"
OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))
LOG: list[dict] = []
RESULTS: dict[str, dict] = {}


def req(label: str, method: str, path: str, body=None) -> dict:
    url = BASE + path
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    if data is not None:
        r.add_header("Content-Type", "application/json")
    entry = {"ts": datetime.now(timezone.utc).isoformat(), "label": label,
             "method": method, "path": path, "request_body": body}
    try:
        with OPENER.open(r, timeout=15) as resp:
            raw = resp.read().decode()
            entry["status"] = resp.status
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        entry["status"] = e.code
    except Exception as e:
        entry["status"] = "error"
        entry["exception"] = repr(e)
        raw = ""
    if raw:
        try:
            entry["response_json"] = json.loads(raw)
        except json.JSONDecodeError:
            entry["response_raw"] = raw[:800]
    else:
        entry["response_json"] = None
    LOG.append(entry)
    return entry


def save(name: str, payload: dict) -> None:
    with open(f"{OUT_DIR}/{name}", "w", encoding="utf-8") as f:
        json.dump(payload, f, ensure_ascii=False, indent=2)
        f.write("\n")


def section(key: str, check: str):
    def deco(fn):
        def run():
            try:
                RESULTS[key] = {"check": check, "result": fn()}
            except Exception:
                RESULTS[key] = {"check": check, "section_crash": traceback.format_exc()}
            try:
                save(key.replace("_", "-", 1) + ".json", RESULTS[key])
            except Exception:
                RESULTS[key]["save_error"] = traceback.format_exc()
        return run
    return deco


# ---------- OM-03(a): concurrent same-key customer creation (N=8) ----------
@section("om03a_concurrent", "OM-03(a) concurrent same-key customer creation, N=8")
def om03a():
    cust_key = f"{NS}-cust1r2"
    n = 8
    barrier = threading.Barrier(n)

    def attempt(i):
        body = {"key": cust_key, "name": f"OM03 concurrent candidate {i}"}
        try:
            barrier.wait(timeout=20)
        except threading.BrokenBarrierError:
            pass
        r = req(f"om03a-concurrent-{i}", "POST", "/api/v3/openmeter/customers", body)
        rj = r.get("response_json") or {}
        return {"worker": i, "status": r["status"], "returned_id": rj.get("id") if isinstance(rj, dict) else None}

    with concurrent.futures.ThreadPoolExecutor(max_workers=n) as ex:
        attempts = sorted(ex.map(attempt, range(n)))
    statuses = [a["status"] for a in attempts]
    winners = [a for a in attempts if a["status"] == 201]
    winner_id = winners[0]["returned_id"] if winners else None
    get_by_id = req("om03a-get-by-id", "GET", f"/api/v3/openmeter/customers/{winner_id}") if winner_id else None
    get_by_key = req("om03a-get-by-key", "GET", f"/api/v3/openmeter/customers/{cust_key}")
    listing = req("om03a-list-customers", "GET", "/api/v3/openmeter/customers?limit=100")
    listed_ids = [c.get("id") for c in ((listing.get("response_json") or {}).get("data") or [])
                  if c.get("key") == cust_key]
    prior = req("om03a-prior-run1-customer", "GET", "/api/v3/openmeter/customers/01M29V7MWK3KRK6WZVX51PJ1KM")
    return {
        "attempts": attempts,
        "summary": {
            "key": cust_key, "n_attempts": n, "n_201": statuses.count(201),
            "n_409": statuses.count(409),
            "other_statuses": sorted(set(map(str, statuses)) - {"201", "409"}),
            "distinct_returned_ids": sorted({a["returned_id"] for a in attempts if a["returned_id"]}),
            "expected": "exactly one 201 winner, seven 409, one distinct id",
        },
        "get_by_id": {"status": get_by_id["status"],
                      "id": ((get_by_id.get("response_json") or {}).get("id")),
                      "key": ((get_by_id.get("response_json") or {}).get("key"))} if get_by_id else None,
        "get_by_key": {"status": get_by_key["status"],
                       "id": ((get_by_key.get("response_json") or {}).get("id")),
                       "key": ((get_by_key.get("response_json") or {}).get("key"))},
        "list_filtered_client_side": {"ids_with_same_key": listed_ids, "count": len(listed_ids)},
        "prior_run1_customer": {"status": prior["status"],
                                "key": ((prior.get("response_json") or {}).get("key"))},
    }


# ---------- OM-03(b): response-lost retry ----------
@section("om03b_retry", "OM-03(b) response-lost retry: identical POST replay (customer + grant)")
def om03b():
    cust_key = f"{NS}-cust2"
    body = {"key": cust_key, "name": "OM03 response-lost retry customer"}
    c1 = req("om03b-cust-first", "POST", "/api/v3/openmeter/customers", body)
    c2 = req("om03b-cust-retry", "POST", "/api/v3/openmeter/customers", body)
    c1j, c2j = c1.get("response_json") or {}, c2.get("response_json") or {}
    cust_id = c1j.get("id") if isinstance(c1j, dict) else None
    if not cust_id and isinstance(c2j, dict):
        cust_id = c2j.get("id")
    if not cust_id:  # leftover from crashed run-1: find via listing
        listing = req("om03b-find-existing", "GET", "/api/v3/openmeter/customers?limit=100")
        cust_id = next((c.get("id") for c in ((listing.get("response_json") or {}).get("data") or [])
                        if c.get("key") == cust_key), None)
    grant_key = f"{NS}-grantA"
    gbody = {"name": "OM03 grant replay", "funding_method": "none", "currency": "USD",
             "amount": "10", "key": grant_key}
    gpath = f"/api/v3/openmeter/customers/{cust_id}/credits/grants"
    g1 = req("om03b-grant-first", "POST", gpath, gbody)
    g2 = req("om03b-grant-retry", "POST", gpath, gbody)
    glist = req("om03b-grant-list", "GET", gpath)
    matching = [g for g in ((glist.get("response_json") or {}).get("data") or [])
                if g.get("key") == grant_key]
    return {
        "customer_retry": {
            "key": cust_key, "cust_id": cust_id,
            "first": {"status": c1["status"], "id": c1j.get("id") if isinstance(c1j, dict) else None},
            "retry": {"status": c2["status"],
                      "id": c2j.get("id") if isinstance(c2j, dict) else None,
                      "error_detail": (c2j.get("error") or c2j.get("title") or c2j.get("detail")) if isinstance(c2j, dict) else str(c2j)[:300]},
            "same_id_returned": (c1j.get("id") if isinstance(c1j, dict) else None) == (c2j.get("id") if isinstance(c2j, dict) else None),
        },
        "grant_retry": {
            "grant_body_key": grant_key,
            "first": {"status": g1["status"], "id": (g1.get("response_json") or {}).get("id")},
            "retry": {"status": g2["status"], "id": (g2.get("response_json") or {}).get("id"),
                      "error_detail": ((g2.get("response_json") or {}).get("error") or (g2.get("response_json") or {}).get("title"))},
            "list_matching_grants": [{"id": m.get("id"), "key": m.get("key"), "amount": m.get("amount")} for m in matching],
            "matching_count": len(matching),
            "expected": "retry 409, exactly one grant with that key, same id as first",
        },
    }


# ---------- OM-03(c): catalog version replay ----------
@section("om03c_plan", "OM-03(c) catalog version replay: create -> publish -> publish again")
def om03c():
    plan_key = "om0304_20260912_plan1"
    body = {
        "name": "OM0304 Replay Plan", "key": plan_key, "currency": "USD", "billing_cadence": "P1M",
        "phases": [{"key": "main", "name": "Main", "rate_cards": [
            {"key": "om02_tokens", "name": "OM0304 Tokens RC",
             "feature": {"id": "01M29SXXZY6WECC4ZDQPWJVBVP"},
             "price": {"type": "unit", "amount": "0.01"}, "billing_cadence": "P1M"}]}],
    }
    p1 = req("om03c-plan-create", "POST", "/api/v3/openmeter/plans", body)
    plan_id = (p1.get("response_json") or {}).get("id")
    if not plan_id:  # leftover from a crashed run: resolve by key
        gk = req("om03c-find-plan-by-key", "GET", f"/api/v3/openmeter/plans/{plan_key}")
        plan_id = (gk.get("response_json") or {}).get("id")
    pub1 = req("om03c-publish-1", "POST", f"/api/v3/openmeter/plans/{plan_id}/publish")
    pub2 = req("om03c-publish-2-replay", "POST", f"/api/v3/openmeter/plans/{plan_id}/publish")
    pget = req("om03c-plan-get", "GET", f"/api/v3/openmeter/plans/{plan_id}")
    pkb = req("om03c-plan-get-by-key", "GET", f"/api/v3/openmeter/plans/{plan_key}")
    pdoc = pget.get("response_json") or {}
    kbdoc = pkb.get("response_json") or {}
    def versions_of(doc):
        v = doc.get("versions") if isinstance(doc, dict) else None
        if isinstance(v, list):
            return [{"id": x.get("id"), "status": x.get("status"), "plan_version_number": x.get("plan_version_number")} for x in v]
        return v
    return {
        "plan_key": plan_key, "plan_id": plan_id,
        "create": {"status": p1["status"], "id": (p1.get("response_json") or {}).get("id")},
        "publish_first": {"status": pub1["status"], "body": pub1.get("response_json")},
        "publish_second": {"status": pub2["status"], "body": pub2.get("response_json")},
        "plan_get": {"status": pget["status"], "versions": versions_of(pdoc)},
        "plan_get_by_key": {"status": pkb["status"], "versions": versions_of(kbdoc),
                            "plan_status": kbdoc.get("status") if isinstance(kbdoc, dict) else None},
        "expected": "second publish idempotent (200) or 409; exactly one published version",
    }


# ---------- OM-04: order line = grant key om0304-orderline-1 amount 50 USD ----------
@section("om04_orderline", "OM-04 duplicate request / crash-reissue / unique query for one order line")
def om04():
    cust_key = f"{NS}-cust3"
    cust = req("om04-cust3-create", "POST", "/api/v3/openmeter/customers",
               {"key": cust_key, "name": "OM04 order-line customer"})
    cust_id = (cust.get("response_json") or {}).get("id")
    if not cust_id:
        listing = req("om04-find-existing", "GET", "/api/v3/openmeter/customers?limit=100")
        cust_id = next((c.get("id") for c in ((listing.get("response_json") or {}).get("data") or [])
                        if c.get("key") == cust_key), None)
    obody = {"name": "OM04 order line 1", "funding_method": "none", "currency": "USD",
             "amount": "50", "key": "om0304-orderline-1"}
    opath = f"/api/v3/openmeter/customers/{cust_id}/credits/grants"
    o1 = req("om04a-orderline-first", "POST", opath, obody)
    o2 = req("om04a-orderline-duplicate", "POST", opath, obody)
    bal1 = req("om04a-balance-after-dup", "GET", f"{opath}/../balance".replace("/grants/../balance", "/balance"))
    o3 = req("om04b-orderline-crash-reissue", "POST", opath, obody)
    bal2 = req("om04b-balance-after-crash", "GET", f"/api/v3/openmeter/customers/{cust_id}/credits/balance")
    txns = req("om04b-transactions", "GET", f"/api/v3/openmeter/customers/{cust_id}/credits/transactions")
    glist = req("om04c-grant-list", "GET", opath)
    txdata = ((txns.get("response_json") or {}).get("data") or [])
    funded = [t for t in txdata if t.get("usage") == "funded" or t.get("type") == "funded"]
    uniq = [g for g in ((glist.get("response_json") or {}).get("data") or [])
            if g.get("key") == "om0304-orderline-1"]
    return {
        "customer": {"key": cust_key, "id": cust_id, "create_status": cust["status"]},
        "a_duplicate_request": {
            "orderline_body": obody,
            "first": {"status": o1["status"], "id": (o1.get("response_json") or {}).get("id")},
            "duplicate_retry": {"status": o2["status"], "body": o2.get("response_json")},
            "balance_after_duplicate": bal1.get("response_json"),
            "expected_balance": "50 USD, not 100",
        },
        "b_crash_after_external_success": {
            "reissue_status": o3["status"], "response_intentionally_ignored": True,
            "balance_after_crash_reissue": bal2.get("response_json"),
            "transactions_total": len(txdata),
            "funded_entries": funded,
        },
        "c_unique_queryability": {
            "grants_matching_key": uniq, "matching_count": len(uniq),
            "distinct_ids": sorted({g.get("id") for g in uniq}),
            "expected": "exactly one grant id retrievable by key from list-credit-grants",
        },
    }


# ---------- OM-04(d): V2 contrast ----------
@section("om04d_v2", "OM-04(d) V2 duplicate-grant contrast (no dedup key field exists)")
def om04d():
    v2cust = "om02-20260912-v2cust"  # existing OM-02 customer with entitlement om02_tokens
    gpath = f"/api/v2/customers/{v2cust}/entitlements/om02_tokens/grants"
    before = req("om04d-v2-grants-before", "GET", gpath)
    before_ids = [g.get("id") for g in ((before.get("response_json") or {}).get("data") or [])]
    vbody = {"amount": 1, "priority": 1, "effectiveAt": "2026-09-12T03:00:00Z",
             "expiration": {"duration": "HOUR", "count": 1}}
    v1 = req("om04d-v2-grant-first", "POST", gpath, vbody)
    v2 = req("om04d-v2-grant-duplicate", "POST", gpath, vbody)
    after = req("om04d-v2-grants-after", "GET", gpath)
    after_ids = [g.get("id") for g in ((after.get("response_json") or {}).get("data") or [])]
    return {
        "customer_key": v2cust, "request_body": vbody,
        "first": {"status": v1["status"], "id": (v1.get("response_json") or {}).get("id")},
        "duplicate": {"status": v2["status"], "id": (v2.get("response_json") or {}).get("id")},
        "grants_before": len(before_ids), "grants_after": len(after_ids),
        "newly_created_ids": [i for i in after_ids if i not in before_ids],
        "expected_observation": "second POST 201 with a DISTINCT id => V2 has no dedup mechanism",
    }


for fn in (om03a, om03b, om03c, om04, om04d):
    fn()

save("om0304-request-log.json", {"base_url": BASE, "namespace": NS,
                                 "request_count": len(LOG), "requests": LOG})
summary = {}
for k, v in RESULTS.items():
    summary[k] = ("CRASH: " + v.get("section_crash", "")[-200:]) if "section_crash" in v else "ok"
print("SECTIONS:", json.dumps(summary))
print("REQUESTS:", len(LOG))
