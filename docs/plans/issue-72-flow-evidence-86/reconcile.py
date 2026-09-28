#!/usr/bin/env python3
"""Offline-capable reconciliation for account-page batches and Lago Wallets."""
import argparse
from collections import Counter
from datetime import datetime, timezone
import json
import os
import re
import sys
import tempfile
from pathlib import Path
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler

from concurrent_consumption_86 import read_wallet_pages, prepare_output_dir

ALLOWED_TARGETS = {"127.0.0.1"}
CUSTOMER = os.environ.get("LAGO_CUSTOMER", "weknora-tenant-10000")
HERE = Path(__file__).resolve().parent
PERIOD_RE = re.compile(r"^\d{4}-(0[1-9]|1[0-2])$")


def base_url():
    raw = os.environ.get("LAGO_BASE", "http://127.0.0.1:48889")
    parts = urlsplit(raw)
    if parts.scheme not in ("http", "https"):
        raise SystemExit("scheme not allowed")
    if parts.hostname not in ALLOWED_TARGETS:
        raise SystemExit("target host not in test-stack allow-list")
    return raw


class _NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def api_key():
    key = os.environ.get("LAGO_API_KEY", "")
    if not key:
        raise RuntimeError("LAGO_API_KEY must be set and nonempty")
    return key


def lago_wallets(fetch_page=None):
    def fetch(page):
        query = "per_page=20&page=" + str(page)
        url = base_url() + "/api/v1/customers/" + CUSTOMER + "/wallets?" + query
        req = Request(url, headers={"Authorization": "Bearer " + api_key()})
        with build_opener(_NoRedirect).open(req, timeout=30) as resp:
            return json.load(resp)
    return read_wallet_pages(fetch_page or fetch)


def assert_no_terminated_residuals(wallets):
    if any(w.get("status") == "terminated" and _integer(w.get("balance_cents"), "wallet balance") != 0
           for w in wallets):
        raise ValueError("terminated wallet has residual balance")


def _integer(value, label):
    if type(value) is int:
        return value
    if isinstance(value, str) and re.fullmatch(r"-?[0-9]+", value):
        return int(value, 10)
    raise ValueError("%s must be an integer" % label)


def batch_error_summary(errors, limit=8):
    return "; ".join(errors[:limit])


def _instant(value, label):
    if not isinstance(value, str) or not value:
        raise ValueError("%s is missing" % label)
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise ValueError("%s is malformed" % label) from exc
    if parsed.tzinfo is None:
        raise ValueError("%s has no timezone" % label)
    return parsed.astimezone(timezone.utc)


def _period_end(period):
    if not isinstance(period, str) or not PERIOD_RE.fullmatch(period):
        raise ValueError("monthly period is malformed")
    year, month = map(int, period.split("-"))
    if month == 12:
        end = datetime(year + 1, 1, 1, tzinfo=timezone.utc)
    else:
        end = datetime(year, month + 1, 1, tzinfo=timezone.utc)
    return end


def _metadata(wallet):
    value = wallet.get("metadata", {})
    if not isinstance(value, dict):
        raise ValueError("wallet metadata is malformed")
    return value


def _family(wallet):
    metadata = _metadata(wallet)
    standard = metadata.get("weknora_period")
    purchase = metadata.get("weknora_purchase_period")
    standard = standard or None
    purchase = purchase or None
    if standard and purchase and standard != purchase:
        raise ValueError("conflicting monthly period markers")
    period = standard or purchase
    if period:
        _period_end(period)
        return "monthly", period
    name = wallet.get("name")
    if not isinstance(name, str):
        raise ValueError("wallet name is missing")
    prefixes = (CUSTOMER + "-purchase-", CUSTOMER + "-")
    for prefix in prefixes:
        if name.startswith(prefix):
            suffix = name[len(prefix):]
            try:
                _period_end(suffix)
            except ValueError:
                continue
            return "monthly", suffix
    if metadata.get("weknora_tenant") == CUSTOMER:
        return "topup", None
    raise ValueError("active wallet family is unanchored")


def _wallet_face(wallet):
    balance = _integer(wallet.get("balance_cents"), "wallet balance")
    if balance < 0:
        raise ValueError("wallet balance must be nonnegative")
    return (_instant(wallet.get("expiration_at"), "wallet expiry"),
            _instant(wallet.get("created_at"), "wallet grant time"), balance)


