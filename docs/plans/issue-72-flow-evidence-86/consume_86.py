#!/usr/bin/env python3
"""Issue #86 flow verification: drive real Lago wallet consumption.

Consumption trigger follows the t03 wallet-semantics lab method
(deploy/lago-lab/wallet-semantics/experiments/): billable metric + plan +
subscription, then POST /api/v1/events. Lago aggregates the events into an
invoice which is paid from the customer's wallets in strict
`priority ASC, created_at ASC` order — the runtime consumption path #86's
priority encoding + authority rebalance exist to steer.

Credentials are read from the server-side .env only (never committed).

Egress policy: local test-stack allow-list only (scheme-validated, no
redirects followed) — the same posture concurrent_consumption_86.py and
reconcile.py already implement (OCR86-R1-03 closed this script's gap).
"""
import json
import argparse
from decimal import Decimal, InvalidOperation
import os
import subprocess
import sys
import tempfile
import time
import urllib.error
import uuid
from pathlib import Path
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler
from concurrent_consumption_86 import read_wallet_pages, prepare_output_dir

ALLOWED_TARGETS = {"127.0.0.1"}  # the local Lago test stack, nothing else
CUSTOMER = os.environ.get("LAGO_CUSTOMER", "weknora-tenant-10000")
# (OCR86-R1-02) 扣费金额由 units × 单价派生（与 wait_until 断言同一常量），
# 不再持有与真实扣减脱节的遗留数字。
EVENT_UNITS = 2
UNIT_PRICE_CNY = "1.00"
def price_to_cents(price: str) -> int:
    """Convert a finite positive exact-cent decimal price without float math."""
    try:
        amount = Decimal(price)
    except (InvalidOperation, TypeError, ValueError):
        raise ValueError("price must be a positive exact-cent decimal") from None
    if not amount.is_finite() or amount <= 0:
        raise ValueError("price must be a positive exact-cent decimal")
    numerator, denominator = amount.as_integer_ratio()
    scaled_numerator = numerator * 100
    if scaled_numerator % denominator:
        raise ValueError("price must be a positive exact-cent decimal")
    return scaled_numerator // denominator


UNIT_CENTS = price_to_cents(UNIT_PRICE_CNY)
CONSUME_CENTS = EVENT_UNITS * UNIT_CENTS


class _NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


OPENER = build_opener(_NoRedirect)


def base_url():
    """(OCR86-R1-03) 同款出网守卫：scheme 白名单 + 测试栈主机白名单——
    LAGO_BASE 被误设/污染时，携带 Bearer key 的请求不会发往任意主机。"""
    raw = os.environ.get("LAGO_BASE", "http://127.0.0.1:48889")
    parts = urlsplit(raw)
    if parts.scheme not in ("http", "https"):
        raise SystemExit("scheme not allowed: " + parts.scheme)
    if parts.hostname not in ALLOWED_TARGETS:
        raise SystemExit("target host not in test-stack allow-list: "
                         + parts.hostname)
    return raw


def api_key():
    # env 注入优先（README 运行口径：LAGO_API_KEY=$KEY python3 consume_86.py）；
    # 回退读取 worktree 根的未跟踪 .env——经 git 自身的仓根解析定位，
    # 不使用任何向上路径穿越字面量（Mimosa 路径安全约束）。
    key = os.environ.get("LAGO_API_KEY")
    if key:
        return key
    try:
        root = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True, text=True, check=True).stdout.strip()
        if root:
            env_path = Path(root) / ".env"
            for line in env_path.read_text(encoding="utf-8").splitlines():
                if line.startswith("WEKNORA_COMMERCIAL_PLATFORM_API_KEY="):
                    key = line.strip().split("=", 1)[1]
                    if key:
                        return key
    except (subprocess.CalledProcessError, OSError, UnicodeError):
        pass
    raise SystemExit("api key not found (export LAGO_API_KEY or provide "
                     "the worktree-root .env)")


def request(method, path, payload=None, *, key=None):
    url = f"{base_url()}{path}"
    body = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    headers["Authorization"] = f"Bearer {api_key() if key is None else key}"
    req = Request(url, data=body, headers=headers, method=method)
    try:
        with OPENER.open(req, timeout=30) as resp:
            raw = resp.read()
            return resp.status, json.loads(raw) if raw else None
    except urllib.error.HTTPError as err:
        raw = err.read()
        return err.code, json.loads(raw) if raw else None


def wallets(fetch_page=None, *, key=None):
    def fetch(page):
        _, body = request("GET", f"/api/v1/customers/{CUSTOMER}/wallets?per_page=20&page={page}", key=key)
        return body
    return {w["name"]: w for w in read_wallet_pages(fetch_page or fetch)}


def balance_snapshot(*, key=None):
    return {name: w.get("balance_cents") for name, w in wallets(key=key).items()
            if w.get("status") == "active"}


def wait_until(predicate, timeout_seconds, interval=3.0):
    started = time.monotonic()
    while True:
        if predicate():
            return True, time.monotonic() - started
        if time.monotonic() - started >= timeout_seconds:
            return False, time.monotonic() - started
        time.sleep(interval)


