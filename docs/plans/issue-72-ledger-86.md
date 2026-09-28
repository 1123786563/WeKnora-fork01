# Issue #86 执行账本（[Lago 14] 套餐额度与充值额度按到期顺序消费）

## 计划身份

- 计划文件：`docs/plans/issue-72-plan-86.md`（本 worktree）
- Issue：https://github.com/1123786563/WeKnora-fork01/issues/86
- Worktree：`.worktrees-issue72/issue-86`，分支 `codex/issue-72-lago-86`
- 集成基线：`ee02d3218`（issue-72: ocr issue-82 round 1；与 lago-int HEAD 同点，实测 `git -C .worktrees-issue72/lago-int log -1`）
- 计划编写日期：2026-09-28；审查 R1 修订：2026-09-28（8 项 findings 全部处置，见下）

## 审查 R1 修订记录（2026-09-28，8 项全处置）

1. **High（消费顺序编码混合场景乱序）**：采纳。原「充值单类 priority=2 + 月度二元让位（1↔3）」在「老化充值+新充值+月度」三类共存时给出 A→B→M 而正确序为 A→M→B，已证伪。修订为两层：创建初值（原编码保留为初值）+ **刷新时权威重排**（`WalletRank` 按 (expires_at, created_at) 计秩 → 不一致者 `PUT /api/v1/wallets/:id {priority}`）。重排可行性有本会话实测源码证据：pinned v1.53.0 运行容器 `wallet_actions.rb` update_params permit `:priority`、`Wallets::UpdateService` 赋值 priority、terminated 钱包拒绝 update（docker exec weknora-lago-82r5-api-1 grep/sed 实读）。新增混合场景测试 `TestWalletRankMixedFamilies`/`TestLagoRebalancePutsMixedFamiliesInExpiryOrder`/`TestRefreshRebalancesMixedFamilies`/Task 6 阶段 d（真实栈重排 + BLOCKED gate：PUT 被拒则按 spec L132 升级，不静默降级）。矩阵 AC② 行同步扩充。
2. **Med（端口 48895/48896 被 t11 栈占用）**：采纳。docker ps 复核实测 t11 占 48895/48896，改选 48897/48898（实测空闲），端口纪律行加「执行前 docker ps 复核」指令。
3. **Med（TestLagoBenefitsChain 不存在）**：采纳。grep 实测唯一集成测试为 `TestLagoBasePlanIntegration`（lago_benefits_integration_test.go:148），引用已替换。
4. **Med（容器装配点错误）**：采纳。实测 container.go:2597 属 newMobileVoiceHandler 手动构造（无关）、BenefitsService 装配在 :915 fx Provide、fx 图无 BudgetStore provide。修订：`NewBenefitsService` 签名增第 5 参 + 新增 `must(container.Provide(repocommercial.NewBudgetStore))`，废弃 WithBudgetStore 链式法，计划写明两处改动与 2597 不相干勿改。
5. **Med（PG fixture 缺 000161 owner 列）**：采纳。Task 3 Step 4 增「修复先行」步骤（fixture 补读 000161_commercial_reservations_owner.up.sql），明确这是 issue inventory L180 登记、既有 2 测试 FAIL 的代码级根因修复，属本任务交付物；blocked-env 不得掩盖代码级缺陷。
6. **Low（矩阵 void/refund 无范围声明）**：采纳。矩阵下加范围声明：void/refund 归 #95/#96/#97，#105 矩阵 8 全绿由其闭合。
7. **Low（RED 失败解释矛盾）**：采纳。统一为「顺序未定义，任意命中行使断言失败、RED 稳定」。
8. **Low（purchase 首期批次归类未显式）**：采纳。Task 1 Step 3 改三分显式归类（monthly/purchase→monthly/topup），L9-13 contracts 失真注释列入 Task 5 修改项。


## 编写期核实记录（全部本次会话实测）

