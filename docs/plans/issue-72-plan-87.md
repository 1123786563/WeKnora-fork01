# 预写基线（待实现阶段复核定稿）

# [Lago 15] 收费调用前原子预占 Task Budget 与空间 Credits — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Issue:** https://github.com/1123786563/WeKnora-fork01/issues/87（父 Issue #72；Blocking #88/#89）

**Goal:** 让每一次真实收费外呼之前的保守上界预占，其**价格投影**来自同一不可变 Lago 定价版本的只读投影、其**空间余额**来自 Lago Wallet 权威保守投影——缺失/过期/不可计算一律 fail closed，重试不重复预占。

**Architecture:** 本地 U02 原子预占核（CAS guarded UPDATE：账户面 + 任务面 + lot 面，任一失败整体回滚）保持不动；#87 在冻结的 `CommercialPlatform` seam 上做**加法**扩展：新增 `SnapshotKindPricing` 快照（Lago 计划读回 → 维度费率的只读投影）与本地 `commercial_price_projections` 投影表；benefits 刷新链在 `SyncLots` 之后把权威钱包 Σ 余额折入 `commercial_budget_accounts` 的 verified 面（watermark=CAS、verified_until=新鲜度 TTL）；派发路径上删除 `remote-v1` 兜底，价格版本缺失即拒绝派发。

**Tech Stack:** Go（gin + gorm + dig）、PostgreSQL/SQLite 双方言、React+Vite（apps/web）、Lago pinned v1.53.0（deploy/lago compose）、pnpm 共享包（packages/contracts、api-client）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（§Credits and Task admission L129-137；§验收矩阵 12/13/14 L225-227）；`docs/adr/0012-lago-as-commercial-billing-authority.md`（含 2026-09-23 75-a2 修订）；`docs/adr/0014*`（seam 加法规则）；`docs/migrations/lago/t04-pricing-group/README.md:79-91`（移交 #87 的 422/唯一性实测契约）；`docs/migrations/lago/t07-plan-version-publish/`（计划读回契约）；`CONTEXT.md`（额度预占词条）。

## 前置接口基线（本计划推断依据；均已在集成分支 codex/issue-72-lago 实读核实）

- #86 已合入（3e4de6e2a）：benefits 快照批次携带 `Source/GrantedAt/WalletRef`（`internal/modules/commercial/subscription_command.go:327-344`）；`BudgetStore.SyncLots` 把权威批次投影为本地 lot（`internal/modules/commercial/repository/commercial/budget.go:186-256`）；benefits 刷新链尾 `SyncLots` + `rebalance_credits_order`（`internal/modules/commercial/service/commercial/benefits.go:649-685`）；余额分解已暴露于 `GET /api/v1/commercial/account` benefits.credits（`internal/handler/commercial.go:646-684`）并由 BillingPage 渲染（`apps/web/src/commercial/BillingPage.tsx:184-226`）；契约 `parseCommercialAccountCredits`（`packages/contracts/src/commercial.ts:154-194`）。**调查简报中「AC2 缺一半」「PG fixture 漂移」两条 gap 已被 #86 关闭**（owner 列经 gorm Migrator 补齐，`budget_pg_test.go:64-71`）。
- #79 已交付 `CommandKindPublishPlanVersion`（`internal/modules/commercial/plan_command.go:28`）与 `PublishPlanVersionPayload.Charges []PlanCharge`（`plan_command.go:96-108`，闭集 fixed_unit/package）；Lago 适配器发布链 resolve 维度→billable metric（按 code 寻址，`commercialplatform/lago.go:512-538`）→ POST /api/v1/plans → 422 读回比对（`lago.go:572-610`）。
- #78 已交付 `SnapshotKindAccount`/`ensure_customer`；#80/#82 交付 `SnapshotKindBenefits` 与购买面；#81/#84 交付 quote/order/payment 面。
- 本地预占核已实现且有测试：`ExecutionGateService.Begin` 先 `Reserve` 后 `MarkDispatched`（`service/commercial/execution.go:109-119`）；Reserve 单事务三面 CAS（`repository/commercial/budget_reservation.go:75-190`）；幂等重放（`budget_reservation.go:51-59`）；rate 缺失 fail closed（`execution.go:80-82`）。
- 集成分支测试基线（本计划编写当日实跑）：`go test ./internal/modules/commercial/... -count=1` → **7 包全 ok**（commercialplatform 73.4s）。

## Global Constraints

- **Seam 冻结纪律**（ADR-0014，`platform.go:14-19`）：`CommercialPlatform` 只允许加法——新 SnapshotKind 常量、新 typed payload、新可选字段；禁止 per-object wrapper 方法；provider 词汇（URL/status/body）不得越过 seam。
- **Fail closed**：价格投影缺失、过期（diverged）、不可解析、或维度无上界费率 ⇒ 拒绝派发，绝不零计价放行（spec L133「Missing, stale, or non-bounded pricing blocks dispatch」）。
- **保守上界**：上界 = (MaxIn+MaxOut tokens 或维度单位上限) × 版本费率，向上取整；`FreeUnits` 免费额度**不得**折减上界（保守方向）。
- **货币换算冻结**：1 Credit = 1 CNY（`TopUpCredits`，`service/commercial/fulfillment.go:85-87`；钱包 `rate_amount: "1"`，`commercialplatform/lago.go:991`）⇒ `RateMicro = AmountFen × 10_000`（`CentsToMicro` 同比，`subscription_command.go:286-290`）。
- **SQL 纪律**：所有查询参数绑定（既有 CAS 语句已如此）；新增 SQL 一律 `?` 占位，禁止拼接/format/f-string。
- **出站请求纪律**：仅 http/https；发请求前校验 host，拒绝 localhost/环回/私有/保留地址（lago 适配器已有 `OutboundAllowLoopback` 测试豁免机制可循，`lago_benefits_integration_test.go`）。
- **凭据纪律**：`LAGO_API_KEY` 等只从 `deploy/lago/.env` source 注入环境变量；任何源码/示例/测试/证据文档不得含可用凭据字面量。
- **端口约束**：验证栈端口避开 :5272/:5273（被外部验证环境占用）。
- **Go/TS 门禁**：`make lint`、`make check-backend-architecture`、`pnpm typecheck:web`/`typecheck:shared` 零新增违规（基线既有违觋试对照 stash 法）。

