#!/usr/bin/env python3
"""OM-05/06 phase 2: fix plan keys (underscore-only), run burn-down, late-reported usage, resets."""
from __future__ import annotations
import json, time, traceback, urllib.error, urllib.request
from datetime import datetime, timezone, timedelta

BASE = "http://127.0.0.1:48888"; NS = "om0506-20260912"
FEAT_ID = "01M29SXXZY6WECC4ZDQPWJVBVP"; FEAT_KEY = "om02_tokens"
OUT = "artifacts/saas-contract/om0506-cases"
OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))
LOG, RESULTS = [], []
now_iso = lambda: datetime.now(timezone.utc).isoformat()

def req(label, method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, method=method)
    if data is not None: r.add_header("Content-Type", "application/json")
    e = {"ts": now_iso(), "label": label, "method": method, "path": path, "request_body": body}
    try:
        with OPENER.open(r, timeout=20) as resp: raw, e["status"] = resp.read().decode(), resp.status
    except urllib.error.HTTPError as ex: raw, e["status"] = ex.read().decode(), ex.code
    except Exception as ex: e.update(status="error", exception=repr(ex)); raw = ""
    e["response_json"] = json.loads(raw) if raw and raw.strip().startswith(("{", "[")) else (raw[:300] or None)
    LOG.append(e); return e

def save(n, p):
    with open(f"{OUT}/{n}", "w") as f: json.dump(p, f, ensure_ascii=False, indent=2); f.write("\n")

def section(key, fn):
    try: res = fn()
    except Exception: res = {"section_crash": traceback.format_exc()}
    RESULTS.append(key); save(key, res)

def v3bal(cid): return req("v3-balance", "GET", f"/api/v3/openmeter/customers/{cid}/credits/balance")["response_json"]
def v3tx(cid): return req("v3-tx", "GET", f"/api/v3/openmeter/customers/{cid}/credits/transactions")["response_json"]
def v3grants(cid):
    r = req("v3-list-grants", "GET", f"/api/v3/openmeter/customers/{cid}/credits/grants")
    return r["response_json"] if r["status"] == 200 else {"failed": r["response_json"]}
def grant_sum(cid):
    g = v3grants(cid); items = g.get("items", g.get("data", [])) if isinstance(g, dict) else g
    return [{k: it.get(k) for k in ("id","key","priority","amount","status","expires_at","name") if k in it} for it in items or []]
def v3evt(eid, subj, tok, when=None):
    b = {"id": eid, "source": "om0506-probe", "specversion": "1.0", "type": "prompt", "subject": subj,
         "data": {"tokens": tok, "model": "gpt-4", "type": "input"}}
    if when: b["time"] = when
    return req("v3-ingest", "POST", "/api/v3/openmeter/events", [b])
def v2val(cust): return req("v2-value", "GET", f"/api/v2/customers/{cust}/entitlements/{FEAT_KEY}/value")["response_json"]
def v2grants(cust):
    r = req("v2-list-grants", "GET", f"/api/v2/customers/{cust}/entitlements/{FEAT_KEY}/grants")
    if r["status"] != 200: return {"_status": r["status"], "_body": r["response_json"]}
    body = r["response_json"]
    return body if isinstance(body, list) else body.get("items", body.get("list", []))

cust = json.load(open(f"{OUT}/s0-customers.json"))
A, F = cust["custA"]["id"], cust["custF"]["id"]

# ---- P1: V3 plans with valid underscore keys + subscriptions ----
def p1():
    out = {}
    for tag, key, amt, custk in (("main", f"{NS}_v3plan1", "0.01", f"{NS}-custA"),
                                  ("frac", f"{NS}_v3planf", "0.0033", f"{NS}-custF")):
        plan = {"name": f"OM0506 V3 {tag}", "key": key, "currency": "USD", "billing_cadence": "P1M",
                "phases": [{"key": "main", "name": "Main", "rate_cards": [
                    {"key": FEAT_KEY, "name": f"OM0506 {tag} RC", "feature": {"id": FEAT_ID},
                     "price": {"type": "unit", "amount": amt}, "billing_cadence": "P1M"}]}]}
        r = req(f"v3-plan-{tag}", "POST", "/api/v3/openmeter/plans", plan)
        s = {"create": r["status"]}
        if r["status"] in (200, 201):
            pid = r["response_json"].get("id")
            r2 = req(f"v3-publish-{tag}", "POST", f"/api/v3/openmeter/plans/{pid}/publish")
            s["publish"] = r2["status"]
            r3 = req(f"v3-sub-{tag}", "POST", "/api/v3/openmeter/subscriptions",
                     {"customer": {"key": custk}, "plan": {"key": key}, "settlement_mode": "credit_then_invoice"})
            s["subscription"] = r3["status"]
            if r3["status"] not in (200, 201): s["subscription_body"] = r3["response_json"]
        else:
            s["body"] = r["response_json"]
        out[tag] = s
    time.sleep(2); out["balanceA_start"] = v3bal(A)
    return out
section("p1-plans-subs", p1)

# ---- P2: OM-06(a) burn-down ordering + precision; OM-05(a) topup preservation ----
def p2():
    out = {"burns": []}
    for n, tok in (("burn1", 500), ("burn2", 2500), ("burn3", 1500)):
        st = v3evt(f"{NS}-a2-evt-{n}", f"{NS}-custA", tok)["status"]
        time.sleep(8)
        out["burns"].append({"label": n, "tokens": tok, "usd_expected": f"{tok/100:.2f}",
                             "ingest_status": st, "balance": v3bal(A), "observed_at": now_iso()})
    out["grants_after_burns"] = grant_sum(A)
    out["transactions_after_burns"] = v3tx(A)
    return out
