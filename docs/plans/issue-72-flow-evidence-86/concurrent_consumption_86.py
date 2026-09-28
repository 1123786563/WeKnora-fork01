#!/usr/bin/env python3
"""Issue #86 concurrent-consumption leg.

The required --output-dir receives an automatic stdout tee at
<output-dir>/concurrent-output.txt and the verdict at <output-dir>/facts.json.

Fires N parallel consumption events against the Lago authority and asserts
the wallets are drawn exactly once per unit with no double-draw, no negative
balance, and the draw order follows the expiry rank (earliest-expiry wallet
drained first). Also replays one identical event transaction_id to assert
event idempotency (no extra draw).

Egress policy: local test-stack allow-list only (scheme-validated, no
redirects followed). The API key arrives via the LAGO_API_KEY env variable.
"""
import json
import os
import tempfile
import sys
import argparse
from pathlib import Path
import threading
import time
import contextlib
import urllib.error
import uuid
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler

ALLOWED_TARGETS = {"127.0.0.1"}  # the local Lago test stack, nothing else
CUSTOMER = os.environ.get("LAGO_CUSTOMER", "weknora-tenant-10000")


class _NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


OPENER = build_opener(_NoRedirect)


def decode_error_body(raw):
    if not raw:
        return None
    try:
        return json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError, TypeError):
        return None


def active_wallet_view(rows):
    active = [w for w in rows if w.get("status") == "active"]
    balances_by_name = {w["name"]: w.get("balance_cents") for w in active}
    ranks_by_name = {w["name"]: (w.get("expiration_at"), w.get("created_at"), w.get("lago_id")) for w in active}
    return balances_by_name, ranks_by_name


@contextlib.contextmanager
def capture_stdout(capture):
    original = sys.stdout
    try:
        sys.stdout = _Tee(original, capture)
        yield
    finally:
        try:
            capture.flush()
        finally:
            sys.stdout = original
            capture.close()


def bounded_error(exc):
    message = str(exc)
    for secret in (os.environ.get("LAGO_API_KEY", ""),):
        if secret:
            message = message.replace(secret, "[REDACTED]")
    message = message[:300]
    return {"type": type(exc).__name__[:80], "message": message}


def _flush_facts_file(file_handle):
    file_handle.flush()


def _close_facts_file(file_handle):
    file_handle.close()


def write_facts_atomic(output_dir, facts):
    """Publish a complete facts document through a same-directory rename."""
    staged = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=output_dir,
                                         prefix=".facts-", suffix=".tmp", delete=False) as fh:
            staged = Path(fh.name)
            json.dump(facts, fh, indent=2)
            _flush_facts_file(fh)
            _close_facts_file(fh)
        os.replace(staged, Path(output_dir) / "facts.json")
        staged = None
    finally:
        if staged is not None:
            try:
                staged.unlink()
            except OSError:
                pass


