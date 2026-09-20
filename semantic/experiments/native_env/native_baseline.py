"""Transactional synthetic native graph smoke with retained, sanitized evidence."""
from __future__ import annotations

import argparse
import hashlib
import json
import secrets
import subprocess
import time
from pathlib import Path
from typing import Any, Callable
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from native_env import NativeEnvClient, NativeEnvConfig
from orchestrate import _compose, paths_for, teardown, up


def _redact(value: str) -> str:
    for marker in ("token=", "password=", "Bearer "):
        if marker in value:
            before, after = value.split(marker, 1)
            value = before + marker + "[redacted]" + (" " + after.split(" ", 1)[1] if " " in after else "")
    return value[:1000]


def _atomic_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    temporary.replace(path)


def request(base: str, method: str, path: str, payload: dict | None = None, token: str | None = None) -> dict:
    headers = {"Accept": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode()
    if data is not None:
        headers["Content-Type"] = "application/json"
    try:
        with urlopen(Request(base + path, data=data, headers=headers, method=method), timeout=45) as response:  # loopback config validates base
            return json.loads(response.read() or b"{}")
    except (HTTPError, URLError) as error:
        raise RuntimeError(f"{method} {path}: {_redact(str(error))}") from error


def documents_terminal_with_graph(rows: list[dict], document_ids: list[str]) -> bool:
    by_id = {row.get("id"): row for row in rows}
    return all(by_id.get(doc_id, {}).get("parse_status") == "completed" and by_id[doc_id].get("pending_subtasks_count") == 0 for doc_id in document_ids)


def verify_persisted_graph_config(response: dict) -> None:
    node_extract = response.get("data", {}).get("nodeExtract", {})
    if node_extract.get("enabled") is not True:
        raise RuntimeError("persisted nodeExtract.enabled is not true")


def _wait_for_documents(config: NativeEnvConfig, token: str, kb_id: str, document_ids: list[str]) -> list[dict]:
    deadline = time.monotonic() + config.task_timeout_seconds
    while time.monotonic() < deadline:
        rows = request(config.app_url, "GET", f"/api/v1/knowledge-bases/{kb_id}/knowledge", token=token).get("data", [])
        if documents_terminal_with_graph(rows, document_ids):
            return rows
        if any(row.get("id") in document_ids and row.get("parse_status") in {"failed", "cancelled"} for row in rows):
            raise RuntimeError(f"terminal indexing failure: {rows}")
        time.sleep(1)
    raise TimeoutError("graph indexing did not complete with zero pending subtasks")


def _graph_count(config: NativeEnvConfig, document_ids: list[str]) -> int:
    paths = paths_for(config)
    values = dict(line.split("=", 1) for line in paths.env.read_text().splitlines() if "=" in line)
    query = "MATCH (n) WHERE n.kg IN [" + ",".join(json.dumps(value) for value in document_ids) + "] RETURN count(n)"
    result = _compose(config, paths, "exec", "-T", "-e", f"NEO4J_PASSWORD={values['NATIVE_NEO4J_PASSWORD']}", "neo4j", "sh", "-c", "cypher-shell -u neo4j -p \"$NEO4J_PASSWORD\" --format plain " + json.dumps(query))
    if result.returncode:
        raise RuntimeError("isolated Neo4j graph probe failed")
    digits = [line.strip() for line in result.stdout.splitlines() if line.strip().isdigit()]
    if not digits:
        raise RuntimeError("isolated Neo4j graph probe returned no count")
    return int(digits[-1])


def load_frozen_cases(path: Path) -> list[dict[str, Any]]:
    cases = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]
    if not cases or any(not case.get("case_id") or not case.get("documents") for case in cases):
        raise ValueError("frozen dataset must contain case_id and visible documents for every case")
    return cases


