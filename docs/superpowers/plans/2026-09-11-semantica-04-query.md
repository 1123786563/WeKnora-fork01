# Semantica 检索与推理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付GraphRAG、规则推导、模型推断与Go统一交付链路。

**Architecture:** 三种算法共享授权子图与来源协议；Go负责融合、最终授权和回答交付。

**Tech Stack:** Go/Gin/GORM、Python/gRPC/Semantica、PostgreSQL/Neo4j、React/TypeScript；按涉及范围使用。

**Spec:** [架构规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)；[总计划与完整类型表](2026-09-11-semantica-implementation.md)。

## Global Constraints

完整继承总计划 Global Constraints，必须先读；本计划不扩大语义服务所有权、授权范围或首版能力。所有代码和测试均为后续实施输入，未执行。

---


## Q01：GraphRAG检索与有界执行

**依赖：** A02,V03。

**文件与职责：**

- `semantic/semantic_service/query/search.py`：GraphRAG服务
- `semantic/semantic_service/query/adapter.py`：冻结上游适配
- `semantic/tests/test_search.py`：实际模式/路径/预算
- `semantic/semantic_service/server.py`：Search RPC绑定

**接口：** SearchService.search(request:SearchRequest)->SearchResponse；使用A02 AuthorizedGraph和I03 pin/release。adapter.retrieve(graph:AuthorizedGraph,query:str,limits:QueryLimits)->RankedEvidence，其中RankedEvidence含evidence IDs、assertion IDs、paths、truncated。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_search_reports_truncation(search_service, scoped_search_request):
    request = replace(scoped_search_request,
        limits=replace(scoped_search_request.limits, max_nodes=1))
    result = search_service.search(request)
    assert result.truncated is True
    assert result.generation
    assert all(e.document_id in {"d1", "d2"} for e in result.evidence)
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_search.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 验证capability、scope、deadline、limits，固定generation并创建读取租约；Search finally释放租约，超期中止数据库和模型操作**

- [ ] **4. 使用授权种子与有界子图调用已验证adapter，输出来源路径；不直接调用会自动加载全库或扩充未知来源的上游方法**

- [ ] **5. 返回实际模式、generation、stale/partial/truncated；上游没有实现的模式返回FAILED_PRECONDITION，不默默切换算法**

- [ ] **6. 证据按revision/chunk/hash稳定排序去重，保留路径中的assertion引用；排序分数仅本引擎内部使用**

关键实现约束：

```
lease = index_store.pin(request.access_scope.scope)
try:
    graph = build_authorized_subgraph(snapshot, lease.manifest, seeds, request.limits)
    ranked = adapter.retrieve(graph, request.query, request.limits)
    validate_result_sources(ranked, graph)
    return to_search_response(ranked, lease.generation, graph.truncated)
finally:
    index_store.release(lease.lease_id)
# validate_result_sources/to_search_response 在search.py定义，拒绝图外来源。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_search.py -q`，预期退出码 0；另完成：单KB真实存储查询完成，多跳结果有原文证据；取消、超限、过期、unsupported明确返回；O03性能报告不以mock耗时替代真实结果。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 Q01 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): q01 GraphRAG检索与有界执行`。

## Q02：注册规则与可核验推导

**依赖：** Q01。

**文件与职责：**

- `semantic/semantic_service/query/rules.py`：版本化受限规则集
- `semantic/semantic_service/query/reason_rules.py`：规则Reasoner适配
- `semantic/tests/test_reason_rules.py`：前提/冲突/循环测试
- `semantic/rules/test-control-v1.json`：仅测试控股传递规则

**接口：** RuleRegistry.load(version:str)->RuleSet、RuleReasoner.reason(request:ReasonRequest)->ReasonResponse；RuleSet含version/digest/rules，规则只由部署注册，客户端只能指定已授权版本。事实推导DAG由assertion ID和rule ID组成。测试facts元组最后一项为assertion ID（a1/a2），其来源evidence另在fixture中绑定，不能混用两种ID。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_transitive_rule_cannot_infer_without_second_edge(rule_reasoner):
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1")], goal=("a", "controls", "c"))
    assert result.status == "insufficient_evidence"

def test_proof_lists_both_premises(rule_reasoner):
    result = rule_reasoner.reason_fixture(
        facts=[("a", "controls", "b", "a1"), ("b", "controls", "c", "a2")],
        goal=("a", "controls", "c"))
    assert set(result.premise_ids) == {"a1", "a2"}
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_reason_rules.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 定义受限规则JSON grammar、谓词白名单和版本digest，禁止eval/任意SPARQL/执行代码；reason_fixture测试fixture调用正式适配而非另一套推理器**

- [ ] **4. 将授权断言映射到已验证Reasoner输入，保存前提ID和规则ID回映射；限制轮数/事实数/时限，检测证明DAG环**

- [ ] **5. 输出supported/insufficient_evidence/conflicting_evidence/budget_exhausted，规则证明不得把缺失当否定；文本解释从证明结构产生，不补造新前提**

关键实现约束：

```
if not proof.premise_ids or not set(proof.premise_ids) <= authorized_assertion_ids:
    return insufficient_evidence()
if proof.has_cycle or not registry.contains(proof.rule_id, requested_version):
    raise InvalidProof()
# 输出conclusion_kind="rule"；不自动持久化为source类型。
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_reason_rules.py -q`，预期退出码 0；另完成：缺前提、隐藏前提、冲突、恶意规则、未知版本、递归限额有测试；黄金规则结果可人工逐条复核。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 Q02 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): q02 注册规则与可核验推导`。

