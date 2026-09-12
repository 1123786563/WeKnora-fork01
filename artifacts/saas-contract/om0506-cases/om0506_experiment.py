#!/usr/bin/env python3
"""OM-05 / OM-06 runtime experiments vs local OpenMeter v1.0.0-beta.232.

Real request/response evidence only (no-proxy urllib, same pattern as om0304).
Fault-isolated sections; every request logged to om0506-request-log.json.
"""
from __future__ import annotations
import json, time, traceback, urllib.error, urllib.request
from datetime import datetime, timezone, timedelta

BASE = "http://127.0.0.1:48888"
NS = "om0506-20260912"
FEAT_ID = "01M29SXXZY6WECC4ZDQPWJVBVP"  # existing feature om02_tokens (meter tokens_total), reused
FEAT_KEY = "om02_tokens"
OUT = "artifacts/saas-contract/om0506-cases"
OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))
LOG, RESULTS = [], {}
now_iso = lambda: datetime.now(timezone.utc).isoformat()

def req(label, method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, method=method)
    if data is not None: r.add_header("Content-Type", "application/json")
    e = {"ts": now_iso(), "label": label, "method": method, "path": path, "request_body": body}
    try:
        with OPENER.open(r, timeout=20) as resp:
            raw, e["status"] = resp.read().decode(), resp.status
    except urllib.error.HTTPError as ex:
        raw, e["status"] = ex.read().decode(), ex.code
    except Exception as ex:
        e.update(status="error", exception=repr(ex)); raw = ""
    e["response_json"] = json.loads(raw) if raw and raw.strip().startswith(("{", "[")) else (raw[:400] or None)
    LOG.append(e); return e

def save(name, payload):
    with open(f"{OUT}/{name}", "w") as f: json.dump(payload, f, ensure_ascii=False, indent=2); f.write("\n")

def section(key, fn):
    try: RESULTS[key] = fn()
    except Exception: RESULTS[key] = {"section_crash": traceback.format_exc()}
    safe = key.replace("(", "").replace(")", "")
    try: save(f"{safe}.json", RESULTS[key])
    except Exception: pass

def v3bal(cid): return req("v3-balance", "GET", f"/api/v3/openmeter/customers/{cid}/credits/balance")["response_json"]
def v3tx(cid): return req("v3-transactions", "GET", f"/api/v3/openmeter/customers/{cid}/credits/transactions")["response_json"]
def v3grants(cid):
    for p in (f"/api/v3/openmeter/customers/{cid}/credits/grants", f"/api/v3/openmeter/customers/{cid}/credit-grants"):
        r = req("v3-list-grants", "GET", p)
        if r["status"] == 200: return r["response_json"]
    return {"failed": r["response_json"]}
def v3evt(eid, subject, tokens, when=None):
    b = {"id": eid, "source": "om0506-probe", "specversion": "1.0", "type": "prompt",
         "subject": subject, "data": {"tokens": tokens, "model": "gpt-4", "type": "input"}}
    if when: b["time"] = when
    return req("v3-ingest-event", "POST", "/api/v3/openmeter/events", [b])

def grant_summaries(cid):
    g = v3grants(cid); items = g.get("items", g.get("data", [])) if isinstance(g, dict) else g
    out = []
    for it in items or []:
        keep = {k: it.get(k) for k in ("id","key","priority","amount","effective_at","expires_at","status","balance","remaining","name") if k in it}
        keep["_all_keys"] = sorted(it.keys())
        out.append(keep)
    return out

# ---------------- S0 customers ----------------
def s0():
    res = {}
    for k in ("custA", "custB", "custF"):
        r = req("create-v3-customer", "POST", "/api/v3/openmeter/customers", {"key": f"{NS}-{k}", "name": f"OM0506 {k}"})
        res[k] = {"key": f"{NS}-{k}", "status": r["status"], "id": (r["response_json"] or {}).get("id") if isinstance(r["response_json"], dict) else None}
    r = req("create-v1-customer", "POST", "/api/v1/customers", {"key": f"{NS}-v2cust", "name": "OM0506 v2cust"})
    res["v2cust"] = {"key": f"{NS}-v2cust", "status": r["status"], "id": (r["response_json"] or {}).get("id") if isinstance(r["response_json"], dict) else None}
    r2 = req("create-v1-customer-sub", "POST", "/api/v1/customers", {"key": f"{NS}-v2sub", "name": "OM0506 v2sub"})
    res["v2sub"] = {"key": f"{NS}-v2sub", "status": r2["status"], "id": (r2["response_json"] or {}).get("id") if isinstance(r2["response_json"], dict) else None}
    return res