def _wait_for_deleted(config: NativeEnvConfig, token: str, kb_id: str, document_id: str) -> None:
    deadline = time.monotonic() + config.task_timeout_seconds
    while time.monotonic() < deadline:
        rows = request(config.app_url, "GET", f"/api/v1/knowledge-bases/{kb_id}/knowledge", token=token).get("data", [])
        if document_id not in {row.get("id") for row in rows}:
            return
        time.sleep(1)
    raise TimeoutError("deleted source remained visible after native delete task")


def run_frozen_case(config: NativeEnvConfig, *, token: str, model_id: str, case: dict[str, Any], suffix: str) -> dict[str, Any]:
    """Run one isolated native transaction; never project an unobserved source."""
    extract = NativeEnvClient.initialization_payload(model_id)["nodeExtract"]
    kb = request(config.app_url, "POST", "/api/v1/knowledge-bases", {"name": f"native v03 {case['case_id']} {suffix}", "type": "document", "indexing_strategy": {"vector_enabled": False, "keyword_enabled": False, "wiki_enabled": False, "graph_enabled": True}, "extract_config": extract}, token)["data"]
    request(config.app_url, "PUT", f"/api/v1/initialization/config/{kb['id']}", NativeEnvClient.initialization_payload(model_id), token)
    verify_persisted_graph_config(request(config.app_url, "GET", f"/api/v1/initialization/config/{kb['id']}", token=token))
    client = NativeEnvClient(config.app_url, token, config.query_timeout_seconds, model_version=config.model_name)
    runtime: dict[str, str] = {}
    sources = list(case["documents"]) + list(case.get("forbidden_sources", []))
    indexing_started = time.monotonic()
    for source in sources:
        document = request(config.app_url, "POST", f"/api/v1/knowledge-bases/{kb['id']}/knowledge/manual", {"title": source["evidence_id"], "content": source["text"], "status": "publish", "channel": "native-v03"}, token)["data"]
        runtime[source["evidence_id"]] = document["id"]
        client.bind_evidence(document["id"], source["evidence_id"])
    _wait_for_documents(config, token, kb["id"], list(runtime.values()))
    indexing_ms = round((time.monotonic() - indexing_started) * 1000, 3)
    allowed_ids = [runtime[evidence_id] for evidence_id in case["allowed_evidence"]]
    actions: list[dict[str, Any]] = []
    for source in case.get("forbidden_sources", []):
        state = source.get("state")
        document_id = runtime[source["evidence_id"]]
        source["runtime_id"] = document_id
        if state == "deleted":
            request(config.app_url, "POST", "/api/v1/knowledge/batch-delete", {"kb_id": kb["id"], "ids": [document_id]}, token)
            _wait_for_deleted(config, token, kb["id"], document_id)
            actions.append({"evidence_id": source["evidence_id"], "state": state, "status": "completed"})
        elif state == "revoked":
            # The current public knowledge API has no per-document revoke action.
            actions.append({"evidence_id": source["evidence_id"], "state": state, "status": "unsupported", "reason": "no native public per-document revocation API"})
        else:
            actions.append({"evidence_id": source["evidence_id"], "state": state, "status": "unsupported", "reason": "unknown forbidden-source state"})
    session = request(config.app_url, "POST", "/api/v1/sessions", {"title": "native v03 " + case["case_id"]}, token)["data"]
    cold = client.run_case(case, allowed_ids, session_id=session["id"], phase="cold")
    warm = client.run_case(case, allowed_ids, session_id=session["id"], phase="warm")
    result = dict(cold)
    result.update({"indexing_ms": indexing_ms, "query_attempts": [cold, warm], "state_actions": actions, "allowed_runtime_ids": allowed_ids, "forbidden_sources": case.get("forbidden_sources", [])})
    if any(action["status"] == "unsupported" for action in actions):
        result["unsupported_stage"] = "revocation"
    if result.get("status") == "completed" and not result.get("evidence_ids"):
        result.update({"status": "negative", "failure_stage": "graph_reference", "error": "completed native query had no mapped graph reference"})
    return result


