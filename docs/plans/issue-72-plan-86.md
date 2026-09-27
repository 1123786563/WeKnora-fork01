# [Lago 14] 套餐额度与充值额度按到期顺序消费 实施计划（Issue #86）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让套餐月度批次与充值批次的消费统一按「最早到期、同到期最早发放」选择：Lago 钱包 priority 编码落线、本地 lot 分配顺序收口、到期不复活，并把余额分解（权威余额/预占/退款锁定/可用/批次）暴露到 Billing API 与账单页，可与 Lago traceable state 对账。

**Architecture:** 在冻结的 CommercialPlatform seam 上做加法（ADR-0014 additive 规则，`internal/modules/commercial/platform.go:14-19`）：批次快照增广 source/granted_at；到期的消费顺序以 **priority 类编码**写进钱包创建（t03 verdict fact b：Lago 消费序 = `priority ASC, created_at ASC`），月度钱包在「有未到期充值批次先于本月期末到期」的月份让位到更高 priority；WeKnora 侧的预占 lot 分配补齐同到期 tie-break 并从权威批次快照同步 lot 集；余额分解经 `GET /commercial/account` 增广字段暴露（closed-token 纪律不变），前端 BillingPage 消费。

**Tech Stack:** Go（gin + gorm，模块 `internal/modules/commercial`）、TypeScript（React+Vite `apps/web`，契约 `packages/contracts`，`packages/api-client`）、Lago Community v1.53.0（deploy/lago compose 栈）、Playwright（浏览器验收）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（§Credits and Task admission L129-135、§验收 L219-221、用户故事 18 L48）；`CONTEXT.md`（套餐内额度/充值额度/额度预占词条 L268-281）；`docs/adr/0012-lago-as-commercial-billing-authority.md`（含 2026-09-23 充值批次修订）；`docs/migrations/lago/t03-wallet-semantics/verdict.md`（消费顺序/到期/撤回实测语义）。

**集成基线与并行说明（执行前必读）:** 本 worktree 基线 `ee02d3218`（lago-int HEAD 同点，已核实 `git -C .worktrees-issue72/lago-int log -1`）。**该基线不含 #85（购买 Credits/充值批次）代码**——`git log --grep="#85"` 无命中、`platform.go` 及各 `*_command.go` 无 top-up 命令、`commercialplatform/lago.go:1005-1011` 明示 top-up 钱包仅为「future」预期；编排指令中「前置 Issue 已落地接口」对 #85 与实况不符（#75/#80/#81/#82/#83 的成果在基线内：`subscription_command.go`、`purchase_command.go` 等）。#85 正在并行 worktree `.worktrees-issue72/issue-85` 实施（无产出可读）。因此：本计划所有任务先在基线上以「月度维度 + 通用批次面」落地并可独立测试；凡消费 #85 约定的任务（Task 1 的 top-up 批次识别、Task 3 的 top-up lot 来源）内置 **#85 对齐步骤**（执行时 rebase 到含 #85 的集成分支后逐项核对常量/表名/命令名，见各任务 Consumes）。

---

## Global Constraints

（spec 原文引用，行号来自 `docs/specs/2026-09-20-lago-billing-migration-design.md`）

- L131: "Included Credits expire at the end of their monthly period and do not roll over. Each top-up batch expires twelve calendar months after becoming effective."
- L132: "Credits consume globally by earliest expiry, then earliest grant time. Lago Wallet and transaction priorities may implement this order only after real runtime verification; inability to preserve this invariant blocks migration."
- L221（验收 8）: "Credits ordering: monthly expiry, twelve-month top-up expiry, earliest-expiry consumption, grant-time tie-break, concurrent use, void, and refund satisfy the approved invariants."
- L48（用户故事 18）: "As a 账单管理员, I want current balance, reserved Credits, refund-locked Credits, and reconciliation age shown separately, so that available funds are not confused with money already committed."
- L21（权威边界）: Lago 是 Wallet/Credits/最终用量计价权威；WeKnora 投影"不能独立发放、扣减或撤回 Credits，也不能形成第二个商业账本"。
- t03 verdict（`docs/migrations/lago/t03-wallet-semantics/verdict.md`）协调义务：a1-1 派发前按协调层注册表拒绝已过期批次（不依赖 Lago 惰性终止，实测 ~65 分钟窗口）；a1-2 发放后 settle-wait 再放行消费；b 到期秩必须编码进 `priority`（1..50），同优先级 `created_at` 决胜；E3 钱包 API 无外部幂等，恢复只能按 metadata 查询。
- ADR-0012 修订（2026-09-23，用户裁决 75-a2=选项 B）：充值批次状态机由 WeKnora 协调层承载；Lago Wallet 交易仅在付款确认到账时刻幂等创建（幂等键绑定渠道支付单号）；不设产品级充值并发上限；Lago 仍是余额、到账事实与消费顺序的权威。
- Closed-token API 纪律（`platform.go:21-23`、`internal/handler/commercial.go:569-578`）：Billing API 不透出 provider URL/路径/外部 id/原始错误文本；金额一律十进制数字字符串。
- 安全红线（实现验收条件）：服务端出站仅 http/https 且请求前校验 host、拒绝 localhost/环回/私有/保留地址（既有 `validateOutboundHostWithBypass`，`lago.go:294`；dev 栈环回需显式 `WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true`）；数据库查询一律参数绑定，禁止拼接/format/f-string 组 SQL；凭据只从环境变量或密钥服务读取，源码、示例与测试不得写入可用凭据字面量。
- 并行 worktree 纪律：真实栈验证避开 :5272/:5273、:48889/:48890（`weknora-lago-82r5-*` 占用）与 :48895/:48896（`weknora-lago-t11-*` 占用，`docker ps` 2026-09-28 两轮实测）；本计划选定 **48897/48898**（编写期实测空闲）——执行日起栈前须 `docker ps` 复核仍空闲，被占则顺延至下一空闲 4889x 对并同步更新本计划端口；浏览器栈端口用 488xx 高段自定义。

## Review Focus

spec 是愿景文档：以下是它隐含、但没有任何任务的测试覆盖时最可能咬人的五类输入/失败模式。每行后缀 owning task 及其测试。

1. **三类批次共存时的跨族乱序**——「老化充值（到期早于本月期末）+ 新充值（到期晚于期末）+ 月度」共存时，任何静态 priority 编码都无法同时表达 A→M→B 全序（r1 审查 High 已证伪单类+二元让位方案）。→ Task 2 权威重排测试 `TestWalletRankMixedFamilies`（混合场景纯函数）+ `TestLagoRebalancePutsMixedFamiliesInExpiryOrder`（PUT wire）+ `TestMonthlyGrantEncodesYieldPriority`（初值）+ Task 6 `TestLagoCreditsOrder` 阶段 d（真实栈三钱包消费序）。
2. **同到期批次分配顺序不确定**——`commercial_budget_lots` 现有 `ORDER BY expires_at ASC`（`budget_reservation.go:149-152`）无发放时间决胜，同到期批次选择不确定，可能先消费晚发放批次。→ Task 3 测试 `TestReserveSameExpiryPicksEarliestIssuedLot`。
3. **已过期/已终止批次经同步复活**——Lago 惰性终止窗口内（verdict a1-1）过期钱包仍 `active` 且带余额，若 lot 同步按快照余额回写 remaining，已到期额度会重新可分配。→ Task 3 测试 `TestSyncLotsNeverResurrectsExpiredBatch`。
4. **预占/锁定与权威余额口径混淆**——分解页把 `balance−held−locked` 之外的钱当可用，或把 pending/未到账充值计入可用（spec 用户故事 9）。→ Task 4 测试 `TestAccountCreditsBreakdownArithmetic`（快照只计 active 钱包、top-up 未到账不在快照）+ Task 5 测试 `loadCommercialAccount` 缺 benefits 时降级。
5. **跨租户批次泄漏与 provider 词汇越界**——批次/lot 查询未按 tenant 收口，或 wire 面透出 lago_id/URL。→ Task 3 测试 `TestSyncLotsTenantScoped`、Task 4 测试 `TestBenefitsWireNoProviderVocabulary`（既有纪律回归）。

---

## 消费顺序编码（本计划冻结的设计决策；r1 审查后由「纯静态」改为「静态初值 + 刷新时权威重排」）

**问题**：Lago 消费序 = `priority ASC, created_at ASC`（t03 E2 实测），priority 是钱包创建时赋值、而「最早到期优先」随时间漂移。

