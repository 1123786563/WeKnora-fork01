# Semantica 能力验证 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 证明冻结版本能支持目标接口，并建立真实质量基线。

**Architecture:** 实验环境独立于生产服务；先证实持久存储、授权子图桥接与两种推理。

**Tech Stack:** Go/Gin/GORM、Python/gRPC/Semantica、PostgreSQL/Neo4j、React/TypeScript；按涉及范围使用。

**Spec:** [架构规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)；[总计划与完整类型表](2026-09-11-semantica-implementation.md)。

## Global Constraints

完整继承总计划 Global Constraints，必须先读；本计划不扩大语义服务所有权、授权范围或首版能力。所有代码和测试均为后续实施输入，未执行。

---


## V01：冻结版本与最小安装契约

**依赖：** 无。

**文件与职责：**

- `semantic/experiments/pyproject.toml`：候选版本的隔离环境
- `semantic/experiments/uv.lock`：精确依赖锁
- `semantic/experiments/test_import_contract.py`：导入/API 契约测试
- `semantic/experiments/verify_version.py`：生成带版本与签名摘要的证据
- `docs/superpowers/plans/semantica/capability-evidence.json`：记录验证结果

**接口：** 产出 evidence.schema_version=1、python_version、semantica_version、source_revision、lock_hash、capabilities[name].status/signature/evidence_path；下游只读取该文件，不从文档猜测当前 API。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_required_import_contract():
    from semantica.semantic_extract import NERExtractor, RelationExtractor
    from semantica.context import ContextGraph, ContextRetriever
    from semantica.reasoning import Reasoner, GraphReasoner
    from semantica.graph_store import GraphStore
    assert all(callable(c) for c in (
        NERExtractor, RelationExtractor, ContextGraph,
        ContextRetriever, Reasoner, GraphReasoner, GraphStore))
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 读取选定 release 的 pyproject、源码和安装元数据；以官方示例中的 0.6.8 为候选，核验可获取性与 Python 兼容要求。候选不可用时记录原因，再选择可验证的明确 release，禁止用 latest 漂移**

- [ ] **4. 为实验环境固定 Python patch 版本与必要 extras；逐项记录调用签名、包锁和上游源码提交。只安装实际需要的 graph/extract/reason 模块依赖**

- [ ] **5. 实现 verify_version.py，记录成功/失败/不可用原因，日志清除凭据；生成摘要包含命令及退出码，禁止只写布尔 passed**

关键实现约束：

```
from importlib.metadata import version
from inspect import signature
from hashlib import sha256
from pathlib import Path
record = {"semantica_version": version("semantica"),
          "lock_hash": sha256(Path("semantic/experiments/uv.lock").read_bytes()).hexdigest()}
# 对每个验证过的 callable 保存 str(signature(callable))，并写入真实测试结果。
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q`，预期退出码 0；另完成：在干净隔离环境重放一次 lock 安装；确认不依赖 docreader 虚拟环境；失败则停止涉及该能力的下游。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 V01 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): v01 冻结版本与最小安装契约`。

## V02：验证持久图桥接和两类推理

**依赖：** V01。

**文件与职责：**

- `semantic/experiments/test_graph_bridge.py`：真实 Neo4j 持久化/重启实验
- `semantic/experiments/test_reasoning_bridge.py`：规则及模型推理验证
- `semantic/experiments/fixtures/controlled_graph.json`：带文档来源的中文图
- `semantic/experiments/bridge_probe.py`：窄上游适配，非生产实现
- `docs/superpowers/plans/semantica/bridge-evidence.md`：成功与缺口记录

**接口：** 新增 probe_roundtrip(fixture_path)->dict、probe_rule(facts,rules)->dict、probe_model(graph,query)->dict；返回 evidence_ids、result、engine_version、actual_backend。它们仅是实验脚本函数，不供生产导入。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_persistent_bridge_keeps_source_ids():
    result = probe_roundtrip("semantic/experiments/fixtures/controlled_graph.json")
    assert result["restart_verified"] is True
    assert set(result["evidence_ids"]) == {"e-d1", "e-d2"}

def test_rule_requires_both_premises():
    result = probe_rule(["controls(a,b)"], ["controls(x,y)&controls(y,z)->controls(x,z)"])
    assert "controls(a,c)" not in result["conclusions"]
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 在隔离 Neo4j 写入 fixture，关闭客户端并重启实验服务后读取；显式转换为保持来源 ID 的内存子图，再执行检索，证明不是仅查询同一内存对象**

- [ ] **4. 按已核实 API 构造注册规则实验和模型自然语言推断实验，加入缺前提/冲突/中文 quote；只允许批准的模型入口，实际调用用量独立记录**

- [ ] **5. 记录无法实现的持久图算法、代理配置或来源映射；能力标记 unavailable 时返回原因，不能改用全库图或直接模型凭据规避**

关键实现约束：

```
persistent_rows = read_fixture_from_neo4j()
allowed_rows = [r for r in persistent_rows if r["document_id"] in {"d1", "d2"}]
subgraph = build_probe_graph(allowed_rows)
# build_probe_graph 在 bridge_probe.py 定义，保留 assertion_id/evidence_ids。
assert all("evidence_ids" in row for row in allowed_rows)
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q`，预期退出码 0；另完成：保留规则实际输出和模型原始用量摘要；无真实模型调用证据时模型能力保持 unverified；确认重启后仍能追踪来源。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 V02 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): v02 验证持久图桥接和两类推理`。

