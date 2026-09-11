# Semantica 索引生命周期 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现可恢复、可删除、可原子发布的文档语义索引。

**Architecture:** Go事务outbox与Python持久operation分工；发布只切换已验证不可变manifest。

**Tech Stack:** Go/Gin/GORM、Python/gRPC/Semantica、PostgreSQL/Neo4j、React/TypeScript；按涉及范围使用。

**Spec:** [架构规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)；[总计划与完整类型表](2026-09-11-semantica-implementation.md)。

## Global Constraints

完整继承总计划 Global Constraints，必须先读；本计划不扩大语义服务所有权、授权范围或首版能力。所有代码和测试均为后续实施输入，未执行。

---

迁移顺序：I01服务库001 → I02业务PG96/SQLite17 → I03服务库002 → I04服务库003。业务迁移号实施前查重；测试必须覆盖PG与SQLite两条业务路径。

## I01：持久操作、幂等与worker租约

**依赖：** C02。

**文件与职责：**

- `semantic/migrations/001_operations.sql`：operations唯一键与租约schema
- `semantic/semantic_service/operations.py`：操作状态机与仓库
- `semantic/semantic_service/worker.py`：领取、续租、恢复
- `semantic/tests/conftest.py`：operation_store真实PG fixture
- `semantic/tests/test_operations.py`：重复投递/失联worker/取消竞争

**接口：** OperationStore.accept(req:ApplyRequest)->Operation、claim(worker_id:str,lease_seconds:int)->Operation|None、transition(operation_id:str,lease_token:int,expected:str,next_state:str)->bool、cancel(scope:ScopeKey,operation_id:str)->Operation；以ScopeKey+idempotency_key唯一，payload_hash冲突报错。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_repeated_apply_returns_same_operation(operation_store, apply_request):
    first = operation_store.accept(apply_request)
    second = operation_store.accept(apply_request)
    assert first.operation_id == second.operation_id

def test_stale_lease_cannot_publish(operation_store, claimed_operation):
    op = claimed_operation
    assert not operation_store.transition(op.operation_id, op.lease_token - 1,
                                          "staged", "publishing")
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_operations.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 建立service专属schema与迁移器；operation表保存请求hash、阶段、租约到期、递增lease_token、错误码、重试时间；唯一键冲突查回原操作并校验payload**

- [ ] **4. 使用事务和FOR UPDATE SKIP LOCKED领取，续租必须匹配token；每次接管增加token，过期worker不能写stage、结果或终态**

- [ ] **5. 实现状态允许表、超时恢复与取消CAS；Cancel succeeded返回FAILED_PRECONDITION；终态不可逆，取消后关闭内部数据库/模型调用上下文**

关键实现约束：

```
UPDATE semantic_operations
SET state = :next_state, updated_at = CURRENT_TIMESTAMP
WHERE operation_id = :operation_id AND state = :expected
  AND lease_token = :token AND lease_until > CURRENT_TIMESTAMP;
-- rowcount=0 表示失去租约或状态竞争，禁止继续发布。
-- worker领取在同一事务内递增token并写lease_until。
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_operations.py -q`，预期退出码 0；另完成：真实PG双连接并发claim只有一个获租约；杀worker后新worker接管；Cancel/publish竞争只能一个终态成立；进程重启后幂等仍成立。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 I01 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): i01 持久操作、幂等与worker租约`。

## I02：业务revision、outbox与授权版本

**依赖：** C01。

**文件与职责：**

- `migrations/versioned/000096_semantic_control.up.sql`：PG业务控制表
- `migrations/versioned/000096_semantic_control.down.sql`：PG回退
- `migrations/sqlite/000017_semantic_control.up.sql`：SQLite同等业务约束
- `migrations/sqlite/000017_semantic_control.down.sql`：SQLite回退
- `internal/types/semantic_control.go`：业务控制记录
- `internal/application/repository/semantic_outbox.go`：同事务revision/outbox/删除屏障
- `internal/application/repository/semantic_outbox_test.go`：事务回滚与重复事件
- `internal/database/semantic_migration_test.go`：PG/SQLite迁移合同

**接口：** 定义 SemanticMutation{TenantID uint64, KBID,DocumentID string, ExpectedRevision uint64, Deleted bool, Payload []byte}；WithSemanticMutation(ctx,mutation,func(tx *gorm.DB)error)(revision uint64,err error)。业务资源修改、revision、outbox、删除屏障必须同事务，禁止另开隐含事务。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
func TestSemanticMutationRollback(t *testing.T) {
    db := newSemanticTestDB(t) // 本测试文件定义：隔离SQLite库并执行正式迁移
    repo := NewSemanticControlRepository(db)
    _, err := repo.WithSemanticMutation(context.Background(), mutationFixture(),
        func(tx *gorm.DB) error { return errors.New("abort resource update") })
    if err == nil { t.Fatal("expected rollback") }
    var count int64
    db.Table("semantic_outbox").Count(&count)
    if count != 0 { t.Fatal("event escaped resource transaction") }
}
```