## Review Focus

1. **发布后本地定义被改**（写入模型 ≠ 定价真相）：admin 修改草稿/重发同版本 ⇒ 读回投影 diverged ⇒ 派发 fail closed。→ Task 2 `TestPricingProjectionDivergedFailsClosed`。
2. **Lago 侧计划被运维手改**（权威被编辑）：刷新时快照费率 ≠ 已存投影 ⇒ 标记 diverged 而非静默覆盖。→ Task 2 `TestPricingRefreshMarksDivergence`（同 Task 2 测试）。
3. **余额折入与并发预占竞态**（折入时正有 Reserve 持有）：verified 不得越过权威 Σ、watermark CAS 强制重读。→ Task 4 `TestBudgetPGFoldCompetesWithReservation`（PG 并发证据）。
4. **快照身份不稳定导致 watermark 倒退/重放歧义**：同内容快照必须同 watermark（幂等），不同内容必须不同 watermark。→ Task 4 `TestBalanceFoldWatermarkIdentity`。
5. **BYOK 与平台资金混淆**：BYOK 豁免平台额度但保留价格绑定；平台资金缺价格版本必须拒。→ Task 3 `TestPlatformBindingWithoutPriceVersionRejects` + 既有 BYOK 用例回归。

---

### Task 1: seam 加法 — `SnapshotKindPricing` 定价只读快照（Lago 计划读回 → 维度费率）

**Files:**
- Create: `internal/modules/commercial/pricing_snapshot.go`（域类型 + 常量 + 校验）
- Create: `internal/modules/commercial/pricing_snapshot_test.go`
- Modify: `internal/modules/commercial/platform.go:88-91`（`SnapshotQuery` 增加可选字段 `PlanCode string`——冻结注释明示允许「new optional struct fields」；`Snapshot` 增加可选 section `Pricing *PricingSnapshot`）
- Modify: `internal/modules/commercial/commercialplatform/lago.go`（ReadSnapshot 分派 + `readPricingSnapshot`）
- Modify: `internal/modules/commercial/commercialplatform/fake.go`（fake 适配器同构实现）
- Modify: `internal/modules/commercial/commercialplatform/contract_test.go`（双适配器契约：unsupported→fake/lago 行为一致）

**Interfaces:**
- Consumes: `CommercialPlatform.ReadSnapshot(ctx, query) (Snapshot, error)`（冻结 seam）；`PublishPlanVersionPayload.PlanCode/DeterministicPlanCode`（#79，`plan_command.go:99-105`）；Lago REST `GET /api/v1/plans/{code}`（t07 已实证可用）。
- Produces（后续任务依赖的精确签名）:
```go
// pricing_snapshot.go
const SnapshotKindPricing SnapshotKind = "pricing" // W9 additive (#87, ADR-0014 additive-kind rule)

// PricingChargeSnapshot: one usage charge of the plan version, product
// vocabulary only (fen + dimension code + closed charge-model set).
type PricingChargeSnapshot struct {
    Dimension    string // WeKnora dimension code (model|connector|audio_seconds|...)
    Model        string // closed: ChargeModelFixedUnit | ChargeModelPackage
    AmountFen    int64  // per unit (fixed_unit) or per package (package)
    PackageUnits int64  // package model only; 1 for fixed_unit
    FreeUnits    int64  // advisory; NEVER reduces the admission bound
}

type PricingSnapshot struct {
    PlanCode  string                // seam-internal deterministic plan code
    Charges   []PricingChargeSnapshot
    CheckedAt time.Time
}

// SnapshotQuery 新增可选字段: PlanCode string（kind==pricing 时必填，其余 kind 忽略）
// Snapshot 新增可选 section: Pricing *PricingSnapshot（nil unless kind==pricing）

// DimensionRates derives the fixed-point admission rates from the snapshot.
// Conservative: package model expresses RateMicro=AmountFen*10_000 per
// PackageUnits units; FreeUnits is ignored (never discounts the bound).
func (p PricingSnapshot) DimensionRates() domain.PriceVersionRates // Version=PlanCode
```
- 错误分类沿用 seam 哨兵：`ErrPlatformUnsupported`（kind 未启用/PlanCode 空）、`ErrPlatformUnreachable`、`ErrPlatformInvalidResponse`（404/费率不可解析/charge model 闭集外）。

- [ ] **Step 1: RED — 写失败测试**

