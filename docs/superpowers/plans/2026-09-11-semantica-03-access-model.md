# Semantica 权限与模型入口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 确保图计算和模型调用只能使用授权知识与预算。

**Architecture:** Go签发不可变范围并维护撤销epoch；Python在候选/边/推理前提阶段执行来源过滤。

**Tech Stack:** Go/Gin/GORM、Python/gRPC/Semantica、PostgreSQL/Neo4j、React/TypeScript；按涉及范围使用。

**Spec:** [架构规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)；[总计划与完整类型表](2026-09-11-semantica-implementation.md)。

## Global Constraints

完整继承总计划 Global Constraints，必须先读；本计划不扩大语义服务所有权、授权范围或首版能力。所有代码和测试均为后续实施输入，未执行。

---


## A01：可信AccessScope与权限变更屏障

**依赖：** C02,I02。

**文件与职责：**

- `internal/application/service/semantic_scope.go`：签发与最终复查
- `internal/application/service/semantic_scope_test.go`：伪造/过期/撤权测试
- `internal/handler/semantic_internal.go`：内部scope解析入口
- `internal/router/routes_infra.go`：内部认证路由隔离
- `internal/application/service/knowledgebase_access.go`：复用现有访问判定
- `internal/application/service/tenant_member.go`：UpdateRole/RemoveMember撤权接线
- `internal/application/service/kbshare.go`：UpdateSharePermission/RemoveShare分享撤销接线
- `internal/application/service/organization.go`：RemoveTenantMember/UpdateTenantMemberRole共享组织访问变更
- `internal/application/service/knowledge_transfer.go`：文档移动与克隆的源目标scope版本
- `internal/application/service/temporary_document.go`：临时资源可见性与到期校验
- `internal/application/repository/semantic_outbox.go`：epoch事务入口
- `docs/superpowers/plans/semantica/acl-write-inventory.md`：实际ACL写入口覆盖清单

**接口：** SemanticScopeService.Issue(ctx,subjectID string,scope types.SemanticScopeKey,purpose string)(types.SemanticAccessScope,error)、Resolve(ctx,scopeRef string)(ScopeSnapshot,error)、ValidateDelivery(ctx,scope types.SemanticAccessScope)error；ScopeSnapshot包含allowed_document_ids、各文档max_source_revision、allow_retained_previous、deny_revision集合及expires_at；已发布旧版本仅在allow_retained_previous为true且未被屏障拒绝时可读，不能把可见文档无限扩展为任意历史版本；令牌与scope metadata一致。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
func TestSemanticScopeRejectsEpochChange(t *testing.T) {
    f := newSemanticScopeFixture(t) // 定义受控用户、KB、文档与真实epoch repo
    scope := f.Issue("member-a", "kb-a")
    f.RevokeDocument("member-a", "private-doc")
    if err := f.ValidateDelivery(scope); !errors.Is(err, ErrSemanticScopeChanged) {
        t.Fatalf("expected scope invalidation, got %v", err)
    }
}
```

- [ ] **2. 确认 RED**。执行 `go test ./internal/application/service -run TestSemanticScope -count=1`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 盘点成员撤销、角色/分享变更、文档移动、KB迁移/删除、临时文档访问等所有能影响可见性的写入口，清单列真实文件/函数/事务；每条接入同事务epoch或fail-closed访问版本**

- [ ] **4. 基于现有访问服务生成不可变ScopeSnapshot，使用resource owner tenant而非请求者tenant；内部解析API只接受语义服务身份，校验audience/purpose/过期/哈希**

- [ ] **5. 提供交付前ValidateDelivery，对当前epoch和删除屏障复查；变化返回可识别错误，使Q04整份结果作废，不允许仅删引用**

关键实现约束：

```
if token.Audience != "weknora-semantic" || token.ExpiresAt.Before(now) {
    return ErrSemanticScopeInvalid
}
if currentEpoch != issued.PermissionEpoch || currentScopeHash != issued.ScopeHash {
    return ErrSemanticScopeChanged
}
// epoch读取必须来自业务权威；权限服务失败时直接返回错误。
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `go test ./internal/application/service -run TestSemanticScope -count=1`，预期退出码 0；另完成：覆盖跨空间共享资源owner、被撤销成员、过期/伪造scope、epoch读取失败；ACL写清单无漏接路径，否则后续查询不得上线。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 A01 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): a01 可信AccessScope与权限变更屏障`。

## A02：授权事实子图与缓存隔离

**依赖：** A01,I03,I04。

**文件与职责：**

- `semantic/semantic_service/access.py`：scope验证和断言可见性
- `semantic/semantic_service/query/subgraph.py`：有界授权子图
- `semantic/semantic_service/query/cache.py`：权限/版本分区缓存
- `semantic/tests/test_access_graph.py`：隐藏边/别名/摘要反例
- `semantic/tests/conftest.py`：access_graph真实来源fixture

**接口：** authorize_assertion(assertion_id,scope_snapshot,manifest)->bool；build_authorized_subgraph(scope_snapshot,manifest,seeds:list[str],limits:QueryLimits)->AuthorizedGraph；AuthorizedGraph包含nodes/assertions/evidence/truncated/generation。cache_key(request,manifest)->str必须包含scope hash和epoch。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_hidden_alias_cannot_seed_visible_entity(access_graph):
    result = access_graph.search_seeds(query="松柏", allowed_documents={"d1", "d2"})
    assert result == []

def test_hidden_premise_invalidates_derived_edge(access_graph):
    assert not access_graph.visible("derived-a-c", allowed_documents={"d1"})
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_access_graph.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 在向量候选和实体别名阶段先过滤来源；数据查询强制owner tenant/KB/generation/deny，后过滤补查不能作为唯一防线**

- [ ] **4. 逐边扩展只接纳有可见支持的事实；规则推导必须所有前提可见，模型派生产物需全部依赖可见；多个独立支持路径可用可见路径重建**

- [ ] **5. 达到节点/边/hop限制停止且标记truncated；禁止全库节点度数、隐藏等价边或摘要影响可见排序；首版按授权子图重算必要图指标**

- [ ] **6. 缓存key使用scope、epoch、generation、query、limits、模型/规则/配置digest；每次命中仍验证scope与deny，缓存不保存跨范围共享摘要**

关键实现约束：

```
def visible_derived(premise_ids, can_read):
    return bool(premise_ids) and all(can_read(p) for p in premise_ids)

