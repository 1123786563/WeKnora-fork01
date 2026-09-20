"""One-process synthetic native graph smoke; credentials live only in memory."""
from __future__ import annotations

import argparse, json, secrets, time
from pathlib import Path
from urllib.request import Request, urlopen

from native_env import NativeEnvClient, NativeEnvConfig


def request(base: str, method: str, path: str, payload: dict | None = None, token: str | None = None) -> dict:
    headers = {"Accept": "application/json"}
    if token: headers["Authorization"] = f"Bearer {token}"
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode()
    if data is not None: headers["Content-Type"] = "application/json"
    with urlopen(Request(base + path, data=data, headers=headers, method=method), timeout=45) as response:  # loopback config validates base
        return json.loads(response.read() or b"{}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    config = NativeEnvConfig.from_file(args.config)
    suffix = secrets.token_hex(4); email = f"native-{suffix}@example.test"; password = "Native-" + secrets.token_urlsafe(12) + "1!"
    request(config.app_url, "POST", "/api/v1/auth/register", {"username": "native" + suffix, "email": email, "password": password})
    token = request(config.app_url, "POST", "/api/v1/auth/login", {"email": email, "password": password})["token"]
    model = request(config.app_url, "POST", "/api/v1/models", {"name": config.model_name, "type": "KnowledgeQA", "source": "remote", "parameters": {"base_url": config.ollama_url + "/v1", "api_key": "experiment-local", "provider": "openai"}}, token)["data"]
    extract = NativeEnvClient.initialization_payload(model["id"])["nodeExtract"]
    kb = request(config.app_url, "POST", "/api/v1/knowledge-bases", {"name": "native graph " + suffix, "type": "document", "indexing_strategy": {"vector_enabled": False, "keyword_enabled": False, "wiki_enabled": False, "graph_enabled": True}, "extract_config": extract}, token)["data"]
    request(config.app_url, "PUT", f"/api/v1/initialization/config/{kb['id']}", NativeEnvClient.initialization_payload(model["id"]), token)
    client = NativeEnvClient(config.app_url, token, config.query_timeout_seconds, model_version=config.model_name)
    documents = (("e-d1", "甲服务", "甲服务依赖乙服务。"), ("e-d2", "乙服务", "乙服务依赖丙服务。"))
    ids = []
    for evidence, title, content in documents:
        doc = request(config.app_url, "POST", f"/api/v1/knowledge-bases/{kb['id']}/knowledge/manual", {"title": title, "content": content, "status": "publish", "channel": "native-v03"}, token)["data"]
        ids.append(doc["id"]); client.bind_evidence(doc["id"], evidence)
    # Runtime-only poll: fail honestly if graph indexing does not reach terminal state.
    deadline = time.monotonic() + config.task_timeout_seconds
    while time.monotonic() < deadline:
        listed = request(config.app_url, "GET", f"/api/v1/knowledge-bases/{kb['id']}/knowledge", token=token).get("data", [])
        states = {row.get("id"): row.get("parse_status") for row in listed}
        if all(states.get(doc) == "completed" for doc in ids): break
        if any(states.get(doc) in {"failed", "cancelled"} for doc in ids): raise RuntimeError(f"terminal indexing failure: {states}")
        time.sleep(1)
    else: raise TimeoutError("graph indexing did not complete")
    session = request(config.app_url, "POST", "/api/v1/sessions", {"title": "native graph smoke"}, token)["data"]
    result = client.run_case({"case_id": "native-smoke", "document_revision": "synthetic-v1", "question": "甲服务依赖什么？"}, ids, session_id=session["id"])
    args.output.parent.mkdir(parents=True, exist_ok=True); args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    return 0 if result["status"] == "completed" else 1


if __name__ == "__main__": raise SystemExit(main())