section("p2-om06a-burndown", p2)

# ---- P3: fractional rate burn (custF): 101 x 0.0033 = 0.3333 ----
def p3():
    out = {}
    if json.load(open(f"{OUT}/p1-plans-subs.json")).get("frac", {}).get("subscription") not in (200, 201):
        return {"skipped": "frac plan/subscription not created"}
    st = v3evt(f"{NS}-f2-evt-1", f"{NS}-custF", 101)["status"]
    time.sleep(8)
    return {"ingest_status": st, "tokens": 101, "rate": "0.0033", "expected_raw": "0.3333",
            "balance": v3bal(F), "transactions": v3tx(F), "observed_at": now_iso()}
section("p3-frac-precision", p3)

# ---- P4: OM-06(e) late-reported events (older explicit time) ----
def p4():
    out = {"before": v3bal(A)}
    now = datetime.now(timezone.utc)
    late1 = (now - timedelta(seconds=240)).isoformat().replace("+00:00", "Z")
    late2 = (now - timedelta(seconds=75)).isoformat().replace("+00:00", "Z")
    r1 = v3evt(f"{NS}-a2-evt-late240", f"{NS}-custA", 200, when=late1)
    r2 = v3evt(f"{NS}-a2-evt-late75", f"{NS}-custA", 300, when=late2)
    out["events"] = [{"time": late1, "tokens": 200, "status": r1["status"]}, {"time": late2, "tokens": 300, "status": r2["status"]}]
    out["ingested_at"] = now_iso()
    time.sleep(9)
    out["balance_after"] = v3bal(A)
    out["transactions_after"] = v3tx(A)
    out["observed_at"] = now_iso()
    return out
section("p4-om06e-late", p4)

# ---- P5: OM-05(b) reset on v2cust (strictly-after last reset) ----
def p5():
    out = {"value_before": v2val(f"{NS}-v2cust")}
    eff = datetime.now(timezone.utc).replace(second=0, microsecond=0)
    if eff.second == 0 and (datetime.now(timezone.utc).second == 0): pass
    eff = eff.isoformat().replace("+00:00", "Z")
    r = req("v2-reset-main", "POST", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/reset",
            {"effectiveAt": eff, "retainAnchor": True})
    out["reset"] = {"status": r["status"], "request_effectiveAt": eff, "body": r["response_json"]}
    time.sleep(4)
    out["value_after"] = v2val(f"{NS}-v2cust")
    out["grants_after"] = v2grants(f"{NS}-v2cust")
    out["observed_at"] = now_iso()
    return out
section("p5-om05b-reset", p5)

# ---- P6: OM-05(c) V2 subscription auto-grant across reset ----
def p6():
    out = {}
    plan = {"name": "OM0506 V2 Plan", "key": f"{NS}_v2plan1", "currency": "USD", "billingCadence": "P1M",
            "phases": [{"key": "main", "name": "Main", "rateCards": [
                {"type": "usage_based", "key": FEAT_KEY, "name": "OM0506 V2 RC", "featureKey": FEAT_KEY,
                 "price": {"type": "unit", "amount": "0.01"},
                 "entitlementTemplate": {"type": "metered", "issueAfterReset": 1}, "billingCadence": "P1M"}],
                "duration": None}]}
    r = req("v2-create-plan", "POST", "/api/v1/plans", plan)
    out["plan_create"] = {"status": r["status"]}
    if r["status"] not in (200, 201): out["plan_create"]["body"] = r["response_json"]; return out
    pid = r["response_json"].get("id")
    out["publish"] = {"status": req("v2-publish", "POST", f"/api/v1/plans/{pid}/publish")["status"]}
    r = req("v2-sub", "POST", "/api/v1/subscriptions", {"plan": {"key": f"{NS}_v2plan1"}, "customerKey": f"{NS}-v2sub", "name": "OM0506 V2 Sub"})
    out["subscription"] = {"status": r["status"]}
    time.sleep(5)
    out["grants_before_reset"] = v2grants(f"{NS}-v2sub")
    out["value_before_reset"] = v2val(f"{NS}-v2sub")
    eff = datetime.now(timezone.utc).replace(second=0, microsecond=0).isoformat().replace("+00:00", "Z")
    r = req("v2-reset-sub", "POST", f"/api/v2/customers/{NS}-v2sub/entitlements/{FEAT_KEY}/reset",
            {"effectiveAt": eff, "retainAnchor": True})
    out["reset"] = {"status": r["status"], "request_effectiveAt": eff, "body": r["response_json"]}
    time.sleep(4)
    out["grants_after_reset"] = v2grants(f"{NS}-v2sub")
    out["value_after_reset"] = v2val(f"{NS}-v2sub")
    nb, na = out["grants_before_reset"], out["grants_after_reset"]
    if isinstance(nb, list) and isinstance(na, list):
        out["counts"] = {"before": len(nb), "after": len(na),
                         "ids_before": sorted(g.get("id") for g in nb), "ids_after": sorted(g.get("id") for g in na)}
    out["observed_at"] = now_iso()
    return out
section("p6-om05c-autogrant", p6)

with open(f"{OUT}/om0506-request-log-phase2.json", "w") as f: json.dump(LOG, f, ensure_ascii=False, indent=2)
print(json.dumps(RESULTS)); print("requests:", len(LOG))