def provision_frozen_cases(config: NativeEnvConfig, dataset: Path) -> dict[str, Any]:
    suffix = secrets.token_hex(4)
    email = f"native-{suffix}@example.test"
    password = "Native-" + secrets.token_urlsafe(12) + "1!"
    request(config.app_url, "POST", "/api/v1/auth/register", {"username": "native" + suffix, "email": email, "password": password})
    token = request(config.app_url, "POST", "/api/v1/auth/login", {"email": email, "password": password})["token"]
    model = request(config.app_url, "POST", "/api/v1/models", {"name": config.model_name, "type": "KnowledgeQA", "source": "remote", "parameters": {"base_url": config.ollama_url + "/v1", "api_key": "experiment-local", "provider": "openai"}}, token)["data"]
    rows: list[dict[str, Any]] = []
    for case in load_frozen_cases(dataset):
        try:
            rows.append(run_frozen_case(config, token=token, model_id=model["id"], case=case, suffix=suffix))
        except Exception as error:
            rows.append({"case_id": case["case_id"], "document_revision": case["document_revision"], "requested_mode": "native", "actual_mode": "native", "status": "failed", "failure_stage": "provision_or_query", "error": _redact(str(error)), "evidence_ids": [], "references": [], "tokens": None, "latency_ms": None, "indexing_ms": None, "query_attempts": [], "state_actions": []})
    return {"cases": rows, "model_version": config.model_name}


def provision_and_query(config: NativeEnvConfig) -> dict[str, Any]:
    suffix = secrets.token_hex(4)
    email = f"native-{suffix}@example.test"
    password = "Native-" + secrets.token_urlsafe(12) + "1!"
    request(config.app_url, "POST", "/api/v1/auth/register", {"username": "native" + suffix, "email": email, "password": password})
    token = request(config.app_url, "POST", "/api/v1/auth/login", {"email": email, "password": password})["token"]
    model = request(config.app_url, "POST", "/api/v1/models", {"name": config.model_name, "type": "KnowledgeQA", "source": "remote", "parameters": {"base_url": config.ollama_url + "/v1", "api_key": "experiment-local", "provider": "openai"}}, token)["data"]
    extract = NativeEnvClient.initialization_payload(model["id"])["nodeExtract"]
    kb = request(config.app_url, "POST", "/api/v1/knowledge-bases", {"name": "native graph " + suffix, "type": "document", "indexing_strategy": {"vector_enabled": False, "keyword_enabled": False, "wiki_enabled": False, "graph_enabled": True}, "extract_config": extract}, token)["data"]
    request(config.app_url, "PUT", f"/api/v1/initialization/config/{kb['id']}", NativeEnvClient.initialization_payload(model["id"]), token)
    verify_persisted_graph_config(request(config.app_url, "GET", f"/api/v1/initialization/config/{kb['id']}", token=token))
    client = NativeEnvClient(config.app_url, token, config.query_timeout_seconds, model_version=config.model_name)
    document_ids = []
    indexing_started_at = time.time()
    for evidence, title, content in (("e-d1", "甲服务", "甲服务依赖乙服务。"), ("e-d2", "乙服务", "乙服务依赖丙服务。")):
        document = request(config.app_url, "POST", f"/api/v1/knowledge-bases/{kb['id']}/knowledge/manual", {"title": title, "content": content, "status": "publish", "channel": "native-v03"}, token)["data"]
        document_ids.append(document["id"])
        client.bind_evidence(document["id"], evidence)
    _wait_for_documents(config, token, kb["id"], document_ids)
    indexing_finished_at = time.time()
    graph_count = _graph_count(config, document_ids)
    if graph_count < 1:
        raise RuntimeError("isolated Neo4j has no source-linked graph data")
    session = request(config.app_url, "POST", "/api/v1/sessions", {"title": "native graph smoke"}, token)["data"]
    first_query_started_at = time.time()
    result = client.run_case({"case_id": "native-smoke", "document_revision": "synthetic-v1", "question": "甲服务依赖什么？"}, document_ids, session_id=session["id"])
    result["graph_node_count"] = graph_count
    result["raw_sse"] = client.last_sse_raw or ""
    result["indexing_started_at"] = indexing_started_at
    result["indexing_finished_at"] = indexing_finished_at
    result["first_query_started_at"] = first_query_started_at
    result["first_query_finished_at"] = time.time()
    return result