section("s0-customers", s0)
cust = RESULTS["s0-customers"]

# ---------------- S1 OM-06(b): PT1M vs PT3M grants, clock starts ----------------
def s1():
    cid = cust["custB"]["id"]; out = {"customer": cust["custB"]}
    r1 = req("grant-pt1m", "POST", f"/api/v3/openmeter/customers/{cid}/credits/grants",
             {"name": "OM0506 PT1M", "funding_method": "none", "currency": "USD", "amount": "40",
              "expires_after": "PT1M", "priority": 10, "key": f"{NS}-b-pt1m"})
    r2 = req("grant-pt3m", "POST", f"/api/v3/openmeter/customers/{cid}/credits/grants",
             {"name": "OM0506 PT3M", "funding_method": "none", "currency": "USD", "amount": "60",
              "expires_after": "PT3M", "priority": 10, "key": f"{NS}-b-pt3m"})
    out["pt1m"] = {"status": r1["status"], "body": r1["response_json"]}
    out["pt3m"] = {"status": r2["status"], "body": r2["response_json"]}
    out["t0"] = now_iso(); out["balance_t0"] = v3bal(cid)
    return out
section("s1-om06b-setup", s1)
b_setup = RESULTS["s1-om06b-setup"]
try: T0 = datetime.fromisoformat(b_setup["t0"])
except Exception: T0 = datetime.now(timezone.utc)

# ---------------- S2 OM-06(d): explicit-timestamp echo probes ----------------
def s2():
    cid = cust["custB"]["id"]; out = {}
    probes = [
        ("leapday", "2028-02-28T23:59:59Z", "PT24H"),   # expect expires_at 2028-02-29T23:59:59Z
        ("monthend-p1m", "2028-01-31T00:00:00Z", "P1M"), # observe month arithmetic (leap Feb)
        ("sec-precision", "2028-12-31T23:59:59Z", "PT1M") # month-end boundary + exact +60s
    ]
    for name, eff, exp in probes:
        r = req(f"echo-{name}", "POST", f"/api/v3/openmeter/customers/{cid}/credits/grants",
                {"name": f"OM0506 echo {name}", "funding_method": "none", "currency": "USD", "amount": "1",
                 "effective_at": eff, "expires_after": exp, "priority": 10, "key": f"{NS}-d-{name}"})
        out[name] = {"status": r["status"], "request": {"effective_at": eff, "expires_after": exp},
                     "echo": {k: (r["response_json"] or {}).get(k) for k in ("effective_at", "expires_at", "status")} if isinstance(r["response_json"], dict) else r["response_json"]}
    return out
section("s2-om06d-echo", s2)

# ---------------- S3 OM-06(a)/OM-05(a) custA: plan+subscription+two-source grants ----------------
def s3():
    out = {}
    plan = {"name": "OM0506 V3 Plan", "key": f"{NS}_v3plan1", "currency": "USD", "billing_cadence": "P1M",
            "phases": [{"key": "main", "name": "Main", "rate_cards": [
                {"key": FEAT_KEY, "name": "OM0506 RC", "feature": {"id": FEAT_ID},
                 "price": {"type": "unit", "amount": "0.01"}, "billing_cadence": "P1M"}]}]}
    r = req("v3-create-plan", "POST", "/api/v3/openmeter/plans", plan)
    out["plan_create"] = {"status": r["status"], "id": (r["response_json"] or {}).get("id") if isinstance(r["response_json"], dict) else None}
    pid = out["plan_create"]["id"]
    r = req("v3-publish-plan", "POST", f"/api/v3/openmeter/plans/{pid}/publish")
    out["plan_publish"] = {"status": r["status"]}
    r = req("v3-create-subscription", "POST", "/api/v3/openmeter/subscriptions",
            {"customer": {"key": f"{NS}-custA"}, "plan": {"key": f"{NS}_v3plan1"}, "settlement_mode": "credit_then_invoice"})
    out["subscription"] = {"status": r["status"], "body": r["response_json"]}
    cid = cust["custA"]["id"]
    r1 = req("grant-period", "POST", f"/api/v3/openmeter/customers/{cid}/credits/grants",
             {"name": "OM0506 period grant", "funding_method": "none", "currency": "USD", "amount": "30",
              "expires_after": "PT2H", "priority": 3, "key": f"{NS}-a-period"})
    r2 = req("grant-topup", "POST", f"/api/v3/openmeter/customers/{cid}/credits/grants",
             {"name": "OM0506 topup grant", "funding_method": "none", "currency": "USD", "amount": "20",
              "priority": 7, "key": f"{NS}-a-topup"})
    out["grants"] = {"period": {"status": r1["status"], "body": r1["response_json"]},
                     "topup": {"status": r2["status"], "body": r2["response_json"]}}
    out["balance_before_burn"] = v3bal(cid)
    return out
