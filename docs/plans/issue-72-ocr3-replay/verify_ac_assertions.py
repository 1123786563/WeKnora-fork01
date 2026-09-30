#!/usr/bin/env python3
"""Issue #74 flow verification: per-AC assertions over the real-run evidence.

Adapted from the plan's Task 2 Step 4 script. AC1's invoice-visibility
assertion follows the plan's documented branch: when the gating invoice
stays API-invisible (v1.53.0 INVISIBLE_STATUS `open`), AC1 is asserted from
`status == pass` + subscription incomplete + entitlements 404 held across
the window (plan: `phases.py` invoice-invisible branch, Task 2 Step 4 note).

ocr-3 replay copy, executed against the docs/plans/issue-72-ocr3-replay/
evidence directory. Derived from the ocr-2 copy (97 lines): it keeps the
two "# ocr-2:" annotations and adds two ocr-3-specific assertion
annotations ("# ocr-3:" beside the deferred no-new-invoice and gate-end-state
checks; R1-V23: the previous note here wrongly claimed "unmodified").
Provenance note (ocr-3 R3-19): the empty-checks existence guard on every
all(...values()) call is INHERITED, not added here — the ocr-81 R1-15 fix
landed the same guard in ALL FOUR copies (flow-evidence-74, ocr1, ocr2 and
this one) in one batch, so relative to the ocr-2 copy these calls are
unchanged. What this copy actually ADDS is the ocr-1 R1-37 existence guard
on the invoice-count check. Later copies: anchor descriptions on check
names, not line numbers.
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
check("env.all_phases_pass", bool(env["run"]["phase_statuses"])
      and all(v == "pass" for v in env["run"]["phase_statuses"].values()))

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
    # window — incomplete OR the canceled endgame are both honest gate
    # outcomes.
    # (A-15) The canceled endgame's REACHABLE evidence shape: the recheck
    # reads the subscription through the ?status= filtered query and a
    # canceled subscription 404s there — the harness records recheck as
    # None and the PASS fact rides on cancellation_reason ==
    # "payment_failed" (the harness's own endgame PASS condition). The
    # literal "canceled" recheck value was UNREACHABLE; demanding it would
    # mis-fail every legitimate canceled-endgame evidence.
    check("AC1 invoice branch: gate held across window",
          o.get("recheck_subscription_status") == "incomplete"
          or (o.get("recheck_subscription_status") is None
              and o.get("cancellation_reason") == "payment_failed"))
else:
    # (final-audit 4) invoice_status open OR the visible failed/closed
    # endgame; a pending payment status only pairs with the open shape.
    # (A-15) The authority records the visible endgame invoice_status as
    # the single literal "failed"/"closed" (nothing in the repo produces
    # a composite); the old "closed-failed" literal was produced by no
    # code path and mis-failed every failed-endgame evidence.
    check("AC1 invoice open/pending/numberless",
          (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
           and not o.get("invoice_number"))
          or o.get("invoice_status") in ("failed", "closed"))
    check("AC1 <=1 non-succeeded payment",
          o.get("payments_non_succeeded_count", 99) <= 1)

# AC4: manual (runtime 403 on Community Premium gating)
m = load("t02-manual")
check("AC4 manual.status==pass", m["status"] == "pass")

# AC2: activate + duplicates (activation exactly once, duplicates inert)
a = load("t02-activation")
oa = a["observed"]
check("AC2 activation.status==pass", a["status"] == "pass")
# ocr-81: the all() below is guarded by an existence check — an empty checks
# dict must be a FAIL, never a vacuous pass (same guard style as the len>0
# sub-a/b/c checks in verify_db_watch.py).
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
# ocr-3: the deferred no-new-invoice assertion anchors on the PRE-PROBE
# baseline count (synchronous invoice issuance cannot slip through).
# ocr-1 R1-37: existence-guarded like every other check in this file —
# two missing keys would compare None==None and pass vacuously, and a
# missing evidence.baseline would crash with KeyError instead of a named
# FAIL (the docstring's "an empty checks dict is a FAIL" invariant).
_baseline = (d.get("evidence") or {}).get("baseline") or {}
check("AC2 duplicates invoice count == baseline",
      dr.get("invoice_count") is not None
      and dr.get("invoice_count") == dr.get("invoice_count_baseline")
      and _baseline.get("invoice_count") == dr.get("invoice_count"))

# AC3: retries (same business identity recoverable, no double charge)
r = load("t02-retries")
check("AC3 retries.status==pass", r["status"] == "pass")
check("AC3 checks all true", bool(r["observed"]["checks"])
      and all(r["observed"]["checks"].values()))
# ocr-2: with the gate invoice API-invisible the retry probe is skipped and
# recorded not_applicable (an unknown-id 404 is not retry evidence).
gr = r["observed"]["pending_gate_retry"]
check("AC3 gate retry probe recorded not_applicable",
      gr.get("http_status") is None
      and r["evidence"]["responses"]["retry_payment"].get("code") == "not_applicable")
# ocr-3: the gate's actual end state is part of the evidence.
check("AC3 gate end state recorded (not active)",
      gr.get("subscription_status_after") in ("incomplete", "canceled"))

# negative control: declined charge never activates
c = load("t02-decline")
check("decline.status==pass", c["status"] == "pass")
check("decline checks all true", bool(c["observed"]["checks"])
      and all(c["observed"]["checks"].values()))

print("RESULT:", "ALL PASS" if not fail else f"{len(fail)} FAILED: {fail}")
sys.exit(0 if not fail else 1)
