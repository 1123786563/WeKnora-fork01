# SaaS 01 空间商业基础 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建空间隔离的商业权限、目录、报价与周期权益投影。

**Architecture:** 纯计算放 internal/commercial；持久化与编排沿用 application/repository、application/service；Gin 只负责认证及契约边界。

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

V03 冻结契约后执行外部相关任务；F01/F02 的纯逻辑可先做，但不能发布收费能力。迁移编号当前预留 PG 000110～000119、SQLite 000030～000039，开始前查占用；冲突则整段重排并同步所有计划，不覆盖他人迁移。

本文件仅编写计划，不表示已实现或已测试。执行时先使用 using-git-worktrees 建立隔离工作区；所有命令从仓库根执行。每一步只做所列一个动作；较长代码修改按列出的文件／函数逐项执行，完成该任务的 RED→GREEN 与独立 review 后再进入下一任务。

## 文件结构与职责

`internal/commercial`：单位与规则；repository/commercial：映射、目录、订阅；service/commercial：周期调度；handler/router：空间商业 API。具体迁移文件由拥有表的任务创建，必须同时提供 PG/SQLite up/down。

### Task 1: F01: 金额、Credits 和月周期原语

**Files:**
- Create: `internal/commercial/amount.go`、`internal/commercial/amount_test.go`
- Create: `internal/commercial/cycle.go`、`internal/commercial/cycle_test.go`

**Interfaces:**
- Consumes: 规格 4.1；V03 确认的精度与时间分辨率。
- Produces: `type Credits int64`（百万分之一 Credit）；`type CNYFen int64`；`ParseCredits(string) (Credits,error)`；`MonthBoundary(time.Time,int) time.Time`。

**行为与边界：** 确保月末、闰年和用户显示时区变化不改变商业锚点；如 V03 要求另一种精度，先修规格和本接口，不能只改一个消费者。

**验收映射：** B03/B04；AC-06；USE-05

- [ ] **Step 1: 在 `internal/commercial/cycle_test.go` 写入以下失败断言。**

```go
package commercial
import("testing"; "time")
func TestMonthBoundaryRestoresAnchor(t *testing.T) {
    loc := time.FixedZone("Asia/Shanghai", 8*3600)
    anchor := time.Date(2028,1,31,10,0,0,0,loc)
    if got := MonthBoundary(anchor,1); got.Day()!=29 { t.Fatal(got) }
    if got := MonthBoundary(anchor,2); got.Day()!=31 { t.Fatal(got) }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial -run "Test(MonthBoundary|ParseCredits)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

`amount.go` 使用 `math/big.Rat` 精确解析，拒绝负数、超过 6 位小数、非有限数字及 int64 溢出，API 输出固定十进制字符串；人民币独立使用分，不能与 Credits 相加。

```go
type Credits int64
type CNYFen int64

func MonthBoundary(anchor time.Time, months int) time.Time {
    first := time.Date(anchor.Year(), anchor.Month()+time.Month(months), 1,
        anchor.Hour(), anchor.Minute(), 0, 0, anchor.Location())
    last := first.AddDate(0, 1, -1).Day()
    day := anchor.Day()
    if day > last { day = last }
    return time.Date(first.Year(), first.Month(), day, first.Hour(), first.Minute(), 0, 0, first.Location())
}
```

`cycle.go` import `time`；调用方始终传原始 anchor，不把上个月末当新锚点。`amount_test.go` 加入 `0.000001→1`、`1.000000→1000000`、`0.0000001` 拒绝、int64 边界、负数拒绝断言。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial -run "Test(MonthBoundary|ParseCredits)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/amount.go internal/commercial/amount_test.go internal/commercial/cycle.go internal/commercial/cycle_test.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: add fixed precision commercial units and cycles"
```

### Task 2: F02: 空间商业权限与唯一 Customer 映射

**Files:**
- Create: `internal/commercial/access.go`、`internal/commercial/access_test.go`
- Create: `internal/application/repository/commercial/account.go`、`internal/application/repository/commercial/account_test.go`
- Create: `migrations/versioned/000110_commercial_accounts.up.sql`、`migrations/versioned/000110_commercial_accounts.down.sql`
- Create: `migrations/sqlite/000030_commercial_accounts.up.sql`、`migrations/sqlite/000030_commercial_accounts.down.sql`

