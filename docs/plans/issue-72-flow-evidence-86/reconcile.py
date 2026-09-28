#!/usr/bin/env python3
"""Issue #86 AC reconciliation: WeKnora billing page (account API) vs Lago
authority wallets for the flow tenant.

Reads:
  - weknora-account-api.json (captured from the authenticated browser session)
  - GET /api/v1/customers/:id/wallets on the Lago authority

Asserts (all must hold for RECONCILE PASS):
  1. sum(active wallet balance_cents) * 10^4 == WeKnora balance_micro
  2. every active wallet appears in WeKnora batches with the same balance,
     expiry and grant time; no extra active wallet, no missing wallet
  3. terminated wallets never contribute to balance (carry-over forbidden)
  4. available_micro == balance_micro - held_micro - refund_locked_micro

Egress policy: this is a LOCAL TEST-STACK verification script; the only
admitted outbound target is the explicit loopback test address below
(declared allow-list, scheme-validated, no redirects followed). Credentials
come from the server-side .env only and never appear in artifacts.
"""
import json
import os
import sys
from pathlib import Path
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler
from urllib.error import HTTPError

ALLOWED_TARGETS = {"127.0.0.1"}  # the local Lago test stack, nothing else
CUSTOMER = os.environ.get("LAGO_CUSTOMER", "weknora-tenant-10000")
HERE = Path(__file__).resolve().parent


def base_url():
    raw = os.environ.get("LAGO_BASE", "http://127.0.0.1:48889")
    parts = urlsplit(raw)
    if parts.scheme not in ("http", "https"):
        raise SystemExit(f"scheme not allowed: {parts.scheme}")
    if parts.hostname not in ALLOWED_TARGETS:
        raise SystemExit(f"target host not in test-stack allow-list: "
                         f"{parts.hostname}")
    return raw


class _NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None  # never follow redirects


def api_key():
    env_path = HERE.parents[2] / ".env"
    for line in env_path.read_text(encoding="utf-8").splitlines():
        if line.startswith("WEKNORA_COMMERCIAL_PLATFORM_API_KEY="):
            return line.strip().split("=", 1)[1]
    raise SystemExit("api key not found in .env")


def lago_wallets():
    url = base_url() + "/api/v1/customers/" + CUSTOMER + "/wallets?per_page=20"
    req = Request(url, headers={"Authorization": "Bearer " + api_key()})
    with build_opener(_NoRedirect).open(req, timeout=30) as resp:
        return json.load(resp)["wallets"]


def main():
    src = os.environ.get("WK_ACCOUNT_JSON", "weknora-account-api.json")
    credits = json.loads((HERE / src).read_text(
        encoding="utf-8"))["data"]["benefits"]["credits"]
    wallets = lago_wallets()
    active = [w for w in wallets if w.get("status") == "active"]
    terminated = [w for w in wallets if w.get("status") == "terminated"]

    checks = []

    total_cents = sum(w["balance_cents"] for w in active)
    checks.append(("sum(active balance_cents)*10^4 == balance_micro",
                   total_cents * 10_000 == int(credits["balance_micro"]),
                   f"lago={total_cents}c page={credits['balance_micro']}u"))

    page_batches = {b["expires_at"]: b for b in credits["batches"]}
    lago_active = {w["expiration_at"]: w for w in active}
    detail = []
    per_batch_ok = True
    for exp, w in lago_active.items():
        b = page_batches.get(exp)
        ok = (b is not None
              and int(b["balance_micro"]) == w["balance_cents"] * 10_000
              and b["granted_at"] == w["created_at"])
        per_batch_ok = per_batch_ok and ok
        detail.append(f"{w['name']}:{w['balance_cents']}c/exp {exp} -> page "
                      f"{b['balance_micro'] if b else 'MISSING'}u "
                      f"granted {b['granted_at'] if b else '-'} vs "
                      f"{w['created_at']} {'OK' if ok else 'MISMATCH'}")
    checks.append(("each active wallet reconciles 1:1 (balance/expiry/grant)",
                   per_batch_ok, "; ".join(detail)))

    # Page rows without an active authority wallet are the registry overlay's
    # terminated/expired batches; the no-carryover invariant REQUIRES them to
    # surface balance 0 (never resurrected, never counted in the total).
    orphan_rows = [b for exp, b in page_batches.items() if exp not in lago_active]
    orphans_ok = all(int(b["balance_micro"]) == 0 for b in orphan_rows)
    checks.append(("page rows without an active wallet surface balance 0 "
                   "(no carry-over, no resurrection)",
                   orphans_ok,
                   f"orphan_rows={[(b['source'], b['period'], b['balance_micro']) for b in orphan_rows]}"))

    avail = int(credits["available_micro"])
    bal = int(credits["balance_micro"])
    held = int(credits["held_micro"])
    locked = int(credits["refund_locked_micro"])
    checks.append(("available == balance - held - refund_locked",
                   avail == bal - held - locked,
                   f"{avail} == {bal} - {held} - {locked}"))

    lines = []
    ok_all = True
    for name, ok, ev in checks:
        ok_all = ok_all and ok
        lines.append(f"[{'PASS' if ok else 'FAIL'}] {name}\n       {ev}")
    verdict = "RECONCILE PASS" if ok_all else "RECONCILE FAIL"
    lines.append(verdict)
    out = HERE / "reconcile-output.txt"
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))
    sys.exit(0 if ok_all else 1)


if __name__ == "__main__":
    main()