```go
// pricing_snapshot_test.go
func TestPricingSnapshotDimensionRatesConservative(t *testing.T) {
    s := PricingSnapshot{PlanCode: "weknora-base-v2", CheckedAt: time.Now().UTC(), Charges: []PricingChargeSnapshot{
        {Dimension: "model", Model: ChargeModelFixedUnit, AmountFen: 2, PackageUnits: 1, FreeUnits: 1000},
        {Dimension: "connector", Model: ChargeModelPackage, AmountFen: 30, PackageUnits: 100},
    }}
    rates := s.DimensionRates()
    if err := rates.Validate(); err != nil { t.Fatalf("validate: %v", err) }
    if rates.Version != "weknora-base-v2" { t.Fatalf("version = %s", rates.Version) }
    if got := rates.Rates["model"]; got.RateMicro != 20_000 || got.Units != 1 {
        t.Fatalf("model rate = %+v (want 20000/1; FreeUnits must not discount)", got)
    }
    if got := rates.Rates["connector"]; got.RateMicro != 300_000 || got.Units != 100 {
        t.Fatalf("connector rate = %+v (want 300000 per 100 units)", got)
    }
}

func TestPricingSnapshotRejectsUnboundedModel(t *testing.T) {
    s := PricingSnapshot{PlanCode: "x", Charges: []PricingChargeSnapshot{{Dimension: "model", Model: "graduated", AmountFen: 1}}}
    if _, err := s.Charges[0].Validate(); err == nil { t.Fatal("unbounded charge model must be rejected") }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/commercial/ -run 'TestPricingSnapshot' -count=1`
Expected: FAIL（`PricingSnapshot` 未定义 / undefined: ChargeModelFixedUnit on snapshot type）

- [ ] **Step 3: GREEN — 实现 `pricing_snapshot.go` + `platform.go` 可选字段/section + 两适配器 ReadSnapshot 分派**

Lago 读回要点：复用 `GET /api/v1/plans/{code}`（`lago.go:574` 已有先例），解析 `plan.charges[]` 的 `billable_metric.code`（或 `billable_metric_code`——**待复核 R1**：v1.53 计划读回中维度反查字段；若响应只带 `billable_metric_id`，则先 `GET /api/v1/billable_metrics/{lago_id}` 反查 code，t04 lab 栈可一条 curl 验证）+ `charge_model` + `amount_cents`（fen）+ `properties/package_size`。fake 适配器以内存 plan 构造同构快照。

- [ ] **Step 4: 运行确认通过 + 契约测试**

Run: `go test ./internal/modules/commercial/ ./internal/modules/commercial/commercialplatform/ -run 'TestPricingSnapshot|TestContract' -count=1`
Expected: PASS（双适配器对 pricing kind 的 unsupported/读回行为一致）

- [ ] **Step 5: REFACTOR + 提交**

```bash
git add internal/modules/commercial/pricing_snapshot.go internal/modules/commercial/pricing_snapshot_test.go internal/modules/commercial/platform.go internal/modules/commercial/commercialplatform/
git commit -m "issue-72(#87): task1 pricing snapshot kind on the frozen seam"
```

### Task 2: 本地只读定价投影 + fail-closed RateResolver（含 diverged 检测）

**Files:**
- Create: `internal/modules/commercial/repository/commercial/pricing_projection.go`
- Create: `internal/modules/commercial/repository/commercial/pricing_projection_test.go`
- Create: `internal/modules/commercial/service/commercial/pricing_projection.go`
- Create: `internal/modules/commercial/service/commercial/pricing_projection_test.go`
- Create: `migrations/versioned/000187_commercial_price_projections.up.sql` / `.down.sql`（编号**待复核 R2**：实现时取集成分支下一个可用号；#86 用至 000186 一线，以 `ls migrations/versioned | tail` 实查为准）+ SQLite 伴随（既有双编号惯例）

**Interfaces:**
- Consumes: Task 1 的 `SnapshotKindPricing`/`PricingSnapshot.DimenstionRates()`；`RateResolver func(string) (domain.PriceVersionRates, error)`（`repository/commercial/usage.go:84`）；`PlanVersionStore` 的 `PublicationRow`（`planversion.go:26-36`，ReceiptJSON 绑定已发布身份）。
- Produces:
```go
// repository/commercial/pricing_projection.go
type PriceProjectionRow struct { // table: commercial_price_projections
    PriceVersion   string    `gorm:"primaryKey;column:price_version"` // = PlanCode
    RatesJSON      string    `gorm:"column:rates_json;not null"`
    SnapshotID     string    `gorm:"column:snapshot_id;not null"` // content fingerprint of the read-back
    Diverged       bool      `gorm:"column:diverged;not null;default:false"`
    ProjectedAt    time.Time `gorm:"column:projected_at;not null"`
}
func (s *PriceProjectionStore) EnsureSchema(ctx context.Context) error            // gorm AutoMigrate（planversion.go 先例）
func (s *PriceProjectionStore) UpsertFromSnapshot(ctx context.Context, snap domain.PricingSnapshot) error
    // append-only upsert: 同版本同 SnapshotID 幂等 no-op；同版本不同费率 ⇒ Diverged=true 且不改 RatesJSON
func (s *PriceProjectionStore) Resolve(priceVersion string) (domain.PriceVersionRates, error)
    // missing | Diverged | unparseable ⇒ ErrUsageRatesUnavailable（fail closed）；命中返回不可变费率

// service/commercial/pricing_projection.go
type PricingProjectionService struct{ /* platform + store + publications */ }
func NewPricingProjectionService(db *gorm.DB, platform domain.CommercialPlatform) (*PricingProjectionService, error)
func (s *PricingProjectionService) EnsurePricing(ctx context.Context, planCode string) error
    // ReadSnapshot(kind=pricing, PlanCode) → UpsertFromSnapshot；unreachable/invalid ⇒ 原样返回错误（调用方决定降级姿态）
func (s *PricingProjectionService) RateResolver() repocommercial.RateResolver
    // 闭包：store.Resolve；任何错误包 ErrUsageRatesUnavailable
```

- [ ] **Step 1: RED — 写失败测试**（store 层三态 + service 层 diverged 链）

