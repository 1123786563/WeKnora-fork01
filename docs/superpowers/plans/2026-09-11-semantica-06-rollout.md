# Semantica 部署与验收 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 以真实环境、故障恢复与质量证据完成上线准备。

**Architecture:** 默认关闭正式流量；CI和人工验收共同控制启用，不以构建成功代替业务可用。

**Tech Stack:** Go/Gin/GORM、Python/gRPC/Semantica、PostgreSQL/Neo4j、React/TypeScript；按涉及范围使用。

**Spec:** [架构规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)；[总计划与完整类型表](2026-09-11-semantica-implementation.md)。

## Global Constraints

完整继承总计划 Global Constraints，必须先读；本计划不扩大语义服务所有权、授权范围或首版能力。所有代码和测试均为后续实施输入，未执行。

---


## O01：独立部署、探针与可观测性

**依赖：** C02,I03,A03。

**文件与职责：**

- `docker/Dockerfile.semantic`：锁定Python运行时与最小依赖
- `docker/compose.semantic.test.yml`：隔离集成测试环境
- `docker-compose.yml`：可选语义服务profile
- `docker-compose.dev.yml`：开发profile
- `helm/templates/semantic.yaml`：API/worker/配置/探针
- `helm/values.yaml`：显式enable和资源配置
- `semantic/semantic_service/telemetry.py`：脱敏日志与指标
- `semantic/tests/test_readiness.py`：恢复屏障与真实依赖健康
- `semantic/tests/test_telemetry.py`：日志脱敏

**接口：** config包含SEMANTIC_ENABLED、内部地址/认证引用、数据库/对象前缀、模型入口、限制、租约、GC保留与重放窗口；readiness必须依赖迁移完成、存储可用、删除屏障同步；metrics仅低基数标签，tenant/KB放受控日志trace不作指标标签。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_restoring_service_is_not_ready(health_state):
    health_state.stores_connected = True
    health_state.migrations_ready = True
    health_state.deletion_barriers_synced = False
    assert health_state.ready() is False
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest semantic/tests/test_readiness.py semantic/tests/test_telemetry.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 固定基础镜像digest和V01锁，镜像分API/worker入口；生产profile默认关闭，内部RPC不publish公网或宿主业务端口，secret只引用不写入镜像**

- [ ] **4. 提供隔离PG/Neo4j/向量/对象存储测试环境，固定测试端口或容器网络名；健康探针区分存活和可用，恢复期间拒绝query**

- [ ] **5. 记录operation/generation/trace及耗时、错误、截断、队列年龄、用量；默认不记录chunk、prompt、凭据；worker shutdown停止claim并释放或等待有效租约**

- [ ] **6. 为PG/SQLite业务控制路径和服务独立PG配置文档；SQLite部署要启用Semantica时需额外启动服务依赖，native-only不受影响**

关键实现约束：

```
def ready(self):
    return (self.migrations_ready and self.stores_connected
            and self.deletion_barriers_synced)
# /health存活不等于ready；容器启动不自动开启任何KB后端。

```

- [ ] **7. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest semantic/tests/test_readiness.py semantic/tests/test_telemetry.py -q`，预期退出码 0；另完成：docker compose -f docker/compose.semantic.test.yml config校验；`helm template semantic ./helm`渲染；真实启动关闭profile与启用profile各验证一次；日志扫描无测试凭据/正文。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 O01 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): o01 独立部署、探针与可观测性`。

## O02：故障注入、清理与恢复演练

**依赖：** O01,I04,W03,Q04。

**文件与职责：**

- `semantic/tests/integration/conftest.py`：RecoveryHarness与recovery fixture
- `semantic/tests/integration/test_recovery.py`：真实进程/存储故障
- `semantic/tests/integration/test_authorization_races.py`：查询撤权竞态
- `scripts/semantic/run_recovery_tests.sh`：统一隔离运行入口
- `docs/superpowers/plans/semantica/recovery-evidence.md`：故障与恢复实测
- `docs/superpowers/plans/semantica/recovery-runbook.md`：实际恢复步骤

**接口：** 新增RecoveryHarness于semantic/tests/integration/conftest.py：start/stop_worker、pause_phase、resume_phase、restart_api、restore_snapshot、query、revoke、wait_operation；只用于隔离测试，故障点不可通过生产用户API触发。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_restore_replays_denials_before_ready(recovery):
    snapshot = recovery.snapshot()
    recovery.delete_document("d1", revision=2)
    recovery.restore_snapshot(snapshot)
    assert recovery.ready() is False
    recovery.replay_current_denials()
    assert recovery.ready() is True
    assert "d1" not in recovery.query("甲公司").document_ids