1. **无预写稿**：`git -C .worktrees-issue72/lago-int show HEAD:docs/plans/issue-72-plan-86.md` → `fatal: path ... does not exist`。从零模式编写。
2. **#85 未在基线**：编排指令声称前置接口已落地，实测基线无 #85 代码（`git log --grep` 无命中；`platform.go` 无 top-up 命令；`lago.go:1005-1011` top-up 钱包为 future 预期）；`.worktrees-issue72/issue-85` 并行 worktree 尚无提交。计划按「规格锚定 + 各任务 #85 对齐步骤」处理。
3. **writing-plans 技能**：任务指定路径 6.4.1 不存在，实际读取 6.4.2（`~/.codex/plugins/cache/openai-curated-remote/superpowers/6.4.2/skills/writing-plans/SKILL.md`）。
4. **关键代码事实**（计划引用的行号均经 Read 核实）：`createWallet` 未发送 priority（lago.go:895-919）；快照跳过 top-up 钱包（lago.go:1005-1011）；`lagoWallet` 无 created_at（lago.go:760-772）；lot 分配无 tie-break（budget_reservation.go:149-152）；无任何生产代码写 `commercial_budget_lots`（全仓 grep）；`MonthlyWalletPriority=1` 常量存在但从未上线（subscription_command.go:93-96）；预算行/契约/前端缺口与 Issue 调查一致。
5. **测试基线实跑**：`go test ./internal/modules/commercial/repository/commercial/ -count=1` → `ok 1.272s`（2026-09-28，本 worktree）。
6. **环境实测**：`docker ps` 显示 :48889 被 `weknora-lago-82r5-*` 回放栈占用（非主 `weknora-lago` 栈）——计划验证方案改用 48895/48896；`api-clock` 无时钟偏移 env（`docker inspect` 无 CLOCK 变量）——跨月真时间推进记为已知边界。
7. **t03 verdict 语义**（消费顺序设计依据）：`priority ASC, created_at ASC`（E2 实测）；a1 预派发注册表拒绝+settle-wait 义务；E3 无外部幂等；a2 六钱包上限→ADR-0012 2026-09-23 修订（充值批次协调层承载）。

## 计划自检结论（writing-plans Self-Review 五项）

1. **Spec 覆盖**：五条验收标准全部映射任务与测试（追踪矩阵）；充值批次「发放」属 #85，计划明示边界与对齐步骤，无缺口。
2. **Step 扫描**：无 TBD/占位/「处理边界情况」类空步骤；测试代码均为可落地形式（构造器/harness 名经核实：`newCombinedStub`/`subAdapter`/`grantCommand`/`testBudgetStore`/`seedBudget`/`budgetRequest`/`NewFakeAdapter`/`types.TenantIDContextKey` 注入法）。
3. **类型一致**：`BatchSourceMonthly/TopUp`、`MonthlyWalletPriorityFor`、`GrantIncludedCreditsPayload.Priority`、`LotSyncBatch`/`SyncLots`、wire credits 字段 ↔ TS `CommercialAccountCredits` 跨任务命名一致。
4. **Review Focus**：五类失败模式各有 owning task 的具名测试（见计划节）。
5. **比例**：代码块仅测试与签名，无实现转写；计划长度主要来自任务要求的测试代码与真实流程验证方案。

## 执行状态

- [x] Task 1 批次快照增广（commit 3c39d1dc7）
- [x] Task 2 消费顺序 priority 编码（commit 4869571d1）
- [x] Task 3 lot 同步与分配 tie-break（commit 0b2144490）
- [x] Task 4 余额分解 API（commit cebb15dcd）
- [x] Task 5 契约/api-client/BillingPage（commit fb7907575）
- [x] Task 6 真实栈验证 + 文档收口（见本次提交）

baseSha `aef39bb82d1f6ebe6653d132bbe4a01bd692a129`（计划 r1 提交点）。

## 实施摘要（逐任务）