def _page_face(row):
    balance = _integer(row.get("balance_micro"), "page balance")
    if balance < 0:
        raise ValueError("page balance must be nonnegative")
    return (_instant(row.get("expires_at"), "page expiry"),
            _instant(row.get("granted_at"), "page grant time"),
            balance)


def reconcile_batches(page_batches, active_wallets):
    """Match monthly aggregates by period and top-ups by their full exposed face."""
    errors = []
    monthly_wallets, topup_wallets = {}, []
    for wallet in active_wallets:
        try:
            family, period = _family(wallet)
            if family == "monthly":
                monthly_wallets.setdefault(period, []).append(wallet)
            else:
                topup_wallets.append(wallet)
        except (TypeError, ValueError):
            errors.append("active wallet family or face is malformed/ambiguous")

    monthly_pages, topup_pages = {}, []
    for row in page_batches:
        if not isinstance(row, dict):
            errors.append("page batch is malformed")
            continue
        source = row.get("source")
        period = row.get("period")
        if source == "monthly":
            try:
                if not isinstance(period, str) or not period:
                    raise ValueError()
                end = _period_end(period)
                expiry = _instant(row.get("expires_at"), "page expiry")
                if expiry != end:
                    errors.append("monthly page expiry differs from UTC period end")
                if period in monthly_pages:
                    errors.append("duplicate monthly page period")
                monthly_pages[period] = row
            except (TypeError, ValueError):
                errors.append("monthly page family has a malformed period or expiry")
        elif source == "topup":
            if period not in (None, ""):
                errors.append("page family/source conflict: top-up row has a period")
                continue
            topup_pages.append(row)
        else:
            errors.append("page family/source is unknown")

    for period, wallets in monthly_wallets.items():
        row = monthly_pages.pop(period, None)
        if row is None:
            errors.append("active monthly period is missing from page")
            continue
        try:
            end = _period_end(period)
            wallet_faces = [_wallet_face(w) for w in wallets]
            if any(face[0] != end for face in wallet_faces):
                errors.append("active monthly wallet expiry differs from UTC period end")
            page_face = _page_face(row)
            if page_face[2] != sum(face[2] for face in wallet_faces) * 10_000:
                errors.append("monthly aggregate balance differs")
            if page_face[1] != min(face[1] for face in wallet_faces):
                errors.append("monthly aggregate grant time differs")
        except (TypeError, ValueError):
            errors.append("monthly wallet or page face is malformed")
    for period, row in monthly_pages.items():
        try:
            _period_end(period)
            if _integer(row.get("balance_micro"), "page balance") != 0:
                errors.append("nonzero page-only monthly orphan")
        except (TypeError, ValueError):
            errors.append("monthly orphan is malformed")

    wallet_faces = []
    for wallet in topup_wallets:
        try:
            wallet_faces.append(_wallet_face(wallet))
        except (TypeError, ValueError):
            errors.append("active top-up face is malformed")
    page_faces = []
    for row in topup_pages:
        try:
            expiry, granted, balance = _page_face(row)
            page_faces.append((expiry, granted, balance))
        except (TypeError, ValueError):
            errors.append("page top-up face is malformed")
    if any(count > 1 for count in Counter(wallet_faces).values()) or any(count > 1 for count in Counter(page_faces).values()):
        errors.append("top-up identity ambiguous: duplicate complete face")
    expected = Counter((expiry, granted, cents * 10_000) for expiry, granted, cents in wallet_faces)
    observed = Counter(page_faces)
    if expected - observed:
        errors.append("active top-up face is missing or changed")
    if observed - expected and any(face[2] != 0 for face in (observed - expected)):
        errors.append("positive page-only top-up orphan")
    return not errors, errors


def _safe_reason(exc):
    # Exception text may contain an authorization header, response body, or
    # provider data. Keep only the exception class as bounded diagnostic data.
    return type(exc).__name__[:80]


def _write_artifact(output_dir, lines):
    destination = output_dir / "reconcile-output.txt"
    staged = None
    try:
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=output_dir,
                                         prefix=".reconcile-", suffix=".tmp",
                                         delete=False) as fh:
            staged = Path(fh.name)
            fh.write("\n".join(lines) + "\n")
            fh.flush()
        os.replace(staged, destination)
        staged = None
    finally:
        if staged is not None:
            try:
                staged.unlink()
            except OSError:
                pass