def read_wallet_pages(fetch_page):
    """Read the pinned Lago v1.53 page protocol and reject incomplete folds."""
    rows, seen_ids, seen_names, page, expected_total_pages, expected_total_count = [], set(), set(), 1, None, None
    while True:
        body = fetch_page(page)
        if not isinstance(body, dict) or not isinstance(body.get("wallets"), list):
            raise ValueError("wallet page has no wallets array")
        meta = body.get("meta")
        if not isinstance(meta, dict):
            raise ValueError("wallet page has no pagination metadata")
        if not {"current_page", "next_page", "total_pages", "total_count"}.issubset(meta):
            raise ValueError("wallet page is missing required pagination metadata")
        current = meta.get("current_page")
        total_pages = meta.get("total_pages")
        total_count = meta.get("total_count")
        next_page = meta.get("next_page")
        if type(current) is not int or type(total_pages) is not int or type(total_count) is not int:
            raise ValueError("wallet pagination metadata must contain integer page/count values")
        if current != page or total_pages < 1 or total_count < 0 or page > total_pages:
            raise ValueError("wallet pagination metadata is inconsistent")
        if expected_total_pages is None:
            expected_total_pages, expected_total_count = total_pages, total_count
        elif total_pages != expected_total_pages or total_count != expected_total_count:
            raise ValueError("wallet pagination totals changed between pages")
        expected_next = page + 1 if page < total_pages else None
        if (next_page is not None and type(next_page) is not int) or next_page != expected_next:
            raise ValueError("wallet next_page does not match total_pages")
        for wallet in body["wallets"]:
            if not isinstance(wallet, dict) or not wallet.get("lago_id"):
                raise ValueError("wallet row has no stable lago_id")
            if wallet["lago_id"] in seen_ids:
                raise ValueError("wallet pagination contains duplicate lago_id")
            seen_ids.add(wallet["lago_id"])
            if not wallet.get("name") or wallet["name"] in seen_names:
                raise ValueError("wallet pagination has missing or duplicate name")
            seen_names.add(wallet["name"])
        rows.extend(body["wallets"])
        if page == total_pages:
            if len(rows) != expected_total_count:
                raise ValueError("wallet count mismatch: expected %d, received %d" % (expected_total_count, len(rows)))
            return rows
        page += 1


def prepare_output_dir(path):
    output = Path(path)
    output.mkdir(parents=False, exist_ok=False)
    return output


class _Tee:
    def __init__(self, original, capture):
        self.original, self.capture = original, capture
    def write(self, value):
        self.original.write(value)
        self.capture.write(value)
    def flush(self):
        self.original.flush()
        self.capture.flush()


def base_url():
    raw = os.environ.get("LAGO_BASE", "http://127.0.0.1:48889")
    parts = urlsplit(raw)
    if parts.scheme not in ("http", "https"):
        raise SystemExit("scheme not allowed: " + parts.scheme)
    if parts.hostname not in ALLOWED_TARGETS:
        raise SystemExit("target host not in test-stack allow-list: "
                         + parts.hostname)
    return raw


def call(method, path, payload=None):
    body = None
    headers = {"Accept": "application/json",
               "Authorization": "Bearer " + os.environ["LAGO_API_KEY"]}
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = Request(base_url() + path, data=body, headers=headers, method=method)
    try:
        with OPENER.open(req, timeout=30) as resp:
            raw = resp.read()
            return resp.status, json.loads(raw) if raw else None
    except urllib.error.HTTPError as err:
        raw = err.read()
        return err.code, decode_error_body(raw)


def require_created(status, body, key, operation):
    if status not in (200, 201) or not isinstance(body, dict):
        raise RuntimeError("%s failed: HTTP %s" % (operation, status))
    obj = body.get(key)
    if not isinstance(obj, dict) or not obj.get("lago_id"):
        raise RuntimeError("%s response missing %s.lago_id" % (operation, key))
    return obj["lago_id"]


def replay_response_ok(status, body):
    """No pinned duplicate-specific response contract exists; fail closed."""
    return False


def replay_response_summary(status, body):
    # Never persist provider response text or arbitrary fields: they can carry
    # identifiers or echoed secrets and do not prove duplicate identity.
    return {"status": status, "body_type": type(body).__name__ if body is not None else "null",
            "duplicate_identity": "unproven"}


def replay_snapshot_matches(expected, final):
    """The last replay snapshot must preserve each wallet, not just the sum."""
    return final == expected


def wallet_list(fetch_page=None):
    def fetch(page):
        status, body = call("GET", "/api/v1/customers/" + CUSTOMER + "/wallets?per_page=20&page=" + str(page))
        if not 200 <= status <= 299:
            raise RuntimeError("wallet list failed: HTTP %s" % status)
        return body
    return read_wallet_pages(fetch_page or fetch)


def balances():
    return {w["name"]: w.get("balance_cents")
            for w in wallet_list() if w.get("status") == "active"}


def wait_until(predicate, timeout_seconds, interval=3.0):
    started = time.monotonic()
    while True:
        if predicate():
            return True, time.monotonic() - started
        if time.monotonic() - started >= timeout_seconds:
            return False, time.monotonic() - started
        time.sleep(interval)