```go
// pricing_projection_test.go（repository 层，共享 SQLite fixture 惯例）
func TestPricingProjectionDivergedFailsClosed(t *testing.T) {
    store := NewPriceProjectionStore(testDB(t)) // 既有 budget_test 的 sqlite fixture 惯例
    snap1 := domain.PricingSnapshot{PlanCode: "weknora-base-v2", CheckedAt: time.Now().UTC(),
        Charges: []domain.PricingChargeSnapshot{{Dimension: "model", Model: domain.ChargeModelFixedUnit, AmountFen: 2, PackageUnits: 1}}}
    if err := store.UpsertFromSnapshot(context.Background(), snap1); err != nil { t.Fatal(err) }
    if _, err := store.Resolve("weknora-base-v2"); err != nil { t.Fatalf("fresh projection must resolve: %v", err) }
    // 权威侧同版本费率被改（运维手改 Lago）：标记 diverged，读侧 fail closed
    snap2 := snap1; snap2.Charges[0].AmountFen = 9
    if err := store.UpsertFromSnapshot(context.Background(), snap2); err != nil { t.Fatal(err) }
    if _, err := store.Resolve("weknora-base-v2"); !errors.Is(err, ErrUsageRatesUnavailable) {
        t.Fatalf("diverged projection must fail closed, got %v", err)
    }
}

func TestPricingResolveMissingVersionFailsClosed(t *testing.T) {
    store := NewPriceProjectionStore(testDB(t))
    if _, err := store.Resolve("never-published"); !errors.Is(err, ErrUsageRatesUnavailable) { t.Fatalf("got %v", err) }
}

// service 层：TestPricingRefreshMarksDivergence — fake platform 两次快照费率不同
// ⇒ 第二次 EnsurePricing 后 RateResolver()(planCode) 返回 ErrUsageRatesUnavailable
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/commercial/repository/commercial/ -run 'TestPricing' -count=1`
Expected: FAIL（`NewPriceProjectionStore` 未定义）

- [ ] **Step 3: GREEN — 实现投影表 + Upsert/Resolve + service；迁移文件（PG/SQLite 双方言，DDL 与 gorm tag 同构）**

Upsert 纪律：`SELECT ... WHERE price_version = ?` → 不存在 `INSERT`（唯一键竞态读回）；存在且 `SnapshotID` 相同 ⇒ no-op；存在且费率指纹不同 ⇒ `UPDATE commercial_price_projections SET diverged = true, snapshot_id = ?, projected_at = ? WHERE price_version = ? AND diverged = false`（参数绑定）。

- [ ] **Step 4: 运行确认通过 + 全包回归**

Run: `go test ./internal/modules/commercial/repository/commercial/ ./internal/modules/commercial/service/commercial/ -run 'TestPricing' -count=1 && go test ./internal/modules/commercial/... -count=1`
Expected: PASS / 7 包 ok

- [ ] **Step 5: 提交**

```bash
git add internal/modules/commercial/ migrations/versioned/
git commit -m "issue-72(#87): task2 read-only pricing projection, fail-closed resolver, divergence guard"
```

### Task 3: 派发路径接线 — 语义模型/语音/connector 的价格版本与上界；删除 `remote-v1` 兜底

**Files:**
- Create: `internal/application/service/semantic_model_admission_pricing.go`（`SemanticModelPricingResolver` 生产实现）
- Create: `internal/application/service/semantic_model_admission_pricing_test.go`
- Modify: `internal/container/container.go:1085`（`NewSemanticModelBudgetAdapter(budget, gate, nil)` 的第三参接入真实 resolver）
- Modify: `internal/container/container.go:2592-2597`（`PriceVersionVoiceAdmission.Resolve` 接入同一 resolver）
- Modify: `internal/modules/workbench/service/workbench/remote_usage.go:98-105`（删 `remote-v1` 兜底 → 空价格版本返回 `ErrRemoteUsageInvalidRequest`）
- Modify: `internal/modules/workbench/service/workbench/admission.go:198-203`（`PlatformAdmissionPolicy()` 的 `PriceVersion: "remote-v1"` → `""`；由 database binding resolver 供给真实版本）
- Modify: `internal/modules/workbench/service/workbench/admission.go`（databaseAdmissionBindingResolver 增加每租户定价解析：benefits 投影 PlanCode → pricing 投影 connector 维度费率 → `Upper`（1 单位 connector 的保守上界，向上取整））
- Test: `internal/modules/workbench/service/workbench/remote_dispatch_test.go`（更新既有 `"remote-v1"` 用例为显式版本）、`internal/modules/workbench/service/workbench/admission_test.go`（新增）

**Interfaces:**
- Consumes: Task 2 `PricingProjectionService.RateResolver()`；`BenefitsStore.GetProjection`（`benefits.go:107-114`，`BenefitsRow.PlanCode`）；`domain.PriceVersionRates.ChargeForCall`（`usage.go:164`）；`SemanticModelPricingResolver func(context.Context, uint64, *types.Model) (SemanticModelPricing, error)`（`semantic_model_policy.go:50`）。
- Produces:
```go
// semantic_model_admission_pricing.go（本文件即 application/service 包）
// NewLagoSemanticModelPricingResolver: tenant → benefits 投影 PlanCode →
// pricing 投影费率。Funding=platform；PriceVersion=PlanCode；Rates=投影；
// TaskUpperMicro=nil（由 policy 输入计算，语义既有行为）。
// 任一环缺失 ⇒ ErrSemanticModelPricingUnavailable（fail closed，绝不回退默认费率）。
func NewLagoSemanticModelPricingResolver(benefits repocommercial.BenefitsStore, pricing *commercialsvc.PricingProjectionService) SemanticModelPricingResolver
```
- 容器接线（production 形状）：`NewSemanticModelBudgetAdapter(budget, gate, pricingSvc.RateResolver())`——第三参由 `nil` 变为真实 resolver，`semantic_model_budget.go:49-54` 既有校验（版本不匹配即拒）随之激活为生产路径。

