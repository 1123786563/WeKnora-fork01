"""Stdlib-only client and isolation contract for the native GraphRAG experiment.

This module is deliberately an experiment adapter.  It never selects a default
service, reads ``.env``, or accepts a non-loopback endpoint.
"""
from __future__ import annotations

import json
import re
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Mapping, Sequence
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen


class OwnershipError(ValueError):
    """Raised when a cleanup target is outside this experiment run."""


def _loopback_url(value: str) -> str:
    parsed = urlparse(value)
    if parsed.scheme != "http" or parsed.hostname not in {"127.0.0.1", "localhost", "::1"}:
        raise ValueError("app_url must be an explicit http loopback URL")
    if not parsed.port:
        raise ValueError("app_url must include an explicit port")
    return value.rstrip("/")


def _slug(value: str) -> str:
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,30}", value):
        raise ValueError("run_id must be lower-case letters, digits, and hyphens")
    return value


@dataclass(frozen=True)
class NativeEnvConfig:
    run_id: str = "v03-native"
    app_url: str = "http://127.0.0.1:18081"
    postgres_port: int = 15432
    redis_port: int = 16379
    neo4j_bolt_port: int = 18687
    neo4j_http_port: int = 18474
    request_timeout_seconds: int = 45
    task_timeout_seconds: int = 240
    query_timeout_seconds: int = 60
    ollama_url: str = "http://127.0.0.1:11434"
    model_name: str = "qwen2.5:0.5b"
    artifact_dir: Path = Path("semantic/experiments/native_env/artifacts")

    @classmethod
    def from_mapping(cls, values: Mapping[str, Any]) -> "NativeEnvConfig":
        defaults = cls()
        config = cls(
            run_id=_slug(str(values.get("run_id", defaults.run_id))),
            app_url=_loopback_url(str(values.get("app_url", defaults.app_url))),
            postgres_port=int(values.get("postgres_port", defaults.postgres_port)),
            redis_port=int(values.get("redis_port", defaults.redis_port)),
            neo4j_bolt_port=int(values.get("neo4j_bolt_port", defaults.neo4j_bolt_port)),
            neo4j_http_port=int(values.get("neo4j_http_port", defaults.neo4j_http_port)),
            request_timeout_seconds=int(values.get("request_timeout_seconds", defaults.request_timeout_seconds)),
            task_timeout_seconds=int(values.get("task_timeout_seconds", defaults.task_timeout_seconds)),
            query_timeout_seconds=int(values.get("query_timeout_seconds", defaults.query_timeout_seconds)),
            ollama_url=_loopback_url(str(values.get("ollama_url", defaults.ollama_url))),
            model_name=str(values.get("model_name", defaults.model_name)),
            artifact_dir=Path(values.get("artifact_dir", defaults.artifact_dir)),
        )
        if config.model_name != "qwen2.5:0.5b":
            raise ValueError("only qwen2.5:0.5b is allowed for this bounded experiment")
        if len({config.postgres_port, config.redis_port, config.neo4j_bolt_port, config.neo4j_http_port}) != 4:
            raise ValueError("native dependency ports must be unique")
        if min(config.postgres_port, config.redis_port, config.neo4j_bolt_port, config.neo4j_http_port) < 1024:
            raise ValueError("native dependency ports must be unprivileged")
        return config

    @classmethod
    def from_file(cls, path: Path) -> "NativeEnvConfig":
        return cls.from_mapping(json.loads(path.read_text(encoding="utf-8")))

    @property
    def prefix(self) -> str:
        return f"semantica-v03-native-{self.run_id}"

    def assert_owned(self, resource_name: str) -> None:
        if not resource_name.startswith(self.prefix + "-"):
            raise OwnershipError(f"refusing to touch non-experiment resource: {resource_name}")

    def compose_document(self) -> dict[str, Any]:
        prefix = self.prefix
        return {
            "name": prefix,
            "services": {
                "postgres": {
                    "image": "postgres:15.2-alpine@sha256:d9c304353c031b21e9a7e33dc4781e272a9fa802a2ab9703fe4199d72ba1422c",
                    "ports": [f"127.0.0.1:{self.postgres_port}:5432"],
                    "environment": {"POSTGRES_USER": "native", "POSTGRES_DB": "native", "POSTGRES_PASSWORD": "${NATIVE_POSTGRES_PASSWORD}"},
                    "volumes": [f"{prefix}-postgres:/var/lib/postgresql/data"],
                    "networks": [f"{prefix}-network"],
                },
                "redis": {
                    "image": "redis:7.4-alpine@sha256:520775a41a63e77e06c73e35d2fd9cc15921a609516818796b4ecbb813078bc7",
                    "ports": [f"127.0.0.1:{self.redis_port}:6379"],
                    "command": ["redis-server", "--requirepass", "${NATIVE_REDIS_PASSWORD}"],
                    "networks": [f"{prefix}-network"],
                },
                "neo4j": {
                    "image": "neo4j:2025.10.1@sha256:155c8aad10d5c838bc3bbc476c0418779086547822acb214ec5e3d49ba336907",
                    "ports": [f"127.0.0.1:{self.neo4j_bolt_port}:7687", f"127.0.0.1:{self.neo4j_http_port}:7474"],
                    "environment": {"NEO4J_AUTH": "neo4j/${NATIVE_NEO4J_PASSWORD}", "NEO4J_PLUGINS": "[\"apoc\"]"},
                    "volumes": [f"{prefix}-neo4j:/data"],
                    "networks": [f"{prefix}-network"],
                },
            },
            "volumes": {f"{prefix}-postgres": {}, f"{prefix}-neo4j": {}},
            "networks": {f"{prefix}-network": {"name": f"{prefix}-network"}},
        }