section("s3-om06a-om05a-setup", s3)

# ---------------- S4 fractional rate card (custF) ----------------
def s4():
    out = {}
    plan = {"name": "OM0506 V3 Frac Plan", "key": f"{NS}_v3planfrac", "currency": "USD", "billing_cadence": "P1M",
            "phases": [{"key": "main", "name": "Main", "rate_cards": [
                {"key": FEAT_KEY, "name": "OM0506 Frac RC", "feature": {"id": FEAT_ID},
                 "price": {"type": "unit", "amount": "0.0033"}, "billing_cadence": "P1M"}]}]}
    r = req("v3-create-frac-plan", "POST", "/api/v3/openmeter/plans", plan)
    out["plan_create"] = {"status": r["status"], "body": r["response_json"]}
    if r["status"] not in (200, 201): return out
    pid = r["response_json"].get("id")
    r = req("v3-publish-frac-plan", "POST", f"/api/v3/openmeter/plans/{pid}/publish")
    out["publish"] = {"status": r["status"]}
    if r["status"] not in (200, 201): return out
    r = req("v3-frac-subscription", "POST", "/api/v3/openmeter/subscriptions",
            {"customer": {"key": f"{NS}-custF"}, "plan": {"key": f"{NS}_v3planfrac"}, "settlement_mode": "credit_then_invoice"})
    out["subscription"] = {"status": r["status"]}
    cid = cust["custF"]["id"]
    r = req("grant-frac", "POST", f"/api/v3/openmeter/customers/{cid}/credits/grants",
            {"name": "OM0506 frac grant", "funding_method": "none", "currency": "USD", "amount": "5",
             "priority": 10, "key": f"{NS}-f-grant"})
    out["grant"] = {"status": r["status"]}
    return out
section("s4-frac-plan", s4)