## Q03：模型推断与证据不足判定

**依赖：** Q01,A03,V03。

**文件与职责：**

- `semantic/semantic_service/query/reason_model.py`：GraphReasoner受控适配
- `semantic/semantic_service/query/conclusion.py`：结构化输出校验
- `semantic/tests/test_reason_model.py`：无来源/注入/冲突测试
- `semantic/semantic_service/server.py`：Reason两类模式分派

**接口：** ModelReasoner.reason(request:ReasonRequest)->ReasonResponse；validate_conclusion(payload:dict,authorized_assertion_ids:set[str])->ReasonResponse。conclusion_kind始终model，输出解释不是规则证明；原始LLM推理过程不作为产品证据要求。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_model_cannot_cite_nonexistent_evidence():
    payload = {"status": "supported", "conclusion": "甲控制丙",
               "premise_ids": ["invented"], "conclusion_kind": "model"}
    with pytest.raises(InvalidConclusion):
        validate_conclusion(payload, {"a1", "a2"})
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_reason_model.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 把授权图和原文证据以数据输入已验证GraphReasoner adapter；所有模型调用通过A03；禁用会自动扩大知识来源或使用全局AgentMemory的功能**

- [ ] **4. 使用版本化输出schema，限制结论长度/前提数量；核验引用均来自本次授权图且引用存在只代表可追踪，不能宣称自动证明语义正确**

- [ ] **5. 无证据、冲突和预算耗尽返回明确status；外部文档的“忽略权限/调用工具”作为普通文本，服务不提供任意工具执行能力**

- [ ] **6. 生产不保存或展示模型内部思考过程，只展示简洁结论说明、证据和局限；支持状态必须有证据且通过结构校验，错误输出不回写图**

关键实现约束：

```
allowed_statuses = {"supported", "insufficient_evidence",
                    "conflicting_evidence", "budget_exhausted"}
if payload["status"] not in allowed_statuses:
    raise InvalidConclusion("invalid status")
if payload["status"] == "supported" and not payload["premise_ids"]:
    raise InvalidConclusion("missing evidence")
if not set(payload["premise_ids"]) <= authorized_assertion_ids:
    raise InvalidConclusion("unknown evidence")
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_reason_model.py -q`，预期退出码 0；另完成：用受控provider覆盖无引用、伪造引用、格式错误和注入；至少一次真实模型端到端留证，未完成不标记模型模式verified。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 Q03 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): q03 模型推断与证据不足判定`。

## Q04：Go检索融合、Agent工具与最终授权

**依赖：** Q02,Q03,I05。

**文件与职责：**

- `internal/application/service/semantic_query.go`：统一查询门面
- `internal/application/service/semantic_query_test.go`：融合/撤权/降级
- `internal/application/service/chat_pipeline/search_entity.go`：按active backend路由
- `internal/agent/tools/query_knowledge_graph.go`：工具调用新门面
- `internal/container/container.go`：注入查询服务

**接口：** SemanticQueryService.Search(ctx,subjectID string,req SearchRequest)(SearchResponse,error)、Reason(ctx,subjectID string,req ReasonRequest)(ReasonResponse,error)；服务内部签发scope，外部调用者不能自行授权。ValidateDelivery在整份回答生成后、响应内容交付前执行。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
func TestSemanticQueryDiscardsWholeResultOnRevoke(t *testing.T) {
    f := newSemanticQueryFixture(t) // 受控RPC在响应前触发真实epoch变更
    f.RevokeDuringQuery()
    _, err := f.QueryReason()
    if !errors.Is(err, ErrSemanticScopeChanged) { t.Fatalf("got %v", err) }
    if f.DeliveredContentBytes() != 0 { t.Fatal("content delivered before authorization") }
}
```

- [ ] **2. 确认 RED**。执行 `go test ./internal/application/service -run TestSemanticQuery -count=1`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 所有chat与Agent图工具进入统一门面；Go构造访问范围和预算，不能继续在semantica后端下走旧graphRepo获取旁路数据**

- [ ] **4. 普通向量/全文与语义结果按RRF或明确排名规则融合，保留revision证据；新增fusionConfig固定RRF k=60作为实验初值，去重键document/revision/chunk**

- [ ] **5. 生成最终回答后复查epoch/deny；变化时整份答案丢弃，最多一次使用新scope重算，仍变化返回可重试错误，禁止无限消耗**

- [ ] **6. 在最终检查前只发送不含知识内容的进度；普通GraphRAG可按请求配置降级并标明mode，明确Reason请求失败不可伪装普通搜索为推理**

关键实现约束：

```
score[identity] += 1.0 / (60.0 + float64(rank))
// identity包含document_id/revision/chunk_id；不直接相加不同引擎的原始score。
if err := scopeService.ValidateDelivery(ctx, issuedScope); err != nil {
    discardEntireAnswer()
    return result, ErrSemanticScopeChanged
}
// 仅检查通过后commit response；discardEntireAnswer在该服务清除缓冲内容。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `go test ./internal/application/service -run TestSemanticQuery -count=1`，预期退出码 0；另完成：go test -race ./internal/application/service ./internal/agent/tools -run Semantic -count=1；验证两入口一致、撤权无内容泄漏、降级实际模式、引用仍可定位历史revision。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 Q04 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): q04 Go检索融合、Agent工具与最终授权`。
