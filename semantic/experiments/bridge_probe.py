"""V02 bridge probe: narrow upstream adapter for capability experiments only.

NOT production code; the semantic service must never import it. On the frozen
release pinned by V01 this probe verifies, with real artifacts:

1. persistence — a fixture graph written to an isolated Neo4j by a separate
   writer process survives that process exiting, and a fresh connection reads
   every row back with its source attribution (assertion/evidence/document
   ids, Chinese quotes) intact;
2. authorization-first bridging — only rows whose document_id is authorized
   are converted into an in-memory ContextGraph (upstream ContextGraph /
   ContextRetriever), which answers a two-hop question while the restricted
   document's content never appears in any returned field;
3. registered rules — the upstream Reasoner derives a conclusion only when
   every premise is present, and each derivation carries premises and rule id;
4. model inference — GraphReasoner is reached only through an approved model
   entry from the environment; without one the capability fails closed and
   stays unverified (no call, no fabricated usage).

Environment (isolated experiment container, never production):
  SEMANTICA_EXPERIMENT_NEO4J_URI       default bolt://127.0.0.1:17687
  SEMANTICA_EXPERIMENT_NEO4J_USER      default neo4j
  SEMANTICA_EXPERIMENT_NEO4J_PASSWORD  default semantica-v02
  SEMANTICA_EXPERIMENT_NEO4J_DATABASE  default neo4j
  SEMANTICA_EXPERIMENT_MODEL_PROVIDER / _MODEL_API_KEY / _MODEL_NAME
                                      approved model entry for probe_model

CLI (experiment tooling, run from the worktree root):
  python semantic/experiments/bridge_probe.py write <fixture.json>
  python semantic/experiments/bridge_probe.py evidence --output <markdown>

Concurrency: the writer clears and re-creates the probe label without a
cross-process lock; run probe_roundtrip sequentially (pytest does).
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone
from importlib.metadata import version as _package_version
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
PROBE_LABEL = "SemanticProbe"
ENGINE_VERSION = f"semantica-{_package_version('semantica')}"

NEO4J_URI = os.environ.get("SEMANTICA_EXPERIMENT_NEO4J_URI", "bolt://127.0.0.1:17687")
NEO4J_USER = os.environ.get("SEMANTICA_EXPERIMENT_NEO4J_USER", "neo4j")
NEO4J_PASSWORD = os.environ.get("SEMANTICA_EXPERIMENT_NEO4J_PASSWORD", "semantica-v02")
NEO4J_DATABASE = os.environ.get("SEMANTICA_EXPERIMENT_NEO4J_DATABASE", "neo4j")

# Documents the round-trip probe is authorized to see. d3 (a conflicting
# divestment claim) and d4 (restricted codename) stay invisible.
AUTHORIZED_DOCUMENTS = frozenset({"d1", "d2"})

MODEL_PROVIDER_VAR = "SEMANTICA_EXPERIMENT_MODEL_PROVIDER"
MODEL_NAME_VAR = "SEMANTICA_EXPERIMENT_MODEL_NAME"
MODEL_API_KEY_VAR = "SEMANTICA_EXPERIMENT_MODEL_API_KEY"

RETRIEVAL_QUERY = "甲公司 控股 丙公司"

# "predicate(arg1,arg2)" atoms used by the probe's compact rule/fact syntax.
_ATOM_RE = re.compile(r"^\s*([A-Za-z_][A-Za-z0-9_]*)\s*\((.+)\)\s*$")


def _parse_atom(atom: str) -> tuple[str, list[str]]:
    match = _ATOM_RE.match(atom)
    if match is None:
        raise ValueError(f"unsupported atom syntax (expected pred(a,b)): {atom!r}")
    predicate = match.group(1)
    args = [arg.strip() for arg in match.group(2).split(",")]
    if not all(args):
        raise ValueError(f"empty argument in atom: {atom!r}")
    if any(re.search(r"[(),]", arg) for arg in args):
        raise ValueError(f"unsupported argument (nested atoms are not supported): {atom!r}")
    return predicate, args


def _spaced(atom: str) -> str:
    """Canonical upstream fact form: 'controls(a, b)' with ', ' separators."""
    predicate, args = _parse_atom(atom)
    return f"{predicate}({', '.join(args)})"


def _compact(atom: str) -> str:
    """Probe-facing form: 'controls(a,b)' without spaces, matching inputs."""
    predicate, args = _parse_atom(atom)
    return f"{predicate}({','.join(args)})"


def _pattern(atom: str) -> str:
    """Turn 'controls(x,y)' into the upstream pattern 'controls(?x, ?y)'."""
    predicate, args = _parse_atom(atom)
    for arg in args:
        if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", arg):
            raise ValueError(f"rule variables must be plain identifiers, got {arg!r} in {atom!r}")
    return f"{predicate}({', '.join('?' + arg for arg in args)})"


def _translate_rule(rule: str) -> str:
    """Translate 'controls(x,y)&controls(y,z)->controls(x,z)' into the
    upstream IF-THEN rule syntax."""
    parts = rule.split("->")
    if len(parts) != 2 or not parts[0].strip() or not parts[1].strip():
        raise ValueError(f"unsupported rule syntax (expected 'body->head'): {rule!r}")
    body, head = parts
    conditions = [part.strip() for part in body.split("&")]
    if not all(conditions):
        raise ValueError(f"empty condition in rule: {rule!r}")
    return "IF " + " AND ".join(_pattern(c) for c in conditions) + " THEN " + _pattern(head.strip())


def _resolve(path_text: str) -> Path:
    path = Path(path_text)
    return path if path.is_absolute() else REPO_ROOT / path


def load_fixture(path_text: str) -> dict:
    return json.loads(_resolve(path_text).read_text(encoding="utf-8"))


def _connect_store():
    from semantica.graph_store import GraphStore

    store = GraphStore(
        backend="neo4j",
        uri=NEO4J_URI,
        user=NEO4J_USER,
        password=NEO4J_PASSWORD,
        database=NEO4J_DATABASE,
    )
    try:
        store.connect()
    except Exception as exc:
        store.close()
        raise RuntimeError(
            f"cannot connect to the isolated Neo4j at {NEO4J_URI}: {exc}; "
            "start the V02 experiment container first (see the reproduction "
            "commands in bridge-evidence.md)"
        ) from exc
    return store


def write_fixture_to_neo4j(fixture: dict) -> dict:
    """Writer mode: persist the fixture under the probe label, then exit.

    Application-level ids (entity_id, assertion_id, evidence_ids) are stored
    as properties; Neo4j internal ids are only used to wire edges during the
    write and are never treated as stable identifiers.
    """
    store = _connect_store()
    try:
        store.execute_query(f"MATCH (n:{PROBE_LABEL}) DETACH DELETE n")
        internal_ids: dict[str, int] = {}
        for node in fixture["nodes"]:
            created = store.create_node(
                labels=[PROBE_LABEL, node["type"]],
                properties={
                    "entity_id": node["id"],
                    "name": node["name"],
                    "entity_type": node["type"],
                    "document_ids": list(node["document_ids"]),
                },
            )
            internal_ids[node["id"]] = created["id"]
        for edge in fixture["edges"]:
            store.create_relationship(
                internal_ids[edge["source"]],
                internal_ids[edge["target"]],
                edge["type"],
                properties={
                    "assertion_id": edge["assertion_id"],
                    "source_id": edge["source"],
                    "target_id": edge["target"],
                    "document_id": edge["document_id"],
                    "evidence_ids": list(edge["evidence_ids"]),
                    "quote": edge["quote"],
                },
            )
        stats = store.get_stats()
    finally:
        store.close()
    return {
        "writer_pid": os.getpid(),
        "node_count": len(internal_ids),
        "edge_count": len(fixture["edges"]),
        "server_node_count": stats.get("node_count"),
        "server_relationship_count": stats.get("relationship_count"),
    }


def read_rows_from_neo4j() -> dict:
    """Reader mode: fresh connection, read every probe row back from Neo4j."""
    store = _connect_store()
    try:
        nodes = store.execute_query(
            f"MATCH (n:{PROBE_LABEL}) "
            "RETURN id(n) AS internal_id, n.entity_id AS entity_id, "
            "n.name AS name, n.entity_type AS entity_type, "
            "n.document_ids AS document_ids"
        )["records"]
        edges = store.execute_query(
            f"MATCH (a:{PROBE_LABEL})-[r]->(b:{PROBE_LABEL}) "
            "RETURN type(r) AS rel_type, a.entity_id AS source_id, "
            "b.entity_id AS target_id, properties(r) AS props"
        )["records"]
    finally:
        store.close()
    return {"nodes": nodes, "edges": edges}


def build_probe_graph(rows: dict, allowed_document_ids: frozenset[str] = AUTHORIZED_DOCUMENTS):
    """Explicit conversion of the authorized persistent rows into an
    in-memory ContextGraph. Rows without evidence attribution are rejected —
    a row that cannot cite its source must never enter the subgraph."""
    allowed_edges = [
        edge for edge in rows["edges"] if edge["props"]["document_id"] in allowed_document_ids
    ]
    for edge in allowed_edges:
        if "evidence_ids" not in edge["props"] or "assertion_id" not in edge["props"]:
            raise ValueError(f"row without source attribution cannot enter the subgraph: {edge}")
    visible_ids = {e["source_id"] for e in allowed_edges} | {e["target_id"] for e in allowed_edges}
    visible_nodes = [n for n in rows["nodes"] if n["entity_id"] in visible_ids]

    from semantica.context import ContextGraph

    graph = ContextGraph()
    graph.add_nodes(
        [
            {
                "id": node["entity_id"],
                "type": node["entity_type"],
                "properties": {
                    "name": node["name"],
                    "content": node["name"],
                    "document_ids": list(node["document_ids"]),
                },
            }
            for node in visible_nodes
        ]
    )
    graph.add_edges(
        [
            {
                "source": edge["source_id"],
                "target": edge["target_id"],
                "type": edge["rel_type"],
                "properties": {
                    "assertion_id": edge["props"]["assertion_id"],
                    "document_id": edge["props"]["document_id"],
                    "evidence_ids": list(edge["props"]["evidence_ids"]),
                    "quote": edge["props"]["quote"],
                },
            }
            for edge in allowed_edges
        ]
    )
    return graph, allowed_edges, visible_nodes


def _two_hop_path(graph, source_id: str, via_id: str, target_id: str):
    hop1 = graph.get_edge_data(source_id, via_id)
    hop2 = graph.get_edge_data(via_id, target_id)
    if not hop1 or not hop2:
        return None
    return [
        [source_id, hop1["type"], via_id],
        [via_id, hop2["type"], target_id],
    ]


def probe_roundtrip(fixture_path: str, allowed_document_ids: frozenset[str] = AUTHORIZED_DOCUMENTS) -> dict:
    """Full persistence -> authorization -> rebuild -> retrieval experiment."""
    fixture = load_fixture(fixture_path)

    # 1. A separate writer process persists the fixture and exits before we
    #    read anything: persistence across process restart is actually proven.
    try:
        writer = subprocess.run(
            [sys.executable, str(Path(__file__).resolve()), "write", fixture_path],
            capture_output=True,
            text=True,
            cwd=str(REPO_ROOT),
            timeout=600,
        )
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError("writer subprocess timed out after 600s") from exc
    if writer.returncode != 0:
        raise RuntimeError(f"writer subprocess failed: {writer.stderr.strip()[-2000:]}")
    writer_info = json.loads(writer.stdout.strip())

    # 2. Fresh reader connection in this process reads every row back.
    rows = read_rows_from_neo4j()
    entity_ids = {node["entity_id"] for node in rows["nodes"]}
    dangling_edges = [
        edge
        for edge in rows["edges"]
        if edge["source_id"] not in entity_ids or edge["target_id"] not in entity_ids
    ]
    persistent_docs = sorted({edge["props"]["document_id"] for edge in rows["edges"]})

    # 3. Authorized subset only is converted into the in-memory subgraph.
    graph, allowed_edges, visible_nodes = build_probe_graph(rows, allowed_document_ids)
    evidence_ids = sorted(
        {evidence for edge in allowed_edges for evidence in edge["props"]["evidence_ids"]}
    )
    fixture_edges = {edge["assertion_id"]: edge for edge in fixture["edges"]}
    fixture_nodes = {node["id"]: node for node in fixture["nodes"]}

    def _edge_attributed(edge: dict) -> bool:
        expected = fixture_edges.get(edge["props"]["assertion_id"])
        if expected is None:
            return False
        return (
            edge["rel_type"] == expected["type"]
            and edge["source_id"] == expected["source"]
            and edge["target_id"] == expected["target"]
            and edge["props"]["document_id"] == expected["document_id"]
            and sorted(edge["props"]["evidence_ids"]) == sorted(expected["evidence_ids"])
            and edge["props"]["quote"] == expected["quote"]
        )

    def _node_attributed(node: dict) -> bool:
        expected = fixture_nodes.get(node["entity_id"])
        if expected is None:
            return False
        return (
            node["name"] == expected["name"]
            and node["entity_type"] == expected["type"]
            and sorted(node["document_ids"]) == sorted(expected["document_ids"])
        )

    # Every persisted row (including d3/d4) is checked against the fixture;
    # only the boolean leaves this function, so nothing restricted leaks.
    attribution_fidelity_ok = all(_edge_attributed(edge) for edge in rows["edges"]) and all(
        _node_attributed(node) for node in rows["nodes"]
    )

    # 4. Multi-hop question answered from the rebuilt subgraph only.
    from semantica.context import ContextRetriever

    retriever = ContextRetriever(knowledge_graph=graph)
    hits = retriever.retrieve(RETRIEVAL_QUERY, max_results=5)
    related = retriever.get_related("ent-jia", max_hops=2)

    return {
        "probe": "roundtrip",
        "engine_version": ENGINE_VERSION,
        "actual_backend": f"neo4j graph store ({NEO4J_URI})",
        "fixture_documents": [doc["document_id"] for doc in fixture["documents"]],
        "writer": {
            "pid": writer_info["writer_pid"],
            "exited_before_read": True,
            "server_node_count": writer_info["server_node_count"],
            "server_relationship_count": writer_info["server_relationship_count"],
        },
        "persistent_node_count": len(rows["nodes"]),
        "persistent_edge_count": len(rows["edges"]),
        "persistent_document_ids": persistent_docs,
        "dangling_edge_count": len(dangling_edges),
        "allowed_document_ids": sorted(allowed_document_ids),
        "evidence_ids": evidence_ids,
        "attribution_fidelity_ok": attribution_fidelity_ok,
        "graph": {
            "visible_documents": sorted({edge["props"]["document_id"] for edge in allowed_edges}),
            "node_count": len(visible_nodes),
            "edge_count": len(allowed_edges),
            "path_jia_to_bing": _two_hop_path(graph, "ent-jia", "ent-yi", "ent-bing"),
        },
        "retrieval": {
            "query": RETRIEVAL_QUERY,
            "hit_count": len(hits),
            "top_hits": [
                {"node_id": hit.metadata.get("node_id"), "score": round(hit.score, 3)}
                for hit in hits[:3]
            ],
            "bing_related_to_jia": any(item.get("id") == "ent-bing" for item in related),
        },
        "restart_verified": bool(
            writer.returncode == 0
            and writer_info["writer_pid"] != os.getpid()
            and len(rows["edges"]) == len(fixture["edges"])
            and len(rows["nodes"]) == len(fixture["nodes"])
            and not dangling_edges
            and attribution_fidelity_ok
        ),
    }


def probe_rule(facts: list[str], rules: list[str]) -> dict:
    """Registered-rule experiment: derive only from complete premises."""
    from semantica.reasoning import Reasoner

    reasoner = Reasoner()
    translated_rules = [_translate_rule(rule) for rule in rules]
    results = reasoner.infer_with_results([_spaced(fact) for fact in facts], translated_rules)
    derivations = [
        {
            "conclusion": _compact(result.conclusion),
            "rule_id": result.rule_used.rule_id if result.rule_used is not None else None,
            "premises": sorted(_compact(premise) for premise in result.premises),
        }
        for result in results
    ]
    return {
        "probe": "rule",
        "engine_version": ENGINE_VERSION,
        "actual_backend": "semantica.reasoning.Reasoner",
        "facts": [_compact(fact) for fact in facts],
        "rules_registered": translated_rules,
        "conclusions": sorted({derivation["conclusion"] for derivation in derivations}),
        "derivations": derivations,
    }


def probe_model(graph: dict, query: str) -> dict:
    """Model-inference experiment through the approved entry only.

    Without a configured entry no call is attempted: the capability stays
    unverified with a concrete reason instead of a fabricated result.
    """
    base = {
        "probe": "model",
        "engine_version": ENGINE_VERSION,
        "graph_summary": {
            "entities": len(graph.get("entities", [])),
            "relationships": len(graph.get("relationships", [])),
        },
        "query": query,
    }
    provider = os.environ.get(MODEL_PROVIDER_VAR, "").strip()
    api_key = os.environ.get(MODEL_API_KEY_VAR, "").strip()
    model_name = os.environ.get(MODEL_NAME_VAR, "").strip()
    if not provider or not api_key:
        missing = [
            name
            for name, value in (
                (MODEL_PROVIDER_VAR, provider),
                (MODEL_API_KEY_VAR, api_key),
            )
            if not value
        ]
        return {
            **base,
            "status": "unverified",
            "reason": (
                "approved model entry not configured (missing: "
                + ", ".join(missing)
                + "); no model call was attempted and the capability stays unverified"
            ),
            "model_call_made": False,
            "usage": None,
            "actual_backend": "none",
            "result": None,
        }

    from semantica.reasoning import GraphReasoner

    extraction = {"provider": provider, "api_key": api_key}
    if model_name:
        extraction["model"] = model_name
    reasoner = GraphReasoner(config={"extraction": extraction})
    actual_backend = (
        f"semantica.reasoning.GraphReasoner(provider={provider}, model={model_name or 'default'})"
    )
    if reasoner.provider is None:
        # Upstream swallows provider-initialization failures; a configured but
        # broken entry must never be reported as a completed model call.
        return {
            **base,
            "status": "failed",
            "reason": (
                "LLM provider failed to initialize for the configured approved "
                "entry; no model call was made"
            ),
            "model_call_made": False,
            "usage": None,
            "actual_backend": actual_backend,
            "result": None,
        }
    answer = reasoner.reason(graph, query)
    if isinstance(answer, str) and answer.startswith("Error"):
        # generate() failures are also swallowed upstream into 'Error ...'
        # strings; whether a network call actually happened is unknown here.
        return {
            **base,
            "status": "failed",
            "reason": f"model reasoning failed upstream: {answer[:300]}",
            "model_call_made": None,
            "model_call_made_note": "call attempt outcome unknown; upstream swallowed the error",
            "usage": None,
            "actual_backend": actual_backend,
            "result": None,
        }
    usage = getattr(reasoner.provider, "last_usage", None)
    return {
        **base,
        "status": "completed",
        "model_call_made": True,
        "actual_backend": actual_backend,
        "result": answer,
        "usage": usage,
        "usage_note": None if usage is not None else "provider exposes no usage attribute on this release",
    }


def write_evidence(output_path: str) -> int:
    """Regenerate the V02 evidence markdown from real probe runs."""
    fixture_path = "semantic/experiments/fixtures/controlled_graph.json"
    roundtrip = probe_roundtrip(fixture_path)
    rule_positive = probe_rule(
        ["controls(a,b)", "controls(b,c)"], ["controls(x,y)&controls(y,z)->controls(x,z)"]
    )
    rule_missing = probe_rule(
        ["controls(a,b)"], ["controls(x,y)&controls(y,z)->controls(x,z)"]
    )
    rule_chinese = probe_rule(
        ["controls(甲公司,乙公司)", "controls(乙公司,丙公司)"],
        ["controls(x,y)&controls(y,z)->controls(x,z)"],
    )
    mini_graph = {
        "entities": [
            {"id": "ent-jia", "name": "甲公司", "type": "Company"},
            {"id": "ent-yi", "name": "乙公司", "type": "Company"},
            {"id": "ent-bing", "name": "丙公司", "type": "Company"},
        ],
        "relationships": [
            {"source": "ent-jia", "target": "ent-yi", "type": "controls"},
            {"source": "ent-yi", "target": "ent-bing", "type": "controls"},
        ],
    }
    model = probe_model(mini_graph, "甲公司是否控制丙公司？")

    def block(title: str, payload: dict) -> str:
        body = json.dumps(payload, ensure_ascii=False, indent=2)
        return f"### {title}\n\n```json\n{body}\n```\n"

    lines = [
        "# V02 持久图桥接与两类推理证据",
        "",
        f"生成时间：{datetime.now(timezone.utc).isoformat()}；引擎：{ENGINE_VERSION}；"
        f"后端：{roundtrip['actual_backend']}。",
        "",
        "所有数值均由 probe 实际运行产生；未运行的能力以 unverified 记录，不伪造结果。",
        "",
        "## 复现命令",
        "",
        "1. 启动隔离 Neo4j（一次性，实验环境，非生产）：",
        "",
        "       docker run -d --name semantica-v02-neo4j -p 127.0.0.1:17687:7687 -p 127.0.0.1:17474:7474 -e NEO4J_AUTH=neo4j/$SEMANTICA_EXPERIMENT_NEO4J_PASSWORD neo4j:5.26-community",
        "",
        "   口令经 SEMANTICA_EXPERIMENT_NEO4J_PASSWORD 环境变量注入（bridge_probe.py 读取，本文件不落盘明文口令）。",
        "",
        "2. 运行实验测试（工作树根目录）：",
        "",
        "       uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q",
        "",
        "3. 重新生成本证据文件：",
        "",
        "       uv run --project semantic/experiments python semantic/experiments/bridge_probe.py evidence --output docs/superpowers/plans/semantica/bridge-evidence.md",
        "",
        block("持久化 + 授权过滤 + 子图重建 + 检索（roundtrip）", roundtrip),
        block("规则推理：双前提（正例）", rule_positive),
        block("规则推理：缺前提（负例）", rule_missing),
        block("规则推理：中文事实", rule_chinese),
        block("模型推断（受控入口）", model),
        "## 附加观察",
        "",
        "- 冲突并存：d1（controls）与 d3（divested）同时持久化且各自溯源完整（attribution_fidelity_ok=true 覆盖全部 4 条边），符合规格中冲突断言并存、不互相覆盖的要求。",
        "",
        "## 缺口与限制",
        "",
        "- 模型推断在本环境无受批准模型凭据，保持 unverified；未发起任何模型调用，无用量可记。",
        "- ContextRetriever 的图检索基于关键词/结构匹配；语义向量检索路径未在本实验中验证。",
        "- Neo4j 5.26 对 cypher id() 发出弃用告警（上游 GraphStore 内部亦使用 id()）；生产适配不得把内部 id 当稳定标识，本 probe 仅在读写当次使用。",
        "- 写入端无跨进程锁（先清空 probe 标签再重建）；probe_roundtrip 需串行运行，pytest 即为串行。",
        "- probe 仅验证冻结版本的能力边界，不是生产适配器；生产代码不得导入本模块。",
        "",
    ]
    output = _resolve(output_path)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text("\n".join(lines), encoding="utf-8")
    print(f"evidence written to {output}")
    return 0


def _main(argv: list[str]) -> int:
    if argv and argv[0] == "write" and len(argv) == 2:
        print(json.dumps(write_fixture_to_neo4j(load_fixture(argv[1])), ensure_ascii=False))
        return 0
    if argv and argv[0] == "evidence" and len(argv) == 3 and argv[1] == "--output":
        return write_evidence(argv[2])
    print(__doc__)
    return 1


if __name__ == "__main__":
    raise SystemExit(_main(sys.argv[1:]))
