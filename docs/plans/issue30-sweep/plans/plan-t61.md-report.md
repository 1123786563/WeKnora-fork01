# T31（Issue #61）Task 6 实现报告：端到端 HTTP 证据（AC1 / AC2 / AC3）

- 执行者：实现员子代理（subagent-driven-development / TDD）
- Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t61`（分支 `codex/issue30-t61`）
- 需求源：`docs/plans/issue30-sweep/plans/plan-t61.md` · Task 6（唯一需求来源，测试代码取自计划全文）
- 前置：Task 0-5 均已在分支上（96e579ad0 → 3301a1fd8 → 2e75e7882 → ef43cd83a → 0a5af2af2 → 7f41cde2f）。
- 本报告覆盖同路径上一份 Task 2 报告（各任务共用报告路径、依次覆盖，本次为 Task 6 的报告）。

## 实现内容

| 文件 | 动作 | 说明 |
|---|---|---|
| `internal/router/routes_agent_upgrade_test.go` | 追加（+448 行） | Task 6 e2e：helper 四件 + `upgradeDiffWire` + 主测试 `TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks`；import 块补齐 7 项 |

交付的 helper（与计划 Step 1 逐字，除下述「与计划的唯一偏差」中的拆分）：

- `newAgentUpgradeTestApp`：真实迁移流（`openTenantAgentMarketplaceHTTPTestDB`，含 000120——迁移号自 Task 1 起按 plan-t48 约定整体顺延，见「修复轮 1/5 偏差记录」）上挂载真实五层服务栈（CustomAgentService / AgentVersionService / AgentMarketplaceService / AgentAdoptionService / AgentUpgradeService）+ 真实 `GET /api/v1/agents`；镜像 `newAgentAdoptionTestApp` 但不改动它（并行批次合并安全）。
- `freezeAndPublishUpgradeRelease`：真实 HTTP 冻结 → 提交 → 审核发布一个语义版本。
- `publishUpgradeVariant`：创建草稿后委托 `mapTestPublishUpgradeVariant`。
- `mapTestPublishUpgradeVariant`（新增拆分，见偏差说明）：对**既有** variant 驱动 #59 既有流程（重映射 → 测试 → 发布）。
- `agentsRowByName`：读真实 `GET /api/v1/agents` wire，按名取 config（移动 Resource Shelf 投影）。
- `upgradeDiffWire`：四维差异的 wire 镜像。

主测试覆盖（全部经真实 HTTP + 真实 sqlite 迁移流 + 真实仓储/服务，零 mock）：

- **AC2**：v1 发布 → adopt → 旧 Variant published → v2 发布（真实改源 agent 配置再冻结）→ 建议出现且恰好 1 条 open，diff 逐字段断言：行为维 `system_prompt`（真实冻结→导出链产生）、安全维 `data_categories`+`external_side_effects`、许可维 `license_id` MIT→Apache-2.0、依赖段在场为空（生产依赖解析器 fail-closed 边界，计划差异记录第 3 条如实声明）。
- **对账幂等（HTTP 面）**：连续两次 list 仍恰好 1 条（Review Focus 2）。
- **治理边界**：viewer 403；tenant-2 空列表 + 跨租户读单条 404 与不存在同形（Review Focus 3）；空 name 400；body 冒充 `tenant_id` 400。
- **接受路径（AC1 接受半边）**：accept 返回 `{proposal, variant}` 成对信封，草稿钉到 v2、state=draft、按新 Release 重算缺失能力 `[knowledge model]`；重复 accept 409；同 adoption 草稿数恒为 1（Review Focus 4）。
- **AC1 零静默升级**：旧 Variant 行 `state=published`、`release_id=v1` 不变；旧本地 agent 经真实 `GET /api/v1/agents` 断言 `system_prompt="Be useful."`、`model_id=gpt-x` 逐字段不变；available-agents 草稿期仍 1 行；新草稿独立映射/测试/发布实例化**新**本地 agent（不覆盖旧）；终局 available-agents 2 行且旧变体在前（created_at 序）；旧冻结版本 `GET /api/v1/agents/:oldAgent/versions/:oldVersion` 仍 200（Review Focus 1）。
- **驳回路径**：v3 → 第二条建议 → dismiss 200（`resolved_by` 落盘）→ 重复 dismiss 409 → dismiss accepted 409 → 终局列表无 open（终态不复活）。

## 与计划的唯一偏差（测试笔误修正，计划 Step 3 明示授权）

计划 Task 6 原文的升级发布调用是 `publishUpgradeVariant(t, r, adoptionBody.Data.ID, "Sales Assistant v1.1", ...)`——该 helper 走 `POST /adoptions/:id/variants` 新建 Variant，而 `CreateVariant` 在 body 不带 `release_id` 时钉到 **`adoption.AcceptedReleaseID`（仍是 v1）**（`internal/application/service/agent_adoption.go:131-134`）。于是新实例化的本地 agent 带旧版本提示词，与计划自己的断言「新 agent 承载新版本行为 / Be extra useful and cite sources.」矛盾，也偏离 spec §9 流程（「接受建议只是以新 Release 创建一个新的 Variant 草稿，随后走既有 #59 的重新映射→测试→发布流程」——应发布 **accept 产生的那只 v2 草稿**）。

修正（仅动授权测试文件，断言逐字保留）：把 helper 的「重映射→测试→发布」段拆为 `mapTestPublishUpgradeVariant`，升级发布改为驱动 `acceptBody.Data.Variant.ID`（accept 响应中的 v2 草稿）；首个 Variant 仍走原 `publishUpgradeVariant`（创建+发布）。实现层（Task 1-5 已提交代码）零改动。

## TDD 证据

### RED

命令（计划 Task 6 Step 3 逐字）：

```sh
go test ./internal/router/ -run TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks -count=1
```

关键输出（首跑，按计划原文 helper）：

```
--- FAIL: TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks (14.77s)
    routes_agent_upgrade_test.go:422:
        	Error:      	Not equal:
        	            	expected: "Be extra useful and cite sources."
        	            	actual  : "Be useful."
        	Messages:   	新 agent 承载新版本行为
