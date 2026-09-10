# SaaS 06 迁移恢复与整体验收 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 以可回退的迁移和真实分层证据交付商业与Connector能力。

**Architecture:** 新动作开关与历史恢复通道分离；迁移不凭空发权益或共享凭据；发布门槛按实际证据判断。

**Tech Stack:** 当前仓库 Go 1.26、Gin、GORM、PostgreSQL／SQLite 版本化迁移；React 19、TypeScript 6、pnpm 10.28.2；官方 OpenMeter 固定版本；优先复用现有依赖。

**Spec:** [完整规格](../specs/2026-09-10-saas-billing-connectors-design.md)；[接口验证清单](../specs/2026-09-10-saas-billing-connectors-interface-verification.md)。执行者必须同时阅读本计划引用的规格章节与上下游契约。

## Global Constraints

- 每个空间始终独立购买、付款并承担费用；共享组织不改变归属，不共享余额或套餐。
- 费用预算批准和外部写入批准是独立条件，两者都满足才能执行。
- 首期不包含：跨空间合并付款或余额池、自动扣款、独立席位与存储加购包、嵌入式第三方 Web 应用、公开开发者上架与交易分成。
- 固定候选基线：`v1.0.0-beta.232`，提交 `887e0cac903ccd06e74d61ed23c651651d10c7a9`。
- 未在验证清单取得真实通过证据的外部能力，不得标记为生产可用；不使用旧本地 OpenMeter fork 接口。
- 保留已有 React、Craft、移动端和其他任务的工作；仅提交本任务明确拥有的文件，不使用 `git add .`。
- 下列内部接口是计划新增契约，不声称当前已存在；外部请求字段以 P00 的已验证契约为准。
- 时区、精度与分段报价采用完整规格的推荐默认值；外部模型无法无损表达时在 P00 停止依赖任务并修正规格，不能自行换商业语义。

---

## 依赖与执行边界

O01依赖C04/C05/U03/A03；O02依赖A07与影子计量；O03在各门槛完成后按能力执行，不强制启动无授权真实外部操作。

本文件仅编写计划，不表示已实现或已测试。执行时先使用 using-git-worktrees 建立隔离工作区；所有命令从仓库根执行。每一步只做所列一个动作；较长代码修改按列出的文件／函数逐项执行，完成该任务的 RED→GREEN 与独立 review 后再进入下一任务。

## 文件结构与职责

operations文档负责逐状态runbook；cmd/saas-migrate只迁移指定空间；release_gate检查真实证据完整性。

### Task O01: 运营恢复队列、审计与收费开关

**Files:**
- Create: `internal/commercial/rollout.go`、`internal/commercial/rollout_test.go`
- Create: `internal/application/service/commercial/recovery.go`
- Modify: `internal/config/config.go`、`internal/container/container.go`
- Create: `docs/operations/saas-commercial-recovery.md`

**Interfaces:**
- Consumes: C01Outbox、C04履约、C05退款、U03结算、A03unknown Action。
- Produces: `AllowDuringRollback(operation string) bool`；`RecoveryService.Replay(ctx context.Context,operationID,actor string) error`；配置开关 commercial_new_orders、commercial_new_dispatch、connector_new_actions 按空间生效。

**行为与边界：** 测试开关关闭后新消费拒绝但回调/历史结算仍处理；告警定位业务ID；已撤销连接不能因重放重启用。

**验收映射：** OPS-01/03；AC-20

- [ ] **Step 1: 在 `internal/commercial/rollout_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestRollbackKeepsPaidRecoveryAndSettlement(t *testing.T) {
    for _,s:=range []string{"payment_callback","refund_query","fulfillment_replay","settlement"} {
        if !AllowDuringRollback(s) { t.Fatal(s) }
    }
    if AllowDuringRollback("new_order") { t.Fatal("new money accepted while paused") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(Rollback|Recovery)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func AllowDuringRollback(operation string) bool {
 switch operation {
 case "payment_callback","refund_query","fulfillment_replay","settlement": return true
 default: return false
 }
}
```

恢复队列按paid未履约、refund_unknown、revocation_pending、usage未确认、余额差异、unknown Action分类；展示年龄、空间和业务ID，重放只使用原operation与幂等键。Replay检查平台运营能力、对象版本及当前状态，记录执行人和结果；不能直接改余额。

