"""Run one isolated, redacted OpenMeter contract case.

The runner deliberately treats a successful HTTP status as insufficient: the
case's ``expected`` object is checked recursively against the JSON response.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from scripts.saas.contract_inventory import operation_paths

WRITE_METHODS = {"POST", "PUT", "PATCH", "DELETE"}
SECRET_KEYS = re.compile(r"authorization|api[-_]?key|token|secret|password|payment|payer", re.I)
CAPTURE_REF = re.compile(r"^\$\{capture:([A-Za-z][A-Za-z0-9_.-]*)\}$")


def assert_subset(actual: object, expected: object) -> None:
    if isinstance(expected, dict):
        assert isinstance(actual, dict), "expected object"
        for key, value in expected.items():
            assert key in actual, f"missing {key}"
            assert_subset(actual[key], value)
    else:
        assert actual == expected, f"mismatch: {actual!r} != {expected!r}"


def json_pointer(document: object, pointer: str) -> object:
    if not pointer.startswith("/"):
        raise ValueError("capture must be a JSON Pointer")
    value = document
    for raw in pointer[1:].split("/"):
        part = raw.replace("~1", "/").replace("~0", "~")
        if isinstance(value, dict) and part in value:
            value = value[part]
        elif isinstance(value, list) and part.isdigit() and int(part) < len(value):
            value = value[int(part)]
        else:
            raise ValueError(f"JSON Pointer not found: {pointer}")
    return value


def redact(value: object) -> object:
    if isinstance(value, dict):
        return {k: "[REDACTED]" if SECRET_KEYS.search(k) else redact(v) for k, v in value.items()}
    if isinstance(value, list):
        return [redact(v) for v in value]
    return value


def _bind(value: object, captures: dict[str, object], *, allow_names: set[str] | None = None) -> object:
    if isinstance(value, str):
        match = CAPTURE_REF.match(value)
        if match:
            name = match.group(1)
            if name not in captures and (allow_names is None or name not in allow_names):
                raise ValueError(f"unknown capture: {name}")
            return captures[name]
        return value
    if isinstance(value, dict):
        return {k: _bind(v, captures, allow_names=allow_names) for k, v in value.items()}
    if isinstance(value, list):
        return [_bind(v, captures, allow_names=allow_names) for v in value]
    return value


def _steps(case: dict) -> list[dict]:
    request = case.get("request")
    if isinstance(request, list):
        return request
    if isinstance(request, dict):
        return [request]
    raise ValueError("case request must be an object or list")


def validate_case(case: dict, namespace: str, allow_test_writes: bool) -> None:
    if not case.get("id") or not case.get("operation_id"):
        raise ValueError("case requires id and operation_id")
    if not isinstance(case.get("expected"), (dict, list, str, int, float, bool, type(None))):
        raise ValueError("expected must be JSON")
    if not namespace:
        raise ValueError("explicit namespace is required")
    steps = _steps(case)
    declared = set(case.get("captures", {}))
    declared.update(name for step in steps for name in step.get("captures", {}))
    for step in steps:
        method = str(step.get("method", "GET")).upper()
        if method in WRITE_METHODS and not allow_test_writes:
            raise PermissionError("write request requires --allow-test-writes")
        if method in WRITE_METHODS and step.get("namespace", namespace) != namespace:
            raise ValueError("write namespace mismatch")
        if "path" in step and not str(step["path"]).startswith("/"):
            raise ValueError("path must be absolute")
        _validate_path_namespace(str(step.get("path", "")), namespace)
        _bind(step.get("body"), {}, allow_names=declared)
        bound = _bind(step.get("body"), {}, allow_names=declared)
        for key, value in _namespace_pairs(bound):
            if value != namespace:
                raise ValueError(f"embedded {key} mismatch")
        if method in WRITE_METHODS and (step.get("repeatable") or step.get("replay")) and not (step.get("idempotency_key") or case.get("idempotency_key")):
            raise ValueError("repeatable write requires idempotency_key")
    cleanup = case.get("cleanup", [])
    if cleanup and not isinstance(cleanup, list):
        raise ValueError("cleanup must be a list")
    for item in cleanup:
        if not isinstance(item, dict) or item.get("capture") not in declared:
            raise ValueError("cleanup must target a declared capture")
        if item.get("unsettled_transactions", 0):
            raise ValueError("cleanup refused: unsettled transactions")
        if not str(item.get("path", "")).startswith("/"):
            raise ValueError("cleanup path must be absolute")
        _validate_path_namespace(str(item.get("path", "")), namespace)
        for key, value in _namespace_pairs(item.get("path")):
            if value != namespace and not CAPTURE_REF.match(str(value)):
                raise ValueError(f"cleanup embedded {key} mismatch")
        for key, value in _namespace_pairs(item.get("body")):
            if value != namespace and not CAPTURE_REF.match(str(value)):
                raise ValueError(f"cleanup embedded {key} mismatch")
    for step in steps:
        if step.get("replay") and not step.get("replay_identity"):
            raise ValueError("replay requires replay_identity JSON Pointer")
        if step.get("replay_identity") and not str(step["replay_identity"]).startswith("/"):
            raise ValueError("replay_identity must be an absolute JSON Pointer")

def _validate_path_namespace(path: str, namespace: str) -> None:
    parts = path.split("/")
    for i, part in enumerate(parts[:-1]):
        if part.lower() in {"namespace", "namespaces", "tenant", "tenants"}:
            value = parts[i + 1]
            if value and value != namespace and not CAPTURE_REF.match(value):
                raise ValueError("path namespace mismatch")


def _walk(value: object):
    if isinstance(value, dict):
        for key, child in value.items():
            yield key
            yield from _walk(child)

def _namespace_pairs(value: object):
    if isinstance(value, dict):
        for key, child in value.items():
            if "namespace" in key.lower() or "tenant" in key.lower():
                yield key, child
            yield from _namespace_pairs(child)
    elif isinstance(value, list):
        for child in value:
            yield from _namespace_pairs(child)
    elif isinstance(value, list):
        for child in value:
            yield from _walk(child)


def _request_json(url: str, method: str, body: object | None, idempotency_key: str | None = None) -> tuple[int, object]:
    data = None if body is None else json.dumps(body).encode()
    headers = {"Content-Type": "application/json"}
    if idempotency_key:
        headers["Idempotency-Key"] = idempotency_key
    request = Request(url, data=data, method=method, headers=headers)
    with urlopen(request, timeout=15) as response:
        payload = response.read()
        return response.status, json.loads(payload) if payload else {}


def run_case(case: dict, base_url: str, namespace: str, allow_test_writes: bool, schema: Path, artifact_root: Path) -> None:
    validate_case(case, namespace, allow_test_writes)
    raw_schema = json.loads(schema.read_text(encoding="utf-8")) if schema.suffix == ".json" else None
    paths = ({x["operation_id"]: f'{x["method"]} {x["local_path"]}' for x in raw_schema["operations"]}
             if raw_schema and "operations" in raw_schema else operation_paths(schema.read_text(encoding="utf-8"), "/api/v3"))
    captures: dict[str, object] = {}
    records = []
    output = artifact_root / str(case["id"])
    output.mkdir(parents=True, exist_ok=True)
    try:
      for step in _steps(case):
        method = str(step.get("method", "GET")).upper()
        operation = step.get("operation_id", case["operation_id"])
        if operation not in paths:
            raise ValueError(f"unknown operation_id: {operation}")
        path = paths[operation].split(" ", 1)[1]
        path = _bind(step.get("path", path), captures)
        body = _bind(step.get("body"), captures)
        key = step.get("idempotency_key", case.get("idempotency_key"))
        try:
            status, response = _request_json(base_url.rstrip("/") + str(path), method, body, key)
        except HTTPError as exc:
            response = json.loads(exc.read().decode() or "{}")
            records.append({"method": method, "operation_id": operation, "request": redact(body), "status": exc.code, "response": redact(response)})
            raise
        record = {"method": method, "operation_id": operation, "request": redact(body), "status": status, "response": redact(response)}
        records.append(record)
        if not 200 <= status < 300:
            raise AssertionError(f"HTTP status {status}")
        for name, pointer in step.get("captures", {}).items():
            captures[name] = json_pointer(response, pointer)
        if "expected" in step:
            assert_subset(response, step["expected"])
        if step.get("replay"):
            replay_status, replay_response = _request_json(base_url.rstrip("/") + str(path), method, body, key)
            identity = json_pointer(response, step["replay_identity"])
            replay_identity = json_pointer(replay_response, step["replay_identity"])
            if replay_status != status or replay_identity != identity:
                raise AssertionError("idempotent replay changed business result")
        if "expected" in case and records:
            assert_subset(records[-1]["response"], redact(case["expected"]))
      for item in case.get("cleanup", []):
        if not allow_test_writes:
            raise PermissionError("cleanup requires --allow-test-writes")
        if item.get("unsettled_transactions", 0):
            raise ValueError("cleanup refused: unsettled transactions")
        if item["capture"] not in captures:
            raise ValueError(f"cleanup capture unavailable: {item['capture']}")
        target = captures[item["capture"]]
        if item.get("namespace", namespace) != namespace:
            raise ValueError("cleanup namespace mismatch")
        cleanup_path = _bind(item["path"], {item["capture"]: target})
        status, response = _request_json(base_url.rstrip("/") + str(cleanup_path), "DELETE", None)
        records.append({"cleanup": True, "status": status, "response": redact(response)})
        if not 200 <= status < 300: raise AssertionError(f"cleanup HTTP status {status}")
    finally:
      (output / "run.json").write_text(json.dumps(redact({"case": case["id"], "namespace": namespace, "steps": records}), indent=2) + "\n", encoding="utf-8")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--case", type=Path, required=True)
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--namespace", required=True)
    parser.add_argument("--schema", type=Path, default=Path("docs/superpowers/specs/evidence/2026-09-10-saas-billing-interface-inventory.json"))
    parser.add_argument("--allow-test-writes", action="store_true")
    parser.add_argument("--artifact-root", type=Path, default=Path("artifacts/saas-contract"))
    args = parser.parse_args(argv)
    try:
        run_case(json.loads(args.case.read_text(encoding="utf-8")), args.base_url, args.namespace, args.allow_test_writes, args.schema, args.artifact_root)
    except HTTPError as exc:
        print(f"probe failed: HTTP status {exc.code}", file=sys.stderr)
        return 1
    except (URLError, TimeoutError, ConnectionError) as exc:
        print(f"blocked-env: {exc}", file=sys.stderr)
        return 2
    except (AssertionError, PermissionError, ValueError, OSError, json.JSONDecodeError) as exc:
        print(f"probe failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