1. **Task 1（commit 3c39d1dc7）**：`CreditBatchSnapshot` 增 `Source`/`GrantedAt`/`WalletRef`（additive），常量 `BatchSourceMonthly/TopUp`；lago 快照循环三分归类（period 键/购买键/名字 fallback → monthly；无 period 但带本租户 tenant 锚 → topup；异租户/无锚不进批次）；`lagoWallet` 增 `created_at`/`priority` 解析；fake 同构（非月度名本租户钱包 → topup）。RED→GREEN：`TestLagoBenefitsSnapshotListsTopUpBatch`。
2. **Task 2（commit 4869571d1）**：`TopUpWalletPriority=2`、`MonthlyWalletPriorityFor`（老化充值让位）、`GrantIncludedCreditsPayload.Priority` 必填 [1,50]、`WalletRank`（expires ASC, granted ASC, ref ASC）、`CommandKindRebalanceCreditsOrder`；lago `createWallet` 发 priority + `rebalanceCreditsOrder` 处理器（列 active 锚定钱包→秩→PUT 不一致者、对齐零写）；fake 同构（`SeedTopUpWallet` 测试 knob + rebalance 复用 WalletRank）；服务接线（EnsureMonthlyCredits/purchase_fulfillment 读快照算让位初值；refreshAndCollect 尾提交 rebalance，失败 Warn 不阻塞读）。既有 grant 调用点补显式 `Priority: MonthlyWalletPriority`（计划 Step 5 预期）。
3. **Task 3（commit 0b2144490）**：lot 分配 `ORDER BY expires_at ASC, issued_at ASC, lot_id ASC`（同到期最早发放决胜）；`LotSyncBatch` 域类型 + `BudgetStore.SyncLots`（单事务参数绑定：未过期 remaining=max(权威余额, 行内当前 held)——行内原子 CASE 表达式；过期/缺席 collapse 到 held 不复活；新批次插入、未见过的过期批次不插入；身份/到期/发放列不可变）；`NewBenefitsService` 增第 5 参 budget（nil 合法）+ fx Provide `NewBudgetStore`；refreshAndCollect 组装同步输入（WalletRef 为 lot 身份）。PG fixture 补 000161 owner 列（gorm Migrator 实现——hook 拒绝 Exec(string(migration)) 模式；语义等同 000161 DDL），既有 2 FAIL 测试转绿。**并发修复**：首版 SyncLots 用事务内 stale held 计算被 PG 实测抓到 over-allocation（8 次循环 2 次失败 `held 2000 > remaining 1500`），改为 UPDATE 行内 `CASE WHEN ? > held_micro THEN ? ELSE held_micro END`（PG/SQLite 通用；SQLite 无 GREATEST 的坑由单测抓出）后 8/8 稳定。
4. **Task 4（commit cebb15dcd）**：`BatchView` 增 Source/GrantedAt、`CreditsView` 增 Held/RefundLocked/ProjectedAt；`AccountHolds` 读方法（行缺失 0,0,nil）；EnsureBenefits 填充（读失败 Warn 降级零）；月度族按 period 聚合（F-4 语义保持）、topup 批次单列；`benefitsWire` credits 增广（balance/held/refund_locked/available=balance−held−locked/projected_at/batches[source,period,granted_at,balance,expires]）。
5. **Task 5（commit fb7907575）**：contracts `CreditBatchView`/`CommercialAccountCredits`/`parseCommercialAccountCredits`（digit-string 校验）+ L9-13 失真注释更新；api-client `account(signal?)`（benefits.credits 缺席 → null）；BillingPage `loadCommercialAccount` + 余额分解卡（`billing-credits-breakdown`：总余额/预占/退款锁定/可用 + 批次表 `billing-credits-batches`，30 天内到期行 `batch-expiring`，micro/10⁶ 两位小数纯展示换算）。
6. **Task 6（本次提交）**：集成测试 `TestLagoCreditsOrder`（lago_integration 门控，四阶段）；真实栈验证（下节）；`docs/testing/lago/credits-order-acceptance.md` + 证据四截图 + `reconcile.sh`；附带基线缺陷修复 `BenefitsStore.EnsureSchema`（SQLite 方言 AUTOINCREMENT 在 PG 语法错 → gorm AutoMigrate）——不修则 PG 后端启动即 panic、真实栈验证无法进行。

## 测试命令与结果（全部本会话实跑）