```

- [ ] **2. 确认 RED**。执行 `bash scripts/semantic/run_recovery_tests.sh`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 实现隔离RecoveryHarness并注册pytest integration标记；依赖不可用时退出非零，不skip。通过容器进程控制和测试专用存储屏障在准确阶段暂停**

- [ ] **4. 执行规格12个场景，重点覆盖图写/向量未写、旧token、cancel/publish、删除迟到、缓存撤权、后台重试、回滚及恢复；每例记录前后active manifest和deny状态**

- [ ] **5. 恢复快照先维护模式，连接Go权威重放当前deny/epoch，验证删除内容不可见后开放readiness；GC和backup保留状态分开记录**

- [ ] **6. 将演练中的确切命令整理手册，包含失败退出与安全重试、对象/向量孤儿清理、禁用语义能力和native追赶条件**

关键实现约束：

```
restore_persistent_artifacts(snapshot)
set_query_ready(False)
replay_denials_from_business_authority()
verify_no_denied_sources_visible()
set_query_ready(True)
# 四个动作由RecoveryHarness调用真实服务/存储，不用内存状态替代恢复。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `bash scripts/semantic/run_recovery_tests.sh`，预期退出码 0；另完成：12场景各有真实输出、退出码、容器版本和失败注入点；未复现的场景保持未验收；总结果不得仅依赖mock测试。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 O02 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): o02 故障注入、清理与恢复演练`。

## O03：质量回归、CI门禁与最终交付

**依赖：** O02,W02,V03。

**文件与职责：**

- `.github/workflows/semantic.yml`：聚焦单测/合约/集成门禁
- `semantic/experiments/evaluate.py`：用生产适配重放冻结语料
- `scripts/semantic/check_acceptance.py`：policy和证据完整性门禁
- `scripts/semantic/test_check_acceptance.py`：门禁失败行为
- `docs/superpowers/plans/semantica/final-evidence.md`：四层证据汇总
- `docs/superpowers/plans/semantica/progress.md`：最终逐任务状态

**接口：** check_acceptance(policy:dict,evidence:dict)->list[str]返回所有阻断原因；release可用条件为policy.approved、无权限泄漏、证据完整、指标满足阈值、所启用能力verified。CI无模型凭据只能运行受控provider合同，真实模型验收独立受保护任务执行。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
def test_unapproved_policy_blocks_release():
    errors = check_acceptance({"approved": False}, {"security_leaks": 0})
    assert "policy_not_approved" in errors

def test_any_permission_leak_blocks_release():
    errors = check_acceptance({"approved": True}, {"security_leaks": 1})
    assert "permission_leak" in errors
```

- [ ] **2. 确认 RED**。执行 `uv run --project semantic python -m pytest scripts/semantic/test_check_acceptance.py -q`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 建立CI阶段：proto生成一致性→Go/Python/TS聚焦单测→真实PG/SQLite控制合同与Neo4j集成→浏览器流程；首次无关失败记录基线，不能删断言使绿**

- [ ] **4. 使用正式adapter重放V03语料，对照冻结基线；报告冷/热延迟、索引耗时、引用准确性、无答案判断、真实模型用量和成本估算依据**

- [ ] **5. 把能力、测试、恢复和用户阈值确认关联到同一commit/version/lock hash；未确认阈值或无真实模型证据时明确blocked，不启用相应正式模式**

- [ ] **6. 最后进行整条分支review、修复发现并重跑受影响检查；填写24任务证据与已知限制，交付部署/回滚手册，正式部署或KB切换另按用户授权执行**

关键实现约束：

```
def check_acceptance(policy, evidence):
    errors = []
    if policy.get("approved") is not True:
        errors.append("policy_not_approved")
    if "security_leaks" not in evidence:
        errors.append("missing_security_measurement")
    elif evidence["security_leaks"] != 0:
        errors.append("permission_leak")
    for required in ("contract", "integration", "recovery", "browser", "live_model"):
        if evidence.get(required) != "verified":
            errors.append("missing_" + required)
    return errors
# 同文件继续逐项比较policy中的明确阈值，并校验commit/version/hash一致。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `uv run --project semantic python -m pytest scripts/semantic/test_check_acceptance.py -q`，预期退出码 0；另完成：门禁对缺证据、假approved、泄漏、质量/延迟/用量超限全部失败；最终报告分别写静态/单测、集成、浏览器、真实模型四层状态。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 O03 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): o03 质量回归、CI门禁与最终交付`。
