# SaaS 03 预算与计量结算 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在所有收费调用前执行有限预算并无重复地结算真实用量。

**Architecture:** 平台协调器只保护在途额度，OpenMeter保持最终商业权威；实际调用去重与价格换算独立于Runtime聚合展示。

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

F01～F05、V03确认策略；U03依赖C04适配，C05依赖U02/U03，构成单向细粒度依赖而不是整计划互相等待。

本文件仅编写计划，不表示已实现或已测试。执行时先使用 using-git-worktrees 建立隔离工作区；所有命令从仓库根执行。每一步只做所列一个动作；较长代码修改按列出的文件／函数逐项执行，完成该任务的 RED→GREEN 与独立 review 后再进入下一任务。

## 文件结构与职责

commercial：用量与预算规则；repository：CAS原子占用和用量事实；service：确认水位与恢复；agent/model入口：出站前准入。

### Task 1: U01: 实际调用身份、资金来源与用量去重

**Files:**
- Create: `internal/commercial/usage.go`、`internal/commercial/usage_test.go`
- Create: `internal/application/repository/commercial/usage.go`
- Create: `migrations/versioned/000115_commercial_usage.up.sql`、`migrations/versioned/000115_commercial_usage.down.sql`
- Create: `migrations/sqlite/000035_commercial_usage.up.sql`、`migrations/sqlite/000035_commercial_usage.down.sql`

**Interfaces:**
- Consumes: F01 Credits；F03 价格版本；Craft 主子用量关联契约；现有 internal/models/chat/usage.go。
- Produces: `UsageFact{TenantID uint64; RunID,DelegationID,CallID,AttemptID,Funding,Service,PriceVersion string; Revision int64; OccurredAt time.Time; Dimensions map[string]int64; Status string}`；`BillableModel(funding string) bool`；`UsageStore.Record(context.Context,UsageFact) error`。

**行为与边界：** 测试重复最终事件、累积 partial、主汇总含子调用、真实重试新 attempt、同 revision 变内容冲突、迟到修正；服务端伪造 funding 不可接受。

**验收映射：** B07/B23；USE-01/02/03/04/05；AC-12/13

- [ ] **Step 1: 在 `internal/commercial/usage_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestBYOKDoesNotChargeModelTokens(t *testing.T) {
    if BillableModel("byok") { t.Fatal("BYOK charged") }
    if !BillableModel("platform") { t.Fatal("platform model omitted") }
    if BillableModel("untrusted") { t.Fatal("unknown funding treated as paid") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/repository/commercial -run "Test(BYOK|Usage)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func BillableModel(funding string) bool { return funding=="platform" }
```

模型 funding 从服务端凭据绑定产生，unknown 在派发前拒绝，不以此函数直接免单。usage 表唯一 `(tenant_id,call_id,attempt_id,revision)`，另有当前版本 `(tenant_id,call_id,attempt_id)`；拒绝同一 revision 不同内容。parent aggregate 标记 display_only，不生成 settlement；流式 partial 保存观测，final 贡献一次差值。UsageStore.Record 原子写事实与 settlement Outbox，status=unknown 不按 0 发结算。