def _emit_text(text, stream):
    """Best-effort text reporting; the artifact and return code are canonical."""
    try:
        print(text, file=stream, flush=True)
    except Exception:
        if stream is sys.stdout:
            try:
                _isolate_failed_stdout(stream)
            except Exception:
                pass


def _isolate_failed_stdout(stream):
    """Replace failed stdout and detach its descriptor from the failing target."""
    try:
        stdout_fd = stream.fileno()
    except (AttributeError, OSError, ValueError):
        stdout_fd = None
    try:
        devnull_stream = open(os.devnull, "w", encoding=getattr(stream, "encoding", None) or "utf-8")
    except (OSError, ValueError):
        return
    if stdout_fd == 1:
        try:
            os.dup2(devnull_stream.fileno(), stdout_fd)
        except (AttributeError, OSError, ValueError):
            pass
    try:
        sys.stdout = devnull_stream
    except (AttributeError, OSError, ValueError):
        pass


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-dir", required=True)
    args = parser.parse_args()
    try:
        output_dir = prepare_output_dir(args.output_dir)
    except SystemExit:
        raise
    except Exception as exc:
        _emit_text("RECONCILE FAIL: stage=output_preflight (%s)" % _safe_reason(exc), sys.stderr)
        return 1
    stage = "account_input"
    try:
        src = os.environ.get("WK_ACCOUNT_JSON", "weknora-account-api.json")
        credits = json.loads((HERE / src).read_text(encoding="utf-8"))["data"]["benefits"]["credits"]
        if not isinstance(credits, dict) or not isinstance(credits.get("batches"), list):
            raise ValueError("account credit batches are malformed")
        stage = "lago_fetch"
        wallets = lago_wallets()
        if not isinstance(wallets, list):
            raise ValueError("wallet pagination result is malformed")
        active = [w for w in wallets if w.get("status") == "active"]
        terminated = [w for w in wallets if w.get("status") == "terminated"]
        stage = "validation"
        assert_no_terminated_residuals(terminated)
        batches_ok, batch_errors = reconcile_batches(credits["batches"], active)
        total_cents = sum(_integer(w.get("balance_cents"), "wallet balance") for w in active)
        balance = _integer(credits.get("balance_micro"), "account balance")
        held = _integer(credits.get("held_micro"), "held balance")
        locked = _integer(credits.get("refund_locked_micro"), "refund-locked balance")
        available = _integer(credits.get("available_micro"), "available balance")
        if held < 0 or locked < 0:
            raise ValueError("held and refund-locked balances must be nonnegative")
        checks = [
            ("sum(active balance_cents)*10^4 == balance_micro", total_cents * 10_000 == balance,
             "lago cents=%d page micro=%d" % (total_cents, balance)),
            ("active monthly and top-up batch faces reconcile", batches_ok,
             batch_error_summary(batch_errors) if batch_errors else "all active faces matched"),
            ("terminated wallets have zero balance", True, "terminated wallet residual check passed"),
            ("available == balance - held - refund_locked", available == balance - held - locked,
             "%d == %d - %d - %d" % (available, balance, held, locked)),
        ]
        passed = all(item[1] for item in checks)
        lines = ["[%s] %s\n       %s" % ("PASS" if ok else "FAIL", name, detail)
                 for name, ok, detail in checks]
        lines.append("RECONCILE PASS" if passed else "RECONCILE FAIL")
        stage = "artifact_write"
        _write_artifact(output_dir, lines)
        # The artifact is canonical. Text-stream failures after publication
        # cannot alter its verdict or the verdict-derived process status.
        _emit_text("\n".join(lines), sys.stdout)
        return 0 if passed else 1
    except (Exception, SystemExit) as exc:
        reason = _safe_reason(exc)
        lines = ["RECONCILE FAIL", "stage=%s" % stage, "reason=%s" % reason]
        try:
            _write_artifact(output_dir, lines)
        except Exception as write_exc:
            safe = _safe_reason(write_exc)
            _emit_text("RECONCILE FAIL: artifact write failed (%s)" % safe, sys.stderr)
        else:
            _emit_text("\n".join(lines), sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