**纯静态编码的反例（审查 R1-High，已证伪）**：充值固定单类 priority=2 + 月度二元让位（1↔3）在「老化充值 A（到期早于本月期末）+ 新充值 B（到期晚于期末）+ 月度 M」三类共存时给出 A(2)→B(2)→M(3)，而正确序为 A→M→B——B 先于月度被消耗、月末月度额度被作废而更晚到期的充值被保留。固定 12 月 TTL 保证 aging 与 fresh 充值必然可共存（spec L131），故该输入不是边角而是常态。

**修订设计（两层）**：

1. **创建时静态初值**（减少重排写入量，非正确性依赖）：
   - 充值批次单类 `TopUpWalletPriority = 2`（统一 TTL 下族内 `expiry ASC ≡ 发放时间 ASC`，`created_at` 决胜即到期序）。
   - 月度钱包 `MonthlyWalletPriorityFor(topUpExpiries, periodEnd)`：无「未到期且到期早于期末」的充值 → `MonthlyWalletPriority`(1)，否则 `TopUpWalletPriority + 1`(3)（对无 fresh 共存的场景已正确）。
2. **刷新时权威重排（不变量的唯一权威保障）**：每次 benefits 刷新（账单访问 lazy 链）后，协调层读权威 active 钱包列表，按 **(expires_at ASC, created_at ASC)** 计算正确秩 rank ∈ [1, n]（n ≤ 6），与各钱包当前 priority 比对，不一致者经 `PUT /api/v1/wallets/:id {priority: rank}` 校准；值相同不发包（幂等零写）。三类共存反例在重排后收敛为 A=1、M=2、B=3 → Lago 序 A→M→B，正确。
   - **真实运行时依据（本计划编写期实测，pinned v1.53.0 运行容器源码）**：`wallet_actions.rb` 的 `update_params` permit `:priority`；`Wallets::UpdateService` 执行 `wallet.priority = params[:priority] if params[:priority]`；已终止钱包拒绝 update（`wallet_is_terminated`）——重排只触达 active 钱包，与终止语义无冲突。
   - **spec L132 落实**："priorities may implement this order only after real runtime verification"——Task 6 集成测试在真实栈验证 PUT 生效与消费序；**若实测 v1.53 拒绝 priority 更新则该点 BLOCKED，按 spec 原文回设计阶段升级，不得静默降级**。
   - **并发姿势**：重排是收敛性校准——并发刷新/发放各自校准，最后写入者收敛（rank 由权威列表计算，重放无害）；重排与发票扣减竞态的最坏情形是一笔按旧序扣减、下笔已校准（刷新 cadence：每次账单访问、每次发放后）。失败 → closed unreachable 状态、不阻塞账单读，下次刷新重试。
- **WeKnora 本地镜像序**：lot 分配 `ORDER BY expires_at ASC, issued_at ASC, lot_id ASC`（`issued_at` 即发放时间；`lot_id` 为确定性终决）——本地序与权威重排同一比较键。

---

### Task 1: 批次快照增广——source/granted_at 与 top-up 批次入列

**Files:**
- Modify: `internal/modules/commercial/subscription_command.go:254-275`（`CreditBatchSnapshot`、`BenefitsSnapshot`）
- Modify: `internal/modules/commercial/commercialplatform/lago.go:760-772`（`lagoWallet` 补 `created_at` 解析）、`lago.go:994-1023`（快照构建循环）
- Modify: `internal/modules/commercial/commercialplatform/fake.go:43-66`（`fakeWallet.Priority`… 本任务只补 `Source`/`GrantedAt` 所需的元数据；Priority 字段在 Task 2 加）
- Test: `internal/modules/commercial/commercialplatform/lago_subscription_test.go`（`TestLagoBenefitsSnapshot` 旁新增）

**Interfaces:**
- Consumes: 既有 `SnapshotKindBenefits` 读路径（`lago.go:990-1022`）；`walletsStub` harness（`lago_subscription_test.go:220-382`）。**#85 对齐点**：top-up 钱包识别键——本计划锚定语义「由 #85 到账命令创建、携带 #85 定义的充值 metadata（渠道支付单号）、无 `weknora_period` 键」；执行时以 #85 实际常量名为准（预期形如 `WalletMetaTopUpOrder`），若 #85 尚未合入则先以「无 period 键 + 本租户 tenant 键」为识别条件，#85 合入后收紧。
- Produces:
  ```go
  // CreditBatchSnapshot 新增字段（additive，零值兼容既有读者）：
  type CreditBatchSnapshot struct {
      Period       string
      BalanceMicro int64
      ExpiresAt    time.Time
      Source       string    // "monthly" | "topup"（closed set：BatchSourceMonthly/BatchSourceTopUp）
      GrantedAt    time.Time // 钱包 created_at（发放时间，tie-break 展示用）
  }
  const (
      BatchSourceMonthly = "monthly"
      BatchSourceTopUp   = "topup"
  )
  ```

- [ ] **Step 1: 写失败测试**（`lago_subscription_test.go`，复用 `newWalletsStub` + `lagoTestConfig`）

```go
// TestLagoBenefitsSnapshotListsTopUpBatch: 一个无 weknora_period 元数据、
// 携带本租户 weknora_tenant 键的 active 钱包（#85 充值批次形状）必须进入
// Batches 且 Source=topup、GrantedAt=钱包 created_at；月度批次 Source=monthly。
// 余额合计仍含两者（既有行为不变）。
func TestLagoBenefitsSnapshotListsTopUpBatch(t *testing.T) {
    stub := newCombinedStub(t) // 既有 harness（TestLagoBenefitsSnapshot 同款）
    stub.wallets.entitlementCustomer = commercial.ExternalCustomerID(subTenant)
    stub.subs.preloaded = []stubSubscription{{ // 同 TestLagoBenefitsSnapshot:754-759
        ExternalID: commercial.ExternalSubscriptionID(subTenant),
        ExternalCustomer: commercial.ExternalCustomerID(subTenant),
        PlanCode: subPlanCode, Status: "active",
    }}
    stub.wallets.mu.Lock()
    stub.wallets.wallets = []stubWallet{
        { // 充值形状：无 period 键（本租户 tenant 键）
            LagoID: "w-topup", Customer: commercial.ExternalCustomerID(subTenant),
            Name: commercial.ExternalCustomerID(subTenant) + "-topup-ord1", Status: "active",
            GrantedCents: 5000, BalanceCents: 5000,
            ExpiresAt: "2100-01-31T00:00:00Z", CreatedAt: "2099-01-10T00:00:00Z",
            Metadata: map[string]string{commercial.WalletMetaTenant: commercial.ExternalCustomerID(subTenant)},
        },
        { // 月度形状（既有 seed 惯例，TestLagoBenefitsSnapshot:761-771 同款）
            LagoID: "w-monthly", Customer: commercial.ExternalCustomerID(subTenant),
            Name: commercial.MonthlyWalletName(subTenant, "2099-01"), Status: "active",
            GrantedCents: 990, BalanceCents: 990, ExpiresAt: "2099-02-01T00:00:00Z",
            CreatedAt: "2099-01-01T00:00:00Z",
            Metadata: map[string]string{
                commercial.WalletMetaTenant: commercial.ExternalCustomerID(subTenant),
                commercial.WalletMetaPeriod: "2099-01",
            },
        },
    }
    stub.wallets.mu.Unlock()
    snap, err := subAdapter(stub.url()).ReadSnapshot(context.Background(), commercial.SnapshotQuery{
        Kind: commercial.SnapshotKindBenefits, TenantID: subTenant})
    if err != nil { t.Fatal(err) }
    var sawTopUp, sawMonthly bool
    for _, b := range snap.Benefits.Batches {
        switch b.Source {
        case commercial.BatchSourceTopUp:
            sawTopUp = true
            if b.BalanceMicro != commercial.CentsToMicro(5000) { t.Fatalf("topup balance: %d", b.BalanceMicro) }
            if !b.GrantedAt.Equal(time.Date(2099, 1, 10, 0, 0, 0, 0, time.UTC)) { t.Fatalf("topup granted_at: %v", b.GrantedAt) }
        case commercial.BatchSourceMonthly:
            sawMonthly = true
        default:
            t.Fatalf("unknown source %q", b.Source)
        }
    }
    if !sawTopUp || !sawMonthly { t.Fatalf("batches incomplete: topup=%v monthly=%v", sawTopUp, sawMonthly) }
    if snap.Benefits.BalanceMicro != commercial.CentsToMicro(5990) { // 两者都计入余额（既有行为）
        t.Fatalf("balance = %d", snap.Benefits.BalanceMicro)
    }
}
```