指标逐类计数与年龄分位，配置明确告警阈值；logs屏蔽密钥和完整私密payload。手册逐状态写查询路径、正常/异常结果和允许动作：unknown写操作只有核对，不能直接重派。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(Rollback|Recovery)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/rollout.go internal/commercial/rollout_test.go internal/application/service/commercial/recovery.go internal/config/config.go internal/container/container.go docs/operations/saas-commercial-recovery.md docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: recover commercial work across rollout pauses"
```

### Task O02: 旧空间与数据源迁移、影子计量和注销保留

**Files:**
- Create: `internal/commercial/migration.go`、`internal/commercial/migration_test.go`
- Create: `cmd/saas-migrate/main.go`
- Modify: `internal/application/service/tenant.go`
- Create: `docs/operations/saas-commercial-migration.md`

**Interfaces:**
- Consumes: F02Customer映射、F04基础档、U01用量、A01/A07连接迁移、O01开关。
- Produces: `MigrationDecision{Plan string; IssueCredits bool; ConnectionState string}`；`InitialCommercialState(provenConnectionOwner bool) MigrationDecision`；CLI `--dry-run`默认，只对显式tenant清单执行。

**行为与边界：** dry-run无写入、重复apply不重复Customer、未知凭据不共享、影子切换不双扣、注销在途记录保留；迁移不覆盖其他并行任务新增表。

**验收映射：** B05/B24；OPS-02/03；SYNC-05；AC-20

- [ ] **Step 1: 在 `internal/commercial/migration_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestMigrationDoesNotInventPaidCreditOrConnectionOwnership(t *testing.T) {
    got:=InitialCommercialState(false)
    if got.Plan!="basic" || got.IssueCredits || got.ConnectionState!="requires_reauthorization" { t.Fatal(got) }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial -run TestMigration -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
type MigrationDecision struct { Plan string; IssueCredits bool; ConnectionState string }
func InitialCommercialState(provenConnectionOwner bool) MigrationDecision {
 state:="requires_reauthorization"
 if provenConnectionOwner { state="ready_to_bind" }
 return MigrationDecision{Plan:"basic",IssueCredits:false,ConnectionState:state}
}
```

CLI执行流程：读取显式tenant清单→输出映射和连接归属证明→dry-run报告→显式`--apply`逐tenant幂等事务应用。旧余额不折成真钱Grant，已有数据源保留直至新绑定验证。影子模式只记录usage，不投外部消费；启用切点按可信调用开始时间和配置版本固定，旧影子事件不能在重放时变成收费事件。

注销先禁新调度、撤销连接并等待已发调用/付款/退款核对；记录保留政策版本，未配置长期保留策略不开自动删除。不能在删除tenant资源时级联删除待结算commercial记录。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial -run TestMigration -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/migration.go internal/commercial/migration_test.go cmd/saas-migrate/main.go internal/application/service/tenant.go docs/operations/saas-commercial-migration.md docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: migrate spaces without invented commercial state"
```

### Task O03: 按证据收口全部接口与产品联验

**Files:**
- Create: `scripts/saas/release_gate.py`、`scripts/saas/release_gate_test.py`
- Create: `docs/superpowers/plans/saas-billing-connectors-acceptance.md`
- Modify: `docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md`

**Interfaces:**
- Consumes: 所有任务产出的真实记录；规格AC-01～AC-20；66个接口检查ID。
- Produces: `require_pass(records:dict, required:set[str]) -> None`；验收报告明确unit/integration/provider/browser层级，不把skip当pass。

**行为与边界：** 先跑fake report证明gate会拒绝缺项，再完成真实联验；如果商户/外部目标未授权，保留对应blocked-env并禁止宣称全链路成功。整分支复核仅涉及本计划拥有文件，必要时用code-review/verification技能。

**验收映射：** AC-01 至 AC-20；G0 至 G5；全部66接口检查

- [ ] **Step 1: 在 `scripts/saas/release_gate_test.py` 写入以下失败断言。**

```python
import unittest
from scripts.saas.release_gate import require_pass
class ReleaseTests(unittest.TestCase):
    def test_mock_cannot_prove_provider_acceptance(self):
        with self.assertRaises(ValueError):
            require_pass({'WX-05':{'status':'pass','level':'mock','evidence':['x']}},{'WX-05'})
    def test_missing_case_is_failure(self):
        with self.assertRaises(ValueError): require_pass({}, {'AC-01'})
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
python3 -m unittest scripts.saas.release_gate_test -v
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```python
def require_pass(records: dict, required: set[str]) -> None:
    provider_cases = {"WX-05", "ALI-05", "FS-04", "NO-04"}
    for key in required:
        row = records.get(key, {})
        if row.get("status") != "pass" or not row.get("evidence"):
            raise ValueError(f"missing passing evidence: {key}")
        if key in provider_cases and row.get("level") != "provider":
            raise ValueError(f"real provider evidence required: {key}")
```

验收文件逐项引用AC-01～20、66接口ID及真实证据路径。按完整规格的输入和期望逐个执行跨空间、双渠道、月度、升级、退款、并发、父子Runtime、同步、两种写操作与回退场景。浏览器需要真实服务，保存对应页面状态/用户动作及后端关联，不只截图静态HTML。

CLI读取JSON记录 `--report artifacts/saas-acceptance/report.json`，从规格和检查清单解析全部必需ID；缺项、blocked-env、skip和mock替代真实提供方均退出1。功能门槛可逐能力关闭，但最终报告必须列出未完成范围。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
python3 -m unittest scripts.saas.release_gate_test -v
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add scripts/saas/release_gate.py scripts/saas/release_gate_test.py docs/superpowers/plans/saas-billing-connectors-acceptance.md docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "test: require complete commercial and connector acceptance evidence"
```