## V03：中文质量与上线阈值评估基线

**依赖：** V02。

**文件与职责：**

- `semantic/experiments/evaluate.py`：统一评分与成本/延迟汇总
- `semantic/experiments/test_evaluate.py`：评分器正确性
- `semantic/experiments/fixtures/questions.jsonl`：至少30个有人工预期的问题
- `docs/superpowers/plans/semantica/evaluation-baseline.md`：native与Semantica对照
- `docs/superpowers/plans/semantica/acceptance-policy.json`：待用户确认的数值门槛

**接口：** 定义 score_case(expected_evidence:set[str],actual_evidence:set[str])->dict；评估产物记录 correct、source_precision、source_recall、unanswerable_correct、latency_ms、tokens、query_mode；policy.approved 初值 false，只有明确验收决策可变 true。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_evidence_score_penalizes_unsupported_source():
    score = score_case({"e1"}, {"e1", "hidden"})
    assert score["precision"] == 0.5
    assert score["recall"] == 1.0
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 建立事实直答、多跳、冲突、无答案、中文定位与权限反例各至少5题；合成数据可公开，真实企业内容须获得授权且不进入仓库**

- [ ] **4. 同一文档版本/模型配置分别运行 native 与候选模式；区分冷启动和热查询，记录索引耗时、p50/p95与实际用量**

- [ ] **5. 根据实测提出明确质量/延迟/单次用量阈值，写入 policy 的 proposed 部分和理由；用户未确认时上线门禁保持关闭，其他独立实现可以继续**

关键实现约束：

```
def score_case(expected_evidence, actual_evidence):
    overlap = len(expected_evidence & actual_evidence)
    return {"precision": overlap / len(actual_evidence) if actual_evidence else 0.0,
            "recall": overlap / len(expected_evidence) if expected_evidence else 1.0}
# 权限泄漏独立 hard gate，不能通过平均分抵消。
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `uv run --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q`，预期退出码 0；另完成：评分器通过、至少30题完整结果和失败例均留档；policy 中不存在无实测支撑的 approved=true。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 V03 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): v03 中文质量与上线阈值评估基线`。

## 实验命令与输出合同

V01实现的命令：`uv run --project semantic/experiments python semantic/experiments/verify_version.py --output docs/superpowers/plans/semantica/capability-evidence.json`。脚本成功退出0；缺任一要求的包/签名退出非零并仍写失败原因。source_revision不能从包版本猜，读取所选release的源码提交并记录来源URL。

V03实现的命令：`uv run --project semantic/experiments python semantic/experiments/evaluate.py --backend native --dataset semantic/experiments/fixtures/questions.jsonl --output docs/superpowers/plans/semantica/native-results.jsonl`；第二次把backend改为semantica、output改为semantica-results.jsonl。支持backend只有native/semantica，native经隔离测试部署的现有图抽取/检索入口调用；URL从明确的实验配置读，禁止默认连接真实生产环境。每行输出关联case_id、document_revision、mode、engine/model版本、证据集合、实际用量、耗时与错误；失败例也写出，不能从结果集中剔除。

O03使用同一CLI与冻结dataset复跑正式适配，并记录commit与锁摘要。live model凭据只通过实验环境注入，不写入结果和命令行。