注：`stubWallet` 与 `walletJSON` 需随本任务补 `CreatedAt string` 字段（harness 既有 `ExpiresAt string` 同款直传），`newCombinedStub`/`subAdapter`/`subTenant`/`subPlanCode` 为该文件既有标识符。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd <worktree> && go test ./internal/modules/commercial/commercialplatform/ -run TestLagoBenefitsSnapshotListsTopUpBatch -count=1`
Expected: FAIL（编译错 `b.Source` undefined，或运行期 topup 批次缺席）

- [ ] **Step 3: 实现**——`CreditBatchSnapshot`/常量按 Produces 定义；`lagoWallet` 增加 `CreatedAt string \`json:"created_at"\``；快照循环改为三分显式归类：①`meta[WalletMetaPeriod]` 非空（或月度名 fallback 命中）→ **monthly**；②`meta[WalletMetaPurchasePeriod]` 非空（或 `ExternalPurchaseSubscriptionID` 前缀名 fallback，`lago.go:1055-1064` 既有分支）→ **monthly**（购买首期批次与月度批次同族语义：期末到期、不结转，#82 D4 已定）；③其余携带本租户 `WalletMetaTenant`（或 #85 充值键）→ **topup**。三类之外（异租户/无锚）不进 Batches（既有排除行为不变）。topup 批次 Period 置 `""`，GrantedAt 解析 `created_at`。fake 适配器 `Wallets()`/快照同构（`fake.go:435` 附近列表构建处，`fakeWalletPeriod` 未命中的本租户钱包 → topup）。

- [ ] **Step 4: 跑包测试确认通过**

Run: `go test ./internal/modules/commercial/commercialplatform/ -count=1`
Expected: PASS（含既有 `TestLagoBenefitsSnapshot` 不回归）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/commercial/subscription_command.go internal/modules/commercial/commercialplatform/lago.go internal/modules/commercial/commercialplatform/fake.go internal/modules/commercial/commercialplatform/lago_subscription_test.go
git commit -m "issue-72(#86): (task1) benefits snapshot lists topup batches with source/granted_at"
```

### Task 2: 消费顺序编码与权威重排——priority 初值、WalletRank 与 rebalance 命令

**Files:**
- Modify: `internal/modules/commercial/subscription_command.go:91-117`（`TopUpWalletPriority`、`MonthlyWalletPriorityFor`、`GrantIncludedCreditsPayload.Priority` + `Validate`）及同文件新增 `CommandKindRebalanceCreditsOrder` + `RebalanceCreditsOrderPayload` + `WalletRankInput`/`WalletRank`
- Modify: `internal/modules/commercial/commercialplatform/lago.go:895-919`（`createWallet` 发送 `priority`）；`lago.go:760-772`（`lagoWallet` 增 `Priority int \`json:"priority"\``）；新增 `rebalanceCreditsOrder` 处理器（读列表 → WalletRank → 比对 → `PUT /api/v1/wallets/{lago_id}` `{"wallet":{"priority":rank}}`，值相同不发包）
- Modify: `internal/modules/commercial/service/commercial/benefits.go:342-394`（`EnsureMonthlyCredits` 计算并传 Priority 初值）、`refreshAndCollect` 末尾提交 rebalance 命令（失败 Warn 不阻塞读）、`service/commercial/purchase_fulfillment.go`（购买首期 grant 初值同规则）
- Modify: `internal/modules/commercial/commercialplatform/fake.go`（`fakeWallet.Priority` + `FakeWallet.Priority` 暴露 + rebalance 命令按同一 `WalletRank` 变更 fake 钱包 priority）
- Test: `internal/modules/commercial/subscription_command_test.go`（域纯函数）、`commercialplatform/lago_subscription_test.go`（wire 断言，`walletsStub` 增 PUT handler + `stubWallet.Priority`）、`service/commercial/benefits_test.go`（服务接线）

**Interfaces:**
- Consumes: Task 1 的 `BatchSourceTopUp`/快照 Source；`GrantIncludedCreditsPayload` 既有 Validate（`subscription_command.go:122-146`）；`listCustomerWallets`（`lago.go:974`）；真实运行时 PUT-priority 证据（v1.53 `wallet_actions.rb:164` update_params permit `:priority`、`update_service.rb:39` 赋值——本计划编写期 docker exec 实测）。#85 对齐点：若 #85 已定义充值 priority 常量则直接采用其名，语义必须等于「晚于全部无老化月度类」。
- Produces:
  ```go
  const TopUpWalletPriority = 2 // 充值批次单类初值；族内 created_at 决胜 = 到期序（统一 12 月 TTL）
  // MonthlyWalletPriorityFor 计算月度批次创建初值：无「未到期且到期早于 periodEnd」
  // 的充值批次 → MonthlyWalletPriority(1)；否则 TopUpWalletPriority+1(3)。
  func MonthlyWalletPriorityFor(topUpExpiries []time.Time, periodEnd time.Time) int
  // GrantIncludedCreditsPayload.Priority int —— 新增必填（Validate: 1..50）
  // —— 不变量的权威保障是 rebalance，初值只为减少重排写入量。

  // WalletRankInput 是参与权威排序的一个 active 批次（seam 内部形状）。
  type WalletRankInput struct {
      WalletRef string    // 钱包确定性名（适配器内部映射 lago_id）
      ExpiresAt time.Time
      GrantedAt time.Time // 权威 created_at（发放时间）
  }
  // WalletRank 按 (ExpiresAt ASC, GrantedAt ASC, WalletRef ASC) 计算正确秩
  // 1..n。三类共存反例（A 老化充值/M 月度/B 新充值）输出 A=1、M=2、B=3。
  func WalletRank(batches []WalletRankInput) map[string]int

  const CommandKindRebalanceCreditsOrder CommandKind = "rebalance_credits_order"
  type RebalanceCreditsOrderPayload struct {
      TenantID           uint64
      ExternalCustomerID string // derived-equality enforced（同 EnsureSubscriptionPayload 惯例）
  }
  // Key: "rebalance_credits_order:<ext-customer>"——收敛性校准，重放=再校准，无害。
  func RebalanceCreditsOrderCommandKey(externalCustomerID string) string
  // 适配器行为：列 active 钱包 → WalletRank → 与当前 priority 比对 →
  // 不一致者 PUT {wallet:{priority:rank}}；一致者零写。Receipt.ExternalID = ext-customer。
  ```

- [ ] **Step 1: 写失败测试（域纯函数——初值让位 + 混合场景秩）**