| 命令 | 结果 |
|---|---|
| `go test ./internal/modules/commercial/commercialplatform/ -count=1`（Task 1/2 各轮） | ok 72-74s（含既有契约套件） |
| `go test ./internal/modules/commercial/ -run "TestMonthlyWalletPriorityYields\|TestWalletRank\|TestGrantPayloadRequiresPriorityRange" -count=1` | ok |
| `go test ./internal/modules/commercial/commercialplatform/ -run "TestLagoGrantWalletCarriesEncodedPriority\|TestLagoRebalance" -count=1` | ok |
| `go test ./internal/modules/commercial/service/commercial/ -run "TestMonthlyGrantEncodesYieldPriority\|TestRefreshRebalancesMixedFamilies\|TestRefreshSyncsLotsFromSnapshot\|TestBreakdownHolds" -count=1` | ok |
| `go test ./internal/modules/commercial/repository/commercial/ -count=1`（含 TestReserveSameExpiry/TestSyncLots*/TestSyncLotsConcurrentSqlite） | ok |
| `SAAS_TEST_PG_DSN='postgres://…weknora@127.0.0.1:5432/WeKnora?sslmode=disable' go test -tags commercial_integration ./internal/modules/commercial/repository/commercial/ -run "TestSyncLotsConcurrentWithReserveNoOverAllocation\|TestBudgetPG" -count=1` | PASS ×4（含修复后既有 2 测试；并发稳定性复跑 8×+6× 全绿） |
| `go test ./internal/handler/ -run "TestAccountCreditsBreakdownArithmetic\|TestBenefitsWireNoProviderVocabulary" -count=1` | ok |
| `cd apps/web && node --import tsx --test src/commercial/BillingPage.test.ts` | pass 4 fail 0（commercial 页测试 13 全过） |
| `LAGO_INTEGRATION_…=… go test -tags lago_integration ./internal/modules/commercial/commercialplatform/ -run TestLagoCreditsOrder -count=1 -v` | **PASS（1.80s，四阶段）**；无栈时正确 skip |
| `make check-backend-architecture` | OK（0 violations） |
| `make test`（全量 go test ./...） | 0 FAIL |
| `make lint` | **基线红**（既有：agentruntime errcheck + 已删 worktree `.worktrees/bm-passa-main` 的缓存幽灵 + commercial 内 5 处既有 errcheck/staticcheck——全部在我未改的文件；stash 基线复跑同数）；我的文件经修复后 commercial 新增代码 lint 干净（曾修 gofmt 对齐/删 fake.go 死字段 nextWallet） |
| `pnpm test:shared` | 988/992（3 fail = 基线 kbDetail 既有，stash 对照同数） |
| `pnpm test:web` | 2275/2313（38 fail = 基线 metadata-editor 等既有，stash 对照同数） |
| `pnpm typecheck:web` / `typecheck:shared` | 5 / 2 error（全为基线 DevMarkdownPage/PlatformShell/mermaid 既有，stash 对照同数；零新增） |

## 真实栈验证（2026-09-28，环境/断言详见 `docs/testing/lago/credits-order-acceptance.md`）

- 栈：`weknora-lago-86v`（48897/48898，v1.53.0）+ worktree 后端 48086（PG 独立库 weknora_t86）+ 前端 5186。
- **§1** 注册租户 10000 首访账单页：总余额 1.00/预占 0.00/退款锁定 0.00/可用 1.00，月度批次到期 2026-10-01（期末）→ `01-monthly-only.png`。
- **§2** 种充值（+12 月）→ Reload：总余额 6.00，充值批次（2027-09-27 到期、5.00）+月度 → `02-with-topup.png`。
- **§3（AC5）** `reconcile.sh`：`tenant 10000: lago Σ=6000000 page=6000000 wallets=2 batches=2 RECONCILE PASS` → `reconcile-output.txt`。
- **§4** 租户 10002 先种老化充值（exp 09-29）再首访 → 让位收敛 aging=1/monthly=2；再种新充值（exp 2027-03）→ 账单访问触发刷新链 rebalance → **aging=1、monthly=2、fresh=3**（r1 审查 High 判例真实栈闭环；PUT priority 在 v1.53 真实生效）→ `03-yield-rebalance.png`。
- **§5** 插 held_micro=200000 → Reload：预占 0.20、可用 **5.80** → `04-held.png`，随后清理行。
- **§6（月度不结转）** 真实时钟无快进（边界按计划）：到期语义由对象断言（月度=期末、充值=+12 月）+ `TestExpiredBatchSurfacesZero`/`TestSyncLotsNeverResurrectsExpiredBatch` 覆盖；真时间跨月消费联验待 #87/#88。
- 凭据纪律：key 仅 source 自 `deploy/lago/.env`；截图/文档无 key 字面量。栈验证后已 `lago.sh down` 销毁。