@dataclass
class NativeEnvClient:
    app_url: str
    token: str
    timeout_seconds: int = 45
    engine_version: str | None = None
    model_version: str = "qwen2.5:0.5b"
    _knowledge_evidence: dict[str, str] = field(default_factory=dict)

    def __post_init__(self) -> None:
        self.app_url = _loopback_url(self.app_url)

    def _request(self, path: str, payload: Mapping[str, Any], *, method: str = "POST", stream: bool = False) -> Any:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        request = Request(self.app_url + path, body, method=method, headers={"Authorization": f"Bearer {self.token}", "Content-Type": "application/json", "Accept": "text/event-stream" if stream else "application/json"})
        try:
            with urlopen(request, timeout=self.timeout_seconds) as response:  # noqa: S310 -- loopback is validated
                raw = response.read().decode("utf-8")
        except HTTPError as error:
            raise RuntimeError(f"{path} returned HTTP {error.code}: {error.read().decode('utf-8', 'replace')[:500]}") from error
        except URLError as error:
            raise RuntimeError(f"{path} is unavailable: {error.reason}") from error
        if stream:
            return self._sse_events(raw)
        return json.loads(raw) if raw else {}

    @staticmethod
    def _sse_events(raw: str) -> list[dict[str, Any]]:
        events: list[dict[str, Any]] = []
        for block in raw.replace("\r\n", "\n").split("\n\n"):
            payload = "\n".join(line[5:].lstrip() for line in block.splitlines() if line.startswith("data:"))
            if not payload:
                continue
            try:
                events.append(json.loads(payload))
            except json.JSONDecodeError:
                events.append({"type": "unparsed", "data": payload})
        return events

    @staticmethod
    def initialization_payload(model_id: str) -> dict[str, Any]:
        return {"llmModelId": model_id, "nodeExtract": {"enabled": True,
            "text": "提取软件依赖关系。", "tags": ["服务", "依赖"],
            "nodes": [{"name": "服务"}],
            "relations": [{"node1": "服务", "node2": "服务", "type": "depends_on"}]}}

    def bind_graph_model(self, knowledge_base_id: str, model_id: str) -> dict[str, Any]:
        return self._request(f"/api/v1/initialization/config/{knowledge_base_id}", self.initialization_payload(model_id), method="PUT")

    def bind_evidence(self, knowledge_id: str, evidence_id: str) -> None:
        """Associate a runtime knowledge ID with its frozen dataset evidence ID."""
        self._knowledge_evidence[knowledge_id] = evidence_id

    def evidence_ids_for_references(self, references: Sequence[Mapping[str, Any]], _allowed_document_ids: Sequence[str]) -> list[str]:
        """Return only evidence actually cited by graph references.

        The allowed document IDs constrain the request body; they are never
        result evidence by themselves.
        """
        return list(dict.fromkeys(
            self._knowledge_evidence[reference["knowledge_id"]]
            for reference in references
            if reference.get("match_type") == "graph" and reference.get("knowledge_id") in self._knowledge_evidence
        ))

    def query(self, case: Mapping[str, Any], allowed_document_ids: Sequence[str], *, session_id: str) -> dict[str, Any]:
        if not case.get("case_id") or not case.get("document_revision") or not case.get("question"):
            raise ValueError("case_id, document_revision, and question are required")
        started = time.monotonic()
        events = self._request(f"/api/v1/knowledge-chat/{session_id}", {"query": case["question"], "knowledge_ids": list(allowed_document_ids), "disable_title": True}, stream=True)
        references: list[dict[str, Any]] = []
        for event in events:
            data = event.get("data")
            if event.get("type") in {"reference", "references"} and isinstance(data, dict):
                references.append(data)
        return {
            "case_id": case["case_id"], "document_revision": case["document_revision"],
            "requested_mode": "native", "actual_mode": "native", "status": "completed",
            "engine_version": self.engine_version, "model_version": self.model_version,
            "evidence_ids": self.evidence_ids_for_references(references, allowed_document_ids), "references": references,
            "latency_ms": round((time.monotonic() - started) * 1000, 3), "tokens": None, "error": None,
        }

    def run_case(self, case: Mapping[str, Any], allowed_document_ids: Sequence[str], *, session_id: str) -> dict[str, Any]:
        try:
            return self.query(case, allowed_document_ids, session_id=session_id)
        except (RuntimeError, ValueError) as error:
            return {"case_id": case.get("case_id"), "document_revision": case.get("document_revision"), "requested_mode": "native", "actual_mode": "native", "status": "failed", "engine_version": self.engine_version, "model_version": self.model_version, "evidence_ids": list(allowed_document_ids), "references": [], "latency_ms": None, "tokens": None, "error": str(error)}
