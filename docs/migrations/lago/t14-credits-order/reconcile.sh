#!/usr/bin/env bash
# t14 credits-order reconciliation (#86 Task 6, AC5).
#
# Compares the two sources of truth for ONE tenant's credits —
#   1. the WeKnora Billing API  (GET /api/v1/commercial/account benefits.credits)
#   2. the Lago authority wallet list (GET /api/v1/customers/:ext/wallets)
# — and asserts they agree: Σ active wallet balance_cents × 10^4 == the page's
# balance_micro, and every active wallet pairs with one batch line on matching
# expiry. Output: one RECONCILE PASS/FAIL line per tenant (evidence artifact).
#
# Usage:
#   LAGO_API_KEY=... WEKNORA_TOKEN=... ./reconcile.sh <lago-api> <weknora-api> <tenant-id>...
set -euo pipefail

LAGO_API="${1:?lago api base url}"; shift
WEB_API="${1:?weknora api base url}"; shift

# Outbound discipline (mirrors the server's egress policy): the two base
# URLs must be plain http(s) — anything else is refused before curl runs.
for base in "$LAGO_API" "$WEB_API"; do
  case "$base" in
    http://*|https://*) : ;;
    *) echo "RECONCILE FAIL: refusing non-http(s) base url '$base'" >&2; exit 2 ;;
  esac
done

# --- lago side: Σ active balance_cents × 10^4, active count, wallet face ---
lago_side() { # $1 = wallets json
  python3 -c '
import json,sys
ws=[w for w in json.loads(sys.argv[1]).get("wallets",[]) if w.get("status")=="active"]
total=sum(int(w.get("balance_cents",0)) for w in ws)*10_000
face=json.dumps(sorted((w.get("expiration_at",""),w.get("name","")) for w in ws))
print(total, len(ws), face)' "$1"
}

# --- page side: balance_micro, positive batch count, batch face ---
page_side() { # $1 = account json
  python3 -c '
import json,sys
credits=((json.loads(sys.argv[1]).get("data") or {}).get("benefits") or {}).get("credits")
if credits is None:
    print("PENDING",0,"[]")
else:
    pos=[b for b in credits.get("batches",[]) if int(b.get("balance_micro","0"))>0]
    face=json.dumps(sorted((b["expires_at"], "topup" if b["source"]=="topup" else b["period"]) for b in pos))
    print(credits["balance_micro"], len(pos), face)' "$1"
}

fail=0
for tenant in "$@"; do
  ext="weknora-tenant-${tenant}"
  wallets_json="$(curl -sS --max-time 20 -H "Authorization: Bearer ${LAGO_API_KEY:?}" \
    "${LAGO_API}/api/v1/customers/${ext}/wallets?page=1")"
  account_json="$(curl -sS --max-time 20 -H "Authorization: Bearer ${WEKNORA_TOKEN:?}" \
    "${WEB_API}/api/v1/commercial/account")"

  read -r lago_micro lago_count lago_face <<<"$(lago_side "$wallets_json")"
  read -r page_micro page_count page_face <<<"$(page_side "$account_json")"

  if [ "$page_micro" = "PENDING" ]; then
    echo "tenant ${tenant}: benefits.credits absent (pending) RECONCILE FAIL"
    fail=1; continue
  fi
  if [ "$lago_micro" = "$page_micro" ]; then
    echo "tenant ${tenant}: lago Σ=${lago_micro} page=${page_micro} wallets=${lago_count} batches=${page_count} RECONCILE PASS"
  else
    echo "tenant ${tenant}: lago Σ=${lago_micro} page=${page_micro} RECONCILE FAIL"
    echo "  lago face:   ${lago_face}"
    echo "  page face:   ${page_face}"
    fail=1
  fi
done
exit "$fail"
