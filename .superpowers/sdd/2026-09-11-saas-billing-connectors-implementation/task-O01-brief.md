### Task 1: O01: 运营恢复队列、审计与收费开关

> **协调者补充（deviation note）**：(1) 服务层模式见 internal/application/service/commercial/（budget.go/fulfillment.go/refund.go/settlement.go——结构体+dig Provide+ctx 首参；U04 BudgetService.Extend 先例）；C01 Outbox 在 internal/application/repository/commercial/outbox.go（OutboxEvent/OutboxStore）。(2) internal/config/config.go（Config struct，约 1131 行）加性新增三个开关字段（commercial_new_orders、commercial_new_dispatch、connector_new_actions，按空间生效——开关为全局默认值，逐空间覆盖由 repository 存储层承载；在 config 里提供默认开启语义与解析）；container.go 加性 Provide。(3) 无迁移编号需求；docs/operations/ 为新目录。开关默认值必须安全（开启），关闭仅为回滚操作。

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

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(Rollback|Recovery)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
go build ./internal/commercial ./internal/application/service/commercial ./internal/config ./internal/container && go vet 同四包
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/rollout.go internal/commercial/rollout_test.go internal/application/service/commercial/recovery.go internal/config/config.go internal/container/container.go docs/operations/saas-commercial-recovery.md docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: recover commercial work across rollout pauses"
```