def write_artifact_atomic(path, payload):
    """Write a complete JSON artifact, then publish it in one rename."""
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=path.parent,
                                         prefix=".consume-cny-", suffix=".tmp",
                                         delete=False) as fh:
            temporary = Path(fh.name)
            json.dump(payload, fh, indent=2)
            fh.flush()
        os.replace(temporary, path)
        temporary = None
    finally:
        if temporary is not None:
            try:
                temporary.unlink()
            except FileNotFoundError:
                pass


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-dir", required=True)
    output_dir = prepare_output_dir(parser.parse_args().output_dir)
    stage = "initialization"
    observations = {}
    result = {}
    out = output_dir / "consume-cny.json"
    try:
        key = api_key()
        tag = uuid.uuid4().hex[:8]
        metric_code = f"weknora-86-metric-{tag}"
        plan_code = f"weknora-86-plan-{tag}"
        sub_ext = f"weknora-86-sub-{tag}"

        stage = "metric_create"
        status, body = request("POST", "/api/v1/billable_metrics", payload={
            "billable_metric": {"code": metric_code, "name": f"WeKnora 86 consume {tag}",
                "description": "Issue 86 flow consumption metric", "aggregation_type": "sum_agg",
                "field_name": "units"}}, key=key)
        if status not in (200, 201) or not isinstance(body, dict) or "billable_metric" not in body:
            raise RuntimeError("metric create failed")
        result["resource_codes"] = {"metric_code": metric_code}
        metric_id = body["billable_metric"]["lago_id"]

        stage = "plan_create"
        status, body = request("POST", "/api/v1/plans", payload={"plan": {
            "code": plan_code, "name": f"WeKnora 86 plan {tag}", "interval": "weekly",
            "pay_in_advance": False, "amount_cents": 0, "amount_currency": "CNY",
            "charges": [{"billable_metric_id": metric_id, "charge_model": "standard",
                "pay_in_advance": True, "invoiceable": True, "properties": {"amount": UNIT_PRICE_CNY}}]}}, key=key)
        if status not in (200, 201) or not isinstance(body, dict) or "plan" not in body:
            raise RuntimeError("plan create failed")
        result["resource_codes"]["plan_code"] = plan_code

        stage = "subscription_create"
        status, body = request("POST", "/api/v1/subscriptions", payload={"subscription": {
            "external_customer_id": CUSTOMER, "plan_code": plan_code, "external_id": sub_ext,
            "name": f"weknora-86 consumption trigger {tag}"}}, key=key)
        if status not in (200, 201) or not isinstance(body, dict) or "subscription" not in body:
            raise RuntimeError("subscription create failed")
        result["resource_codes"]["subscription_code"] = sub_ext

        stage = "before_balance_snapshot"
        before = balance_snapshot(key=key)
        observations["before"] = before
        print("balances before:", before)

        stage = "event_post"
        txn_id = f"weknora-86-evt-{uuid.uuid4()}"
        status, body = request("POST", "/api/v1/events", payload={"event": {
            "transaction_id": txn_id, "external_customer_id": CUSTOMER,
            "external_subscription_id": sub_ext, "code": metric_code,
            "properties": {"units": EVENT_UNITS}}}, key=key)
        if status not in (200, 201):
            raise RuntimeError("event post failed")
        result["resource_codes"]["event_transaction_id"] = txn_id

        stage = "settlement"
        total_before = sum(before.values())
        ok, elapsed = wait_until(lambda: sum(balance_snapshot(key=key).values()) == total_before - CONSUME_CENTS, 300)
        after = balance_snapshot(key=key)
        observations["after"] = after
        observations["settled"] = ok
        observations["settle_seconds"] = round(elapsed)
        deltas = {k: before[k] - after[k] for k in before if k in after}
        observations["deltas"] = deltas
        result = {"metric": metric_code, "plan": plan_code, "subscription": sub_ext,
            "event": txn_id, "consume_cents": CONSUME_CENTS, "before": before,
            "after": after, "deltas": deltas, "settled": ok, "settle_seconds": round(elapsed),
            "resource_codes": dict(result["resource_codes"])}

        if not ok:
            raise RuntimeError("consumption did not settle before timeout")

        stage = "draw_validation"
        def pick(predicate, label):
            matches = [k for k in deltas if predicate(k)]
            if len(matches) != 1:
                raise RuntimeError("expected exactly one %s wallet" % label)
            return matches[0]
        m = pick(lambda k: k.endswith("-2026-09"), "monthly")
        c = pick(lambda k: "topup-c" in k, "topup-c")
        d = pick(lambda k: "topup-d" in k, "topup-d")
        verdict = (ok and after[m] == 0 and deltas[m] == 100 and deltas[c] == 100
                   and after[c] == 400 and deltas[d] == 0 and after[d] == 500)
        if not verdict:
            raise RuntimeError("wallet draw distribution did not match expected order")
        result["verdict"] = "PASS"
        write_artifact_atomic(out, result)
        return 0
    except KeyboardInterrupt as exc:
        failed = dict(result)
        failed.update({"verdict": "FAIL", "failed_stage": stage,
                       "error": {"type": type(exc).__name__, "message": "operation interrupted"},
                       "observations": observations})
        try:
            write_artifact_atomic(out, failed)
        except Exception as write_error:
            print("FAIL: unable to persist failure evidence (%s)" % type(write_error).__name__[:80],
                  file=sys.stderr)
        return 130
    except (Exception, SystemExit) as exc:
        # Do not serialize provider response bodies or arbitrary exception text.
        safe_error = {"type": type(exc).__name__[:80], "message": "operation failed"}
        failed = dict(result)
        failed.update({"verdict": "FAIL", "failed_stage": stage, "error": safe_error,
                       "observations": observations})
        try:
            write_artifact_atomic(out, failed)
        except Exception as write_error:
            print("FAIL: unable to persist failure evidence (%s)" % type(write_error).__name__[:80],
                  file=sys.stderr)
        print("FAIL: %s: operation failed" % stage, file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