**Interfaces:**
- Consumes: 现有 types.TenantRole；V03 Customer 幂等契约。
- Produces: `CanManageBilling(role string, active, billingGrant bool) bool`；repository `Account{TenantID uint64; CustomerID string; Version int64}`、`NewAccountStore(*gorm.DB) *AccountStore`、`Bind(ctx context.Context, a Account) error`、`Get(ctx context.Context, tenant uint64) (Account,error)`。

**行为与边界：** 为 AccountStore 编写 PostgreSQL／SQLite 集成测试，两个 goroutine 抢同 tenant 和同 customer 的不同映射均最终唯一。SQLite 不能替代 PostgreSQL 并发证据。Owner 转移后无显式授权的原 Owner 失去商业能力。

**验收映射：** B01/B20；COM-01；AC-01/02

- [ ] **Step 1: 在 `internal/commercial/access_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestBillingAccessDoesNotInheritAdmin(t *testing.T) {
    if CanManageBilling("admin",true,false) { t.Fatal("admin inherited billing") }
    if !CanManageBilling("viewer",true,true) { t.Fatal("explicit grant ignored") }
    if CanManageBilling("owner",false,true) { t.Fatal("inactive member accepted") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/repository/commercial -run "Test(BillingAccess|Account)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func CanManageBilling(role string, active, billingGrant bool) bool {
    return active && (role == "owner" || billingGrant)
}
```

迁移 up 在两个数据库创建相同逻辑约束；SQLite 的整数类型保持兼容：

```sql
CREATE TABLE commercial_accounts (
 tenant_id BIGINT PRIMARY KEY, customer_id TEXT NOT NULL UNIQUE,
 version BIGINT NOT NULL DEFAULT 1
);
CREATE TABLE commercial_grants (
 tenant_id BIGINT NOT NULL, user_id TEXT NOT NULL, capability TEXT NOT NULL,
 granted_by TEXT NOT NULL, version BIGINT NOT NULL DEFAULT 1,
 PRIMARY KEY (tenant_id,user_id,capability)
);
```