```go
// TestMonthlyWalletPriorityYieldsToAgingTopUp: 充值批次到期早于本月期末时
// 月度创建初值必须让位到充值类之上；无老化充值时保持 1。
func TestMonthlyWalletPriorityYieldsToAgingTopUp(t *testing.T) {
    periodEnd := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
    if got := commercial.MonthlyWalletPriorityFor(nil, periodEnd); got != commercial.MonthlyWalletPriority {
        t.Fatalf("no topup: got %d", got)
    }
    aging := []time.Time{time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)} // 早于期末
    if got := commercial.MonthlyWalletPriorityFor(aging, periodEnd); got != commercial.TopUpWalletPriority+1 {
        t.Fatalf("aging topup: got %d", got)
    }
    later := []time.Time{time.Date(2027, 2, 15, 0, 0, 0, 0, time.UTC)} // 晚于期末
    if got := commercial.MonthlyWalletPriorityFor(later, periodEnd); got != commercial.MonthlyWalletPriority {
        t.Fatalf("later topup: got %d", got)
    }
}

// TestWalletRankMixedFamilies（r1 审查 High 的判例）: 老化充值 A（到期早于
// 本月期末）+ 月度 M（期末）+ 新充值 B（晚于期末）共存时，正确全序是
// A→M→B——任何静态 priority 编码都表达不了（初值方案此时为 A=2,B=2,M=3，
// Lago 会消费 A→B→M），WalletRank 必须给出 A=1、M=2、B=3。
func TestWalletRankMixedFamilies(t *testing.T) {
    got := commercial.WalletRank([]commercial.WalletRankInput{
        {WalletRef: "B", ExpiresAt: time.Date(2027, 7, 10, 0, 0, 0, 0, time.UTC), GrantedAt: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)},
        {WalletRef: "M", ExpiresAt: time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC), GrantedAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
        {WalletRef: "A", ExpiresAt: time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC), GrantedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
    })
    if got["A"] != 1 || got["M"] != 2 || got["B"] != 3 {
        t.Fatalf("mixed-family rank = %+v, want A=1 M=2 B=3", got)
    }
}

// TestWalletRankSameExpiryEarliestGrant: 同到期按 GrantedAt 决胜（spec tie-break）。
func TestWalletRankSameExpiryEarliestGrant(t *testing.T) {
    exp := time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC)
    got := commercial.WalletRank([]commercial.WalletRankInput{
        {WalletRef: "late", ExpiresAt: exp, GrantedAt: exp.Add(-1 * time.Hour)},
        {WalletRef: "early", ExpiresAt: exp, GrantedAt: exp.Add(-2 * time.Hour)},
    })
    if got["early"] != 1 || got["late"] != 2 {
        t.Fatalf("same-expiry rank = %+v", got)
    }
}

// TestGrantPayloadRequiresPriorityRange: Priority ∈ [1,50] 必填。
func TestGrantPayloadRequiresPriorityRange(t *testing.T) {
    base := commercial.GrantIncludedCreditsPayload{
        TenantID: 9, ExternalCustomerID: commercial.ExternalCustomerID(9),
        Period: "2099-01", CreditsMicro: 1_000_000,
        ExpiresAt: time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC),
    }
    for _, p := range []int{0, -1, 51} { // 0 = 缺省，必须显式编码
        bad := base
        bad.Priority = p
        if err := bad.Validate(); err == nil {
            t.Fatalf("priority %d must be rejected", p)
        }
    }
    ok := base
    ok.Priority = 1
    if err := ok.Validate(); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/commercial/ -run "TestMonthlyWalletPriorityYields|TestWalletRank" -count=1`
Expected: FAIL（`MonthlyWalletPriorityFor`/`WalletRank` undefined）

- [ ] **Step 3: 实现域函数与 payload 校验**；随后写 wire 失败测试（grant 初值 + rebalance PUT）：

```go
// TestLagoGrantWalletCarriesEncodedPriority: grant 的 POST /api/v1/wallets
// 请求体必须携带 payload.Priority（沿 TestLagoGrantHappyPath:649 的
// recorded() 请求体断言法）。
func TestLagoGrantWalletCarriesEncodedPriority(t *testing.T) {
    stub := newWalletsStub(t)
    cmd := grantCommand(9_900_000)
    cmd.Payload = commercial.GrantIncludedCreditsPayload{
        TenantID: subTenant, ExternalCustomerID: commercial.ExternalCustomerID(subTenant),
        Period: "2099-01", CreditsMicro: 9_900_000,
        ExpiresAt: time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC),
        Priority: 3, // 让位月的月度批次
    }
    if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(), cmd); err != nil {
        t.Fatalf("grant: %v", err)
    }
    var body string
    for _, r := range stub.recorded() {
        if r.Method == http.MethodPost && r.Path == "/api/v1/wallets" { body = r.Body }
    }
    if !strings.Contains(body, `"priority":3`) {
        t.Fatalf("wallet create body must carry encoded priority, got %s", body)
    }
}

// TestLagoRebalancePutsMixedFamiliesInExpiryOrder: 三钱包按初值编码落库
// （A=2、B=2、M=3）后提交 rebalance → stub 收到恰三个 PUT，priority 分别
// 改为 A→1、M→2、B→3；钱包最终 priority 序 == 到期序。
// （walletsStub 先增 PUT /api/v1/wallets/{id} handler：解析 body 的
// wallet.priority、更新对应 stubWallet.Priority 并记录请求。）
func TestLagoRebalancePutsMixedFamiliesInExpiryOrder(t *testing.T) {
    stub := newWalletsStub(t)
    // seed：A（aging topup，exp 2099-01-15，priority 2，meta 无 period 键）
    //      B（fresh topup，exp 2099-07-10，priority 2）
    //      M（monthly，exp 2099-01-31，priority 3，meta 带 period）——同
    // TestLagoBenefitsSnapshotListsTopUpBatch 的直接 seed 法。
    if _, err := subAdapter(stub.url()).SubmitCommand(context.Background(),
        commercial.Command{Kind: commercial.CommandKindRebalanceCreditsOrder,
            Key: commercial.RebalanceCreditsOrderCommandKey(commercial.ExternalCustomerID(subTenant)),
            Payload: commercial.RebalanceCreditsOrderPayload{TenantID: subTenant,
                ExternalCustomerID: commercial.ExternalCustomerID(subTenant)}}); err != nil {
        t.Fatalf("rebalance: %v", err)
    }
    puts := map[string]int{} // 钱包 lago_id → 请求体 priority
    for _, r := range stub.recorded() {
        if r.Method == http.MethodPut && strings.HasPrefix(r.Path, "/api/v1/wallets/") {
            var p struct{ Wallet struct{ Priority int `json:"priority"` } }
            json.Unmarshal([]byte(r.Body), &p)
            puts[strings.TrimPrefix(r.Path, "/api/v1/wallets/")] = p.Wallet.Priority
        }
    }
    if len(puts) != 3 || puts["w-a"] != 1 || puts["w-m"] != 2 || puts["w-b"] != 3 {
        t.Fatalf("rebalance PUTs = %+v, want w-a=1 w-m=2 w-b=3", puts)
    }
}

// TestLagoRebalanceSkipsAlignedWallets: 权威 priority 已等于正确秩时零 PUT
// （幂等零写——刷新高频路径不发无谓写请求）。
func TestLagoRebalanceSkipsAlignedWallets(t *testing.T) {
    // seed 两钱包 priority 已 == WalletRank 结果 → 提交 rebalance →
    // recorded() 中无任何 PUT。
}
```

Run: `go test ./internal/modules/commercial/commercialplatform/ -run "TestLagoGrantWalletCarriesEncodedPriority|TestLagoRebalance" -count=1` → 先 FAIL（grant 请求体无 priority；无 PUT 发出）→ `createWallet` body 增 `"priority": payload.Priority`（int 直传）+ `rebalanceCreditsOrder` 实现 + `SubmitCommand` 分派新 Kind → PASS。

- [ ] **Step 4: 服务接线（初值 + 刷新链重排）**——`EnsureMonthlyCredits` 在构造 payload 前读一次 benefits 快照（`ReadSnapshot(SnapshotKindBenefits)`），收集 `Source == BatchSourceTopUp && ExpiresAt.After(now)` 的到期集合，`Priority: domain.MonthlyWalletPriorityFor(exps, end)`；快照读失败按既有 pending 姿势返回（`isPlatformFailure` 分支），不重试。`purchase_fulfillment.go` 的首期 grant 初值同规则（同月期末）。`refreshAndCollect` 末尾（投影写入后）提交 `rebalance_credits_order` 命令：失败记 Warn 且不改账单读结果（A-23 同款姿势，下次刷新重试）；成功无返回态。写服务测试：

```go
// TestMonthlyGrantEncodesYieldPriority（benefits_test.go，fake 适配器 +
// 既有服务测试 harness）：预先在 fake 种一个 topup 形状钱包（到期在本月中旬）
// → EnsureMonthlyCredits → fake.Wallets() 中月度钱包 Priority == 3；
// 移除 topup 后的下一期（SetNow 推进月份）== 1。
func TestMonthlyGrantEncodesYieldPriority(t *testing.T) {
    // 沿 TestMonthlyGrantNewPeriod（benefits_test.go:448）的 fake+store 装配；
    // fake 种入：非月度名钱包（充值形状）{Customer: ext, Name: ext+"-topup-x",
    //   GrantedCents: 5000, ExpiresAt: <本月 15 日>, CreatedAt: now}。
    // 断言 fake.Wallets() 中 MonthlyWalletName(tenant, 本期) 的 Priority == 3。
    // 第二段：SetNow 推进到下月（无 topup 到期早于新期末）→ 新月度钱包 Priority == 1。
}

// TestRefreshRebalancesMixedFamilies: fake 种 A/B/M 三钱包（初值编码）→
// 一次 EnsureBenefits（触发刷新链）→ fake.Wallets() 的 priority 变为
// A=1、M=2、B=3（fake 的 rebalance 处理器复用域 WalletRank）。
func TestRefreshRebalancesMixedFamilies(t *testing.T) {
    // 装配同上；断言遍历 fake.Wallets() 按 Name 匹配 A/M/B 的 Priority。
}
```