# ---------------- S5 OM-05(b) setup: V2 entitlement + rollover grants + consume ----------------
def s5():
    out = {}
    r = req("v2-create-entitlement", "POST", f"/api/v2/customers/{NS}-v2cust/entitlements",
            {"featureKey": FEAT_KEY, "type": "metered", "usagePeriod": {"interval": "DAY"}})
    out["entitlement"] = {"status": r["status"], "body": r["response_json"]}
    eff = datetime.now(timezone.utc).replace(second=0, microsecond=0).isoformat().replace("+00:00", "Z")
    body1 = {"amount": 1000, "priority": 1, "effectiveAt": eff, "expiration": {"duration": "HOUR", "count": 1},
             "maxRolloverAmount": 1000}
    r1 = req("v2-grant-1", "POST", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/grants", body1)
    if r1["status"] not in (200, 201):
        body1.pop("maxRolloverAmount")
        r1 = req("v2-grant-1-retry", "POST", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/grants", body1)
        out["grant1_rollover_field_rejected"] = True
    body2 = {"amount": 500, "priority": 2, "effectiveAt": eff, "expiration": {"duration": "HOUR", "count": 1},
             "maxRolloverAmount": 500}
    r2 = req("v2-grant-2", "POST", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/grants", body2)
    if r2["status"] not in (200, 201):
        body2.pop("maxRolloverAmount")
        r2 = req("v2-grant-2-retry", "POST", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/grants", body2)
    out["grant1"] = {"status": r1["status"], "body": r1["response_json"]}
    out["grant2"] = {"status": r2["status"], "body": r2["response_json"]}
    req("v1-consume-250", "POST", "/api/v1/events",
        {"id": f"{NS}-v2evt-1", "source": "om0506-probe", "specversion": "1.0", "type": "prompt",
         "subject": f"{NS}-v2cust", "data": {"tokens": 250, "model": "gpt-4", "type": "input"}})
    time.sleep(6)
    r = req("v2-value-after-consume", "GET", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/value")
    out["value_after_consume_250"] = r["response_json"]
    out["observed_at"] = now_iso()
    return out
section("s5-om05b-setup-consume", s5)

# ---------------- S6 OM-05(c): V2 subscription auto-grant across reset ----------------
def s6():
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
    pid = (r["response_json"] or {}).get("id")
    r = req("v2-publish-plan", "POST", f"/api/v1/plans/{pid}/publish")
    out["publish"] = {"status": r["status"]}
    r = req("v2-create-subscription", "POST", "/api/v1/subscriptions",
            {"plan": {"key": f"{NS}_v2plan1"}, "customerKey": f"{NS}-v2sub", "name": "OM0506 V2 Sub"})
    out["subscription"] = {"status": r["status"], "body": r["response_json"] if r["status"] != 201 else {"id": (r["response_json"] or {}).get("id"), "status": (r["response_json"] or {}).get("status")}}
    time.sleep(4)
    r = req("v2-list-grants-before", "GET", f"/api/v2/customers/{NS}-v2sub/entitlements/{FEAT_KEY}/grants")
    gb = r["response_json"] if isinstance(r["response_json"], list) else (r["response_json"] or {}).get("list", [])
    out["grants_before_reset"] = gb
    # manual reset = new usage period boundary (minute-truncated effectiveAt)
    eff = datetime.now(timezone.utc).replace(second=0, microsecond=0).isoformat().replace("+00:00", "Z")
    r = req("v2-reset", "POST", f"/api/v2/customers/{NS}-v2sub/entitlements/{FEAT_KEY}/reset",
            {"effectiveAt": eff, "retainAnchor": True})
    out["reset"] = {"status": r["status"], "request_effectiveAt": eff, "body": r["response_json"]}
    time.sleep(3)
    r = req("v2-list-grants-after", "GET", f"/api/v2/customers/{NS}-v2sub/entitlements/{FEAT_KEY}/grants")
    ga = r["response_json"] if isinstance(r["response_json"], list) else (r["response_json"] or {}).get("list", [])
    out["grants_after_reset"] = ga
    out["counts"] = {"before": len(gb) if isinstance(gb, list) else None, "after": len(ga) if isinstance(ga, list) else None}
    r = req("v2-value-after-reset", "GET", f"/api/v2/customers/{NS}-v2sub/entitlements/{FEAT_KEY}/value")
    out["value_after_reset"] = r["response_json"]
    out["observed_at"] = now_iso()
    return out
section("s6-om05c-autogrant", s6)

# ---------------- S7 OM-06(a) burn-down ordering + precision ----------------
def s7():
    cid = cust["custA"]["id"]; out = {"burns": []}
    for n, tok in (("burn1", 500), ("burn2", 2500), ("burn3", 1500)):
        v3evt(f"{NS}-a-evt-{n}", f"{NS}-custA", tok)
        time.sleep(7)
        out["burns"].append({"label": n, "tokens": tok, "usd_expected": f"{tok*0.01:.2f}",
                             "balance": v3bal(cid), "observed_at": now_iso()})
    out["grants_after_burns"] = grant_summaries(cid)
    # fractional precision on custF: 101 x 0.0033 = 0.3333
    if RESULTS.get("s4-frac-plan", {}).get("subscription", {}).get("status") == 201:
        v3evt(f"{NS}-f-evt-1", f"{NS}-custF", 101)
        time.sleep(7)
        out["frac_burn"] = {"tokens": 101, "expected_raw": "0.3333",
                            "balance": v3bal(cust["custF"]["id"]),
                            "transactions": v3tx(cust["custF"]["id"]), "observed_at": now_iso()}
    out["transactions_custA"] = v3tx(cid)
    return out
section("s7-om06a-burndown", s7)

# ---------------- S8 OM-06(e) late-reported event ----------------
def s8():
    cid = cust["custA"]["id"]; out = {}
    out["balance_before_late"] = v3bal(cid)
    late = (datetime.now(timezone.utc) - timedelta(seconds=300)).isoformat().replace("+00:00", "Z")
    r = v3evt(f"{NS}-a-evt-late", f"{NS}-custA", 200, when=late)
    out["late_event"] = {"status": r["status"], "event_time": late, "ingested_at": now_iso(), "tokens": 200}
    time.sleep(8)
    out["balance_after_late"] = v3bal(cid)
    out["transactions_after_late"] = v3tx(cid)
    out["observed_at"] = now_iso()
    return out
section("s8-om06e-late", s8)

# ---------------- S10 OM-05(b) reset boundary ----------------
def s10():
    out = {}
    r = req("v2-value-before-reset", "GET", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/value")
    out["value_before_reset"] = r["response_json"]
    eff = datetime.now(timezone.utc).replace(second=0, microsecond=0).isoformat().replace("+00:00", "Z")
    r = req("v2-reset-main", "POST", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/reset",
            {"effectiveAt": eff, "retainAnchor": True})
    out["reset"] = {"status": r["status"], "request_effectiveAt": eff, "body": r["response_json"]}
    time.sleep(3)
    r = req("v2-value-after-reset", "GET", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/value")
    out["value_after_reset"] = r["response_json"]
    r = req("v2-grants-after-reset", "GET", f"/api/v2/customers/{NS}-v2cust/entitlements/{FEAT_KEY}/grants")
    out["grants_after_reset"] = r["response_json"]
    out["observed_at"] = now_iso()
    return out
section("s10-om05b-reset", s10)

# ---------------- S9 OM-06(b) time-lapse: poll PT1M expiry ----------------
def s9():
    cid = cust["custB"]["id"]; out = {"t0": b_setup.get("t0"), "polls": []}
    deadline = T0 + timedelta(seconds=100)
    seen_expired = False
    while datetime.now(timezone.utc) < deadline:
        elapsed = (datetime.now(timezone.utc) - T0).total_seconds()
        bal, grants = v3bal(cid), grant_summaries(cid)
        st = {g.get("key", g.get("id")): {"status": g.get("status"), "expires_at": g.get("expires_at")} for g in grants}
        poll = {"t_plus_s": round(elapsed, 1), "observed_at": now_iso(),
                "live": bal.get("balances", [{}])[0].get("live") if isinstance(bal, dict) else None,
                "balance_raw": bal, "grants": st}
        out["polls"].append(poll)
        if elapsed >= 70 and any(v["status"] == "expired" for v in st.values()):
            seen_expired = True
            time.sleep(8)  # one confirmation poll after transition
            bal2, g2 = v3bal(cid), grant_summaries(cid)
            out["post_expiry_confirmation"] = {
                "t_plus_s": round((datetime.now(timezone.utc) - T0).total_seconds(), 1),
                "balance_raw": bal2,
                "grants": {x.get("key", x.get("id")): {"status": x.get("status"), "expires_at": x.get("expires_at")} for x in g2}}
            break
        time.sleep(10 if elapsed < 55 else 6)
    out["seen_expired"] = seen_expired
    return out
section("s9-om06b-timelapse", s9)

with open(f"{OUT}/om0506-request-log.json", "w") as f: json.dump(LOG, f, ensure_ascii=False, indent=2)
save("om0506-summary", {"results": {k: (v if not isinstance(v, dict) or len(json.dumps(v)) < 400 else "see per-section artifact") for k, v in RESULTS.items()}, "request_count": len(LOG)})
print(json.dumps({k: (list(v)[:8] if isinstance(v, dict) else type(v).__name__) for k, v in RESULTS.items()}, indent=1))
print("requests:", len(LOG))
for k in ("s1-om06b-setup", "s2-om06d-echo"):
    print(k, json.dumps(RESULTS.get(k), default=str)[:600])