def observe_replay_balance(expected_balances, observe_seconds=300, interval=3):
    """Require every wallet balance to stay unchanged through full settlement bound."""
    started = time.monotonic()
    deadline = started + observe_seconds
    while True:
        current = balances()
        if current != expected_balances:
            return False, current, time.monotonic() - started
        now = time.monotonic()
        if now >= deadline:
            return True, current, now - started
        time.sleep(min(interval, deadline - now))


def draw_order_ok(before, after, draw_cents, rank_of):
    """(OCR86-R1-01) 全钱包级联判定：按 expiry 序遍历全部钱包——被触及的钱包
    之前的必须扣空（after == 0），最多最后一个被触及的钱包部分扣减（after >= 0
    且未扣穿），其后的钱包完全不动（delta == 0）；总扣减恰为 draw_cents。

    取代旧的 early/late 两钱包模型（early 扣空 + spill 全落 late）：draw 超过
    前两钱包余额需要级联到第三个及以后钱包时，after[late] == before[late] -
    spill 必然不成立——正确的级联扣减会被误判 FAIL；late 之后的钱包也完全
    未被断言。返回 (ok, detail)。
    """
    if not after:
        return False, "no active wallets to assert"
    if set(before) != set(after):
        return False, "wallet identity set changed: before=%s after=%s" % (sorted(before), sorted(after))
    if any(n not in rank_of or not all(rank_of[n]) for n in after):
        return False, "missing expiry/grant/stable identity rank"
    rank_values = [tuple(rank_of[n]) for n in after]
    if len(set(rank_values)) != len(rank_values):
        return False, "duplicate wallet rank"
    names = sorted(after, key=lambda n: tuple(rank_of[n]))
    if draw_cents < 0:
        return False, "negative requested draw"
    remaining = draw_cents
    expected = {}
    for name in names:
        balance = before[name]
        if balance < 0:
            return False, "negative starting balance for %s" % name
        draw = min(balance, remaining)
        expected[name] = balance - draw
        remaining -= draw
    if remaining:
        return False, "requested draw exceeds active wallet balance by %d cents" % remaining
    if any(after[name] != expected[name] for name in names):
        return False, "observed balances differ from sequential draw: expected=%s observed=%s" % (expected, after)
    drawn = [before.get(n, 0) - after[n] for n in names]
    touched = [i for i, d in enumerate(drawn) if d > 0]
    if not touched:
        return (draw_cents == 0,
                "no wallet drawn (draw_cents=%d)" % draw_cents)
    last = touched[-1]
    prefix_zeroed = all(after[n] == 0 for n in names[:last])
    suffix_untouched = all(d == 0 for d in drawn[last + 1:])
    no_negative = all(v >= 0 for v in after.values())
    total_exact = sum(drawn) == draw_cents
    ok = prefix_zeroed and suffix_untouched and no_negative and total_exact
    detail = "; ".join(
        "%s %d->%d%s" % (n, before.get(n, 0), after[n],
                         " (drained)" if after[n] == 0 else "")
        for n in names)
    detail += (" | touched through rank %d of %d, last_remaining=%d"
               % (last + 1, len(names), after[names[last]]))
    return ok, detail


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-dir", required=True)
    try:
        args = parser.parse_args()
        output_dir = prepare_output_dir(args.output_dir)
    except SystemExit:
        raise
    except Exception as exc:
        try:
            sys.stderr.write("FAIL: output-dir preflight: %s\n" % bounded_error(exc))
        except Exception:
            pass
        return 2
    state = {"stage": "capture_setup", "observations": {}, "checks": []}
    capture = None
    try:
        capture = (output_dir / "concurrent-output.txt").open("x", encoding="utf-8")
        with capture_stdout(capture):
            return run_probe(output_dir, state)
    except BaseException as exc:
        if state.get("committed"):
            try:
                sys.stderr.write("WARNING: evidence transcript incomplete (%s)\n" % type(exc).__name__[:80])
            except Exception:
                pass
            return 0
        code = exc.code if isinstance(exc, SystemExit) and isinstance(exc.code, int) else 1
        error = bounded_error(exc)
        diagnostic = "FAIL: %s" % error
        try:
            sys.stderr.write(diagnostic + "\n")
        except Exception:
            pass
        facts = {"verdict": "FAIL", "failed_stage": str(state.get("stage", "unknown"))[:100],
                 "error": error, "observations": state.get("observations", {}),
                 "checks": state.get("checks", [])}
        try:
            write_facts_atomic(output_dir, facts)
        except Exception as persist_exc:
            try:
                sys.stderr.write("could not persist facts.json: %s\n" % bounded_error(persist_exc))
            except Exception:
                pass
        if capture is not None and not capture.closed:
            try:
                capture.flush()
            except Exception:
                pass
            try:
                capture.close()
            except Exception:
                pass
        return code or 1