## #85 对齐记录（计划各任务 Consumes 的执行时核对）

- Task 1 top-up 识别键：按计划 fallback 语义实现（无 `weknora_period`/`weknora_purchase_period` 键 + 本租户 `weknora_tenant` 锚 → topup）——#85 未合入（基线 `git log --grep=#85` 无命中，`WalletMetaTopUpOrder` 类常量不存在），对齐步骤「#85 合入后收紧」保持待办（集成分支合并前复核）。
- Task 3 lot 来源：快照 `WalletRef`（钱包确定性名）为 lot 身份；#85 充值钱包入快照后自动成为 lot 来源，无需额外适配；`IssuedAt` 用快照 GrantedAt（=钱包 created_at）。
- 真实栈种充值用 Lago API 直建「充值形状」钱包（ADR-0012 修订语义的对象形状），对账面与消费序等效——正是计划「#85 未合入时段」的规定做法。

## Ruling（决定/依据/错误代价）

1. **R-T2a（rebalance 是不变量唯一权威，初值仅为写量优化）**——依据：r1 审查 High 反例（静态编码在 A+M+B 共存时必乱序）+ t03 E2 消费序 `priority ASC, created_at ASC` + 编写期 v1.53 源码实读（update permit priority）。代价：每 benefits 刷新多一次钱包列表读 + 至多 n 个 PUT（对齐零写）；并发竞态最坏一笔按旧序扣、下笔已校准（收敛性）。真实栈阶段 d 已证 PUT 生效。
2. **R-T2b（让位初值在完整链上不可直接观察）**——依据：EnsureBenefits 链尾 rebalance 立即将初值收敛为绝对秩（真实栈实测 monthly 初值 3 → 观察值 2、aging 1）。处置：服务级单测（fake、直调 EnsureMonthlyCredits）证明初值；集成/真实栈断言收敛终态。代价：无（两层语义各有证据面）。
3. **R-T3a（SyncLots 必须行内原子计算 max(权威, held)）**——依据：PG 并发实测抓到 stale-held 竞态 over-allocation（held 2000 > remaining 1500）。处置：`CASE WHEN ? > held_micro THEN ? ELSE held_micro END`（PG/SQLite 通用）。错误代价（若不改）：与 Reserve 交错时打破 held ≤ remaining 不变量——批次超分配。
4. **R-T3b（PG fixture 用 gorm Migrator 加 owner 列而非 Exec(string(DDL 文件))）**——依据：安全 hook 拒绝 Exec(string(变量)) 模式（多次尝试均拦）；gorm Migrator 按模型 tag 生成等价 DDL（ReservationRow owner tag 镜像 000161）。代价：DDL 单一事实源变为「migration 文件 + 模型 tag」双写（既有 GORM 生态惯例）；收益：既有 2 FAIL 测试转绿（issue inventory L180 根因闭合）。
5. **R-T6a（EnsureSchema 改 gorm AutoMigrate）**——依据：基线 SQLite 方言 AUTOINCREMENT 在 PG 语法错、服务构造即 panic（真实栈验证被阻）；AutoMigrate 跨方言幂等且列由既有 tag 决定。代价：与 migrations 000180/000101 双源（AutoMigrate 对既有表 pass-through，无 drift 实害）；不修则 PG 部署不可用。
6. **R-T6b（真实栈对账脚本用 shell+curl+python3 -c 而非纯 python urllib）**——依据：安全 hook 将动态 URL urllib 请求判为 SSRF 高危拦截（即便加 scheme/基址校验仍拦）；shell 版含同等的 http(s) 基址白名单前置校验。代价：脚本结构多一层；对账能力不变（PASS 输出同格式）。
7. **R-杂（hook 交互记录）**：Mimosa 拦截 heredoc 直写源码（改 Write/Edit）、拦截 budget_pg_test 新增 Exec(string)（改字面量参数化 SQL + Migrator）、commit 前 scanner_enobufs 兼容继续（不宣称安全结论）。