Run: `go test ./internal/modules/commercial/service/commercial/ -run "TestMonthlyGrantEncodesYieldPriority|TestRefreshRebalancesMixedFamilies" -count=1` → 先 FAIL 后 PASS。

- [ ] **Step 5: 全量回归 + Commit**

Run: `go test ./internal/modules/commercial/... -count=1 && make lint`
Expected: PASS（若 #82 既有 grant 调用测试因 Priority 必填报错，为其补显式 `Priority: domain.MonthlyWalletPriority`——语义即其原默认）

```bash
git add internal/modules/commercial/ internal/modules/commercial/commercialplatform/ internal/modules/commercial/service/commercial/
git commit -m "issue-72(#86): (task2) wallet priority initial encoding + authority rebalance to expiry order"
```

### Task 3: 本地 lot 同步与分配顺序收口（同到期 tie-break、不复活）

**Files:**
- Modify: `internal/modules/commercial/repository/commercial/budget_reservation.go:147-152`（ORDER BY 补 `issued_at ASC, lot_id ASC`）
- Modify: `internal/modules/commercial/repository/commercial/budget.go`（新增 `SyncLots` + `domain.LotSyncBatch` 或等价参数形状，放 `internal/modules/commercial/budget_types.go` 新文件亦可——跟随既有域类型文件布局）
- Modify: `internal/modules/commercial/service/commercial/benefits.go`（`NewBenefitsService` 签名增第 5 参 `budget *repocommercial.BudgetStore`（nil 合法=不同步）+ `refreshAndCollect` 末尾调用 `SyncLots`）
- Modify: `internal/container/container.go`——两处：①`container.go:915` 的 `must(container.Provide(commercialsvc.NewBenefitsService))` 不动，但需先在 Provide 区新增 `must(container.Provide(repocommercial.NewBudgetStore))`（`NewBudgetStore(db *gorm.DB)` 单参，fx 自动由已提供的 `*gorm.DB` 构造——注意 `container.go:2597` 处 `newMobileVoiceHandler` 内是**手动** `NewBudgetStore(db)`，属该 handler 私有实例，与本装配无冲突、不相干勿改）；②fx 构造图连通后 `NewBenefitsService` 的 budget 参数由 fx 注入，无需手写接线。测试调用点（`benefits_test.go` 等直接 `NewBenefitsService(...)` 处）补传 `NewBudgetStore(db)` 或 nil。
- Test: `internal/modules/commercial/repository/commercial/budget_test.go`（顺序/不复活/租户隔离）、`repository/commercial/budget_pg_test.go`（并发，`commercial_integration` tag）

**Interfaces:**
- Consumes: Task 1 的批次快照（`Source`/`GrantedAt`/`ExpiresAt`/余额）；既有 `BudgetLotRow`（`budget.go:117-124`，列 `remaining_micro/held_micro/expires_at/issued_at`）。#85 对齐点：充值批次进入快照后自动成为 lot 来源，无需本任务额外适配；#85 注册表若给出更精确的发放时间，`IssuedAt` 以快照 `GrantedAt` 为准。
- Produces:
  ```go
  // LotSyncBatch 是一次权威批次读回的 lot 同步输入（tenant 由调用方收口）。
  type LotSyncBatch struct {
      LotID          string    // 确定性钱包名（MonthlyWalletName/#85 充值钱包名）
      RemainingMicro int64     // 权威可见余额（micro）
      ExpiresAt      time.Time
      IssuedAt       time.Time
  }
  // SyncLots 单事务收口租户 lot 集：
  //  - 快照有且未过期: remaining = max(权威余额, held)（不削减在途预占）
  //  - 快照无/已过期（惰性窗口）: remaining = held（预占仍被背书，新增不可分配）
  //  - 新批次: 插入（remaining=权威余额, held=0）
  //  - 已过期批次绝不因快照出现而恢复可分配额度；lot 身份（lot_id）与
  //    expires_at/issued_at 写入后不变。
  func (s *BudgetStore) SyncLots(ctx context.Context, tenantID uint64, batches []LotSyncBatch, now time.Time) error
  ```

- [ ] **Step 1: 写失败测试（分配顺序）**

```go
// TestReserveSameExpiryPicksEarliestIssuedLot: 两个同到期 lot，先发放者
// （issued_at 早）必须先被分配——预占落在 earliest 上。
func TestReserveSameExpiryPicksEarliestIssuedLot(t *testing.T) {
    store, db := testBudgetStore(t) // 既有 harness（budget_test.go:22）
    end := time.Now().UTC().Add(time.Hour)
    seedBudget(t, db, 1_000_000, 1_000_000) // 既有 helper：租户 7 账户/任务/lot1
    exp := end
    for _, row := range []any{ // 测试内直接构造（seedBudget 同款 db.Create）
        &BudgetLotRow{TenantID: 7, LotID: "lot-late", RemainingMicro: 1_000_000,
            ExpiresAt: &exp, IssuedAt: time.Now().UTC().Add(-1 * time.Hour)},
        &BudgetLotRow{TenantID: 7, LotID: "lot-early", RemainingMicro: 1_000_000,
            ExpiresAt: &exp, IssuedAt: time.Now().UTC().Add(-2 * time.Hour)},
    } {
        if err := db.Create(row).Error; err != nil { t.Fatal(err) }
    }
    if _, err := store.Reserve(context.Background(), budgetRequest("r1", "k1", 500_000)); err != nil {
        t.Fatalf("reserve: %v", err)
    }
    var early, late, seeded BudgetLotRow
    db.Where("tenant_id = ? AND lot_id = ?", 7, "lot-early").First(&early)
    db.Where("tenant_id = ? AND lot_id = ?", 7, "lot-late").First(&late)
    db.Where("tenant_id = ? AND lot_id = ?", 7, "lot1").First(&seeded)
    // Reserve 500_000 ≤ 单行容量：必须整额落在 issued 最早的行（lot-early，
    // issued -2h；lot-late -1h、lot1（seedBudget 自带）now）。
    if early.HeldMicro != 500_000 || late.HeldMicro != 0 || seeded.HeldMicro != 0 {
        t.Fatalf("earliest-issued lot must be allocated first: early=%d late=%d lot1=%d",
            early.HeldMicro, late.HeldMicro, seeded.HeldMicro)
    }
}
```

注：三行同 `expires_at`（`end`），唯一决胜维度即 `issued_at`。修复前 `ORDER BY expires_at ASC` 下并列行顺序**未定义**——无论实现实际命中哪一行（SQLite 实测按插入序命中 `lot1`），断言 `early.HeldMicro == 500_000` 都不成立，故 RED 稳定；执行者若观察到第三种命中行不构成测试不稳定，只说明未定义序落在了别处。

Run: `go test ./internal/modules/commercial/repository/commercial/ -run TestReserveSameExpiryPicksEarliestIssuedLot -count=1` → FAIL（无 tie-break，任意命中行都使断言失败）→ 改 ORDER BY → PASS。

- [ ] **Step 2: 写失败测试（不复活 + 租户隔离）**

```go
// TestSyncLotsNeverResurrectsExpiredBatch: 一个 lot 的批次已过期（快照携带
// 该钱包、Lago 尚未终止）→ SyncLots 后 remaining == held（不可再分配）；
// 快照中消失的批次同语义；从未见过的过期批次不插入。
// TestSyncLotsNeverShrinksBelowHolds: 快照余额 < held → remaining 保持 held。
// TestSyncLotsTenantScoped: tenant 8 的同步绝不改 tenant 7 的 lot 行。
```

Run: `go test ./internal/modules/commercial/repository/commercial/ -run "TestSyncLots" -count=1` → FAIL（SyncLots undefined）→ 实现（单事务、全部参数绑定）→ PASS。

- [ ] **Step 3: 接线 benefits 刷新**——`refreshAndCollect` 在批次 overlay 计算后组装 `[]LotSyncBatch`（快照 `Batches` × `CentsToMicro`；快照缺席但注册表在册的批次不产生同步输入——由「快照无 → remaining=held」路径天然覆盖），`s.budget != nil` 时调用；失败记 Warn 不改状态（lot 同步是预占面，失败不得把账单读变成错误——A-23 同款姿势）。容器装配见 Files 条目（fx Provide 新增 + 签名注入）。服务测试：`TestRefreshSyncsLotsFromSnapshot`（fake 快照两批次 → lot 表两行，顺序列正确）。