`Bind` 使用 `INSERT ... ON CONFLICT DO NOTHING` 后读取 tenant 和 customer 两个唯一索引；同映射返回成功，不同映射返回 `errors.New("account_mapping_conflict")`。`Get` 始终按 tenant 查询。down 仅逆序 DROP 新表；有商业数据的环境禁止执行 down，使用前向补偿。映射外部创建由 V03 稳定 key 保证，不在数据库事务内长时间等待外部 HTTP。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/repository/commercial -run "Test(BillingAccess|Account)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/access.go internal/commercial/access_test.go internal/application/repository/commercial/account.go internal/application/repository/commercial/account_test.go migrations/versioned/000110_commercial_accounts.up.sql migrations/versioned/000110_commercial_accounts.down.sql migrations/sqlite/000030_commercial_accounts.up.sql migrations/sqlite/000030_commercial_accounts.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: isolate space commercial authority"
```

### Task 3: F03: 不可变套餐目录与分段升级报价

**Files:**
- Create: `internal/commercial/catalog.go`、`internal/commercial/quote.go`、`internal/commercial/quote_test.go`
- Create: `internal/application/repository/commercial/catalog.go`
- Create: `migrations/versioned/000111_commercial_catalog.up.sql`、`migrations/versioned/000111_commercial_catalog.down.sql`
- Create: `migrations/sqlite/000031_commercial_catalog.up.sql`、`migrations/sqlite/000031_commercial_catalog.down.sql`

**Interfaces:**
- Consumes: F01 Credits/CNYFen；V03 Publish 契约；F02 AccountStore。
- Produces: `PlanVersion{Key string; Version int64; Price CNYFen; Monthly Credits; Features map[string]bool; Limits map[string]int64}`；`Prorate(delta int64, remaining int64, duration int64, roundUp bool) (int64,error)`；`Quote{ID string; TenantID uint64; SubscriptionVersion int64; Amount CNYFen; CreditDelta Credits; ExpiresAt time.Time}`。

**行为与边界：** 测试当前月补差、连续升级、未来已付区间、月底、过期报价、并发变更冲突；定义 0 与无限上限分开，未知价格／基础档配置不得发布。

**验收映射：** B16/B17/B21；COM-02/07；AC-07

- [ ] **Step 1: 在 `internal/commercial/quote_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestProrateDoesNotRefillMonth(t *testing.T) {
    v,err:=Prorate(9000000,10,30,false)
    if err!=nil || v!=3000000 { t.Fatalf("%d %v",v,err) }
    cents,err:=Prorate(101,1,2,true)
    if err!=nil || cents!=51 { t.Fatalf("%d %v",cents,err) }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial -run "Test(Prorate|Quote|Catalog)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

`quote.go` import `math/big` 与 `errors`；执行乘法时使用 big.Int，最后检查 IsInt64：

```go
func Prorate(delta, remaining, duration int64, roundUp bool) (int64,error) {
    if delta<0 || remaining<0 || duration<=0 || remaining>duration {
        return 0, errors.New("invalid_quote_interval")
    }
    n:=new(big.Int).Mul(big.NewInt(delta),big.NewInt(remaining))
    q,r:=new(big.Int),new(big.Int)
    q.QuoRem(n,big.NewInt(duration),r)
    if roundUp && r.Sign()!=0 { q.Add(q,big.NewInt(1)) }
    if !q.IsInt64() { return 0,errors.New("quote_overflow") }
    return q.Int64(),nil
}
```

金额报价逐段保持有理数并在总和末尾舍入一次；上面的函数用于单段最终舍入和月额度补发，不能逐段 roundUp 后再求和。catalog SQL 为 `(plan_key,version)` 主键，保存 `definition_json TEXT`、`external_id TEXT UNIQUE`、`state TEXT`；发布状态仅 draft/publishing/published/archived，published 定义禁止 UPDATE。quote 表保存 tenant、订阅版本、快照 JSON、expires_at 和 used_order_id 唯一。同一事务校验版本并消费 quote。发布 API 外部成功后丢响应按 V03 查询原版。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial -run "Test(Prorate|Quote|Catalog)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/catalog.go internal/commercial/quote.go internal/commercial/quote_test.go internal/application/repository/commercial/catalog.go migrations/versioned/000111_commercial_catalog.up.sql migrations/versioned/000111_commercial_catalog.down.sql migrations/sqlite/000031_commercial_catalog.up.sql migrations/sqlite/000031_commercial_catalog.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: version catalog and quote remaining paid intervals"
```

### Task 4: F04: 周期发放与到期降级投影

**Files:**
- Create: `internal/commercial/lifecycle.go`、`internal/commercial/lifecycle_test.go`
- Create: `internal/application/service/commercial/lifecycle.go`
- Create: `internal/application/repository/commercial/subscription.go`
- Create: `migrations/versioned/000112_commercial_lifecycle.up.sql`、`migrations/versioned/000112_commercial_lifecycle.down.sql`
- Create: `migrations/sqlite/000032_commercial_lifecycle.up.sql`、`migrations/sqlite/000032_commercial_lifecycle.down.sql`

**Interfaces:**
- Consumes: F01 MonthBoundary、F03 PlanVersion；V03 月度发放与到期契约。
- Produces: `MonthlyGrantKey(subscription string, start time.Time) string`；`Subscription{ID string; TenantID uint64; Plan PlanVersion; Anchor, PaidUntil time.Time; Version int64}`；`LifecycleService.Tick(ctx context.Context, now time.Time) error`。

**行为与边界：** 实现幂等 Tick 的 DB 集成测试：同一周期多 worker、月发已成功但本地未确认、年付逐月、提前续费不提前发、过期仍保留充值；不得靠测试进程时间快进假装外部已验证分钟边界。

**验收映射：** B03/B04/B05/B06；OM-05/06/07；AC-06/08/09

- [ ] **Step 1: 在 `internal/commercial/lifecycle_test.go` 写入以下失败断言。**

```go
package commercial
import("testing";"time")
func TestMonthlyGrantKeySurvivesWorkerReplay(t *testing.T) {
    a:=time.Date(2028,2,29,2,0,0,0,time.UTC)
    if MonthlyGrantKey("s1",a)!=MonthlyGrantKey("s1",a.In(time.FixedZone("display",3600))) { t.Fatal("display zone changed key") }
    if MonthlyGrantKey("s1",a)==MonthlyGrantKey("s2",a) { t.Fatal("subscriptions collided") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(Monthly|Lifecycle)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func MonthlyGrantKey(subscription string,start time.Time) string {
    return subscription+"/monthly/"+start.UTC().Format(time.RFC3339)
}
```

`subscription.go` 使用本任务独立的 `000112_commercial_lifecycle`／SQLite `000032` 配对迁移创建 `commercial_subscriptions` 与 `commercial_benefit_jobs`，不改已执行历史迁移。订阅记录含 tenant、外部 ID、anchor、paid_until、未来区间 JSON、version；benefit job 的 key 唯一，含 state/external_ref。

Tick 只生成 `month_start<=now<paid_until` 的到期任务，以唯一键 claim；外部自动发放与本地任务只能有一个权威发放者，按照 V03 明确配置。成功后保存外部批次 ID，未知不更换 key。到期更新基础档投影和超限原因，不删除数据、成员和未到期充值。升级差额使用 `order_line_id`，与月度 key 分开。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/application/service/commercial -run "Test(Monthly|Lifecycle)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/lifecycle.go internal/commercial/lifecycle_test.go internal/application/service/commercial/lifecycle.go internal/application/repository/commercial/subscription.go migrations/versioned/000112_commercial_lifecycle.up.sql migrations/versioned/000112_commercial_lifecycle.down.sql migrations/sqlite/000032_commercial_lifecycle.up.sql migrations/sqlite/000032_commercial_lifecycle.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: schedule idempotent monthly benefits and downgrade"
```

### Task 5: F05: 原子资源配额与商业路由权限

**Files:**
- Create: `internal/commercial/quota.go`、`internal/commercial/quota_test.go`
- Create: `internal/handler/commercial.go`、`internal/router/routes_commercial.go`
- Modify: `internal/router/router.go`、`internal/container/container.go`
- Create: `internal/router/commercial_scope_test.go`

**Interfaces:**
- Consumes: F02 CanManageBilling；F03 catalog；现有 RouterParams 和 tenant membership 中间件。
- Produces: `CanIncrease(current,delta int64,limit *int64) bool`；`RegisterCommercialRoutes(*gin.RouterGroup,*handler.CommercialHandler)`；`NewCommercialHandler(*gorm.DB) *CommercialHandler`，查询与命令从认证上下文取 tenant。

**行为与边界：** 路由测试使用 httptest：相同用户 A/B 空间查询不串；未授权 Admin 的 POST 返回403；并发新增资源不超过限制；充值不修改配额；不在健康检查时发起收费请求。

**验收映射：** B02/B05/B20；COM-01/07；AC-01/02/09

- [ ] **Step 1: 在 `internal/commercial/quota_test.go` 写入以下失败断言。**

```go
package commercial
import "testing"
func TestQuotaAllowsCleanupWhileOverLimit(t *testing.T) {
    limit:=int64(10)
    if CanIncrease(12,1,&limit) { t.Fatal("over quota growth") }
    if !CanIncrease(12,-1,&limit) { t.Fatal("cleanup blocked") }
    zero:=int64(0)
    if CanIncrease(0,1,&zero) { t.Fatal("zero treated as unlimited") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/commercial ./internal/router -run "Test(Quota|CommercialScope)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func CanIncrease(current,delta int64,limit *int64) bool {
    if current<0 { return false }
    if delta<=0 { return delta>=-current }
    if current>math.MaxInt64-delta { return false }
    return limit==nil || current+delta<=*limit
}
```

`quota.go` 导入 `math`。提交资源新增时在资源计数与实际资源写入的同事务中执行 CAS 条件，不只调用此纯判断后无锁写入。新增商业路由前缀 `/api/v1/commercial`，GET `/summary`、`/plans`、`/usage`、`/orders`；写操作由后续任务注册到同 group。路径无 tenant 参数，服务端 scope 必填；API key 缺独立商业 capability 时拒绝，不能用现有 full-access 自动推导购买权。把 handler/service/repository 通过现有 dig 注册并新增 RouterParams 字段，不重建 router。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/commercial ./internal/router -run "Test(Quota|CommercialScope)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/commercial/quota.go internal/commercial/quota_test.go internal/handler/commercial.go internal/router/routes_commercial.go internal/router/commercial_scope_test.go internal/router/router.go internal/container/container.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: enforce commercial scope and resource quota"
```