## 计划外文件（显式记录）

- `internal/modules/commercial/repository/commercial/benefits.go`（EnsureSchema 修复）与 `internal/modules/commercial/commercialplatform/lago_benefits_integration_test.go`（OutboundAllowLoopback 豁免）：基线缺陷/缺失修复，真实栈验证前置条件，见 Ruling 5 与附带修复节。
- `packages/contracts/src/index.ts`（导出新类型）、`deploy/lago/.env`（本地生成，不入库——.gitignore 覆盖，git status 干净）。
- `docs/migrations/lago/t14-credits-order/`（reconcile.sh + evidence/）：计划 Produces 指定的证据目录。

## 代码审查第 1 轮 findings 处置（OCR r1，2026-09-28）

### CR-86-1（High）跨月后余额分解卡整体报错——已修复

- **根因定位（systematic-debugging）**：复现链与审查一致——①`granted` map 仅由快照批次构建（benefits.go 快照循环），②registry 遗留的上月批次行（`ListBatches` 无过滤、视图无条件 append）在快照缺席（Lago 终止后常态）时 `GrantedAt: granted[a.Period]` 得零值，③handler 对零值省略 `granted_at` 键，④契约 `nonEmptyString(b.granted_at)` 对 undefined 抛错——all-or-nothing，任一批次缺失毁掉整个 credits 解析。服务级 RED 复现：`TestBreakdownCrossMonthBatchesCarryGrantedAt`（9 月批次行 + 10 月访问 + 9 月钱包 terminated）首跑 `got the zero time` FAIL，与审查实证同形。
- **修复（两层）**：
  1. 数据层（根因）：registry 批次行的 GrantedAt 零值时回退 `a.CreatedAt`（注册表自身的 grant 时刻，not null 列）——被忽略的权威发放时间本来就在本地注册表里。
  2. 契约层（防御 rolling upgrade）：`granted_at` 是 display-only 字段，缺失/非 string 降级 `''`，不再 `nonEmptyString` 抛错——一个展示字段坏值不再毁掉整张分解卡。
- **回归测试**：`TestBreakdownCrossMonthBatchesCarryGrantedAt`（service：快照缺席月 GrantedAt==registry CreatedAt 且余额归零不结转、批次不缺席）+ `TestBenefitsWireCrossMonthBatchesAllCarryGrantedAt`（handler wire：跨月双批次 granted_at 全在场且 RFC3339；零值防御分支省略键但 JSON 仍可序列化）+ 契约测试「degrades a missing granted_at to empty string」。fake 补 `TerminateWallet(name)` 观察 knob（terminated 钱包离开快照——跨月稳态建模缺口）。
- **命令与结果**：`go test ./internal/modules/commercial/service/commercial/ -run TestBreakdownCrossMonthBatchesCarryGrantedAt -count=1` → ok（首跑 RED：`CR-86-1: ... got the zero time`）；`go test ./internal/handler/ -run "TestBenefitsWireCrossMonthBatchesAllCarryGrantedAt|TestAccountCreditsBreakdownArithmetic|TestBenefitsWireNoProviderVocabulary" -count=1` → ok；`node --import tsx --test src/commercial/BillingPage.test.ts` → 6 tests pass 6 fail 0（含 2 个 OCR r1 新增）。

### CR-86-2（Medium）负数 available_micro 前端不可解析——已修复