价格表以不可变版本和维度单位存储定点比率，乘除用 big.Int/big.Rat，单次物理调用末尾舍入。BYOK 仅免模型维度，沙箱／解析分别为独立服务事实。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/repository/commercial -run "Test(BYOK|Usage)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/usage.go internal/commercial/usage_test.go internal/application/repository/commercial/usage.go migrations/versioned/000115_commercial_usage.up.sql migrations/versioned/000115_commercial_usage.down.sql migrations/sqlite/000035_commercial_usage.up.sql migrations/sqlite/000035_commercial_usage.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: persist physical call usage without parent double count"
```

### Task 2: U02: 跨进程任务预算与额度原子预占

**Files:**
- Create: `internal/commercial/budget.go`、`internal/commercial/budget_test.go`
- Create: `internal/application/repository/commercial/budget.go`、`internal/application/repository/commercial/budget_test.go`、`internal/application/repository/commercial/budget_pg_test.go`
- Create: `migrations/versioned/000116_commercial_budgets.up.sql`、`migrations/versioned/000116_commercial_budgets.down.sql`
- Create: `migrations/sqlite/000036_commercial_budgets.up.sql`、`migrations/sqlite/000036_commercial_budgets.down.sql`

**Interfaces:**
- Consumes: F01 Credits；V03 settlement_ack_strategy；U01 UsageFact。
- Produces: `BudgetRequest{TenantID uint64; RunID,Key string; Upper Credits; Deadline time.Time}`；`Reservation{ID,RunID,State string; Upper Credits; Deadline time.Time; Version int64}`；`NewBudgetStore(db *gorm.DB) *BudgetStore`；`BudgetStore.Reserve(ctx context.Context,req BudgetRequest)(Reservation,error)`；`Available(verified,unreflected,held,refundLocked Credits)(Credits,error)`。

**行为与边界：** 20个并发请求竞争有限余额，断言成功总额、DB占用和真实派发数一致；父子不复制预算；不能将独立读外部余额的结果随意刷新覆盖本地 unreflected。

**验收映射：** B22；BUD-01/02/03；AC-11

- [ ] **Step 1: 在 `internal/commercial/budget_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestAvailableIncludesUnacknowledgedSpend(t *testing.T) {
    got,err:=Available(100,20,30,10)
    if err!=nil || got!=40 { t.Fatalf("%d %v",got,err) }
    if _,err:=Available(100,-1,0,0); err==nil { t.Fatal("negative protection") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/repository/commercial -run "Test(Available|Budget)" -race -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func Available(verified,unreflected,held,refundLocked Credits)(Credits,error) {
 if verified<0 || unreflected<0 || held<0 || refundLocked<0 { return 0,errors.New("invalid_projection") }
 remaining:=verified
 for _,n:=range []Credits{unreflected,held,refundLocked} {
   if n>remaining { return 0,nil }; remaining-=n
 }
 return remaining,nil
}
```

SQL 表 commercial_budget_accounts：tenant PK、verified_micro、unreflected_micro、held_micro、refund_locked_micro、watermark、version；commercial_task_budgets：tenant/run 唯一、limit_micro、spent_micro、held_micro、deadline、version；reservation：tenant/key 唯一、run、upper_micro、state、deadline、fence。

Reserve 一个事务内以版本 CAS 更新空间与任务计数，再插入 reservation；任一步受影响行数不是 1 则整体回滚重读。CAS 条件包括 `available>=upper`、`task_limit-task_spent-task_held>=upper`、有效期限、投影水位可接受。SQLite 使用同样版本 CAS，PG 也需真实并发测试；不是单进程 mutex。按先到期批次分配，保存 reservation→lot 分配；所有退款锁也在同一个协调事务核算。

为本任务固定补充列：account 的 `verified_until TIMESTAMP NOT NULL`；lot 表 `commercial_budget_lots(tenant_id BIGINT, lot_id TEXT, remaining_micro BIGINT, held_micro BIGINT, expires_at TIMESTAMP, issued_at TIMESTAMP, PRIMARY KEY(tenant_id,lot_id))`。账户、任务和 lot 的保护量在同一事务更新。迁移 up/down 包含该表。

- [ ] **Step 3.1: 创建实际 PostgreSQL 并发回归，不能仅用纯 Available 函数验收预占。**

`internal/application/repository/commercial/budget_pg_test.go`：

```go
//go:build commercial_integration

package commercial

import (
 "context"
 "fmt"
 "net/url"
 "os"
 "sync"
 "sync/atomic"
 "testing"
 "time"
 domain "github.com/Tencent/WeKnora/internal/commercial"
 "gorm.io/driver/postgres"
 "gorm.io/gorm"
)

func TestBudgetPGConcurrentReservation(t *testing.T) {
 raw:=os.Getenv("SAAS_TEST_PG_DSN")
 if raw=="" { t.Fatal("SAAS_TEST_PG_DSN is required for isolated integration evidence") }
 parsed,err:=url.Parse(raw)
 if err!=nil || (parsed.Scheme!="postgres" && parsed.Scheme!="postgresql") { t.Fatal("test DSN must be a PostgreSQL URL") }
 admin,err:=gorm.Open(postgres.Open(raw),&gorm.Config{}); if err!=nil { t.Fatal("test database unavailable") }
 adminSQL,err:=admin.DB(); if err!=nil { t.Fatal(err) }; defer adminSQL.Close()
 schema:=fmt.Sprintf("saas_budget_%d",time.Now().UnixNano())
 if err:=admin.Exec("CREATE SCHEMA "+schema).Error; err!=nil { t.Fatal(err) }
 defer admin.Exec("DROP SCHEMA "+schema+" CASCADE")
 query:=parsed.Query(); query.Set("search_path",schema); parsed.RawQuery=query.Encode()
 db,err:=gorm.Open(postgres.Open(parsed.String()),&gorm.Config{}); if err!=nil { t.Fatal(err) }
 pool,err:=db.DB(); if err!=nil { t.Fatal(err) }; defer pool.Close(); pool.SetMaxOpenConns(20)
 migration,err:=os.ReadFile("../../../../migrations/versioned/000116_commercial_budgets.up.sql")
 if err!=nil { t.Fatal(err) }
 if err:=db.Exec(string(migration)).Error; err!=nil { t.Fatal(err) }
 end:=time.Now().UTC().Add(time.Hour)
 statements:=[]struct{sql string;args []any}{
  {"INSERT INTO commercial_budget_accounts (tenant_id,verified_micro,unreflected_micro,held_micro,refund_locked_micro,watermark,version,verified_until) VALUES (7,100,0,0,0,'w1',1,?)",[]any{end}},
  {"INSERT INTO commercial_task_budgets (tenant_id,run_id,limit_micro,spent_micro,held_micro,deadline,version) VALUES (7,'r1',100,0,0,?,1)",[]any{end}},
  {"INSERT INTO commercial_budget_lots (tenant_id,lot_id,remaining_micro,held_micro,expires_at,issued_at) VALUES (7,'lot1',100,0,?,?)",[]any{end,time.Now().UTC()}},
 }
 for _,stmt:=range statements { if err:=db.Exec(stmt.sql,stmt.args...).Error; err!=nil { t.Fatal(err) } }
 store:=NewBudgetStore(db)
 var accepted atomic.Int64
 var wg sync.WaitGroup
 for i:=0;i<20;i++ {
  wg.Add(1)
  go func(i int) {
   defer wg.Done()
   _,err:=store.Reserve(context.Background(),domain.BudgetRequest{TenantID:7,RunID:"r1",Key:fmt.Sprintf("call-%d",i),Upper:60,Deadline:time.Now().Add(time.Minute)})
   if err==nil { accepted.Add(1) }
  }(i)
 }
 wg.Wait()
 if got:=accepted.Load(); got!=1 { t.Fatalf("accepted=%d; want exactly one reservation",got) }
 var held int64
 if err:=db.Raw("SELECT held_micro FROM commercial_budget_accounts WHERE tenant_id=7").Scan(&held).Error; err!=nil { t.Fatal(err) }
 if held!=60 { t.Fatalf("held=%d; want 60",held) }
}
```

Run: `go test -tags=commercial_integration ./internal/application/repository/commercial -run TestBudgetPGConcurrentReservation -race -count=1`。测试只使用显式测试 DSN 并在新建独立schema中工作；没有该环境是blocked-env，不算通过。必须再加入两独立任务、退款锁与预占竞争的同结构场景，各自断言最终DB保护量。


- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/repository/commercial -run "Test(Available|Budget)" -race -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/budget.go internal/commercial/budget_test.go internal/application/repository/commercial/budget.go internal/application/repository/commercial/budget_test.go internal/application/repository/commercial/budget_pg_test.go migrations/versioned/000116_commercial_budgets.up.sql migrations/versioned/000116_commercial_budgets.down.sql migrations/sqlite/000036_commercial_budgets.up.sql migrations/sqlite/000036_commercial_budgets.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: reserve task budgets atomically across workers"
```

### Task 3: U03: 最终消费与外部确认水位交接

**Files:**
- Create: `internal/commercial/settlement.go`、`internal/commercial/settlement_test.go`
- Create: `internal/application/service/commercial/settlement.go`
- Modify: `internal/application/repository/commercial/budget.go`、`internal/infrastructure/openmeter/commercial.go`

**Interfaces:**
- Consumes: U01 UsageStore；U02 Reservation；C04 CommercialGateway。
- Produces: `Settlement{ID,CallID,ReservationID string; TenantID uint64; Amount Credits; Revision int64; OccurredAt time.Time}`；`SettlementReceipt{ExternalID,Watermark string}`；`CommercialGateway.Settle(context.Context,Settlement)(SettlementReceipt,error)`、`ConfirmSettlement(context.Context,string)(SettlementReceipt,error)`；`KeepProtection(state string) bool`。

**行为与边界：** 模拟远端聚合延迟、HTTP接收后崩溃、确认丢失、外部水位倒退、退款同时发生；断言没有短暂重复准入和永久双扣；gate不能证明确认策略时本任务状态 blocked。

**验收映射：** BUD-05/07；USE-03/04；AC-12/14

- [ ] **Step 1: 在 `internal/commercial/settlement_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestSettlementRetainsProtectionUntilAcknowledged(t *testing.T) {
    for _,s:=range []string{"dispatched","unknown","sent","accepted"} {
        if !KeepProtection(s) { t.Fatalf("released %s",s) }
    }
    if KeepProtection("confirmed") { t.Fatal("confirmed spend still held") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "TestSettlement" -race -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func KeepProtection(state string) bool { return state!="confirmed" }
```

外部 ingest 接收不等于消费已反映，只有 V03 选定的明确确认策略产生 confirmed。Finalize 的本地事务保存 UsageFact、最终金额、settlement Outbox，同时把 reservation 的消费部分转 unreflected，释放实际未用部分；在远端确认前 unreflected 保留。ConfirmSettlement 成功后在同一事务推进 watermark 并移除已被该水位覆盖的保护量。

禁止“看到余额下降就认为自己的事件已计入”，必须有事件／交易关联证据。外部修正、负数冲正或补偿从所选模型的实际能力选择且复用 settlement revision 键；结果不明保留并核对，不直接删除历史用量。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "TestSettlement" -race -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/settlement.go internal/commercial/settlement_test.go internal/application/service/commercial/settlement.go internal/application/repository/commercial/budget.go internal/infrastructure/openmeter/commercial.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: hand off reserved usage only after confirmed settlement"
```

### Task 4: U04: 取消、到期、追加与恢复租约

**Files:**
- Create: `internal/commercial/reservation_state.go`、`internal/commercial/reservation_state_test.go`
- Modify: `internal/application/repository/commercial/budget.go`
- Create: `internal/application/service/commercial/budget.go`

**Interfaces:**
- Consumes: U02 Reserve/Reservation；U03结算；F02权限与F04周期。
- Produces: `MayReleaseWithoutQuery(state string) bool`；`BudgetService.Extend(ctx context.Context,tenant uint64,runID,key string,extra commercial.Credits) error`；`BudgetService.Reconcile(ctx context.Context,reservationID string) error`。

**行为与边界：** 覆盖到期与退款竞争、空间降级后的新增步骤、任务取消中远端已成功、重复追加、旧leaseworker恢复；未获授权预算必须暂停并返回明确原因。

**验收映射：** B06/B22/B23；BUD-03/04/06；USE-06；AC-14

- [ ] **Step 1: 在 `internal/commercial/reservation_state_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestExpiredLeaseDoesNotReleaseDispatchedCost(t *testing.T) {
    if MayReleaseWithoutQuery("dispatched") { t.Fatal("unknown spend released") }
    if MayReleaseWithoutQuery("settling") { t.Fatal("pending settlement released") }
    if !MayReleaseWithoutQuery("held") { t.Fatal("unstarted reservation stranded") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(ExpiredLease|BudgetRecovery|BudgetExtend)" -race -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func MayReleaseWithoutQuery(state string) bool { return state=="held" }
```

released 前 CAS 同时验证 held 和版本，防止并发派发后仍释放。Extend 同事务验证当前预算授权、套餐、资金和幂等 key，再增加 task limit；不自动把空间余额当授权。租约只切换核对所有者，fence 递增，旧工作者不得提交结果。

截止时间取调用可控制上界与批次有效期；连续资源分段预占。额度过期不延长有效期，迟报按实际发生区间核对；未派发可以释放，已发不能以取消/到期直接记0。查询确认没有产生外部使用后才释放保护量。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(ExpiredLease|BudgetRecovery|BudgetExtend)" -race -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/reservation_state.go internal/commercial/reservation_state_test.go internal/application/repository/commercial/budget.go internal/application/service/commercial/budget.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: reconcile cancelled and expired reservations safely"
```

### Task 5: U05: 主子 Runtime、模型与平台服务的收费准入接线

**Files:**
- Create: `internal/commercial/execution.go`、`internal/commercial/execution_test.go`
- Modify: `internal/agent/engine.go`、`internal/models/chat/usage.go`
- Create: `internal/agent/commercial_adapter.go`
- Create: `internal/application/service/commercial/execution.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- Consumes: U01～U04；当前 AgentEngine.Execute；Craft规格的run/delegation/call关联。
- Produces: `ExecutionGate.Begin(ctx context.Context,req BudgetRequest)(Reservation,error)`、`Finish(ctx context.Context,reservationID string,fact UsageFact) error`；`ValidateFunding(funding string) error`；`ExecutionGate` 定义在 internal/commercial/execution.go。

**行为与边界：** 测试主任务调用子任务后父汇总不重复、BYOK失败不静默切平台、模型输出/沙箱时长上界实际生效、异常成本停后续派发；依赖Craft工作未交付时只有当前入口可声明通过。

**验收映射：** B07/B22/B23；BUD-08；USE-01/02/06；AC-11/13/14

- [ ] **Step 1: 在 `internal/commercial/execution_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestUnknownFundingCannotBypassChargeGate(t *testing.T) {
    if ValidateFunding("from_model_argument")==nil { t.Fatal("untrusted funding accepted") }
    if ValidateFunding("platform")!=nil || ValidateFunding("byok")!=nil { t.Fatal("known funding rejected") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/agent ./internal/models/chat -run "Test(UnknownFunding|Commercial|Usage)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func ValidateFunding(funding string) error {
 if funding!="platform" && funding!="byok" { return errors.New("funding_source_required") }
 return nil
}
```

Begin 在真实出站前调用 U02，持久化 dispatched intent；Finish 只接受可信 provider usage。把原始事件适配到 UsageFact，不替换已有展示用途 TokenUsage。BYOK模型可记录0平台模型费用，但所有收费解析、嵌入、rerank、沙箱和Connector各自走对应服务维度的gate。

对 Craft/OpenCode 按实际落地后的适配入口注入同一 ExecutionGate。当前新 Runtime 尚未落地时不创建同名假模块：在 `commercial_adapter.go` 暴露接口并做当前 AgentEngine 接线，记录 runtime_binding 检查未完成；真正开放子 Runtime 收费必须在其出站模型/工具/沙箱时长控制处有 Begin/Finish 证据。任何绕过网络路径使 BUD-08失败。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/agent ./internal/models/chat -run "Test(UnknownFunding|Commercial|Usage)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/execution.go internal/commercial/execution_test.go internal/agent/engine.go internal/models/chat/usage.go internal/agent/commercial_adapter.go internal/application/service/commercial/execution.go internal/container/container.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: enforce execution budgets at billable call boundaries"
```