- [ ] **Step 1: RED — 写失败测试**

```go
// semantic_model_admission_pricing_test.go
func TestAdmissionPricingMissingProjectionFailsClosed(t *testing.T) {
    // benefits 投影有 PlanCode=weknora-base-v2，pricing 投影无该版本
    resolver := NewLagoSemanticModelPricingResolver(benefitsStore, pricingSvc)
    if _, err := resolver(ctx, 10001, model); !errors.Is(err, ErrSemanticModelPricingUnavailable) {
        t.Fatalf("missing pricing projection must fail closed, got %v", err)
    }
}

func TestAdmissionPricingResolvesTenantPlanRates(t *testing.T) {
    // 预置 benefits 投影 + pricing 投影（model 维度 2 fen/单位）
    p, err := resolver(ctx, 10001, model)
    if err != nil { t.Fatal(err) }
    if p.PriceVersion != "weknora-base-v2" || p.Rates.Rates["model"].RateMicro != 20_000 { t.Fatalf("got %+v", p) }
}

// remote_usage_test.go / admission_test.go:
func TestPlatformBindingWithoutPriceVersionRejects(t *testing.T) {
    fence := agentruntime.Fence{TenantID: 1, RunID: "r", /* UsagePriceVersion 留空 */}
    if _, err := remoteRequestFromFence(fence, "cmd-1"); !errors.Is(err, ErrRemoteUsageInvalidRequest) {
        t.Fatalf("empty price version must reject, got %v", err) // 不再回退 remote-v1
    }
}
```
同时更新 `remote_dispatch_test.go` 既有 4 处 `"remote-v1"`（:78/:303/:333/:362）为显式测试版本（如 `"weknora-base-v2"`）并预置投影——它们从「兜底可用」转为「显式解析」。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/service/ -run 'TestAdmissionPricing' -count=1 && go test ./internal/modules/workbench/service/workbench/ -run 'TestPlatformBinding|TestRemote' -count=1`
Expected: FAIL（resolver 未定义 / 空版本仍回退）

- [ ] **Step 3: GREEN — 实现 resolver + 删两处兜底 + binding resolver 增定价解析 + 容器接线**

binding resolver 上界计算：`Upper = ceil(ChargeRat("connector", 1))`（`big.Rat` 向上取整一单位——保守方向）；解析失败 ⇒ binding 保留空 PriceVersion ⇒ 派发前 `remoteRequestFromFence` 拒绝（fail closed 一致）。

- [ ] **Step 4: 运行确认通过 + 模块级回归**

Run: `go test ./internal/application/service/ ./internal/modules/workbench/service/workbench/ ./internal/modules/commercial/... -count=1`
Expected: PASS（workbench 包若有既有基线 FAIL，stash 对照确认零新增）

- [ ] **Step 5: 提交**

```bash
git add internal/application/service/ internal/modules/workbench/ internal/container/ migrations/versioned/
git commit -m "issue-72(#87): task3 wire admission pricing, remove remote-v1 fallback (fail closed)"
```

### Task 4: 权威钱包 Σ 余额折入 verified 面（保守空间余额的 Lago 权威化）

**Files:**
- Modify: `internal/modules/commercial/repository/commercial/budget_account.go`（新增 `FoldAuthoritativeBalance` + `FreshUntil`；复用 `casRetry` 纪律）
- Modify: `internal/modules/commercial/repository/commercial/budget_account_test.go`
- Modify: `internal/modules/commercial/service/commercial/benefits.go:649-665`（SyncLots 之后追加折入，同事务姿态：失败 Warn 不阻塞账单读）
- Modify: `internal/modules/commercial/service/commercial/benefits_test.go`
- Test: `internal/modules/commercial/repository/commercial/budget_pg_test.go`（并发竞态 PG 证据）

**Interfaces:**
- Consumes: benefits 快照 `b.Batches`（含过期 overlay 后余额——折入只取**未到期**批次 Σ，与 `BenefitsStatus` 视图同源）；`BudgetAccountRow`（`budget.go:52-61`）。
- Produces:
```go
// budget_account.go
// FoldAuthoritativeBalance folds the authoritative wallet sum into the
// account row: creates the row on first fold, CAS-advances the watermark
// (content-fingerprint identity: same snapshot ⇒ same watermark ⇒ idempotent
// no-op; different content ⇒ strictly different watermark), sets
// verified_micro=Σ(unexpired batches) and verified_until=now+ttl.
// A LOWER authority sum is folded honestly (consumption reflected): the
// guard then rejects until settlement confirms — conservative, never fabricated.
func (s *BudgetStore) FoldAuthoritativeBalance(ctx context.Context, tenantID uint64, verified domain.Credits, watermark string, until time.Time) error
func (s *BudgetStore) FreshUntil(ctx context.Context, tenantID uint64) (time.Time, bool) // missing row ⇒ (zero,false)
```
watermark 生成（服务侧，benefits.go）：`fmt.Sprintf("%016x:%s", checkedAt.UnixNano(), fingerprint16(batches))`——时间前缀保序（`watermark < acct.Watermark` 拒绝的既有单调性，`budget_account.go:31`），指纹保内容唯一。

- [ ] **Step 1: RED — 写失败测试**

```go
// budget_account_test.go
func TestBalanceFoldWatermarkIdentity(t *testing.T) {
    db := sqliteFixture(t) // 既有 fixture
    s := NewBudgetStore(db)
    wm1 := fmt.Sprintf("%016x:%s", now.UnixNano(), "aaaa")
    if err := s.FoldAuthoritativeBalance(ctx, 1, 5_000_000, wm1, now.Add(time.Hour)); err != nil { t.Fatal(err) }
    if err := s.FoldAuthoritativeBalance(ctx, 1, 5_000_000, wm1, now.Add(time.Hour)); err != nil { t.Fatal(err) } // 幂等重放
    until, ok := s.FreshUntil(ctx, 1)
    if !ok || !until.After(now) { t.Fatalf("fresh=%v ok=%v", until, ok) }
    wmOld := fmt.Sprintf("%016x:%s", now.Add(-time.Minute).UnixNano(), "bbbb")
    if err := s.FoldAuthoritativeBalance(ctx, 1, 9_000_000, wmOld, now.Add(time.Hour)); !errors.Is(err, ErrStaleWatermark) {
        t.Fatalf("older watermark must be rejected, got %v", err)
    }
}