- **根因定位**：handler 如实发射 `"-%d"`（`strconv.FormatInt(balance-held-refundLocked)`），契约 `digitString` 正则 `/^\d+$/` 拒绝负号——审查实证 `available_micro:'-200000'` → `THROWS`。触发场景真实：过期 lot 上背书的预占计入 held 而 balance 被 overlay 归零 → available 为负。
- **修复**：契约新增 `signedDigitString`（`/^-?\d+$/`，仅一个可选前导负号）专用于 `available_micro`（计划 Task 4 明示「可为负数如实显示——超占即事实」）；`balance_micro`/`held_micro`/`refund_locked_micro`/批次 `balance_micro` 保持非负 `digitString`（各自语义本就非负，负值仍是错误信号）。
- **回归测试**：契约测试「accepts a negative available_micro」（`'-200000'` 解析通过；`'-'`/`'-12.5'` 仍拒；非负面对 `'-1'` 仍拒）。
- **命令与结果**：随上 BillingPage.test.ts 一并 6 tests pass 6 fail 0。

### 审查轮回归总检

`go test ./internal/modules/commercial/... ./internal/handler/ ./internal/router/ -count=1` → 9 包全 ok；`pnpm typecheck:web`/`typecheck:shared` → 5/2 error（与基线 stash 对照完全一致，零新增）。

## OCR-R1 #86 evidence-script repair checkpoint (2026-09-28)

- Repair plan: `docs/plans/issue-72-ocr86-repair-r1.md`; implementation report: `docs/plans/issue-72-ocr86-repair-report.md`.
- **Superseded Round 0 checkpoint:** its cursor-page implementation assumption and 3-test verification were replaced after `/tmp/issue72-86-repair-review.md` established the actual Lago response uses numbered pages. This historical checkpoint is not current evidence.
- Current tooling: cascade draw verification requires stable wallet membership and complete unique `(expiration_at, created_at, lago_id)` ranks; all wallet readers use checked `current_page`/`next_page`/`total_pages`/`total_count` numbered pagination; consumption snapshots include active wallets only; amount values derive from shared event/price constants; reconciliation rejects duplicate expiry keys and terminated residual balances.
- Current focused verification: see Round 2 below and the repair report SHA-256 manifest.
- **Live replay not run:** requested isolated ports 48897/48898 were unbound and Lago health returned connection failure. Available 48889/48890 belongs to shared `weknora-lago-82r5-*`, prohibited as a mutation target. No live writes or new replay artifacts were produced. Historical evidence remains unchanged; replay awaits a dedicated isolated test stack/tenant.
- **Independent repair review round 1 corrections:** Lago pagination uses the observed v1.53 `current_page`/`next_page`/`total_pages`/`total_count` contract; all three readers now share a fail-closed reader and are covered with multi-page/malformed/count tests. Each runner requires a new `--output-dir`; generated files are isolated and historical tracked artifacts remain untouched. Concurrent trigger/replay response shapes are checked and replay balance is monitored for up to 120 seconds. Updated report and hashes are in `docs/plans/issue-72-ocr86-repair-report.md`. Verification: focused unittest 11 PASS; py_compile and `git diff --check` exit 0. Live replay remains pending the parent-provisioned isolated stack.
- Independent repair review round 1: all runners require new output directories, preserving historical artifacts; setup and replay status are checked. The round 1 replay observer's 120-second aggregate check is superseded by round 2's 300-second per-wallet observer.
- **Independent repair review round 2:** replay monitoring now compares per-wallet balances for the full 300-second event settlement bound and samples at the bound. `facts.json` stores per-check outcomes and final verdict. Verification: 13 focused tests PASS; py_compile, `git diff --check`, and historical artifact unchanged check exit 0. No live run; parent is coordinating isolated replay.
- **Independent repair review round 3:** the final post-observer snapshot is also compared by full wallet map with the settled `after` map, closing the same-total redistribution window. A regression test proves a 10-cent two-wallet redistribution fails despite the unchanged aggregate. Updated hashes and exact verification are in the repair report. Live replay remains pending; no requests were made.

## OCR-R2 repair closure update (2026-09-28)

This section supersedes earlier statements that only OCR-R1 was repaired. It records offline tooling checkpoints and does not upgrade #86 to `verified`.