Run: `go test ./internal/modules/commercial/service/commercial/ -run TestRefreshSyncsLotsFromSnapshot -count=1` → FAIL → 实现 → PASS。

- [ ] **Step 4: 并发安全（PG 门控；先修 fixture 已知漂移）**——`budgetPGFixture`（`budget_pg_test.go:56-61`）当前只应用 `000116_commercial_budgets.up.sql`，而 `ReservationRow.Owner` 列由 `migrations/versioned/000161_commercial_reservations_owner.up.sql` 补建（该 migration 自述「the 000116 DDL never created it — Reserve INSERTs failed on production Postgres」；issue inventory L180 已登记既有 2 测试 FAIL 根因）。**修复步骤先行**：fixture 在 000116 之后按序读入并执行 `000161_commercial_reservations_owner.up.sql`（沿用现有 `os.ReadFile`+Exec 模式、路径拼接仅用常量文件名），既有 `TestBudgetPGConcurrentReservation` 等 2 个 FAIL 测试随之转绿——这是本任务的显式交付物，不是环境问题。随后新增：

```go
// TestSyncLotsConcurrentWithReserveNoOverAllocation（//go:build commercial_integration，
// SAAS_TEST_PG_DSN 门控，budgetPGFixture harness）: 8 goroutine 循环
// {SyncLots(余额递减快照) × Reserve}，收敛后 Σ(lot 分配) ≤ Σ(remaining-held)+Σheld，
// 无 lot 负值、无超分配；到期切换（快照批次过期）与并发 Reserve 交错不产生
// 已过期 lot 上的新增分配。
```

Run: `SAAS_TEST_PG_DSN=<dev DSN> go test -tags commercial_integration ./internal/modules/commercial/repository/commercial/ -run "TestSyncLotsConcurrentWithReserveNoOverAllocation|TestBudgetPG" -count=1 -v`（复用宿主 WeKnora-postgres-dev:5432，DSN 由执行者本地注入，不入库）
Expected: 全部 PASS（含修复后的既有 2 测试）；无 PG 环境时记录 blocked-env（既有惯例）——但既有 2 测试的 fixture 修复不依赖 PG 可用性（代码级修复先行合入），并以 sqlite 并发版本（`TestSyncLotsConcurrentSqlite`，8 goroutine 同断言）作为底线证据。blocked-env 不得用于掩盖 fixture 缺列这类代码级缺陷。

- [ ] **Step 5: 回归 + Commit**

Run: `go test ./internal/modules/commercial/... -count=1`
Expected: PASS

```bash
git add internal/modules/commercial/ internal/container/container.go
git commit -m "issue-72(#86): (task3) lot sync from authority batches + earliest-issued tie-break, no resurrection"
```

### Task 4: 余额分解 API——GET /commercial/account 增广

**Files:**
- Modify: `internal/modules/commercial/service/commercial/benefits.go`（`BatchView` 增 `Source/GrantedAt`；`BenefitsStatus` 增 `HeldMicro/RefundLockedMicro/ProjectedAt`；`refreshAndCollect`/`EnsureBenefits` 填充——从 `BudgetStore` 新读方法取 held/locked）
- Modify: `internal/modules/commercial/repository/commercial/budget_account.go`（新增只读 `AccountHolds(ctx, tenantID) (held, refundLocked int64, err)`——行缺失返回 0,0,nil）
- Modify: `internal/handler/commercial.go:635-665`（`benefitsWire` credits 增广）
- Test: `internal/handler/commercial_purchase_test.go` 同目录新增 `internal/handler/commercial_account_breakdown_test.go`、`service/commercial/benefits_test.go`

**Interfaces:**
- Consumes: Task 1 `Source/GrantedAt`；Task 3 的 budget 装配（`NewBenefitsService` 第 5 参 + fx Provide，held/locked 经 `AccountHolds` 读）；既有 closed-token 纪律（`commercial.go:569-578` 注释）。
- Produces（wire 契约，数字字符串惯例）：
  ```json
  "credits": {
    "balance_micro": "…", "held_micro": "…", "refund_locked_micro": "…",
    "available_micro": "…", "projected_at": "RFC3339",
    "batches": [{"source":"monthly|topup","period":"2026-09","granted_at":"RFC3339",
                 "balance_micro":"…","expires_at":"RFC3339"}]
  }
  ```
  `available_micro = balance_micro − held_micro − refund_locked_micro`（可为负数如实显示——超占即事实）；`projected_at` = benefits 投影行的 `projected_at`（对账年龄）。pending 时整个 benefits 缺席（既有行为）。

- [ ] **Step 1: 写失败测试（handler 层，`internal/handler/commercial_account_breakdown_test.go` 新文件；装配沿 `commercial_purchase_test.go` 的 sqlite+fake 适配器+gin 模式，租户注入沿 `commercialTenantScope` 既有测试法）**

```go
// TestAccountCreditsBreakdownArithmetic: fake 适配器（种月度钱包 1_000_000
// micro）+ sqlite 预算行 held_micro=200_000 → GET /commercial/account 响应体：
// credits.held_micro=="200000"、refund_locked_micro=="0"、available_micro=="800000"、
// balance_micro=="1000000"、batches[0].source=="monthly"、projected_at 非空。
func TestAccountCreditsBreakdownArithmetic(t *testing.T) {
    db := /* sqlite + AutoMigrate（budget 五表 + benefits EnsureSchema），沿既有 handler 测试 */
    fake := commercialplatform.NewFakeAdapter()
    benefits, err := commercialsvc.NewBenefitsService(db, accounts, plans, fake)
    // …fake 种订阅+月度钱包（fake.go 既有 knob）…
    db.Exec(`INSERT INTO commercial_budget_accounts
        (tenant_id, verified_micro, unreflected_micro, held_micro, refund_locked_micro, watermark, version, verified_until)
        VALUES (?, 0, 0, 200000, 0, 'w1', 1, ?)`, tenantID, time.Now().Add(time.Hour)) // 参数绑定
    h := NewCommercialHandler(db)
    h.benefits = benefits // 同包直挂（容器外的既有测试法）
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID) // commercial_purchase_test.go:119 同款
    ctx = context.WithValue(ctx, types.UserIDContextKey, "user-"+strconv.FormatUint(tenantID, 10))
    c.Request = httptest.NewRequest(http.MethodGet, "/commercial/account", nil).WithContext(ctx)
    h.AccountStatus(c)
    var out struct{ Data struct{ Benefits struct{ Credits map[string]any } } }
    json.Unmarshal(w.Body.Bytes(), &out)
    if out.Data.Benefits.Credits["held_micro"] != "200000" ||
       out.Data.Benefits.Credits["available_micro"] != "800000" {
        t.Fatalf("breakdown arithmetic: %+v", out.Data.Benefits.Credits)
    }
}

// TestBenefitsWireNoProviderVocabulary: 同一响应 JSON 不含 "lago"、"http"、
// 钱包名/lago_id 子串（closed-token 纪律回归）。
```

Run: `go test ./internal/handler/ -run TestAccountCreditsBreakdownArithmetic -count=1` → FAIL（字段缺失）→ 实现三层（store 读 → service 填充 → wire 投影）→ PASS。

- [ ] **Step 2: 服务层过期语义回归**——`benefits_test.go` 增 `TestBreakdownHoldsSurviveMissingAccountRow`（无 budget 行 → held/locked = 0，available = balance）。

Run: `go test ./internal/modules/commercial/service/commercial/ -run TestBreakdownHolds -count=1` → FAIL → PASS。

- [ ] **Step 3: 回归 + Commit**

Run: `go test ./internal/handler/ ./internal/modules/commercial/... -count=1 && make check-backend-architecture`
Expected: PASS

```bash
git add internal/handler/ internal/modules/commercial/
git commit -m "issue-72(#86): (task4) account credits breakdown: held/refund-locked/available/projected-at"
```

### Task 5: 契约、api-client 与账单页余额分解卡

**Files:**
- Modify: `packages/contracts/src/commercial.ts`（类型 + `parseCommercialAccountCredits`；并更新 L9-13 既有注释「available/held/refund_locked are served by no endpoint」——Task 4 落地后该句失真，改为指向 `GET /api/v1/commercial/account` 的 benefits.credits 分解）
- Modify: `packages/api-client/src/commercial.ts`（`account(signal?)` 方法）
- Modify: `apps/web/src/commercial/BillingPage.tsx`（余额分解卡）+ `apps/web/src/commercial/BillingPage.test.ts`
- Test: `packages/contracts/src/commercial.test.ts`（若在 test:shared 套件内；否则随 BillingPage.test.ts 覆盖 parser）

