#!/usr/bin/env python3
"""Fix-up: rebuild om03a artifact from run-2 request log (sorted() crash lost
the in-memory analysis AFTER the 8 concurrent POSTs had already fired), do the
uniqueness GETs now, and normalize artifact filenames."""
import json, shutil, urllib.request, urllib.error
BASE = "http://127.0.0.1:48888"
NS = "om0304-20260912"
OP = urllib.request.build_opener(urllib.request.ProxyHandler({}))

def get(path):
    r = urllib.request.Request(BASE + path)
    try:
        with OP.open(r, timeout=15) as resp:
            return resp.status, json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try: return e.code, json.loads(raw)
        except Exception: return e.code, raw[:300]

log = json.load(open("artifacts/saas-contract/om0304-request-log.json"))
att = [e for e in log["requests"] if e["label"].startswith("om03a-concurrent-")]
attempts = [{"worker": int(e["label"].rsplit("-",1)[1]), "status": e["status"],
             "returned_id": (e.get("response_json") or {}).get("id") if isinstance(e.get("response_json"), dict) else None,
             "request_name": (e.get("request_body") or {}).get("name")}
            for e in att]
attempts.sort(key=lambda a: a["worker"])
statuses = [str(a["status"]) for a in attempts]
winners = [a for a in attempts if a["status"] == 201]
winner_id = winners[0]["returned_id"] if winners else None
cust_key = f"{NS}-cust1r2"
s_id, d_id = get(f"/api/v3/openmeter/customers/{winner_id}")
s_key, d_key = get(f"/api/v3/openmeter/customers/{cust_key}")
s_list, d_list = get("/api/v3/openmeter/customers?limit=100")
listed = [c.get("id") for c in (d_list.get("data") or []) if c.get("key") == cust_key]
s_prior, d_prior = get("/api/v3/openmeter/customers/01M29V7MWK3KRK6WZVX51PJ1KM")
out = {
  "check": "OM-03(a) concurrent same-key customer creation, N=8 (ThreadPoolExecutor+Barrier, run-2 key cust1r2)",
  "method_note": "8 threads, threading.Barrier(8) for simultaneity; analysis rebuilt from om0304-request-log.json because the driver crashed on sorted() AFTER the POSTs fired (crash recorded, requests are primary evidence)",
  "attempts": attempts,
  "summary": {"key": cust_key, "n_attempts": len(attempts), "n_201": statuses.count("201"),
              "n_409": statuses.count("409"), "other_statuses": sorted(set(statuses)-{"201","409"}),
              "distinct_returned_ids": sorted({a["returned_id"] for a in attempts if a["returned_id"]}),
              "expected": "exactly one 201 winner, seven 409, one distinct id"},
  "get_by_id": {"status": s_id, "id": (d_id or {}).get("id"), "key": (d_id or {}).get("key")},
  "get_by_key": {"status": s_key, "id": (d_key or {}).get("id") if isinstance(d_key, dict) else None,
                 "key": (d_key or {}).get("key") if isinstance(d_key, dict) else None},
  "list_filtered_client_side": {"status": s_list, "ids_with_same_key": listed, "count": len(listed)},
  "prior_run1_customer": {"id": "01M29V7MWK3KRK6WZVX51PJ1KM", "status": s_prior,
                          "key": (d_prior or {}).get("key") if isinstance(d_prior, dict) else None},
}
json.dump(out, open("artifacts/saas-contract/om03-concurrent-customer.json","w"), ensure_ascii=False, indent=2)
for src, dst in [("om03b-retry.json","om03-retry-replay.json"), ("om03c-plan.json","om03-plan-replay.json"), ("om04d-v2.json","om04-v2-contrast.json")]:
    shutil.move(f"artifacts/saas-contract/{src}", f"artifacts/saas-contract/{dst}")
shutil.move("artifacts/saas-contract/om03a-concurrent.json", "artifacts/saas-contract/om03a-concurrent-crashrecord.json")
print(json.dumps(out["summary"], indent=1))
print("get_by_id", out["get_by_id"], "| get_by_key", out["get_by_key"], "| listed", listed)
print("--- om03b ---"); print(open("artifacts/saas-contract/om03-retry-replay.json").read())
print("--- om03c ---"); print(open("artifacts/saas-contract/om03-plan-replay.json").read())
print("--- om04d ---"); print(open("artifacts/saas-contract/om04-v2-contrast.json").read())
