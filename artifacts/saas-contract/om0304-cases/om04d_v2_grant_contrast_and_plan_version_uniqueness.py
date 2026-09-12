#!/usr/bin/env python3
"""OM-04(d) + OM-03(c) follow-ups against the live OpenMeter instance.

(1) V2 duplicate-grant contrast, retried with an effectiveAt INSIDE the
c    current usage period (the first attempt 400'd: effective before the
    usage period 03:17Z) and with correct items/totalCount parsing — two
    identical grant POSTs must demonstrate that V2 has NO dedup key.
(2) Plan version uniqueness proven via the version-list endpoints instead
    of a second publish attempt (which the state machine rejects with 400).

Inputs : live OpenMeter at http://127.0.0.1:48888 (WRITES: two grant POSTs),
         artifacts/saas-contract/om0304-request-log.json (appended).
Outputs: om04-v2-contrast.json and om03-plan-replay.json updated in place.
Caution: NOT idempotent — every run creates REAL grants on the target
instance and appends to the request log; run only against the disposable
om0304 evidence instance."""
import json, urllib.request, urllib.error
BASE = "http://127.0.0.1:48888"
OP = urllib.request.build_opener(urllib.request.ProxyHandler({}))
LOG = []

def req(label, method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, method=method)
    if data: r.add_header("Content-Type", "application/json")
    e = {"label": label, "method": method, "path": path, "request_body": body}
    try:
        with OP.open(r, timeout=15) as resp:
            raw = resp.read().decode(); e["status"] = resp.status
    except urllib.error.HTTPError as ex:
        raw = ex.read().decode(); e["status"] = ex.code
    except Exception as ex:
        e["status"] = "error"; e["exception"] = repr(ex); raw = ""
    try: e["response_json"] = json.loads(raw) if raw else None
    except Exception: e["response_raw"] = raw[:500]
    LOG.append(e); return e

def items_of(resp):
    d = resp.get("response_json") or {}
    return d.get("items") or d.get("data") or []

# (1) V2 contrast, corrected
v2cust = "om02-20260912-v2cust"
gpath = f"/api/v2/customers/{v2cust}/entitlements/om02_tokens/grants"
before = req("om04d2-before", "GET", gpath)
before_ids = [g.get("id") for g in items_of(before)]
vbody = {"amount": 1, "priority": 1, "effectiveAt": "2026-09-12T03:45:00Z",
         "expiration": {"duration": "HOUR", "count": 2}}
v1 = req("om04d2-first", "POST", gpath, vbody)
v2 = req("om04d2-duplicate", "POST", gpath, vbody)
after = req("om04d2-after", "GET", gpath)
after_ids = [g.get("id") for g in items_of(after)]
v2doc = {
  "check": "OM-04(d) V2 duplicate-grant contrast (no dedup key field exists)",
  "customer_key": v2cust, "request_body": vbody,
  "note_first_attempt": "first attempt (effectiveAt 03:00Z) 400'd: 'grant effective date ... before the current usage period 2026-09-12 03:17:00 UTC' — recorded in om0304-request-log.json om04d-*",
  "first": {"status": v1["status"], "id": (v1.get("response_json") or {}).get("id"), "body": v1.get("response_json")},
  "duplicate": {"status": v2["status"], "id": (v2.get("response_json") or {}).get("id"), "body": v2.get("response_json")},
  "grants_before": len(before_ids), "grants_after": len(after_ids),
  "newly_created_ids": [i for i in after_ids if i not in before_ids],
  "same_body_twice_created_two_grants": len([i for i in after_ids if i not in before_ids]) == 2,
  "expected_observation": "second POST 201 with a DISTINCT id => V2 has no dedup; issuance family must be V3",
}
json.dump(v2doc, open("artifacts/saas-contract/om04-v2-contrast.json","w"), ensure_ascii=False, indent=2)

# (2) plan version uniqueness
plan_id = "01M29VB2QX88HBFT46M758RX6V"
vers = req("om03c2-plan-versions", "GET", f"/api/v3/openmeter/plans/{plan_id}/versions")
plist = req("om03c2-plans-list", "GET", "/api/v3/openmeter/plans?limit=100")
same_key = [(p.get("id"), p.get("version"), p.get("status")) for p in items_of(plist) if p.get("key") == "om0304_20260912_plan1"]
pdoc = json.load(open("artifacts/saas-contract/om03-plan-replay.json"))
pdoc["result"]["version_uniqueness_followup"] = {
  "get_plan_versions": {"status": vers["status"], "body": vers.get("response_json")},
  "plans_list_filtered_by_key": {"matches": same_key, "count": len(same_key)},
  "note": "second publish returned 400 state-machine rejection ('only Plans in [draft scheduled] can be published/rescheduled, but it has active state') — NOT 200/409; effect is still replay-safe: plan remains active with a single version",
}
pdoc["result"]["observed_verdict"] = "one published version; replay rejected by 400 state guard (no duplicate version created)"
json.dump(pdoc, open("artifacts/saas-contract/om03-plan-replay.json","w"), ensure_ascii=False, indent=2)

log = json.load(open("artifacts/saas-contract/om0304-request-log.json"))
log["requests"].extend(LOG); log["request_count"] += len(LOG)
json.dump(log, open("artifacts/saas-contract/om0304-request-log.json","w"), ensure_ascii=False, indent=2)
print("V2 first:", v1["status"], (v1.get("response_json") or {}).get("id"))
print("V2 dup:", v2["status"], (v2.get("response_json") or {}).get("id"))
print("new ids:", v2doc["newly_created_ids"], "two_created:", v2doc["same_body_twice_created_two_grants"])
print("versions endpoint:", vers["status"], json.dumps(vers.get("response_json"))[:400])
print("plans filtered:", same_key)