**Interfaces:**
- Consumes: Task 4 wire 契约；既有 `unwrap`/`ApiError` 模式（`api-client/src/commercial.ts:20-40`）；`@weknora/ui` 的 `Card/Status`。
- Produces:
  ```ts
  export interface CreditBatchView { source:'monthly'|'topup'; period:string; granted_at:string; balance_micro:string; expires_at:string }
  export interface CommercialAccountCredits {
    balance_micro:string; held_micro:string; refund_locked_micro:string;
    available_micro:string; projected_at:string; batches:CreditBatchView[];
  }
  export function parseCommercialAccountCredits(value:unknown):CommercialAccountCredits; // digit-string 校验，非法抛 Error
  // api-client: async account(signal?:AbortSignal): Promise<CommercialAccountCredits | null>
  //   —— GET /api/v1/commercial/account；data.benefits.credits 缺席（pending）→ null
  ```

- [ ] **Step 1: 写失败测试**（`BillingPage.test.ts`，沿用 loader 纯函数模式）

```ts
test('loadCommercialAccount maps breakdown and pending degradation', async () => {
  const ok = { commercial: { account: async () => ({
    balance_micro: '1000000', held_micro: '200000', refund_locked_micro: '0',
    available_micro: '800000', projected_at: '2026-09-28T00:00:00Z',
    batches: [{ source: 'monthly', period: '2026-09', granted_at: '2026-09-01T00:00:00Z',
                balance_micro: '1000000', expires_at: '2026-10-01T00:00:00Z' }],
  }) } };
  const st = await loadCommercialAccount(ok.commercial);
  assert.equal(st.status, 'success');
  if (st.status === 'success') assert.equal(st.credits.available_micro, '800000');
  const pending = { commercial: { account: async () => null } };
  const pd = await loadCommercialAccount(pending.commercial);
  assert.equal(pd.status, 'success'); // credits: null —— 卡片隐藏而非报错
});
test('parseCommercialAccountCredits rejects malformed digit strings', () => { /* 非法 balance_micro 抛错 */ });
```

Run: `pnpm test:web -- --test-name-pattern loadCommercialAccount`（或 `cd apps/web && node --import tsx --test src/commercial/BillingPage.test.ts`）
Expected: FAIL（`loadCommercialAccount` 不存在）

- [ ] **Step 2: 实现契约 parser + api-client 方法**；Run 同上 → PASS（contracts 单测若独立文件则跑 `pnpm test:shared`）。

- [ ] **Step 3: BillingPage 余额分解卡**——新增 state `creditsState`（加载/降级同 usage 卡模式）；渲染 `data-testid="billing-credits-breakdown"`：总余额/预占/退款锁定/可用 四行 + 批次表（来源〔套餐月度/充值〕、批次/发放日、到期、余额），到期近 30 天的批次行加 `data-testid="batch-expiring"`；金额以 micro/10⁶ 显示两位小数（纯展示换算，不在契约层做）。

Run: `cd apps/web && node --import tsx --test src/commercial/BillingPage.test.ts && pnpm typecheck:web`
Expected: PASS / 0 error

- [ ] **Step 4: Commit**

```bash
git add packages/contracts/src/commercial.ts packages/api-client/src/commercial.ts apps/web/src/commercial/
git commit -m "issue-72(#86): (task5) billing page credits breakdown (monthly/topup batches, held, refund-locked, available)"
```

### Task 6: 真实栈验证、集成测试与文档

**Files:**
- Create: `internal/modules/commercial/commercialplatform/lago_credits_order_integration_test.go`（`//go:build lago_integration`，env 门控 `LAGO_INTEGRATION_BASE_URL/API_KEY`，沿 `lago_benefits_integration_test.go:1-40` 模式）
- Create: `docs/testing/lago/credits-order-acceptance.md`（验收记录，引用 `docs/testing/craft/web-acceptance.md` 惯例）
- Modify: `docs/plans/issue-72-ledger-86.md`（执行账本收口）

**Interfaces:**
- Consumes: Task 1-5 全部产出；真实 Lago 栈（deploy/lago，端口改 48897/48898）；worktree 后端+前端。
- Produces: 真实环境证据（截图 + API 抓取落 `docs/migrations/lago/t14-credits-order/evidence/` 或本 Issue 证据目录——沿用 `deploy/lago/evidence/` 命名惯例）。

- [ ] **Step 1: 写集成测试**（单测试多阶段，模式沿既有唯一集成测试 `TestLagoBasePlanIntegration`（`lago_benefits_integration_test.go:148`，单测试多阶段先例））：
  阶段 a：经适配器 `EnsureBenefits` 全链（月度钱包创建）→ `GET /api/v1/wallets/:id` 断言 `priority == 1`（初值编码落线）；
  阶段 b：直接以 Lago API 创建充值形状钱包（`POST /api/v1/wallets`，无 period 元数据、`priority=2`、`expiration_at=+12 月`、granted_credits>0，幂等锚 metadata 复用既有键）→ benefits 快照列出 `source=topup`、余额合计含它；
  阶段 c：**新租户**先种老化充值钱包（`expiration_at` 设在当月期末之前，`priority=2`）再走 `EnsureBenefits` → 断言其月度钱包创建初值 `priority == 3`（让位）；
  阶段 d（**权威重排在真实运行时成立——spec L132 的 runtime verification**）：该租户再种一个新充值钱包（`expiration_at`=+6 月，`priority=2`），形成 A(老化,2)/M(月度,3)/B(新,2) 三类共存 → 提交 `rebalance_credits_order` → 逐钱包 `GET /api/v1/wallets/:id` 断言 priority 变为 **A=1、M=2、B=3**（PUT 真实生效且序 == 到期序）；再提交一次 → 断言零变更（幂等零写）。消费序由 priority 决定已由 t03 E2 在同版本运行时实证（`priority ASC, created_at ASC`），此处验「重排把 priority 序摆对」即闭环。
  **BLOCKED gate**：若阶段 d 实测 PUT 被 v1.53 拒绝（422/403 或 priority 不变），该点按 spec L132 属 BLOCKED——停止实现、记录证据、升级回设计，不得静默降级为「接受偏差」。
  安全红线落线：测试仅经适配器既有 `do()`（host 校验 + Bearer env key）；不落任何密钥字面量。

Run（无栈时）: `go test -tags lago_integration ./internal/modules/commercial/commercialplatform/ -run TestLagoCreditsOrder -count=1` → skip（blocked-env 记录）；有栈: PASS。

- [ ] **Step 2: 真实流程端到端验证**（见下「真实流程验证方案」整节，产出截图与 curl 抓取）。
- [ ] **Step 3: 文档 + Ledger 收口**（验收命令、证据路径、known-limits：跨月时间推进受栈时钟能力限制时的边界记录）。

Run: `make test && pnpm test:shared && pnpm typecheck:web && pnpm typecheck:shared`
Expected: PASS

```bash
git add internal/modules/commercial/commercialplatform/ docs/
git commit -m "issue-72(#86): (task6) real-stack credits-order evidence + acceptance docs"
```

---

## 真实流程验证方案

**环境**（全部 127.0.0.1，避开 :5272/:5273 与已被 `weknora-lago-82r5` 占用的 :48889/:48890）：