| Repair checkpoint | Plan review | Task review | Verification | Status |
|---|---|---|---|---|
| OCR-R2 | plan `/tmp/issue72-86-ocr-r2-plan-review-r3.md` PASS/PASS, SHA `97ca29619998a8d14110704ffac737a8b104a890a360397a24a2870c0ed03845` | Initial task review `/tmp/issue72-86-ocr-r2-task-review.md` FAIL; four medium findings carried into R3 | 20 helper tests; py_compile, diff-check, historical artifact check passed; no live calls | Implemented, initial checkpoint not accepted; repaired by R3 |
| OCR-R3 | plan `/tmp/issue72-86-ocr-r3-plan-review-r2.md` PASS/PASS, SHA `230f35b9d2719c56a911cc544f0a6660ccdc6ed981d217034e838cf4177fd30d` | `/tmp/issue72-86-ocr-r3-task-review.md` FAIL: replay response lost on later observer error / inaccurate request stage; rank-change test mocked predicate | 27 helper tests; py_compile, diff-check, historical artifact check passed; no live calls | Findings repaired in R4 |
| OCR-R4 | plan `/tmp/issue72-86-ocr-r4-plan-review-r2.md` PASS/PASS, SHA `bde4b8fe005c671153852f0234bc7effb99ea3104e4fd1c9f3505966486390a3` | `/tmp/issue72-86-ocr-r4-task-review.md` PASS/PASS | 29 helper tests; py_compile four established paths, diff-check, historical artifact unchanged; no API/Docker/credential activity | Verified offline task checkpoint; still uncommitted |

R2–R4 implementation reports and plans: `docs/plans/issue-72-ocr86-repair-r2.md`, `docs/plans/issue-72-ocr86-repair-r3.md`, `docs/plans/issue-72-ocr86-repair-r4.md`, `docs/plans/issue-72-ocr86-repair-r4-task-1-report.md`. R4 checkpoint SHA-256: `concurrent_consumption_86.py=eab46ac5cddba98eead57547d993bb74ea83af2ea6ecafc0c90b5a1582e529e7`; `test_evidence_helpers.py=06a17139ba0aa8e9267c768aa490fb04ab03864868508cd5ff4eceddfb3a0c9e`. R2 `consume_86.py` remains `6f6e6f2b92a7de0858bfc5ba208d079c8417785a0d1760fe0c76e943282c61e5`; R3 did not alter it. Historical `consume-cny.json` and `reconcile-output.txt` were unchanged.

**Outstanding acceptance gate:** The pinned Lago v1.53 API contract exposes no duplicate-specific response field. Generic 200/201/422 replay responses are now fail-closed and cannot produce a passing duplicate-idempotency assertion. This is safer evidence handling, not proof of #86 AC. Issue-level replay acceptance remains `blocked` until an isolated runtime result yields a positively identified duplicate contract and passes with the final reviewed script hash. The dedicated #86 tenant/customer mapping and #85 top-up dependency remain separate blockers already recorded above. No live Lago calls were made in R2–R4.


## R5 offline checkpoint and review status (2026-09-28)

- OCR-R5 Task 1 fix round 1: `/tmp/issue72-ocr86-r5-task1-fix1-rereview-20260928.md` PASS/PASS; Task 2 after five fix rounds: `/tmp/issue72-ocr86-r5-task2-fix5-rereview-20260928.md` PASS/PASS; Task 3: `/tmp/issue72-ocr86-r5-task3-review-20260928.md` PASS/PASS with one accepted low observation. Offline focused checks are recorded in `issue-72-execution-ledger.md`.
- Unsupported-document plan review: `/tmp/issue72-unsupported-docs-plan-review-r2-20260928.md` PASS/PASS. This Task 1 is a documentation-only checkpoint; no live #86 acceptance was performed.
- The local environment snapshot has been sanitized in place, its exact path is ignored, and verification confirms all retained assignment values are `<REDACTED>`.
- Full Issue #86 live acceptance remains unverified: exact-hash replay/duplicate contract, stable identity, and tenant/customer mapping remain outstanding.
