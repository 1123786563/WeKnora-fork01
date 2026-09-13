# V02 持久图桥接与两类推理证据

生成时间：2026-09-11T15:49:11.941863+00:00；引擎：semantica-0.6.8；后端：neo4j graph store (bolt://127.0.0.1:17687)。

所有数值均由 probe 实际运行产生；未运行的能力以 unverified 记录，不伪造结果。

## 复现命令

1. 启动隔离 Neo4j（一次性，实验环境，非生产）：

       docker run -d --name semantica-v02-neo4j -p 127.0.0.1:17687:7687 -p 127.0.0.1:17474:7474 -e NEO4J_AUTH=neo4j/$SEMANTICA_EXPERIMENT_NEO4J_PASSWORD neo4j:5.26-community

   口令经 SEMANTICA_EXPERIMENT_NEO4J_PASSWORD 环境变量注入（bridge_probe.py 读取，本文件不落盘明文口令）。

2. 运行实验测试（工作树根目录）：

       uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q

3. 重新生成本证据文件：

       uv run --project semantic/experiments python semantic/experiments/bridge_probe.py evidence --output docs/superpowers/plans/semantica/bridge-evidence.md

### 持久化 + 授权过滤 + 子图重建 + 检索（roundtrip）

```json
{
  "probe": "roundtrip",
  "engine_version": "semantica-0.6.8",
  "actual_backend": "neo4j graph store (bolt://127.0.0.1:17687)",
  "fixture_documents": [
    "d1",
    "d2",
    "d3",
    "d4"
  ],
  "writer": {
    "pid": 59731,
    "exited_before_read": true,
    "server_node_count": 4,
    "server_relationship_count": 4
  },
  "persistent_node_count": 4,
  "persistent_edge_count": 4,
  "persistent_document_ids": [
    "d1",
    "d2",
    "d3",
    "d4"
  ],
  "dangling_edge_count": 0,
  "allowed_document_ids": [
    "d1",
    "d2"
  ],
  "evidence_ids": [
    "e-d1",
    "e-d2"
  ],
  "attribution_fidelity_ok": true,
  "graph": {
    "visible_documents": [
      "d1",
      "d2"
    ],
    "node_count": 3,
    "edge_count": 2,
    "path_jia_to_bing": [
      [
        "ent-jia",
        "controls",
        "ent-yi"
      ],
      [
        "ent-yi",
        "controls",
        "ent-bing"
      ]
    ]
  },
  "retrieval": {
    "query": "甲公司 控股 丙公司",
    "hit_count": 2,
    "top_hits": [
      {
        "node_id": "ent-jia",
        "score": 0.187
      },
      {
        "node_id": "ent-bing",
        "score": 0.167
      }
    ],
    "bing_related_to_jia": true
  },
  "restart_verified": true
}
```

### 规则推理：双前提（正例）

```json
{
  "probe": "rule",
  "engine_version": "semantica-0.6.8",
  "actual_backend": "semantica.reasoning.Reasoner",
  "facts": [
    "controls(a,b)",
    "controls(b,c)"
  ],
  "rules_registered": [
    "IF controls(?x, ?y) AND controls(?y, ?z) THEN controls(?x, ?z)"
  ],
  "conclusions": [
    "controls(a,c)"
  ],
  "derivations": [
    {
      "conclusion": "controls(a,c)",
      "rule_id": "rule_1",
      "premises": [
        "controls(a,b)",
        "controls(b,c)"
      ]
    }
  ]
}
```

### 规则推理：缺前提（负例）

```json
{
  "probe": "rule",
  "engine_version": "semantica-0.6.8",
  "actual_backend": "semantica.reasoning.Reasoner",
  "facts": [
    "controls(a,b)"
  ],
  "rules_registered": [
    "IF controls(?x, ?y) AND controls(?y, ?z) THEN controls(?x, ?z)"
  ],
  "conclusions": [],
  "derivations": []
}
```

### 规则推理：中文事实

```json
{
  "probe": "rule",
  "engine_version": "semantica-0.6.8",
  "actual_backend": "semantica.reasoning.Reasoner",
  "facts": [
    "controls(甲公司,乙公司)",
    "controls(乙公司,丙公司)"
  ],
  "rules_registered": [
    "IF controls(?x, ?y) AND controls(?y, ?z) THEN controls(?x, ?z)"
  ],
  "conclusions": [
    "controls(甲公司,丙公司)"
  ],
  "derivations": [
    {
      "conclusion": "controls(甲公司,丙公司)",
      "rule_id": "rule_1",
      "premises": [
        "controls(乙公司,丙公司)",
        "controls(甲公司,乙公司)"
      ]
    }
  ]
}
```

### 模型推断（受控入口）

```json
{
  "probe": "model",
  "engine_version": "semantica-0.6.8",
  "graph_summary": {
    "entities": 3,
    "relationships": 2
  },
  "query": "甲公司是否控制丙公司？",
  "status": "unverified",
  "reason": "approved model entry not configured (missing: SEMANTICA_EXPERIMENT_MODEL_PROVIDER, SEMANTICA_EXPERIMENT_MODEL_API_KEY); no model call was attempted and the capability stays unverified",
  "model_call_made": false,
  "usage": null,
  "actual_backend": "none",
  "result": null
}
```

## 附加观察

- 冲突并存：d1（controls）与 d3（divested）同时持久化且各自溯源完整（attribution_fidelity_ok=true 覆盖全部 4 条边），符合规格中冲突断言并存、不互相覆盖的要求。

## 缺口与限制

- 模型推断在本环境无受批准模型凭据，保持 unverified；未发起任何模型调用，无用量可记。
- ContextRetriever 的图检索基于关键词/结构匹配；语义向量检索路径未在本实验中验证。
- Neo4j 5.26 对 cypher id() 发出弃用告警（上游 GraphStore 内部亦使用 id()）；生产适配不得把内部 id 当稳定标识，本 probe 仅在读写当次使用。
- 写入端无跨进程锁（先清空 probe 标签再重建）；probe_roundtrip 需串行运行，pytest 即为串行。
- probe 仅验证冻结版本的能力边界，不是生产适配器；生产代码不得导入本模块。