cache_identity = (owner_tenant, kb_id, generation, scope_hash,
                  permission_epoch, query_digest, limits_digest,
                  config_digest, model_version, rule_set_version)
# can_read递归检查来源/前提并防环；授权图计算不得读取未授权图统计。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_access_graph.py -q`，预期退出码 0；另完成：真实存储fixture验证D4隐藏词不出现在种子/路径/排序/摘要/缓存；同名跨tenant不关联；大scope使用scope_ref而非突破RPC体积。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 A02 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): a02 授权事实子图与缓存隔离`。

## A03：模型代理、原始用量与预算

**依赖：** C02,I01,A01。

**文件与职责：**

- `internal/application/service/semantic_model.go`：模型准入/调用/用量记录
- `internal/application/service/semantic_model_test.go`：预算与重复用量
- `internal/handler/semantic_model_internal.go`：内部模型调用接口
- `semantic/semantic_service/model_gateway.py`：上游模型适配
- `semantic/tests/test_model_gateway.py`：取消/重试/代理适配
- `semantic/tests/conftest.py`：model_gateway fixture启动Go测试代理和受控provider
- `internal/router/routes_infra.go`：内部模型调用路由，不能走普通用户API权限
- `migrations/versioned/000097_semantic_invocations.up.sql`：原始调用与预算记录
- `migrations/versioned/000097_semantic_invocations.down.sql`：PG回退
- `migrations/sqlite/000018_semantic_invocations.up.sql`：SQLite等价记录
- `migrations/sqlite/000018_semantic_invocations.down.sql`：SQLite回退

**接口：** ModelGateway.invoke(invocation_id:str,operation_id:str,model_profile_ref:str,messages:list,budget_ref:str)->ModelResult；ModelResult含text、input_tokens、output_tokens、provider_request_id、status。ledger.claim仅获执行权的首次调用返回new，其余返回completed/in_flight/unknown，避免把本次claim误当重复请求。Go SemanticBudgetPort.Reserve/Finalize/Reconcile围绕invocation ID；复用已实现预算系统，否则建立本任务预算仓库和原始用量表，不伪称已接商业结算。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_retry_same_invocation_does_not_double_finalize(model_gateway):
    first = model_gateway.invoke("inv-1", "op-1", "model-a", MESSAGES, "budget-a")
    second = model_gateway.invoke("inv-1", "op-1", "model-a", MESSAGES, "budget-a")
    assert first == second
    assert model_gateway.recorded_invocations("inv-1") == 1
# model_gateway fixture：真实Go内部HTTP+受控provider，计数来自持久调用表。
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_model_gateway.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 定义operation/query关联短期模型能力凭据，校验tenant、用途、model_profile、budget；禁止任意URL/长期key透传；验证Semantica各调用入口都经过该adapter**

- [ ] **4. 把预算预占、调用记录、实际用量和未知结果分开；未发送前失败可释放，provider可能执行但响应丢失进入unknown并对账，不能盲重发同一invocation**

- [ ] **5. 重试的新真实provider调用分配新的invocation ID，关联原任务；父任务只引用子调用，不再次记录聚合消费。平台模型与BYOK均记录原始用量但不自行创造收费规则**

- [ ] **6. 传播deadline/cancel并限制输出token；生产凭据不落日志；PG97/SQLite18实施前重核迁移号，权限/额度不足返回明确错误**

关键实现约束：

```
claim = ledger.claim(invocation_id, request_hash)
if claim.state == "completed":
    return claim.saved_result
if claim.state in {"in_flight", "unknown"}:
    raise InvocationNeedsReconciliation()
reservation = budget.reserve(budget_ref, invocation_id, upper_bound)
# provider调用后ledger.save_actual并按同一invocation finalize；未知结果保留预占待对账。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_model_gateway.py -q`，预期退出码 0；另完成：Go TestSemanticModel覆盖额度竞争、BYOK原始用量、unknown对账；真实模型至少一次证明上游无旁路直连；无凭据保持真实调用项未通过。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 A03 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): a03 模型代理、原始用量与预算`。