- [ ] **2. 确认 RED**。执行 `go test ./internal/application/repository -run TestSemanticMutation -count=1`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 实施前检查最新迁移号；当前建议PG96/SQLite17，如被占用则分配下一空号并同步本计划/台账，不重写已执行迁移**

- [ ] **4. 建立semantic_document_revisions、semantic_outbox、semantic_access_epochs、semantic_denials、semantic_backend_states与semantic_completion_receipts；唯一键包含tenant/KB/document或event身份，PG/SQLite分别实现等价CAS**

- [ ] **5. 实现transaction回调，按expected_revision CAS递增；删除同时创建deny并提高KB epoch。权限变更提供同事务BumpSemanticEpoch(tx,scope)入口给A01接线**

- [ ] **6. 增加outbox领取/确认/失败退避；事件含完整revision/config与payload hash。收件方幂等，outbox只在对方持久接收后确认**

关键实现约束：

```
BEGIN;
UPDATE semantic_document_revisions SET revision=revision+1
WHERE tenant_id=:tenant AND kb_id=:kb AND document_id=:doc
  AND revision=:expected;
-- 检查rowcount；首次插入使用唯一键保护。
-- 在同一tx内执行业务回调、写outbox，删除时写deny并更新epoch。
COMMIT;
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `go test ./internal/application/repository -run TestSemanticMutation -count=1`，预期退出码 0；另完成：执行 go test ./internal/database -run TestSemanticMigration -count=1；PG真实迁移测试必须另连隔离库，验证up/down/up、唯一键和事务失败无孤立事件；SQLite不开服务时仍可使用native。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 I02 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): i02 业务revision、outbox与授权版本`。

## I03：有来源的构图与generation原子发布

**依赖：** C03,I01,I02,A03。

**文件与职责：**

- `semantic/migrations/002_generations.sql`：active指针、manifest和读取租约
- `semantic/semantic_service/indexing/manifest.py`：不可变版本清单
- `semantic/semantic_service/indexing/builder.py`：抽取、实体等价和产物准备
- `semantic/semantic_service/indexing/store.py`：Neo4j与隔离向量适配
- `semantic/semantic_service/indexing/publisher.py`：CAS发布与read lease
- `semantic/tests/test_generation_publish.py`：故障与并发发布
- `semantic/tests/conftest.py`：index_store真实PG/Neo4j与隔离向量fixture

**接口：** IndexBuilder.stage(op:Operation,request:ApplyRequest)->IndexManifest；Publisher.publish(scope:ScopeKey,base_generation:str|None,manifest:IndexManifest,lease_token:int)->bool；IndexStore.pin(scope)->ReadLease，release(lease_id)->None。IndexManifest含generation、base_generation、documents映射、配置digest、artifact hash列表、complete标志；ReadLease含lease_id/generation/expires_at。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_incomplete_generation_never_becomes_active(index_store, complete_manifest):
    before = index_store.active(complete_manifest.scope)
    incomplete = replace(complete_manifest, complete=False)
    with pytest.raises(ValueError):
        index_store.publish(incomplete.scope, before, incomplete, lease_token=1)
    assert index_store.active(incomplete.scope) == before
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_generation_publish.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 接收小批chunk或限定前缀manifest，检查size/hash/revision；用已验证抽取适配与A03模型入口构建来源断言，等价边可撤销，跨tenant/KB候选直接拒绝**

- [ ] **4. 实现完整KB清单；未变文档引用既有不可变产物，变更文档产物写staging且携带generation或不可变artifact ID。跨文档合并不修改旧generation数据**

- [ ] **5. 图、证据、向量全部持久校验后进入publishing；在PG同一事务校验operation租约/取消、最新revision与墓碑、base_generation，再更新active指针与operation终态**

- [ ] **6. 实现pin/release和有界租约续期；query按manifest闭包读取产物，GC跳过仍被active或有效read lease引用的产物**

关键实现约束：

