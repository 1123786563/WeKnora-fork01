### Task 2: O02: 旧空间与数据源迁移、影子计量和注销保留

> **协调者补充（deviation note）**：(1) F02 Customer 映射在 internal/application/repository/commercial/account.go（AccountStore.Bind 幂等、ErrAccountMappingConflict）；F04 基础档/订阅在 repository/commercial/subscription.go；U01 用量在 repository/commercial/usage.go；A07 绑定在 appconnector sync（BindingState/requires_reauthorization 词表）；O01 开关在 config（CommercialNewOrders 等 *bool nil 安全默认开）+ commercial/rollout.go。(2) internal/application/service/tenant.go DeleteTenant（167-201 行）当前直接 repo.DeleteTenant——本任务加性改造：删除前禁新调度/撤销连接/核对在途，且不级联删除待结算 commercial 记录；保留政策版本记录，未配置长期保留策略不开自动删除。(3) cmd/ 下已有 desktop/download/server 三个 CLI 先例可参照 main.go 结构；CLI 不引入新依赖。(4) 影子计量语义：只记录 usage 不投外部消费；切点按可信调用开始时间+配置版本固定，旧影子事件重放不得变成收费事件。

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

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回用例，不能为制造失败删除他人代码。

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

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial -run TestMigration -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
go build ./internal/commercial ./cmd/saas-migrate ./internal/application/service && go vet 同三包
go test ./internal/application/service -count=1
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/migration.go internal/commercial/migration_test.go cmd/saas-migrate/main.go internal/application/service/tenant.go docs/operations/saas-commercial-migration.md docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: migrate spaces without invented commercial state"
```