def run_probe(output_dir, state):
    tag = uuid.uuid4().hex[:8]
    metric_code = "weknora-86-metric-" + tag
    plan_code = "weknora-86-plan-" + tag
    sub_ext = "weknora-86-sub-" + tag
    state["observations"]["generated_candidates"] = {
        "tag": tag,
        "metric_code": metric_code,
        "plan_code": plan_code,
        "subscription_code": sub_ext,
    }
    state["observations"]["created_resources"] = {}

    state["stage"] = "create_metric"
    status, body = call("POST", "/api/v1/billable_metrics", payload={
        "billable_metric": {"code": metric_code,
                            "name": "WeKnora 86 concurrent " + tag,
                            "aggregation_type": "sum_agg",
                            "field_name": "units"}})
    metric_id = require_created(status, body, "billable_metric", "create billable metric")
    state["observations"]["created_resources"]["metric_code"] = metric_code
    state["stage"] = "create_plan"
    status, body = call("POST", "/api/v1/plans", payload={
        "plan": {"code": plan_code, "name": "WeKnora 86 concurrent " + tag,
                 "interval": "weekly", "pay_in_advance": False,
                 "amount_cents": 0, "amount_currency": "CNY",
                 "charges": [{"billable_metric_id": metric_id,
                              "charge_model": "standard",
                              "pay_in_advance": True, "invoiceable": True,
                              "properties": {"amount": "1.00"}}]}})
    require_created(status, body, "plan", "create plan")
    state["observations"]["created_resources"]["plan_code"] = plan_code
    state["stage"] = "create_subscription"
    status, body = call("POST", "/api/v1/subscriptions", payload={
        "subscription": {"external_id": sub_ext,
                         "external_customer_id": CUSTOMER,
                         "plan_code": plan_code}})
    require_created(status, body, "subscription", "create subscription")
    state["observations"]["created_resources"]["subscription_code"] = sub_ext
    print("trigger ready (concurrent leg): sub=" + sub_ext)

    state["stage"] = "read_before"
    before = balances()
    total_before = sum(before.values())
    print("balances before:", before, "total:", total_before)

    n_events = 5
    txn_ids = ["weknora-86-con-" + uuid.uuid4().hex for _ in range(n_events)]
    results = {}

    def fire(txn):
        try:
            status, _ = call("POST", "/api/v1/events", payload={
                "event": {"transaction_id": txn,
                          "external_customer_id": CUSTOMER,
                          "external_subscription_id": sub_ext,
                          "code": metric_code,
                          "properties": {"units": 1}}})
            results[txn] = "posted" if status in (200, 201) else "http" + str(status)
        except Exception as exc:  # noqa: BLE001
            results[txn] = "error:" + type(exc).__name__

    state["stage"] = "post_events"
    threads = [threading.Thread(target=fire, args=(t,)) for t in txn_ids]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    print("event posts:", results)
    state["observations"].update({"before": before, "event_posts": results})
    if any(v != "posted" for v in results.values()):
        raise RuntimeError("event post failed")

    draw_cents = n_events * 100
    ok, elapsed = wait_until(
        lambda: sum(balances().values()) == total_before - draw_cents, 300)
    state["stage"] = "settle_events"
    settled_rows = wallet_list()
    after, rank_of = active_wallet_view(settled_rows)
    settled_after = dict(after)
    state["observations"].update({"settled_after": settled_after})
    total_after = sum(after.values())
    print("draw settled: %s after %.0fs; after: %s total: %s"
          % (ok, elapsed, after, total_after))

    # replay one identical transaction_id — Lago rejects duplicate events
    # with 422 (idempotency by refusal) and never draws anything extra
    state["stage"] = "duplicate_replay_request"
    replay_status, replay_body = call("POST", "/api/v1/events", payload={
            "event": {"transaction_id": txn_ids[0],
                      "external_customer_id": CUSTOMER,
                      "external_subscription_id": sub_ext,
                      "code": metric_code,
                      "properties": {"units": 1}}})
    state["observations"].update({
        "replay_status": replay_status,
        "replay_response": replay_response_summary(replay_status, replay_body),
    })
    state["stage"] = "duplicate_replay_observation"
    replay_stable, replay_balances_observed, replay_observation_seconds = observe_replay_balance(settled_after)
    replay_after = balances()
    replay_total = sum(replay_after.values())
    print("replay status:", replay_status,
          "after replay:", replay_after, "total:", replay_total,
          "stable per-wallet observation:", replay_stable,
          "duration:", round(replay_observation_seconds, 1), "seconds")

    # (OCR86-R1-01) 级联断言交给全钱包判定的共享助手（见 draw_order_ok）。
    order_ok, order_detail = draw_order_ok(before, settled_after, draw_cents, rank_of)

    facts = {"txn_ids": txn_ids, "before": before, "after": settled_after,
             "replay_after": replay_after,
             "replay_status": replay_status,
             "replay_observed_balances": replay_balances_observed,
             "replay_observation_seconds": round(replay_observation_seconds, 3),
             "draw_cents": draw_cents,
             "replay_response": replay_response_summary(replay_status, replay_body)}

    checks = [
        ("identical event replay receives an expected response",
         replay_response_ok(replay_status, replay_body), str(replay_response_summary(replay_status, replay_body))),
        ("total drawn exactly once per event (no double draw)",
         total_before - total_after == draw_cents,
         "%d - %d == %d" % (total_before, total_after, draw_cents)),
        ("no wallet negative",
         all(v >= 0 for v in after.values()), str(after)),
        ("draw order follows expiry rank (earliest drained first, "
         "cascade across ALL wallets)",
         order_ok, order_detail),
        ("identical event replay draws nothing extra",
         replay_snapshot_matches(after, replay_after),
         "final=%s expected=%s" % (replay_after, after)),
        ("per-wallet balances remained stable during 300-second observation",
         replay_stable and replay_balances_observed == after,
         "observed=%s expected=%s" % (replay_balances_observed, after)),
    ]
    for name, ok_, ev in checks:
        print("[%s] %s" % ("PASS" if ok_ else "FAIL", name))
        print("       %s" % ev)
    verdict = all(c[1] for c in checks)
    facts["checks"] = [{"name": name, "passed": passed, "evidence": evidence}
                       for name, passed, evidence in checks]
    state["checks"] = facts["checks"]
    state["observations"].update({key: value for key, value in facts.items() if key != "checks"})
    facts["verdict"] = "PASS" if verdict else "FAIL"
    if not verdict:
        print("CONCURRENT CONSUMPTION FAIL")
        state["stage"] = "assertions"
        raise RuntimeError("one or more evidence checks failed")
    write_facts_atomic(output_dir, facts)
    state["committed"] = True
    print("FACTS_JSON " + json.dumps(facts))
    print("CONCURRENT CONSUMPTION PASS")
    state["stage"] = "complete"
    return 0


if __name__ == "__main__":
    sys.exit(main())
