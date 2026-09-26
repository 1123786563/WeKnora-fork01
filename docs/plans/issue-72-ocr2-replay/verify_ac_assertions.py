#!/usr/bin/env python3
"""Issue #74 flow verification: per-AC assertions over the real-run evidence.

Adapted from the plan's Task 2 Step 4 script. AC1's invoice-visibility
assertion follows the plan's documented branch: when the gating invoice
stays API-invisible (v1.53.0 INVISIBLE_STATUS `open`), AC1 is asserted from
`status == pass` + subscription incomplete + entitlements 404 held across
the window (plan: `phases.py` invoice-invisible branch, Task 2 Step 4 note).

ocr-2 replay copy, executed against the docs/plans/issue-72-ocr2-replay/
evidence directory. Relative to the ocr-1 copy (86 lines) this copy adds
two ocr-2-specific assertion annotations: "# ocr-2:" at lines 74 and 84
(R1-V23: the previous note here wrongly claimed "unmodified").
"""
import json
import sys
from pathlib import Path

EVID = Path(__file__).resolve().parent


def load(n):
    return json.loads((EVID / f"{n}.json").read_text())


fail = []


def check(name, cond):
    print(("PASS " if cond else "FAIL ") + name)
    if not cond:
        fail.append(name)


env = load("t02-environment")
check("env.overall==pass", env["run"]["overall"] == "pass")
check("env.release==v1.53.0", env["release"]["release"] == "v1.53.0")
check("env.stripe.test_mode_key_present", env["stripe"]["test_mode_key_present"] is True)
check("env.graphql_login_ok", env["lago"]["graphql_login_ok"] is True)
check("env.secrets_scan.clean", env["secrets_scan"]["clean"] is True)
check("env.all_phases_pass", bool(env["run"]["phase_statuses"]) and all(v == "pass" for v in env["run"]["phase_statuses"].values()))

# AC1: gate (payment-gated subscription stays incomplete, entitlements unusable)
g = load("t02-gating")
o = g["observed"]
check("AC1 gate.status==pass", g["status"] == "pass")
check("AC1 subscription incomplete", o["subscription_status"] == "incomplete")
check("AC1 entitlements 404", o["entitlements_status"] == 404)
if o.get("invoice_api_visible") is False:
    # documented branch: invoice API-invisible; AC1 on core observation held.
    # (final-audit 4) The recheck accepts the charge-failure ENDGAME too:
    # the gate may have lapsed to canceled(payment_failed) inside the
    # window — incomplete OR canceled are both honest gate outcomes.
    check("AC1 invoice branch: gate held across window",
          o.get("recheck_subscription_status") in ("incomplete", "canceled"))
else:
    # (final-audit 4) invoice_status open OR the closed-failed endgame;
    # a pending payment status only pairs with the open shape.
    check("AC1 invoice open/pending/numberless",
          (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
           and not o.get("invoice_number"))
          or o.get("invoice_status") == "closed-failed")
    check("AC1 <=1 non-succeeded payment",
          o.get("payments_non_succeeded_count", 99) <= 1)

# AC4: manual (runtime 403 on Community Premium gating)
m = load("t02-manual")
check("AC4 manual.status==pass", m["status"] == "pass")

# AC2: activate + duplicates (activation exactly once, duplicates inert)
a = load("t02-activation")
oa = a["observed"]
check("AC2 activation.status==pass", a["status"] == "pass")
check("AC2 checks all true", bool(oa["checks"]) and all(oa["checks"].values()))
check("AC2 exactly one succeeded payment", oa["payments_succeeded_count"] == 1)
check("AC2 provider_payment_id recorded", bool(oa["provider_payment_id"]))
d = load("t02-duplicates")
check("AC2 duplicates.status==pass", d["status"] == "pass")
fin = d["observed"]["final"]
check("AC2 duplicates final state intact",
      fin["payments_succeeded_count"] == 1 and fin["subscription_status"] == "active")
# ocr-2: the PASS now also stands on the deferred re-check after the settle
# window (minutes-late drift from a 200-answered duplicate would fail here).
dr = d["observed"].get("deferred_recheck", {})
check("AC2 duplicates deferred re-check clean",
      dr.get("checked") is True and dr.get("ok") is True)

# AC3: retries (same business identity recoverable, no double charge)
r = load("t02-retries")
check("AC3 retries.status==pass", r["status"] == "pass")
check("AC3 checks all true", bool(r["observed"]["checks"]) and all(r["observed"]["checks"].values()))
# ocr-2: with the gate invoice API-invisible the retry probe is skipped and
# recorded not_applicable (an unknown-id 404 is not retry evidence).
gr = r["observed"]["pending_gate_retry"]
check("AC3 gate retry probe recorded not_applicable",
      gr.get("http_status") is None
      and r["evidence"]["responses"]["retry_payment"].get("code") == "not_applicable")

# negative control: declined charge never activates
c = load("t02-decline")
check("decline.status==pass", c["status"] == "pass")
check("decline checks all true", bool(c["observed"]["checks"]) and all(c["observed"]["checks"].values()))

print("RESULT:", "ALL PASS" if not fail else f"{len(fail)} FAILED: {fail}")
sys.exit(0 if not fail else 1)