func TestFoldLowersVerifiedHonestly(t *testing.T) {
    // fold 9M → fold 2M（权威已消费）⇒ verified=2M；Reserve(upper=5M) 拒 ErrInsufficientBudget
}

// benefits_test.go:
func TestBenefitsRefreshFoldsAccountFace(t *testing.T) {
    // fake platform 带两批次（1.00 + 5.00）⇒ 刷新后 FreshUntil 在 TTL 内、
    // Reserve 可用额度 = 6M - 0；批次数为 0 的快照 ⇒ verified 折为 0（诚实归零，不复活）
}

// budget_pg_test.go（build tag commercial_integration，SAAS_TEST_PG_DSN）：
func TestBudgetPGFoldCompetesWithReservation(t *testing.T) {
    // N=8 goroutine 折入 vs N=8 并发 Reserve：断言 Σ(held) ≤ Σ(verified)（无超占），
    // 全部终态合法（无部分提交）；fixture 复用既有 budgetPGFixture（000116+owner 列已修）
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/commercial/repository/commercial/ -run 'TestBalanceFold|TestFoldLowers|TestBenefitsRefreshFolds' -count=1`
Expected: FAIL（方法未定义）

- [ ] **Step 3: GREEN — 实现 Fold/FreshUntil + benefits 链接线**

折入实现要点：事务内 `SELECT ... FOR UPDATE` 语义由既有 version-CAS 承担：行不存在 ⇒ `INSERT`（唯一键竞态读回后走 CAS 分支）；存在 ⇒ watermark 同值同 Σ no-op / 同值异 Σ `ErrStaleWatermark` / 新值 `UPDATE verified_micro=?, watermark=?, verified_until=?, version=version+1 WHERE tenant_id=? AND version=?`。TTL 常量 `balanceFreshnessTTL = 60 * time.Minute`（env `WEKNORA_BALANCE_FRESHNESS_TTL` 可覆盖——**默认值待复核 R3**，与刷新触发点 Task 5 联动定稿）。

- [ ] **Step 4: 运行确认通过（含 PG 并发证据）**

Run: `go test ./internal/modules/commercial/repository/commercial/ -run 'TestBalanceFold|TestFoldLowers' -count=1 && SAAS_TEST_PG_DSN=<env> go test -tags commercial_integration ./internal/modules/commercial/repository/commercial/ -run 'TestBudgetPGFoldCompetesWithReservation|TestBudgetPGConcurrentReservation|TestBudgetPGTwoTasksCompeteForAccount' -count=1 -v`
Expected: PASS（后三条需本机 PG；无 DSN 时显式记 skip 与理由，不得谎报）

- [ ] **Step 5: 提交**

```bash
git add internal/modules/commercial/
git commit -m "issue-72(#87): task4 fold authoritative wallet sum into the verified account face"
```

### Task 5: 派发前新鲜度守卫（stale ⇒ 有界刷新一次 ⇒ 仍 stale 则 fail closed）

**Files:**
- Create: `internal/modules/commercial/service/commercial/admission_freshness.go`
- Create: `internal/modules/commercial/service/commercial/admission_freshness_test.go`
- Modify: `internal/application/service/craft_budget.go:229-270`（Admit 在 `EnsureTaskBudget` 前调用守卫）
- Modify: `internal/application/service/semantic_model_capability.go:70-80`（任务首发能力签发前调用守卫）
- Test: 上述两文件对应 `_test.go`

**Interfaces:**
- Consumes: Task 4 `FreshUntil`；BenefitsService 的 lazy 刷新链（`benefits.go:240+` EnsureBenefits——**待复核 R4**：提取可单租户重入的 `RefreshProjection(ctx, tenantID)` 公共入口，避免经 HTTP handler 路径；实现时若链入口已可直调则零改动复用）。
- Produces:
```go
// admission_freshness.go
type AdmissionFreshnessGuard struct{ benefits *BenefitsService; budget *repocommercial.BudgetStore; ttl time.Duration }
func NewAdmissionFreshnessGuard(benefits *BenefitsService, budget *repocommercial.BudgetStore) *AdmissionFreshnessGuard
// EnsureFresh: fresh ⇒ nil；stale ⇒ 有界单次刷新（ctx 5s 超时）后再查；
// 仍 stale/无行 ⇒ ErrBalanceProjectionStale（派发方视同 insufficient——fail closed）。
// 刷新失败不区分 unreachable/invalid：保守拒绝，下次派发再试。
func (g *AdmissionFreshnessGuard) EnsureFresh(ctx context.Context, tenantID uint64) error

var ErrBalanceProjectionStale = errors.New("balance_projection_stale")
```
调用点语义：craft Admit / 语义首签处仅对**平台资金**任务调用（BYOK 不触发——其豁免平台额度）；守卫错误 ⇒ Admit 拒绝（任务不建立，外部零调用）。

- [ ] **Step 1: RED — 写失败测试**

```go
func TestEnsureFreshRejectsWhenProjectionStale(t *testing.T) {
    // 无 benefits 链（nil platform ⇒ 刷新必败）+ 无账户行 ⇒ ErrBalanceProjectionStale
}
func TestEnsureFreshRefreshesThenAdmits(t *testing.T) {
    // fake platform 带余额 ⇒ 首次 EnsureFresh 触发刷新并放行；二次调用直接放行（不再外呼——fake 计数断言）
}
func TestCraftAdmitBlocksOnStaleBalance(t *testing.T) {
    // stale 投影下 Admit 返回错误且 EnsureTaskBudget 未被调用（任务未建立）
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/modules/commercial/service/commercial/ ./internal/application/service/ -run 'TestEnsureFresh|TestCraftAdmitBlocks' -count=1`
Expected: FAIL

- [ ] **Step 3: GREEN — 实现守卫 + 两处调用点**

- [ ] **Step 4: 运行确认通过 + 回归**

Run: `go test ./internal/modules/commercial/... ./internal/application/service/ -count=1`
Expected: PASS（application 包如有既有基线 FAIL，stash 对照零新增）

- [ ] **Step 5: 提交**

```bash
git add internal/modules/commercial/ internal/application/service/
git commit -m "issue-72(#87): task5 admission freshness guard (bounded refresh, fail closed)"
```

### Task 6: 验收矩阵锚定测试 + AC 追踪 + 真实链路验证与证据

**Files:**
- Create: `internal/modules/commercial/execution_gate_ac_trace_test.go`（matrix 13/14 的域内锚定：`ErrAbnormalCost` ⇒ `commercial_adapter.go:104-106` halted 路径回归断言；unknown/partial observation ⇒ hold 不释放（既有 `ReconcileRemoteObservation` 语义，`remote_usage.go:71-79`）——用例收拢为 #87 的 AC 追踪面）
- Create: `docs/testing/lago/admission-reservation-acceptance.md`（真实链路证据文档，循 `docs/testing/lago/credits-order-acceptance.md` 惯例）
- Create: `docs/migrations/lago/t15-admission-pricing/README.md` + `evidence/`（复刻 #86 的 t14 目录惯例：probe 脚本 + 截图）
- Test: `apps/web/src/commercial/BillingPage.test.ts`（零改动回归跑——AC2 已由 #86 交付，本票只验证不重建）。注：TaskBudget 渲染列表在 `TaskBudget.tsx:85-87`（快照类型 :5-15）。

**Interfaces:**
- Consumes: Task 1-5 全部产出；真实栈 deploy/lago（pinned v1.53.0）；`deploy/lago/lago.sh` 运维。
- Produces: 验收证据（截图/输出文件），#88/#89 可引用的「预占受理且不重复」基线。

- [ ] **Step 1: RED — AC 锚定测试**（若既有用例已覆盖则改为引用断言，不重复造）

```go
func TestACRetryDoesNotDoubleReserve(t *testing.T) {
    // 同 (tenant,key,runID,upper) 两次 Begin ⇒ 第二次返回同 ID、账户 held 只加一次
    // （budget_reservation.go:51-59 幂等语义的 gate 层锚定）
}
func TestACUpperBoundViolationHalts(t *testing.T) {
    // Finish 结算价 > upper ⇒ ErrAbnormalCost（execution.go:139-146）；
    // commercial_adapter 路径断言 halted=true（agent/commercial_adapter.go:104-106 语义）
}
```

Run: `go test ./internal/modules/commercial/ ./internal/modules/agentruntime/agent/ -run 'TestAC' -count=1` → 先 FAIL（用例未建）→ 实现/收拢 → PASS。

- [ ] **Step 2: 全量门禁**

```bash
go test ./... 2>&1 | grep -c FAIL        # 期望 0（或与基线 stash 对照零新增）
make lint                                # 零新增（基线既有 errcheck 对照）
make check-backend-architecture          # PASS
pnpm test:web && pnpm typecheck:web && pnpm typecheck:shared   # 与基线对照零新增
```

- [ ] **Step 3: 真实链路验证（见下节方案）并落证据** → `git add docs/ && git commit -m "issue-72(#87): task6 acceptance anchoring + real-chain evidence"`

## 验收标准 → Task → 测试追踪矩阵

| # | 验收标准（Issue/spec） | Task | 测试/证据 |
|---|---|---|---|
| AC1 | 预占成功发生在真实调用之前，拒绝时调用未派发 | 既有（U02）+ Task 3 接线回归 | `execution_gate_test.go` 既有；Task 3 Step 4 回归；真实链路 §2 |
| AC2 | Task Budget 与空间 Wallet 余额分别展示和限制 | #86 已交付；Task 6 验证 | `BillingPage.test.ts`（:184-226 余额卡）+ `TaskBudget.tsx`（:85-87 上限/已用/占用）；真实链路 §3 |
| AC3 | 缺失/过期/不可计算上界的价格投影 fail closed | Task 1+2+3 | `TestPricingResolveMissingVersionFailsClosed`、`TestPricingProjectionDivergedFailsClosed`、`TestAdmissionPricingMissingProjectionFailsClosed`、`TestPlatformBindingWithoutPriceVersionRejects`；真实链路 §4 |
| AC4 | 相同调用重试不重复预占 | 既有 + Task 6 锚定 | `TestACRetryDoesNotDoubleReserve`；真实链路 §2（占用数值不变） |
| AC5a | admission-price = 同一不可变 Lago 定价版本只读投影；预占同时针对 Task Budget 与保守 Tenant 可用余额 | Task 1+2（价格）+ Task 4（余额）+ Task 5（新鲜度） | Task 1-5 全部用例；真实链路 §1-§2 |
| 矩阵12 | 并发 worker 预占不超 Tenant 可用余额/Task Budget；父子共享上限 | 既有 PG + Task 4 扩展 | `TestBudgetPGConcurrentReservation`、`TestBudgetPGTwoTasksCompeteForAccount`、`TestBudgetPGFoldCompetesWithReservation`（PG DSN 环境实跑） |
| 矩阵13 | 事件受理保留预占；未知结果保 hold | Task 6 锚定（Lago 事件腿属 #88 边界，见下） | 域内：unknown observation ⇒ hold 保留（`remote_usage.go:71-79` 语义锚定）；真实链路 §2 |
| 矩阵14 | 实测计价高于预占上界 ⇒ 暂停 Task | 既有 + Task 6 锚定 | `TestACUpperBoundViolationHalts`（`commercial_adapter.go:104-106` halted）；真实链路可选 §5 |

**边界声明（不在本票、不静默扩scope）**：settlement 从旧 `CommercialGateway`（`service/commercial/fulfillment.go`）切换到 CommercialPlatform/Lago usage event 是 **#88** 的交付；#87 只保证「受理证据 ≠ 释放依据」（域内 hold 语义）与 t04 移交契约（同 id 异内容 ⇒ 冲突；id 以 Task 为作用域生成）在预占键设计上兼容——`stableUsageKey(req)`（tenant+call+attempt 派生）天然满足三元组唯一性作用域，Task 6 锚定测试附断言。

## 真实流程验证方案（真实 API 链路；端口避开 :5272/:5273）

栈编排（循 #86 ledger 惯例）：`deploy/lago` pinned v1.53.0 起独立栈 `weknora-lago-87v`（**:48901/:48902**，`LAGO_API_KEY` 仅 source `deploy/lago/.env`）；后端 worktree 起于 **:48087**（PG 独立库 `weknora_t87`）；前端 Vite **:5187**（`/api` 代理 48087）。

1. **§1 定价投影就位**：admin 发布含 `model`/`connector` fixed_unit 费率的 base 计划版本（`POST /api/v1/admin/...` plan publish 面，#79 交付）；probe：`GET /api/v1/plans/{code}`（Lago 侧）读回费率 = 投影表内容（SQL 查 `commercial_price_projections`）；预期一致且 `diverged=false`。
2. **§2 预占-拒绝-重试幂等（AC1/AC4/AC5a）**：空间 Owner 经 Billing 页（:5187）确认钱包余额（月度 1.00）；Task Owner 发起付费语义问答（平台资金）→ 任务建立、`commercial_reservations` 出现 held 行、`commercial_task_budgets.held_micro` 增加、任务详情 TaskBudget「占用」数值变化；同一调用重发（同 callID 重试）→ 占用数值**不变**、无第二笔 reservation；构造低余额空间（余额 0.01、上界 0.02+）→ 调用被拒且 Lago 零 usage event（`GET /api/v1/events?external_subscription_id=...` 空）、模型侧零外呼记录。
3. **§3 分别展示（AC2）**：BillingPage 余额卡（总余额/预占/退款锁定/可用 + 批次表）与任务详情 TaskBudget（上限/已用/占用）两处截图，数值与 SQL 对账一致。
4. **§4 fail closed（AC3）**：运维侧删 `commercial_price_projections` 行（或置 `diverged=true`）→ 再次付费调用立即拒绝、无派发、无外呼；恢复投影后放行。另一腿：停 Lago（`lago.sh stop`）→ Task 新建被 `ErrBalanceProjectionStale` 拒（Task 5）。
5. **§5（可选）矩阵14**：种低于实际上界的 reservation（SQL 直改 upper）→ 完成调用 → 任务 halted 证据（run 状态 + adapter 日志）。
6. 凭据纪律：key 仅 source 注入；截图/文档无 key 字面量；验证后 `lago.sh down` 销毁栈，证据入 `docs/migrations/lago/t15-admission-pricing/evidence/`。

## 待复核项（实现阶段复核后定稿）

- **R1**：v1.53 计划读回 `GET /api/v1/plans/{code}` 的 charges 反查维度字段（`billable_metric.code` vs 仅 `billable_metric_id`）；若仅 id ⇒ 加一次 metric 反查 GET。probe：t04/t07 lab 栈一条 curl。
- **R2**：迁移编号 000187 起是否空闲（实现时 `ls migrations/versioned | tail` 实查，遵循 PG/SQLite 双编号惯例）。
- **R3**：`WEKNORA_BALANCE_FRESHNESS_TTL` 默认 60m 是否成立（与 Task 5 刷新触发点、Lago 钱包刷新延迟联合裁决；#86 实测 termination 窗口 ~65min 为上限参考）。
- **R4**：BenefitsService 刷新链提取单租户重入入口的最小改动面（现为账单访问 lazy 链）。
- **R5**：base 计划版本是否携带 model/connector 费率是**数据前置**（admin 发布动作），不是代码；若现网 base 版本无 charges，§1 需先发布 v2（对既有空间无破坏——计划版本 append-only）。
- **R6**：`PlatformAdmissionPolicy()` 默认绑定改 fail closed 后，未发布定价的部署 connector 付费调用将全拒——产品姿态确认（spec L133 要求如此；如需灰度可加 env 白名单，默认仍 fail closed）。
