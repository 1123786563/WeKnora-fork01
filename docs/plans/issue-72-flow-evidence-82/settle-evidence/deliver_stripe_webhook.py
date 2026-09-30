#!/usr/bin/env python3
"""#82 Task 10 — the Stripe→Lago webhook transport-leg stand-in (D8).

The local stack's LAGO_API_URL is loopback, so Stripe's cloud cannot
deliver webhook events to it (t11 DECISION.md boundary). This stand-in
performs EXACTLY that one leg: it reads the REAL PaymentIntent back from
the Stripe API, wraps it in a payment_intent.succeeded event, signs the
payload with the provider's webhook secret (HMAC-SHA256, the same header
shape Stripe uses), and POSTs it to Lago's BUILT-IN webhook route. The
receive/verify/process chain is 100% Lago built-in.

Credentials arrive ONLY via the environment (STRIPE_SECRET_KEY sourced
from the operator store; LAGO_* from the t11 lab.env by the caller).
Usage: deliver_stripe_webhook.py --customer cus_x [--base URL --org ID
--code weknora-stripe]
"""
import argparse
import base64
import hashlib
import hmac
import json
import os
import subprocess
import time
# (OCR r2) urllib.error is imported EXPLICITLY — the previous file relied on
# CPython's transitive import side effect of urllib.request (fragile: an
# import-set change or a different runtime raises AttributeError).
import urllib.error
import urllib.parse
import urllib.request

STRIPE_BASE = "https://api.stripe.com"


def stripe(method, path, key, form=None):
    data = urllib.parse.urlencode(form).encode() if form else None
    req = urllib.request.Request(
        STRIPE_BASE + path, data=data,
        headers={
            "Authorization": "Basic " + base64.b64encode((key + ":").encode()).decode(),
            "Content-Type": "application/x-www-form-urlencoded",
        }, method=method)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    last = None
    for attempt in range(3):  # bounded transport retry (HTTP errors return)
        try:
            with opener.open(req, timeout=30) as r:
                return r.status, json.loads(r.read().decode())
        except urllib.error.HTTPError as e:
            return e.code, json.loads(e.read().decode())
        except OSError as e:
            last = e
            time.sleep(1.5 * (attempt + 1))
    raise SystemExit(f"stripe {method} {path}: {last}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--customer", required=True, help="the Stripe customer id (cus_...)")
    # (OCR r2) LAGO_INTEGRATION_BASE_URL (the tagged Go integration tests'
    # env contract) takes precedence; LAGO_API_URL (the t11 lab.env face) is
    # the fallback.
    ap.add_argument("--base", default=os.environ.get("LAGO_INTEGRATION_BASE_URL")
                    or os.environ.get("LAGO_API_URL", "http://127.0.0.1:48895"))
    ap.add_argument("--org", default=os.environ.get("LAGO_INTEGRATION_ORG_ID", ""))
    ap.add_argument("--code", default="weknora-stripe")
    args = ap.parse_args()

    key = os.environ.get("STRIPE_SECRET_KEY")
    if not key:
        raise SystemExit("STRIPE_SECRET_KEY not exported (source the operator env)")
    secret = os.environ.get("LAGO_INTEGRATION_WEBHOOK_SECRET", "")
    if not secret:
        raise SystemExit("LAGO_INTEGRATION_WEBHOOK_SECRET not exported (source prepare_t9_env.sh)")

    status, body = stripe("GET", "/v1/payment_intents?customer="
                          + urllib.parse.quote(args.customer) + "&limit=20", key)
    if status != 200:
        raise SystemExit(f"intent list HTTP {status}")
    intent = None
    for row in body.get("data", []):
        if row.get("status") == "succeeded" and (row.get("metadata") or {}).get("lago_invoice_id"):
            cand = row
            if intent is None or cand.get("created", 0) > intent.get("created", 0):
                intent = cand
    if intent is None:
        raise SystemExit("no succeeded intent carrying lago_invoice_id")

    if not args.org:
        out = subprocess.run(
            ["docker", "compose", "-f", "deploy/lago/compose.yaml",
             "--env-file", "deploy/lago-lab/payment-settle-trigger/lab.env",
             "-p", "weknora-lago-t11", "exec", "-T", "db", "psql", "-U", "lago",
             "-tAc", "select id from organizations order by created_at limit 1"],
            capture_output=True, text=True, timeout=30)
        args.org = (out.stdout or "").strip()
    if not args.org:
        raise SystemExit("organization id unavailable")

    event = {
        "id": f"evt_t10_{int(time.time()*1000)}",
        "object": "event", "api_version": "2024-06-20",
        "created": int(time.time()), "livemode": False,
        "type": "payment_intent.succeeded",
        "data": {"object": intent},
    }
    payload = json.dumps(event, separators=(",", ":")).encode()
    ts = str(int(time.time()))
    mac = hmac.new(secret.encode(), f"{ts}.".encode() + payload, hashlib.sha256)
    url = f"{args.base}/webhooks/stripe/{args.org}?code={urllib.parse.quote(args.code)}"
    req = urllib.request.Request(url, data=payload, headers={
        "Content-Type": "application/json",
        "Stripe-Signature": f"t={ts},v1={mac.hexdigest()}",
    }, method="POST")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(req, timeout=30) as r:
        print(f"delivered event {event['id']} for intent {intent['id']} -> HTTP {r.status}")
        print(f"invoice: {intent['metadata'].get('lago_invoice_id')}")


if __name__ == "__main__":
    main()