def _source_commit() -> str | None:
    result = subprocess.run(["git", "rev-parse", "HEAD"], check=False, text=True, capture_output=True)
    return result.stdout.strip() if result.returncode == 0 else None


def run_lifecycle(config: NativeEnvConfig, output: Path, *, dataset: Path | None = None, up_fn: Callable[[NativeEnvConfig], int] = up, provision_fn: Callable[[NativeEnvConfig], dict[str, Any]] = provision_and_query, teardown_fn: Callable[[NativeEnvConfig], int] = teardown) -> int:
    started = time.time()
    source_commit = _source_commit()
    result: dict[str, Any] = {"run_id": config.run_id, "source_commit": source_commit, "engine_version": source_commit, "images": {name: service["image"] for name, service in config.compose_document()["services"].items()}, "started_at": started, "indexing_started_at": None, "command_exits": {}}
    exit_code = 1
    try:
        result["command_exits"]["up"] = up_fn(config)
        if result["command_exits"]["up"] != 0:
            raise RuntimeError("isolated startup failed")
        result["indexing_started_at"] = time.time()
        result.update(provision_frozen_cases(config, dataset) if dataset else provision_fn(config))
        if dataset:
            result["status"] = "completed" if all(row.get("status") == "completed" for row in result["cases"]) else "negative"
            result["failure_stage"] = None if result["status"] == "completed" else "case_results"
            exit_code = 0 if result["status"] == "completed" else 1
            return exit_code
        raw_sse = _redact(str(result.pop("raw_sse", "")))
        if raw_sse:
            sse_path = output.with_name(output.stem + "-sse.log")
            sse_path.write_text(raw_sse, encoding="utf-8")
            result["sanitized_sse"] = {"path": sse_path.name, "sha256": hashlib.sha256(raw_sse.encode()).hexdigest()}
        if result.get("status") != "completed":
            raise RuntimeError(str(result.get("error") or "native query failed"))
        if not result.get("evidence_ids"):
            result.update({"status": "negative", "failure_stage": "graph_reference", "error": "completed SSE had no mapped graph reference"})
        else:
            exit_code = 0
    except Exception as error:  # preserve every operational failure as experiment evidence
        result.update({"status": "failed", "failure_stage": result.get("failure_stage") or ("startup" if result["command_exits"].get("up", 0) else "provision"), "error": _redact(str(error)), "evidence_ids": [], "references": []})
    finally:
        result["finished_at"] = time.time()
        try:
            result["command_exits"]["teardown"] = teardown_fn(config)
            if result["command_exits"]["teardown"] != 0:
                result["teardown_failure"] = f"teardown exited {result['command_exits']['teardown']}"
                result.update({"status": "failed", "failure_stage": "teardown", "error": result["teardown_failure"]})
                exit_code = 1
        except Exception as error:
            result["command_exits"]["teardown"] = 1
            result["teardown_failure"] = _redact(str(error))
            result.update({"status": "failed", "failure_stage": "teardown", "error": result["teardown_failure"]})
            exit_code = 1
        _atomic_json(output, result)
    return exit_code


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--dataset", type=Path)
    args = parser.parse_args()
    return run_lifecycle(NativeEnvConfig.from_file(args.config), args.output, dataset=args.dataset)


if __name__ == "__main__":
    raise SystemExit(main())