| 组件 | 来源 | 端口 | 说明 |
|---|---|---|---|
| Lago 栈 | `deploy/lago`（独立 .env：`COMPOSE_PROJECT_NAME=weknora-lago-86v`、`LAGO_API_PORT=48897`、`LAGO_FRONT_PORT=48898`） | 48897/48898 | `./deploy/lago/lago.sh init` 后手改 .env 端口再 `up`；首启 `.env` 追加 `LAGO_CREATE_ORG=true` + `LAGO_ORG_USER_EMAIL/PASSWORD/NAME/LAGO_ORG_API_KEY`（seed 值，仅落 .env） |
| WeKnora 后端 | worktree `./scripts/dev.sh app`（或 craft-stack 式：`go build ./cmd/server` + 独立 `config/config.yaml` 副本 sed 端口，`apps/web/e2e/craft-stack.sh:236-262` 惯例） | 48086 | env：`WEKNORA_COMMERCIAL_PLATFORM_PROVIDER=lago`、`WEKNORA_COMMERCIAL_PLATFORM_URL=http://127.0.0.1:48897`、`WEKNORA_COMMERCIAL_PLATFORM_API_KEY`（source 自 deploy/lago/.env 的 `LAGO_ORG_API_KEY`）、`WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true`；`DB_DRIVER=postgres` 复用宿主 WeKnora-postgres-dev:5432/redis:6379；worktree 内先 `cp .env.example .env` |
| 前端 | `pnpm --filter @weknora/web dev`（`--port 5186`，`/api` 代理到 48086：`VITE_DEV_PROXY_TARGET=http://127.0.0.1:48086`，`apps/web/vite.config.ts:29-44`） | 5186 | |
| 浏览器 | Playwright MCP（browser_navigate/snapshot/take_screenshot）headless | — | 截图落 worktree `docs/migrations/lago/t14-credits-order/evidence/*.png` |

**种子数据**：注册/登录一个新空间（注册流即建租户）→ 首次打开账单页触发 lazy ensure（customer+subscription+月度 grant 1 credit）。充值批次：#85 未合入时段用 Lago API 直接种「充值形状」钱包（`granted_credits "5"`、`rate_amount "1"`、`expiration_at` = now+12 月、metadata 仅 `weknora_tenant`、`priority 2`）——这正是 #85 到账命令将创建的对象形状（ADR-0012 修订语义），对账面与消费序等效。

**浏览器操作序列（Playwright，页面 URL `http://127.0.0.1:5186/platform/billing`）**：
1. 登录 → 进入账单页 → snapshot 断言 `data-testid="billing-credits-breakdown"` 存在：总余额=1.00（月度）、预占=0.00、退款锁定=0.00、可用=1.00；批次表一行 `套餐月度`、到期=本月末（`expires_at` RFC3339 值等于下月 1 日 00:00Z）→ 截图 `01-monthly-only.png`。
2. 种充值钱包（上表形状）→ 页面 Reload → 断言：总余额=6.00；批次表两行——充值行 `到期=+12 个月`、余额 5.00，月度行不变 → 截图 `02-with-topup.png`。
3. **对账一致（AC5）**：`curl -s -H "Authorization: Bearer $KEY" http://127.0.0.1:48897/api/v1/customers/weknora-tenant-<id>/wallets` → 断言 Σ balance_cents × 10⁴ == 页面 balance_micro、每钱包 expiration_at/priority 与页面批次一一对应（priority 1/2）。脚本 `docs/migrations/lago/t14-credits-order/reconcile.py`（读两源、断言相等、输出 PASS 行，入证据）。
4. **priority 编码与混合重排（AC1/AC2 真实运行时）**：同 curl 断言月度钱包 `priority == 1`、充值钱包 `priority == 2`（初值）；再为**第二租户**种「到期早于本月期末」的老化充值钱包（priority 2）→ 首次账单访问后 curl 断言其月度钱包 `priority == 3`（让位初值）；**混合场景**：该租户再种新充值钱包（+6 月，priority 2）→ 触发一次账单访问（刷新链提交 rebalance）→ curl 断言三钱包 priority 为 老化=1、月度=2、新充值=3（权威重排后序 == 到期序——r1 审查 High 判例的真实栈闭环）→ 截图 `03-yield-rebalance.png`（Lago front :48898 该 customer wallets 视图）。
5. **可用扣减口径（AC 预占面展示）**：向 `commercial_budget_accounts` 插入 held_micro=200_000 行（psql 参数化 INSERT，仅验证展示）→ Reload → 可用=6.00−0.20−0.00=5.80 → 截图 `04-held.png`，随后删除该行。
6. **月度不结转**：受栈时钟能力限制（api-clock 为真实小时钟，无冻结/快进 env——`docker inspect` 实测无 CLOCK 变量），真时间跨月不可等；边界处理：(a) API 断言月度钱包 `expiration_at` = 期末、充值钱包 = +12 月（对象语义即「月度到期/充值跨月保留」的权威表达）；(b) 注册表 overlay 的过期归零由 `TestExpiredBatchSurfacesZero`（既有）+ Task 3 `TestSyncLotsNeverResurrectsExpiredBatch` 覆盖；(c) 在验收文档记录该边界（栈时钟推进待 #87/#88 admission 波次的真实消费联验时一并补）。

**通过判据**：上述 1-5 全部断言 PASS + 截图四张落证据目录 + reconcile.py 输出 `RECONCILE PASS` + `go test ./... (commercial 相关包)`、`pnpm test:web`、`pnpm typecheck:web` 全绿。凡断言失败：修复后重跑，不得放宽断言。

**凭据纪律**：所有 key 只经 `source deploy/lago/.env`（或 read + export）注入环境；截图/抓取/文档中不得出现 `sk_live/sk_test` 或 API key 字面量（截图前打码或裁剪 header 区）。

---

## 验收标准 → Task → 测试 追踪矩阵

| Issue #86 验收标准 | 实现任务 | 证明测试/证据 |
|---|---|---|
| 套餐月额度不结转（期末到期归零） | 既有 #80 短 TTL 钱包；Task 2 priority 编码不改变到期语义；Task 3 同步不复活 | `TestExpiredBatchSurfacesZero`（既有，`benefits_test.go:488`）；`TestSyncLotsNeverResurrectsExpiredBatch`（Task 3）；真实栈验证 §6 |
| 充值额度跨月保留至十二个月到期 | Task 1（topup 批次入列）+ #85 到账命令（外部） | `TestLagoBenefitsSnapshotListsTopUpBatch`（Task 1）；真实栈验证 §2/§6(a)（expires_at=+12 月断言） |
| 统一按最早到期、同到期最早发放消费 | Task 2（初值编码 + WalletRank 权威重排）+ Task 3（lot tie-break） | `TestWalletRankMixedFamilies`（混合场景判例）、`TestWalletRankSameExpiryEarliestGrant`、`TestMonthlyWalletPriorityYieldsToAgingTopUp`、`TestLagoGrantWalletCarriesEncodedPriority`、`TestLagoRebalancePutsMixedFamiliesInExpiryOrder`、`TestLagoRebalanceSkipsAlignedWallets`、`TestMonthlyGrantEncodesYieldPriority`、`TestRefreshRebalancesMixedFamilies`、`TestReserveSameExpiryPicksEarliestIssuedLot`；`TestLagoCreditsOrder` 阶段 a-d（Task 6 集成，含真实栈重排）；真实栈验证 §4 |
| 并发消费与到期任务不重复消费、不错误恢复批次 | Task 3（SyncLots CAS 语义 + 并发测试） | `TestSyncLotsConcurrentWithReserveNoOverAllocation`（PG）/`…Sqlite`；既有 `TestBudgetReserveConcurrent*` 回归 |
| 余额页面与 Lago traceable state 对账一致 | Task 4（分解 API）+ Task 5（页面）+ Task 6（对账脚本） | `TestAccountCreditsBreakdownArithmetic`、`TestBenefitsWireNoProviderVocabulary`、`loadCommercialAccount` 测试；reconcile.py PASS + 截图 01-04 |

**范围声明（void/refund 维度）**：spec 验收矩阵 8（L221）含 "concurrent use, void, and refund"——本 Issue 票面四条验收不含 void/refund，计划不实现之：充值退款三段式（退款锁定→渠道退款→Lago 撤回）属 #95（W9 充值退款），套餐退款属 #96、微信退款 #97；#86 的边界是「消费顺序 + 到期安全 + 余额分解对账」。#105 切换 gate 所需的矩阵 8 全绿由 #95/#96/#97 闭合，本计划的并发维度（concurrent use）由 Task 3 PG/sqlite 并发测试覆盖。

## 文档与提交步骤（收口）

1. 每 Task 一个 commit（信息见各 Step）；全部合入后 `docs/plans/issue-72-ledger-86.md` 记录执行证据（命令+输出摘要+截图路径）。
2. `docs/testing/lago/credits-order-acceptance.md` 写入真实栈验收记录（环境、端口、种子、断言结果、known-limits）。
3. 最终门禁：`make test && make lint && make check-backend-architecture && pnpm test:shared && pnpm test:web && pnpm typecheck:web && pnpm typecheck:shared`。
4. 合回集成分支前确认 #85 已合入且 Task 1/3 的对齐步骤已执行（差异记录进 Ledger）。