FAIL
FAIL	github.com/Tencent/WeKnora/internal/router	27.918s
```

失败为何符合预期：编译干净（Task 1-5 接口就绪），失败集中在上述 helper 笔误——前面所有断言（含 AC2 四维 diff、accept 草稿钉 v2、draftCount==1、旧世界不变）**已通过**，恰好证明实现层正确、测试自身有误；与计划 Step 3 预告的「按错误信息修正测试自身的笔误；实现层已在 Task 1-5 就绪」一致。

### GREEN

修正测试笔误后，命令（计划 Task 6 Step 4 逐字）：

```sh
go test ./internal/router/ -run 'TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks|TestAgentUpgradeRoutesRequireAdminAndFullAccess' -count=1 -v
```

完整结果行：

```
=== RUN   TestAgentUpgradeRoutesRequireAdminAndFullAccess
--- PASS: TestAgentUpgradeRoutesRequireAdminAndFullAccess (0.01s)
=== RUN   TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks
--- PASS: TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks (13.02s)
PASS
ok  	github.com/Tencent/WeKnora/internal/router	21.693s
```

gofmt 修正（结构体字段对齐，纯空白）后复跑同命令：`ok github.com/Tencent/WeKnora/internal/router 8.077s`。

## 计划级验证（Task 6 提交后、计划「计划级验证」节命令逐字）

```sh
go build ./internal/... && go test ./internal/database/ -run 'TestSQLiteMigrations|TestSemanticMigrationSQLiteUpDownUp' -count=1 && go test ./internal/application/repository/ -run 'TestAgentUpgrade|TestAgentAdoption|TestMobileDevice|TestMobilePush' -count=1 && go test ./internal/application/service/ -run 'TestAgentUpgrade|TestDiffUpgradeBundles|TestAgentAdoption' -count=1 && go test ./internal/handler/ -run 'TestMobileDevice|TestPublishVariant' -count=1 && go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicy|TestAppRouting|TestHTTPNotificationProvider|TestDisabledNotificationProvider' -count=1 && go test ./internal/router/ -run 'TestAgentUpgrade|TestTenantAgentAdoption|TestTenantAgentVariant|TestTenantAgentAdoptionPublishes|TestAvailableAgentsAPIKeyFloor' -count=1
```

退出码 0，六段全部 `ok`：

```
ok  	github.com/Tencent/WeKnora/internal/database	36.971s
ok  	github.com/Tencent/WeKnora/internal/application/repository	24.946s
ok  	github.com/Tencent/WeKnora/internal/application/service	25.218s
ok  	github.com/Tencent/WeKnora/internal/handler	12.269s
ok  	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench	13.343s
ok  	github.com/Tencent/WeKnora/internal/router	31.541s
```

静态检查：`gofmt -l`（修正后为空）、`go vet ./internal/router/`（无输出）均干净。

## 提交

- `762ab07d2` test(marketplace): upgrade proposal end-to-end lifecycle over real migrations (T31 #61 task 6)
  （先以 `fef85067f` 提交，gofmt 对齐修正后 `--amend` 并入，最终 SHA 为 `762ab07d2`；均未推送远端。）

## 文件变更

- `internal/router/routes_agent_upgrade_test.go`：+448 行（唯一改动文件；生产代码零改动，符合计划「零 TS 面 + 不触碰实现层」边界）。

## 自检发现

1. **helper 笔误已修**（见「唯一偏差」）：属计划明示授权的测试侧修正；未改任何 Task 1-5 产出。
2. `oldAgent` 行仅作「已入库」存在性加载（计划原文如此），未做进一步逐字段断言——旧世界的逐字段不变由 `agentsRowByName`（真实 wire）+ available-agents + 旧 Variant 行断言共同承载，覆盖充分。
3. 依赖维在 e2e 断言为空数组：生产依赖解析器 `tenantReleaseDependencyResolver` fail-closed（`internal/container/container.go:120-137`），真实发布路径产生不了非空 DependencyLock；四维全量覆盖由 Task 3 纯函数测试承载（计划差异记录第 3 条），e2e 如实声明该边界，不以 mock 冒充。
4. 报告路径为各任务共用：覆盖前内容为 Task 2 报告（本任务按指令写到同一 `plan-t61.md-report.md`）。

## 疑虑 / 移交事项

- 无阻塞疑虑。AC1/AC2/AC3 证据齐备且全部本地可复现（无真机、无外部凭据依赖）。
- 给 #63/#64 的钩子已在 e2e 中隐式钉住：对账只作用于 active Adoption、终态建议不复活。

---

## 独立复验记录（第二执行会话，2026-09-26 20:42）

Task 6 提交（`762ab07d2`）落盘后，编排器以「第 7/7 任务」再次派发同一任务。第二个执行会话未改动任何代码，独立复跑全部检查，结果与上文一致：

1. **偏差依据复核**：实读 `internal/application/service/agent_adoption.go:131-134`，确认 `CreateVariant` 在 `input.ReleaseID == ""` 时回退 `adoption.AcceptedReleaseID`——计划原文 helper 笔误与拆分修正的论证成立。
2. **GREEN 复验**（Task 6 Step 4 命令逐字，`go test ./internal/router/ -run 'TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks|TestAgentUpgradeRoutesRequireAdminAndFullAccess' -count=1 -v`）：

```
--- PASS: TestAgentUpgradeRoutesRequireAdminAndFullAccess (0.00s)
--- PASS: TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks (2.08s)
PASS
ok  	github.com/Tencent/WeKnora/internal/router	5.606s
```

3. **计划级验证复验**（「计划级验证」节命令逐字）：退出码 0，六段全 `ok`——database 14.994s / repository 15.533s / service 24.176s / handler 1.671s / workbench 1.399s / router 12.096s。
4. **逐测试计数复核**：service 计划新测试 8 个 PASS（TestDiffUpgradeBundles* 4 + TestAgentUpgradeService* 4）、repository 计划新测试 3 个 PASS（TestAgentUpgradeRepository*）、database 4 个 PASS（含 `TestSQLiteMigrationsCreateVersionedSchema`——000120 表 + `uq_agent_upgrade_proposals_scope` 索引随迁移流真实装载）。
5. **静态检查复验**：`gofmt -l`（9 个本计划 Go 文件）为空；`go vet ./internal/router/ ./internal/handler/ ./internal/application/service/ ./internal/application/repository/` 无输出。
6. **HEAD 状态**：`git log` 确认 Task 0-6 七个提交（`96e579ad0`→`3301a1fd8`→`2e75e7882`→`ef43cd83a`→`0a5af2af2`→`7f41cde2f`→`762ab07d2`）依次在 `codex/issue30-t61` 分支上，工作区除本报告文件外 clean。

复验结论：本报告全部声明经独立第二会话实跑确认，无需任何代码变更。

---

## 修复轮 1/5（审查问题处置，2026-09-26）

### 审查问题（important）

> 迁移号顺延未入账且报告两处事实错误：实际迁移为 000120/000200（自 Task 1 提交 3301a1fd8 起），计划写 000119/000198；……唯一存活的 Task 6 报告第 17、142 行仍写「含 000119」……修法为报告补偏差记录并改正两处号段，无需动代码。

### 修复内容（仅报告文件，代码零改动）

1. `:17`「含 000119」→「含 000120」（附顺延指引）。
2. `:142`「000119 表」→「000120 表」。
3. 本节偏差记录补账。

### 偏差记录：迁移号整体顺延 000119→000120 / 000198→000200（补账）

计划 Task 0 注记已预告此情形（「若合并时 000118/000197 已被先集成者占用，按 plan-t48 既有约定『整体顺延、DDL 零变化』处理」），但此前执行会话未把实际顺延写入任何存活的账面文件（Task 1-5 报告被共享路径覆盖），且两处残留旧号构成事实错误。本轮逐项核实（全部为本轮实跑/实读）：

1. **实际号段**：本 worktree 迁移文件为 `migrations/sqlite/000120_agent_upgrade_proposals.{up,down}.sql` 与 `migrations/versioned/000200_agent_upgrade_proposals.{up,down}.sql`；`git show --stat 3301a1fd8` 证实自 Task 1 提交起即为 000120/000200，此后无改名（`git log --follow` 该文件仅 3301a1fd8 一条）。
2. **避让依据真实**：兄弟 worktree `issue30-sweep-t51` 占用 `000119/000198_app_action_plans.*`，`issue30-sweep-t53` 占用 `000119/000198_space_connection_grants.*`——若本计划仍用 000119/000198 即三方碰撞，golang-migrate 将报 duplicate migration file（与 Task 0 同型）。本计划作为后集成方按 plan-t48「整体顺延」约定取下一空闲号。
3. **空号确认**：`migrations/sqlite/` 无任何 `000119_*` 文件、`migrations/versioned/` 无任何 `000199_*` 文件（ls+grep 实测无匹配）——两流各留一个空号位，与审查员「流装载到 version 120、000119/000199 为空号」一致。
4. **DDL 零变化逐字比对**：000120/000200 四文件的 `CREATE TABLE`/`CREATE INDEX`/`CREATE UNIQUE INDEX`/`DROP` 语句与计划 Task 1 Step 3 给出的 000119/000198 DDL 全文逐字一致；唯一差异为 `000120_...up.sql` 首行 twin 注释由「versioned migration 000198」写作「000200」（双号指向自身的顺延自然伴随，非 DDL 变化）。表结构、复合主键 `(id, tenant_id)`、复合外键 `(adoption_id, tenant_id)→agent_adoptions`、唯一索引 `uq_agent_upgrade_proposals_scope(tenant_id, adoption_id, to_release_id)` 全部不变。
5. **下游影响面**：计划内五处测试路径字符串本就不引用 000119/000198（Task 1 schema 测试经 `sqliteMigrationHead` 动态推导头版本，Task 6 e2e 走全量迁移流）；仓储测试直建表（AutoMigrate+显式索引）同样无号段耦合。故顺延只需账面更正，无需任何代码/测试改动。

### 回归覆盖测试（本轮实跑）

修复仅涉报告文件，回归面按计划「计划级验证」节命令逐字复跑（顺带消除审查待核实项 1：handler 段与 workbench 段此前两轮均未由审查员实跑，本轮为第三会话实跑补证）。退出码 0，六段全 `ok`：

```
ok  	github.com/Tencent/WeKnora/internal/database	76.475s        （含 TestSQLiteMigrationsCreateVersionedSchema：000120 表 + uq_agent_upgrade_proposals_scope 索引随真实迁移流装载断言）
ok  	github.com/Tencent/WeKnora/internal/application/repository	47.928s
ok  	github.com/Tencent/WeKnora/internal/application/service	171.914s
ok  	github.com/Tencent/WeKnora/internal/handler	6.277s                 （审查待核实项 1 的 handler 段，本轮实跑全绿）
ok  	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench	18.964s（同上 workbench 段）
ok  	github.com/Tencent/WeKnora/internal/router	168.024s               （含 TestAgentUpgrade 两测试）
```

（本轮时长普遍高于前两轮系后台执行与机器负载所致，与断言强度无关。）

### 审查待核实项处置

1. **handler/workbench 段未实跑** → 本轮实跑全绿（见上），消除。
2. **第二会话 GREEN 计时 2.08s 与首会话 13.02s 差异** → 计时差异源于机器负载与 sqlite 内存库状态，非断言强度差异；本轮计划级验证 router 段（含同一 e2e 测试）再次全绿，与审查员 HEAD 独立复跑（4.50s PASS）三方一致。
3. **RED 首跑输出不可重演** → 维持采信为前一会话记录（工作已提交，本会话不回退他人提交）；其偏差代码依据（`agent_adoption.go:131-134`）已由第二会话实读核实、与失败断言位置内在一致，计划 Task 6 Step 3 明示授权修测试笔误。
4. **顺延决策无第一手记录** → 本节偏差记录即为补账，依据 `3301a1fd8` 提交统计、兄弟 worktree 现状与 DDL 逐字比对反推，反推性质如实注明。

### 修复轮结论

代码零改动；报告两处号段改正 + 顺延偏差记录入册；计划级验证全链（含 handler/workbench 回归段）六段实跑全绿。