```
with control_db.transaction() as tx:
    tx.assert_live_operation(operation_id, lease_token)
    tx.assert_not_deleted_or_superseded(manifest.documents)
    tx.assert_artifacts_verified(manifest.generation)
    changed = tx.cas_active(manifest.scope, base_generation, manifest.generation)
    if not changed:
        raise GenerationConflict()
    tx.finish_operation(operation_id, lease_token, "succeeded")
# tx方法在publisher.py实现；图/向量写入必须在此事务之前完成。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_generation_publish.py -q`，预期退出码 0；另完成：真实库模拟图成功/向量失败、两个base相同发布、租约过期、重启、read lease延迟GC；查询始终只见完整旧版或完整新版；记录实际后端。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 I03 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): i03 有来源的构图与generation原子发布`。

## I04：删除屏障、支持撤销和清理receipt

**依赖：** I03,A01。

**文件与职责：**

- `semantic/migrations/003_deletion_receipts.sql`：墓碑与分存储清理状态
- `semantic/semantic_service/indexing/deletion.py`：撤销支持与派生失效
- `semantic/semantic_service/indexing/gc.py`：租约/保留期/备份边界
- `semantic/tests/test_delete_races.py`：删除/晚到/多来源测试

**接口：** DeletionService.apply(document:DocumentRevision)->Operation、cleanup(operation_id:str)->DeletionReceipt；DeletionReceipt含operation_id/tombstone_revision/graph/vector/object/cache/backup状态及完成时间。backup为retention_pending或expired，不伪装立即擦除备份。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_last_support_removes_derived_fact(index_store, two_source_fact):
    index_store.delete_revision(two_source_fact.scope, "d1", revision=2)
    assert index_store.visible_assertion("shared-fact") is True
    index_store.delete_revision(two_source_fact.scope, "d2", revision=2)
    assert index_store.visible_assertion("shared-fact") is False
    assert index_store.visible_assertion("derived-fact") is False
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_delete_races.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. Delete接收单调tombstone并持久化，所有查询generation叠加最新deny；旧Apply与旧worker再次检查revision时被拒绝**

- [ ] **4. 撤销原文来源、别名、等价和推导支持，沿premise反向依赖递归失效；保留还有有效来源的事实，禁止删除整个同名实体**

- [ ] **5. 按图/向量/对象/缓存推进receipt；单存储失败保持pending独立重试。GC保留期与消息重放窗口强制配置，墓碑不得早于窗口清除**

- [ ] **6. 恢复模式默认不ready，先从Go重放当前deny/epoch再开放查询；备份保留期与逻辑不可见性分开报告**

关键实现约束：

```
visible_supports = [s for s in assertion.supports if not tombstones.denied(s)]
if not visible_supports:
    invalidate(assertion.assertion_id)
    for dependent_id in reverse_premise_index[assertion.assertion_id]:
        recompute_supports(dependent_id)
# invalidate/recompute_supports 在deletion.py实现，使用幂等工作队列防循环。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_delete_races.py -q`，预期退出码 0；另完成：测试删除时服务离线、旧generation查询、旧Apply迟到、多来源删除、清理半失败及备份恢复；deny优先于历史索引，即便物理清理未完成也不可检索。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 I04 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): i04 删除屏障、支持撤销和清理receipt`。

## I05：文档任务、attempt与终态协调

**依赖：** I04。

**文件与职责：**

- `internal/application/service/semantic_tasks.go`：提交/轮询/取消operation
- `internal/application/service/semantic_tasks_test.go`：终态去重与attempt隔离
- `internal/application/service/knowledge_post_process.go`：按后端派发语义任务
- `internal/application/service/knowledge_process.go`：文档更新/取消接线
- `internal/application/service/knowledge_delete_plan.go`：删除事件接线
- `internal/application/repository/semantic_outbox.go`：补齐操作映射与completion receipt
- `internal/container/container.go`：注册处理器

**接口：** 定义 SemanticTaskCoordinator.Submit(ctx,scope,documentID,attempt uint64)error、Reconcile(ctx,operationID string)error、Cancel(ctx,operationID string)error；receipt唯一键(scope,documentID,attempt,operationID)。semantic_status独立于parse_status，native分支继续原任务。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
func TestSemanticCompletionCannotDrainNewAttempt(t *testing.T) {
    f := newSemanticTaskFixture(t) // 本文件创建真实repo+fake RPC，便于控制晚到响应
    f.StartAttempt(2)
    f.DeliverTerminal("old-operation", 1, "succeeded")
    if f.PendingCount(2) != 1 { t.Fatal("old attempt drained current counter") }
    f.DeliverTerminal("new-operation", 2, "succeeded")
    f.DeliverTerminal("new-operation", 2, "succeeded")
    if f.PendingCount(2) != 0 { t.Fatal("completion was not idempotent") }
}
```

- [ ] **2. 确认 RED**。执行 `go test ./internal/application/service -run TestSemanticCompletion -count=1`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 使用I02事务API接入创建/更新/重解析/删除，复查实际调用链，禁止先更新业务再单独发消息；标准chunk落库后产生manifest**

- [ ] **4. 将业务任务与operation建立持久映射；Go只提交并协调状态，重试复用idempotency key；Python内部phase不创建重复Go子任务**

- [ ] **5. 在同事务写completion receipt并对匹配attempt的pending计数完成一次；取消/被替代operation不得扣新计数，operation成功后重复通知无副作用**

- [ ] **6. 区分解析完成、语义失败、stale和deleting；新增只重建语义的重试入口，保留原parse和普通索引结果**

关键实现约束：

```
INSERT INTO semantic_completion_receipts
(tenant_id,kb_id,document_id,attempt,operation_id)
VALUES (:tenant,:kb,:doc,:attempt,:operation)
ON CONFLICT DO NOTHING;
-- 仅本次确实插入receipt且current_attempt=:attempt时，原子减少本attempt待完成计数。
-- 不对新attempt做任何更新；RPC终态与业务状态在下一次reconcile可恢复。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `go test ./internal/application/service -run TestSemanticCompletion -count=1`，预期退出码 0；另完成：运行现有knowledge_post_process相关回归；验证丢响应、重复终态、取消、重解析和删除；真实RPC至少完成一次文档→generation闭环。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 I05 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): i05 文档任务、attempt与终态协调`。
