# 25c — Marketplace（公共/租户 agent/skill/expert 市场）实施计划（b2-ac-market）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 agentcatalog 模块 55 文件总盘中归属 25c 的「市场」面（13 个 legacy 文件：公共 agent release/review/发布、SkillHub skill/skillset 市场、租户 skill/expert 市场）中可物理搬迁的 7 个文件迁入 `internal/modules/agentcatalog/{repository,service,handler}`，其余 6 个因 agentruntime 内部包 import 或宿主 handler helper 依赖（Go 包导入环，物理不可行）按已批准先例推迟并逐文件登记裁定请求；全程 `go build ./...` 绿、既有测试经过渡 shim 全绿、633/23+23/58/537 计数零漂移、行为不变，交付差分证据与 Integration Brief。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §4.4（两遍迁移模型/薄别名过渡）、§5.4（Agent Catalog 拥有 Release/Listing/Adoption/Variant、安装和发布记录；「不拥有运行状态」）、§11–§17.2（Pass B 图景）

**节点裁定：** 25c 消费 25a/25b 的 release/install 门面，故串行于两者之后（framework:139）；IB2 按 25a→25b→25c 依赖顺序集成（framework:152）。DAG notes 残留的「BLOCKED（2026-09-23）：前置 b0 阻塞」为历史登记文本——b0 已 done（DAG 实测，见前置条件 1），以 DAG 字段为准（12-commercial 前置条件 1 同口径）。

---

## 0. Spec 与事实源指针（全部为本计划撰写会话实读/实跑/实证）

| 事实源 | 位置 | 本计划取用的内容 |
|---|---|---|
| Spec | `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | §4.4（薄别名过渡、不复制第二份实现）、§5.4（市场面所有权）、§4.2（依赖方向/接口定义在使用方）、§13（提交隔离与回滚）、§14.2/14.3（纯移动验证与高风险差分）、§15（模块 API 最小化） |
| Pass B 框架 | `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` | :135-139（25c=marketplace 面；「25c follows both because it consumes their release/install façades」）、:29（`_test.go` 随迁、host 不留转发性业务声明）、:32（计数基线 633/23+23/58/537）、:38（禁复制实现/塞 common/双注册）、:152（IB2 按 25a→25b→25c 集成） |
| 执行公约 | `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | §1 派发契约（只改 owned_files、跨 owner 调用点上报、TDD、随迁测试）、§1.1/1.2 报告路径与命令、§2 门禁、§3 禁改清单、§4 提交规范、§5 升级契约、§6 高风险差分证据、§7 package-private 耦合（§7.1 isUniqueViolation 族 barrier 收口）、§8 计数基线、§9 DAG 字段仅协调者可写 |
| B0 冻结产物 | `.worktrees/passb-int/docs/architecture/passb/contracts.yaml` | `agentcatalog.marketplace-service`（:78-89，frozen，5 方法签名 + consumers 含 container.go）、`agentcatalog.skill-market-service`（:119-129）、`agentcatalog.tenant-skill-market-service`（:130-140）、`agentcatalog.agent-version-service`（:9-23，**25a release 门面**，consumers 含 `service/agent_marketplace.go` 与 `handler/agent_marketplace.go`）、`agentcatalog.routes`（:90-118，11 入口中 4 条属本面：RegisterAgentMarketplaceRoutes :112 / RegisterSkillMarketRoutes :116 / RegisterTenantSkillMarketRoutes :117 / RegisterTenantExpertMarketRoutes :118） |
| B0 冻结产物 | 同目录 `ownership-matrix.yaml` | agentcatalog 56 行全部 `plan: 25-agentcatalog-program`（无 25a/25b/25c 子行，子面拆分由本计划 §2.1 表提交协调者确认，25a §2.1 同律）；本面 13 文件行 `integration_owner: ib2`、`delete_barrier: ib2`；文件头 B0.3 裁定 4：`upload_limit.go`/`list_pagination.go` 保持 platform、任何子计划不得认领 |
| B0 冻结产物 | 同目录 `exception-ledger.yaml` | 105 条例外中 **agentcatalog/marketplace 相关行 = 0**（本会话 grep `agent_marketplace\|skill_market\|expert_market\|expert_install\|published_skill\|published_expert` 零命中）→ 本节点零例外删除/新增义务 |
| B0 冻结产物 | 同目录 `event-catalog.yaml` | 无 25c 面事件（唯一 agentcatalog 相关行为 :113 `conversation.turn.appended` consumer `tenant_skill_transcript.go`，属 25b）→ 本节点零事件义务 |
| MOVE-MANIFEST | `docs/architecture/moves/agentcatalog.yaml` | 55 条 legacy_files（本面 13 行见 §2.1）、`alias_obligations: []`、11 路由入口、`forbidden_shared_files`；README:11 schema LOCKED（`KnownFields(true)`）→ 无「已搬迁」标记字段，物理搬迁后原路径必须保留（§3 机制，25a §2.3-③ 同型） |
| DAG | `.worktrees/passb-int/docs/plans/passb/execution-dag.json` 节点 `b2-ac-market` | owned_files（manifest 25c 行 + `internal/modules/agentcatalog/**` marketplace 面）、required_contracts（25a release 门面、25b install/verify 门面）、gates 四条、produced_artifacts、evidence_paths |
| 前置节点 | DAG `b2-ac-definition`（status=done，分支头 `938087598`）、`b2-ac-skills`（status=done，分支头 `5ed64d324`）+ 两分支 worktree 实测 | 25a 已搬 7 文件入模块（`internal/modules/agentcatalog/{repository,service,handler}/` 实测存在）+ 7 个宿主 shim；25b 已搬 20 生产文件入模块 + 宿主残差（`internal/application/service/tenant_skill_service.go` 残差实测含 `CatalogInstallResult`/`newKeyedMutex`/`zipSkillFiles` 转发，注释点名「25c 市场服务文件（skill_market_service.go 等，禁改）」） |
| 同型先例 | `docs/plans/passb/25a-agent-definition-version.md`（v3，已实施）、`25b-skill-catalog-install.md`（已实施）、`12-commercial.md` §5.4 | shim 机制（模块新文件 1:1 + 原路径薄 shim）、推迟台账格式（25a §4-② 13 行）、构造器注入（25b §4.3 HostAdapters）、`repoRoot` 深度修正（25a §4-②#4b）、同层哨兵成环论证（25a §2.3） |
| 本会话 grep/实跑实证 | §4 全表每一行（13 文件 import 块、未导出符号调用点、宿主消费方全量清单）+ 基线门禁实跑（见前置条件 4） | 拆批判定、shim 逐符号清单、预期输出 |

---

## 1. 前置条件（实施前逐条核验，任一不满足即按 conventions §5 上报，不得现场改判）

1. **上游节点 done**：DAG 实测 `b0: done`（head `d57a2fa70`，B0 四产物固化）、`b2-ac-definition: done`（分支 `codex/passb-b2-ac-definition` 头 `938087598`）、`b2-ac-skills: done`（分支 `codex/passb-b2-ac-skills` 头 `5ed64d324`）。注意：截至本计划撰写（2026-09-24），集成头 `e586552d1` **尚未包含** 25a/25b 合并——派发时协调者必须把 `b2-ac-market.base_sha` 指向已含两者模块树的头（conventions §9：base_sha 派发时回填）。
2. **25a/25b 门面存在性（required_contracts 的可执行验证，T1 Step 3 实跑）**：25a release 门面 = `internal/types/interfaces/agent_version.go:AgentVersionService` 冻结三方法（contracts.yaml :13）+ 实现留宿 `internal/application/service/agent_version.go`（25a 批次 2 推迟件，宿主消费方经 `routes_agent_marketplace_test.go:57` 等原样编译）；25b install/verify 门面 = 模块包 `internal/modules/agentcatalog/service/`（`tenant_skill_service.go` 的 `CatalogInstallResult`/`InstallCatalogToConfigs`、`tenant_skill_install.go` 安装状态机）+ 宿主残差 `internal/application/service/tenant_skill_service.go`（类型别名族 + `NewTenantSkillService` 12 参残差构造器）。核验命令：`test -f internal/modules/agentcatalog/service/tenant_skill_install.go && test -f internal/application/service/tenant_skill_service.go && grep -q "CatalogInstallResult = " internal/application/service/tenant_skill_service.go && echo FACADES-OK`（预期 `FACADES-OK`）；缺失即 §5 blocked 上报，**不自行实现上游门面**（conventions §5 尾条）。
3. **写权边界认知**（conventions §1.2/§3）：本节点只写 §2.3/§2.4 白名单文件；`internal/router/**`、`internal/container/container.go`、`internal/bootstrap/**`、`migrations/`、`go.mod`、`go.sum`、`tools/**`、四份治理 YAML、`docs/architecture/moves/*.yaml`（schema LOCKED）、`internal/modules/agentcatalog/module.go`（门面归 IB2）、25a/25b 已搬文件与残差、6 个推迟文件（§2.1 推迟表）一律禁写；禁止新增任何例外行。
4. **基线门禁绿（本会话在 market worktree `e586552d1` 未改动工作树实跑记录；实施者须在派发基线复跑同命令）**：
   - `go build ./...` → 退出码 0（仅 cmd/server、cmd/desktop 的 `ld: warning: ignoring duplicate libraries: '-lc++'` 链接告警，非错误）；
   - `go test -count=1 ./internal/application/repository -run 'TestAgentMarketplace|TestExpertInstall|TestPublishedExpertRepo|TestPublishedSkillRepo'` → `ok ... 7.864s`（21 用例）；
   - `go test -count=1 ./internal/application/service -run 'TestAgentMarketplaceSubmit|TestAgentMarketplaceReview|TestAgentMarketplaceApproval|TestAgentMarketplaceConcurrent|TestAgentMarketplaceExclusive|TestAgentMarketplaceCatalog|TestTenantMarket'` → `ok ... 0.875s`（17 用例，`-v` 计数实测）；
   - `go test -count=1 ./internal/router -run 'TestTenantAgentMarketplaceLifecycleAndAuthorization'` → `ok ... 1.093s`；
   - `go test -count=1 ./internal/modules/agentcatalog/...` → `ok`（e586552d1 实测 `[no test files]`；派发基线含 25a/25b 随迁测试后应为全 ok——以复跑输出为准）；
   - `make check-backend-architecture` → `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` + `OK (0 violations)`；
   - `make verify-module-moves` → `modulemove: OK (16 manifests verified)`。
5. **拆分确认状态**：ownership-matrix 56 行 `plan: 25-agentcatalog-program` 无子行；本计划 §2.1 的 25c=13 文件拆分表与 25a 计划 §2.1（22/20/13）一致，随 T5 Brief 提交协调者在 `25-agentcatalog-program.md` 确认（矩阵行级回写归 barrier/协调者，conventions §3）。

---

## 2. 范围、文件清单与写所有权

### 2.1 25c=13 文件拆分与批次裁定（7 搬迁：原路径重写为 7 个同路径 shim + 6 推迟）

**搬迁批次（7 生产 + 6 随迁测试；判定标准 = 25a §2.2 三条：(a) 搬入模块树后零新增跨模块内部包 import；(b) 零同层宿主包未导出符号引用（成环）；(c) 具体类型/方法接收者不被留宿文件引用）：**

| # | 层 | 文件（原路径 → 模块路径） | 随迁测试（framework:29） | 判定要点（本会话实证） |
|---|---|---|---|---|
| 1 | repository | `internal/application/repository/agent_marketplace.go` → `internal/modules/agentcatalog/repository/agent_marketplace.go` | `agent_marketplace_test.go`（339 行，9 用例；`:238` 自包含 `repoRoot`，随迁须 3 层改 4 层 `../../../..`，25a §4-②#4b 实跑先例） | 唯一宿主耦合 `isUniqueViolation`@`voice_session.go:289`（调用点 :179）→ 模块本地副本（§4-①#1），不构成环 |
| 2 | repository | `internal/application/repository/expert_install.go` → 模块 repository | `expert_install_test.go`（123 行，4 用例，自包含 types/uuid/testify） | import 仅 context/errors/types/gorm/clause，零耦合 |
| 3 | repository | `internal/application/repository/published_skill.go` → 模块 repository | `published_skill_test.go`（110 行，4 用例，自包含） | 同上零耦合 |
| 4 | repository | `internal/application/repository/published_expert.go` → 模块 repository | `published_expert_test.go`（124 行，4 用例，自包含） | import 仅 context/errors/types/gorm/clause，零耦合 |
| 5 | service | `internal/application/service/tenant_skill_market_service.go` → 模块 service | `tenant_skill_market_service_test.go`（437 行，9 用例，import 仅 apperrors/types/interfaces/testify，fake 自包含，**零改动随迁**） | `CatalogInstallResult`（:41）经 25b 搬迁后即模块 service 同包符号；`repository.PublishedSkillRepository`（:46）随 #3 同批同包；无 agentruntime import、无宿主裸标识符 |
| 6 | service | `internal/application/service/agent_marketplace.go` → 模块 service | `agent_marketplace_test.go`（331 行，8 用例；`:14` import `agentruntime/agent/experts` —— guard 豁免 `_test.go`（check.go:1182，25b §2.4 先例），随迁可保留；8 处 `NewAgentMarketplaceService` 调用点（:135/:152/:162/:174/:192/:272/:307/:327，`grep -c` 实测）加 adapters 实参，`:309` `svc.publishNoReplace` 同包访问不受影响） | 唯一 agentruntime 耦合 `experts.BuildAgentReleaseBundle`（:60）签名全为 `internal/types` 公共类型（`agent_release.go:64-69` 实测）→ 单字段注入（§4-③），零数据视图映射 |
| 7 | handler | `internal/handler/agent_marketplace.go` → `internal/modules/agentcatalog/handler/agent_marketplace.go` | （无既有测试；HTTP 面由宿主 `internal/router/routes_agent_marketplace_test.go` 覆盖，经 shim 零改动继续跑） | 请求体上限自包含（`:21` 本文件常量 + `:160/:193` 直接 `http.MaxBytesReader`，不依赖 `upload_limit.go`）；`marketrepo.`/`marketservice.` 哨兵引用（:119-125）随 #1/#6 转模块包内引用；`sandboxConfigTenantID` ×4（:171/:180/:204/:217）→ 本地一行等价 helper（§4-④，25b §4.6 `skillTenantID` 先例） |

**推迟批次（6 文件，本节点不搬，逐文件登记 + 裁定请求，见 §4-②）**：service `{skill_market_service, tenant_expert_market_service, expert_market_source}.go`、handler `{skill_market, tenant_expert_market, tenant_skill_market}.go`。推迟不是 scope 缩水，而是与已批准先例同型的物理约束（25a §2.2 批次 2 推迟 15/22 文件且节点 done；10-identity §3.2、13-execution:65 同律）。宿主现状：这 6 个文件的依赖（`skillhub.*`/`experts.*`、`expert.go` 三 helper、`upload_limit.go` 平台 helper、`ErrAgentNotFound`、`ExpertSource`）在 25c 时点全部留宿，模块侧 import 宿主 handler/service 即成环（§4-② 论证）。

（7 搬迁 + 6 推迟 = 13，与 25a 计划 §2.1 的 25c 拆分表一致：repo `{agent_marketplace,expert_install,published_expert,published_skill}` + service `{agent_marketplace,expert_market_source,skill_market_service,tenant_expert_market_service,tenant_skill_market_service}` + handler `{agent_marketplace,skill_market,tenant_expert_market,tenant_skill_market}`。）

### 2.2 搬迁机制：模块新文件 1:1 + 原路径薄 shim（25a §2.3 / 12-commercial §5.4 同型）

每个搬迁文件拆为两个写动作：

1. **新建** `internal/modules/agentcatalog/{repository,service,handler}/<name>.go`：内容 = 原文件 1:1（零逻辑改动），仅 package 名、import 路径与 §4 列明的注入点/本地化 helper 调整。
2. **原路径文件重写为薄 shim**：仅含零逻辑别名/转发（type 别名、构造器 var/转发函数、哨兵 var 转发），逐符号清单 = §5 shim 表（按本会话 `grep -rln` 全仓消费方审计精确圈定），文件头注释注明「Pass B (25c) 过渡 shim — 消费方由 IB2 切换后删除；禁止新增业务逻辑 — remove_at: ib2」。

三重门禁一致性（25a §2.3 已论证，此处复用）：原 legacy 路径继续存在 → `make verify-module-moves` 的 legacy `os.Stat`（tools/modulemove/verify.go:327-334）通过；宿主消费方（container/router/留宿推迟件/宿主测试）经别名继续编译 → `go build ./...` 通过；shim 是宿主→模块方向的纯别名文件 → architectureguard forbidden-import（仅扫 `internal/modules/**`，check.go:1175-1219）与 legacy-guard 通过。

方向合法性（25a §2.3 引 12-commercial.md:302 已裁定）：宿主包 → 模块包 import 合法（25a 七个 shim、25b 残差、25b 模块 service import 宿主 repository 的 `TenantSandboxConfigRepository` 均为在案先例）；**模块包 import 宿主 handler/service 不可行**——宿主 handler 已含 25a shim（`user_resource_favorite.go`/`subagent.go`）、宿主 repository 已含 25a shim、宿主 service 已含 25b 残差，均 import 模块包，模块侧反向 import 即成环（§4-② 推迟裁定的物理依据）。

### 2.3 本节点物理产出（新文件白名单，`git diff --name-only` 差集核对基准）

- 模块侧新建/随迁：`internal/modules/agentcatalog/repository/{agent_marketplace,expert_install,published_skill,published_expert}.go` + 4 个同名 `_test.go` + `is_unique_violation_parity_test.go`（新写，§5）；`internal/modules/agentcatalog/service/{agent_marketplace,tenant_skill_market_service}.go` + 2 个同名 `_test.go`；`internal/modules/agentcatalog/handler/agent_marketplace.go`。
- 宿主 shim 重写（原路径，§5 清单）：`internal/application/repository/{agent_marketplace,expert_install,published_skill,published_expert}.go`、`internal/application/service/{agent_marketplace,tenant_skill_market_service}.go`、`internal/handler/agent_marketplace.go`。
- 文档产出：`docs/architecture/evidence/passb/b2-ac-market.md`、`docs/architecture/passb/briefs/b2-ac-market.md`、`docs/plans/passb/reports/b2-ac-market.md`（目录不存在则创建）。审查报告 `docs/plans/passb/reviews/b2-ac-market.md` 归审查者（conventions §1.1）。

### 2.4 禁改清单（违反即节点失败）

conventions §3 全文适用。本节点实证涉及的特别项：`internal/router/router.go`（:100-112 handler 字段、:396/:402-404 注册行）、`internal/router/routes_{agent_marketplace,skill_market,tenant_skill_market,tenant_expert_market}.go`（frozen `agentcatalog.routes` 4 条入口，经 shim 类型别名保持编译，一个字节不改）、`internal/container/container.go`（:295-301/:425-429/:785/:813-851 Provide 行原样）、推迟的 6 个文件及其宿主测试（`routes_agent_marketplace_test.go` 经 shim 零改动）、25a 已搬 7 文件与 shim、25b 已搬文件与残差、`internal/application/service/{expert_service,custom_agent}.go` 宿主文件（25a 批次 2 属主：`expert_service.go:51` 定义 `ExpertSource`、`custom_agent.go:21` 定义 `ErrAgentNotFound`）、`internal/handler/{expert,sandbox_config,sandbox_skill,upload_limit}.go`（25a 推迟件/execution 属主/platform，B0.3 裁定 4）、`internal/application/repository/voice_session.go`（40-workbench 属主）、`internal/modules/agentcatalog/module.go`（IB2）。

---

## 3. 目标包结构与真实 Go 接口签名（不发明；坐标为本会话实测）

```text
internal/modules/agentcatalog/
  repository/               # package repository（25a/25b 已建；import 别名 acrepo 为宿主侧惯例）
    agent_marketplace.go    # 6 哨兵 :17-24、AgentMarketplaceRepository 接口 :28-37、reviewAndPublishAttempts :26、
                            # agentMarketplaceRepository :51 + NewAgentMarketplaceRepository :53 + 全部方法 1:1 随迁；
                            # isUniqueViolation 改调模块本地副本（§4-①#1）；isReleaseNumberCollision/isTransientDatabaseContention/
                            # findReviewResult 随文件同包，零改动
    expert_install.go       # ExpertInstallRepository 接口 + NewExpertInstallRepository(db *gorm.DB) 1:1
    published_skill.go      # PublishedSkillRepository 接口 + NewPublishedSkillRepository(db *gorm.DB) 1:1
    published_expert.go     # PublishedExpertRepository 接口 + NewPublishedExpertRepository(db *gorm.DB) 1:1
    is_unique_violation_parity_test.go   # 新写（§5），remove_at: ib2
  service/                  # package service（25b 已建）
    market_host_adapters.go # 新增：MarketHostAdapters（§4-③，单注入位 + nil fail-fast）
    agent_marketplace.go    # 3 哨兵 :22-26、AgentMarketplaceService :28-37（`var _` 断言 :38，增 adapters 字段）、
                            # NewAgentMarketplaceService 5 参定稿（§4-③）、:60 调用点改 s.adapters.BuildReleaseBundle；
                            # SubmitRelease/ListReviewQueue/ReviewSubmission/ListTenantCatalog/GetRelease、
                            # lockReleaseBundle :185/stageReleaseBundle :226/ensurePayloadReferencesLocked :84/
                            # validMarketplaceDigest :218 逐字随迁
    tenant_skill_market_service.go  # TenantSkillMarketCatalogStore :32-36 / TenantSkillMarketInstaller :40-42（未导出接口，
                            # spec §15 API 最小化）/ TenantSkillMarketService :45-53 / NewTenantSkillMarketService(4 参，签名不变)
                            # :60-65 / withClock :77 全部 1:1 随迁；CatalogInstallResult（:41）恢复同包直接引用
  handler/                  # package handler（25a/25b 已建）
    agent_marketplace.go    # AgentMarketplaceHandler :26-29、NewAgentMarketplaceHandler(market interfaces.AgentMarketplaceService,
                            # versions interfaces.AgentVersionService) :31-33、AgentIDForVersion :37、marketplaceClientError :117
                            # （哨兵改引 acrepo/acatsvc 同一 var 实体）、decodeAgentMarketplaceBody :139 等 1:1 随迁；
                            # sandboxConfigTenantID(c) 调用点 ×4 改本地 marketTenantID（§4-④）
```

**冻结契约兼容义务（contracts.yaml，签名逐字不变）**：

- `agentcatalog.marketplace-service`（:82）：`SubmitRelease(ctx, tenantID uint64, actorID, versionID string, input SubmitReleaseInput) (ReleaseSubmissionView, error)`、`ListReviewQueue(ctx, tenantID uint64) ([]ReleaseSubmissionView, error)`、`ReviewSubmission(ctx, tenantID uint64, actorID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (ReleaseReviewResult, error)`、`ListTenantCatalog(ctx, tenantID uint64) ([]TenantListingView, error)`、`GetRelease(ctx, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error)`——接口定义留 `internal/types/interfaces/agent_marketplace.go` 不动，实现随 #6 搬迁（`:38` `var _ interfaces.AgentMarketplaceService` 断言随迁）。
- `agentcatalog.agent-version-service`（:13，25a release 门面，**本节点只消费不改**）：`#6`/`#7` 的 `versions interfaces.AgentVersionService` 字段与 `GetAgentVersion` 调用（service :49、handler :38）零改动。
- `agentcatalog.tenant-skill-market-service`（:134，**消费 25b install 门面**）：`PublishSkill/UnpublishSkill/ListPublishedSkills/InstallPublishedSkill` 四方法随 #5 搬迁；`TenantSkillMarketInstaller.InstallCatalogToConfigs`（:41）返回 `*CatalogInstallResult`——模块包内该类型即 25b 搬迁件（`tenant_skill_catalog.go` 定义），实现方 `*TenantSkillService` 结构满足关系不变。
- `agentcatalog.skill-market-service`（:123）：实现文件 `skill_market_service.go` **推迟**（§4-②#1），接口留 `internal/types/interfaces/skill_market.go` 不动，本节点零触及。
- `agentcatalog.routes` 4 条入口（:112/:116-118）与 633 总数：经 §4-⑤ handler shim 类型别名，路由面零漂移（T5 用 `make check-backend-architecture` 断言 `total=633`）。
- `agentcatalog.facade/lifecycle/workers`：module.go 不写（IB2）；本面无 worker/生命周期义务（manifest `workers: []`，`agentcatalog.workers` items 空）。

**公共可观察行为兼容要求**：全部 HTTP 路由/方法/状态码/RBAC 守卫/错误码映射（`marketplaceClientError` 的 400/403/409/422 语义 :117-127）、release 发布事务语义（CAS 指针 + `reviewAndPublishAttempts=5` 重试 + 文件锁 + 补偿删除 :157-177）、bundle 文件系统布局（`<bundleRoot>/tenant-<id>/releases/<submissionID>/<digest>`，:189/:237）、`errors.Is` 哨兵判等链（经 var 转发保持同一 var 实体）、租户隔离与 404 防枚举——全部不变；搬迁 = 模块新文件 1:1 内容 + 原路径零逻辑 shim（spec §4.4/§7 纪律）。

---

## 4. 耦合与断链面全量表（本会话扫描实证；符号/行号均可复核）

### ① 搬迁文件的对外依赖与就地解决（迁入后合法）

| # | 文件 | 依赖（实证） | 解决 |
|---|---|---|---|
| 1 | `repository/agent_marketplace.go` | `isUniqueViolation`@`internal/application/repository/voice_session.go:289`（40-workbench 留宿）← 调用点 :179（`ReviewAndPublishTx` 冲突重试路径）；import 仅 context/json/errors/fmt/strings/time/uuid/gorm/types | **模块本地副本**：在模块 repository 包声明同包未导出 `isUniqueViolation(err error) bool`，函数体逐字复制 voice_session.go:289-300（`gorm.ErrDuplicatedKey` + `"UNIQUE constraint failed"`/`"duplicate key value"`/`"23505"` 三标记）。族登记：这是第 4 份副本（既有三份 = voice_session.go:289（40-workbench）、`internal/application/service/resource.go:358`（11-airesource）、`internal/modules/commercial/repository/commercial/planversion.go:323`（12-commercial 模块本地副本，Pass A 在案）），conventions §7.1 收口属主 = IB2（`remove_at: ib2`，Brief 登记 + parity 测试防漂移（§5 新写项））。禁 import 宿主 repository——宿主 repository 已含 25a shim（import 模块 repository），反向即环（§2.2） |
| 2 | `repository/{expert_install,published_skill,published_expert}.go` | import 仅 context/errors/types/gorm/clause（实测三文件 import 块） | 零调整 |
| 3 | `service/tenant_skill_market_service.go` | `repository.PublishedSkillRepository`（:46，随 #3 同批入模块 repository）；`CatalogInstallResult`（:41、:269-276，25b 搬迁后为模块 service 同包符号）；`interfaces.TenantSkillPublisherNames`/`TenantSkillMarketService`、`types.*`、apperrors、logger、uuid | import 改 `acrepo "…/internal/modules/agentcatalog/repository"`，`repository.` 前缀改 `acrepo.`；其余零调整 |
| 4 | `service/agent_marketplace.go` | `experts.BuildAgentReleaseBundle`（:60）← `internal/modules/agentruntime/agent/experts`（guard forbidden-import，check.go:1210-1215）；`repository.ErrAgentMarketplaceNotFound`（:132/:148）；`interfaces.AgentVersionService`（:29 字段，frozen 25a 门面）、`ReleaseDependencyResolver`（:30）、`AgentMarketplaceRepository`（:31）；`os.Link`/`syscall.Flock`/stdlib | **BuildReleaseBundle 注入**（§4-③）；`repository.` 改 `acrepo.`；`interfaces.*`/`types.*` 公共契约零改动 |
| 5 | `handler/agent_marketplace.go` | `marketrepo.ErrAgentMarketplaceNotFound/DigestMismatch/PointerConflict/ReviewConflict/InvalidDecision/VersionAgentMismatch`（:119-125，6 哨兵随 #1 入模块 repository）；`marketservice.ErrAgentMarketplaceStaleDigest/MissingDependency/InvalidInput`（:121-125，3 哨兵随 #6 入模块 service）；`sandboxConfigTenantID`@`sandbox_config.go:92`（execution 属主，留宿）← 调用点 :171/:180/:204/:217 | 哨兵改 `acrepo.`/`acatsvc.` 同包跨层引用（同一 var 实体，`errors.Is` 不变）；`sandboxConfigTenantID(c)` → 本地 `marketTenantID`（§4-④） |

### ④ 本地化 helper（25b §4.6 `skillTenantID` 同型先例）

模块 handler 包新增（置于随迁的 `agent_marketplace.go` 内或 handler 包共享文件，随 T4 commit）：

```go
// marketTenantID 与宿主 sandboxConfigTenantID（internal/handler/sandbox_config.go:92，
// execution 属主留宿）同一公共键表达式，无业务逻辑；宿主函数不随迁故本地等价重述。
func marketTenantID(c *gin.Context) uint64 { return c.GetUint64(types.TenantIDContextKey.String()) }
```

### ② 推迟件登记表（T5 Brief 的核心正文；每行 = 硬阻塞 + 约束类别 + 建议裁定；行项可复核）

| # | 推迟文件 | 硬阻塞（符号 @ 定义处 ← 调用点，本会话实证） | 约束类别 | 建议裁定（供协调者/IB2/B3） |
|---|---|---|---|---|
| 1 | `service/skill_market_service.go`（519 行） | `skillhub.Client/SkillsetClient` 接口与 `SkillSummary/SkillsetSummary/ZipFiles` 数据类型、`skillhub.New/NewCached/DefaultCacheTTL/ParsePackage/MarketExpertID` 与 7 哨兵（`ErrStaleOnly:142,150,315,356`/`ErrUnreachable:168`/`ErrUnsupportedRanking:172`/`ErrInvalidSkillsetSlug:175,386`/`ErrPackage:294,441,495,498`/`ErrSkillsetNotFound:383`/`ErrMarket:495`）@`agentruntime/agent/skills/skillhub`（import :24）；`experts.MaterializeSkillset:434/MarketInstallDir:450/MarketDataRoot:98/WriteMaterializedExpert:451`（import :23）；`uniqueNonEmptyStrings`@`session_knowledge_qa.go:638-648`（conversation，35b 留宿）← :197 | guard forbidden-import（agentruntime 内部包，25a §4-②#9 同类）+ 跨 owner 未导出符号（成环）；**数据形状面宽**：接口方法签名携带 `SkillSummary/SkillsetSummary/ZipFiles/MaterializedExpert`，注入需模块侧视图类型 + 逐字段映射，漂移风险高于 25b 单函数注入 | 优先：B3 R 面（33/34-agentruntime）对 `agentruntime/agent/experts`、`…/skills/skillhub` 根门面 re-export 消费符号（25a §4-②#9-12 已登记的同型裁定通道）；备选：另立串行契约任务按 25b HostAdapters 模式做宽视图注入（含 uniqueNonEmptyStrings 注入位）；推迟期间宿主文件原样跑绿，`skillhub`/`experts` import 在宿主包合法 |
| 2 | `service/tenant_expert_market_service.go`（339 行） | `experts.MaterializePublishedAgent:125/PublishedAgentExport:125-129/PublishedSnapshotDir:138,299/WriteMaterializedExpert:139,313/TenantExpertID:233,298/LoadExpertDir:299/MaterializedExpert:306/MarketInstallDir:312`（import :27）；`ErrAgentNotFound`@`custom_agent.go:21`（25a 批次 2 留宿）← :116 | 同 #1 + 同层哨兵成环（25a §4-②#4b 与 `agent_version.go:107` 完全同型） | 排序约束：`custom_agent.go`(service) 属 25a 批次 2（IB2 内 25b 合并后搬迁，25a §4-②#4），同包化后哨兵恢复同包可见；experts 面同 #1 re-export/注入裁定；两者齐备前留宿 |
| 3 | `service/expert_market_source.go`（32 行） | `ExpertSource`@`expert_service.go:51`（25a 批次 2 留宿，接口断言 `:32 var _ ExpertSource`）；`experts.InstalledExperts:29` 与返回类型 `[]*experts.Expert`（import :6）；消费方 `container.go:785` | 同层接口成环 + 方法签名携带 agentruntime 数据类型 `experts.Expert`（注入无法消除签名本身） | 随 `expert_service.go` 搬迁（25a 批次 2，IB2/IB3）+ experts 面 re-export 后同包化；期间留宿零改动 |
| 4 | `handler/skill_market.go`（262 行） | `limitJSONBody`/`skillSourceJSONMaxBytes`@`upload_limit.go:33/:19`（platform，B0.3 裁定 4 不得认领）← :124/:218；`isRequestBodyTooLarge`@`upload_limit.go:55` ← :127/:221；`skillJSONRequestTooLargeError`@`sandbox_skill.go:458` ← :128/:222；`decodeExpertInstantiateBody:178`/`maxExpertAgentNameLen:23`@`expert.go`（25a 批次 2 留宿）← :219/:228；`sandboxConfigTenantID`@`sandbox_config.go:92` ← :135/:164/:183 | **模块 handler ← 宿主 handler 物理环**：宿主 handler 已含 25a shim（import 模块 handler），本文件搬入模块后引用上述宿主 handler 包符号即环，物理不可行（§2.2） | expert.go 三 helper 随 25a handler 批次 2 搬迁同包化（IB2/IB3）；upload_limit 四 helper 须等「独立消费者抽取计划」（ownership-matrix B0.3 裁定 4 明文：任何子计划不得认领、直至另行裁定）产出 platform 共享位或冻结导出；齐备前留宿 |
| 5 | `handler/tenant_expert_market.go`（218 行） | 同 #4 全部（:100-110/:185-195）+ `expertServiceError`@`expert.go:260`（其体内引用宿主 `service.ErrExpertNotFound/ErrAgentNameRequired`，expert_service.go 系 25a 批次 2）← :202；`sandboxConfigTenantID` ← :122/:141/:158/:200 | 同 #4 | 同 #4 |
| 6 | `handler/tenant_skill_market.go`（209 行） | `limitJSONBody/skillSourceJSONMaxBytes` ← :102/:178；`isRequestBodyTooLarge` ← :104/:181；`skillJSONRequestTooLargeError` ← :105/:182；`sandboxConfigTenantID` ← :114/:133/:150/:189 | 同 #4（expert.go 依赖仅经 routes 层间接，本文件直接依赖为 platform/sandbox 系） | 同 #4（upload helper 抽取裁定后可先行，expert.go 依赖无） |

**推迟批次的反向义务核对（25a 计划 §4-②尾段登记的消费面，本节点维持现状零改动）**：`handler/skill_market.go:219` + `handler/tenant_expert_market.go:186,202` 等对 `expert.go` 三 helper 的消费按 25a Brief 原登记继续由宿主同包满足；`skill_market_service.go:33,216`、`tenant_skill_market_service.go:41` 对 `service.CatalogInstallResult` 的引用——后者随 #5 搬入模块包（同包直引），前者经 25b 残差别名（`internal/application/service/tenant_skill_service.go` 实测 `CatalogInstallResult = acatsvc.CatalogInstallResult`）继续编译，IB2 删残差时随 #1 推迟件处置一并收口。

### ③ `service/agent_marketplace.go` 注入定稿（真实签名，全 `internal/types` 公共类型，零映射）

`experts.BuildAgentReleaseBundle` 实测签名（`internal/modules/agentruntime/agent/experts/agent_release.go:64-69`）：

```go
func BuildAgentReleaseBundle(version types.AgentVersionSnapshot, input types.ReleaseMetadata, lock types.DependencyLock) (types.AgentReleaseBundle, error)
```

新增 `internal/modules/agentcatalog/service/market_host_adapters.go`（25b `host_adapters.go` 同型）：

```go
// MarketHostAdapters 承接旧宿主包内仍留驻的 agentruntime 能力位（25b HostAdapters
// 同型；IB2/B3 按 re-export 裁定收口）。字段语义与被替换符号 1:1，nil 即 fail-fast。
type MarketHostAdapters struct {
    // BuildReleaseBundle 替代 experts.BuildAgentReleaseBundle（agent_release.go:64，
    // 纯函数：无盘无网）。宿主残差构造器绑真源 experts.BuildAgentReleaseBundle。
    BuildReleaseBundle func(version types.AgentVersionSnapshot, metadata types.ReleaseMetadata, lock types.DependencyLock) (types.AgentReleaseBundle, error)
}
```

模块构造器定稿（原 4 参 + 1 尾参；`AgentMarketplaceService` 增未导出字段 `adapters MarketHostAdapters`，构造器内校验 `BuildReleaseBundle != nil` 否则报错，25b fail-fast 同律）：

```go
func NewAgentMarketplaceService(versions interfaces.AgentVersionService, resolver interfaces.ReleaseDependencyResolver, repo interfaces.AgentMarketplaceRepository, bundleRoot string, adapters MarketHostAdapters) *AgentMarketplaceService
```

调用点改写（函数体最小变更，逻辑不变）：`agent_marketplace.go:60` `bundle, err := experts.BuildAgentReleaseBundle(version, input.Metadata, lock)` → `bundle, err := s.adapters.BuildReleaseBundle(version, input.Metadata, lock)`。消费的 bundle 字段（`Manifest`/`Lock`/`Payload`/`Bytes`/`SHA256`）全为 `types.AgentReleaseBundle` 公共字段，无视图类型。

宿主 shim 保持旧 4 参签名（container.go:425-426 与 `routes_agent_marketplace_test.go:59` 零改动），并绑真源：

```go
func NewAgentMarketplaceService(versions interfaces.AgentVersionService, resolver interfaces.ReleaseDependencyResolver, repo interfaces.AgentMarketplaceRepository, bundleRoot string) *AgentMarketplaceService {
    return acatsvc.NewAgentMarketplaceService(versions, resolver, repo, bundleRoot,
        acatsvc.MarketHostAdapters{BuildReleaseBundle: experts.BuildAgentReleaseBundle})
}
```

（`experts` import 在宿主 service 包合法——现状 `skill_market_service.go:23` 在案。）

### ⑤ 宿主 shim 逐符号清单（`grep -rln` 全仓消费方审计，含宿主测试；本节点唯一兼容面）

| 原路径（重写为 shim） | shim 符号（= 模块包同名符号的别名/转发） | 消费方（全部实证） |
|---|---|---|
| `internal/application/repository/agent_marketplace.go` | `type AgentMarketplaceRepository`（接口别名）；`var NewAgentMarketplaceRepository`；`var ErrAgentMarketplaceDigestMismatch/ErrAgentMarketplacePointerConflict/ErrAgentMarketplaceNotFound/ErrAgentMarketplaceInvalidDecision/ErrAgentMarketplaceReviewConflict/ErrAgentMarketplaceVersionAgentMismatch`（哨兵转发，保 `errors.Is` 同一性） | container.go:300-301；宿主消费方仅此（handler/agent_marketplace.go 与 service/agent_marketplace.go 均随迁、测试随迁；`routes_agent_marketplace_test.go:51` 经 `repository.` 前缀命中本行别名） |
| `internal/application/repository/expert_install.go` | `type ExpertInstallRepository`；`var NewExpertInstallRepository` | container.go:295、:813、:844（Provide 形参与闭包参数）；推迟件 `skill_market_service.go:49`/`tenant_expert_market_service.go:43` 字段类型 |
| `internal/application/repository/published_skill.go` | `type PublishedSkillRepository`；`var NewPublishedSkillRepository` | container.go:296、:829；推迟件无（#5 随迁） |
| `internal/application/repository/published_expert.go` | `type PublishedExpertRepository`；`var NewPublishedExpertRepository` | container.go:297、:843；推迟件 `tenant_expert_market_service.go:42` |
| `internal/application/service/agent_marketplace.go` | `func NewAgentMarketplaceService`（4 参转发，§4-③）；`var ErrAgentMarketplaceInvalidInput/ErrAgentMarketplaceStaleDigest/ErrAgentMarketplaceMissingDependency`（哨兵转发） | container.go:426；`routes_agent_marketplace_test.go:59`；宿主哨兵消费方随 handler #7 随迁归零，保留转发以稳定 `errors.Is` 链与未来宿主测试（25a §4-③ user_resource_favorite 行同口径） |
| `internal/application/service/tenant_skill_market_service.go` | `type TenantSkillMarketCatalogStore = acatsvc.TenantSkillMarketCatalogStore`；`type TenantSkillMarketInstaller = acatsvc.TenantSkillMarketInstaller`（未导出名跨包别名，单一真源）；`func NewTenantSkillMarketService`（4 参 1:1 转发） | container.go:832（闭包位置传参，经类型满足编译）；`tenant_skill_market_service_test.go` 随迁不涉本行 |
| `internal/handler/agent_marketplace.go` | `type AgentMarketplaceHandler`；`var NewAgentMarketplaceHandler` | router.go:100（字段）、:396；`routes_agent_marketplace.go:21/:40`；container.go:429；`routes_agent_marketplace_test.go:61` |

---

## 5. 测试策略

- **复用现有（随迁即特征化，38 用例双跑比对 + 路由用例 1 例 = 等价证据，T5 落盘 evidence；计数为本会话 `-v` 实测）**：repository 4 件（`TestAgentMarketplace*` 9 用例含并发唯一发布号 `TestAgentMarketplaceConcurrentPublishingUsesUniqueReleaseNumbers` 与 SQLite down-migration 校验 `TestAgentMarketplaceSQLiteDownWithPublishedListing`、`TestExpertInstall*` 4、`TestPublishedExpertRepo*` 4、`TestPublishedSkillRepo*` 4）；service 2 件（`TestAgentMarketplace*` 8 用例，含补偿/独占发布/暂存字节校验；`TestTenantMarket*` 9 用例）；宿主留置 HTTP 面：`internal/router/routes_agent_marketplace_test.go`（`TestTenantAgentMarketplaceLifecycleAndAuthorization`，经 §4-⑤ shim 零改动跑真实构造链：`NewAgentMarketplaceRepository` → `NewAgentMarketplaceService`（残差绑 `experts.BuildAgentReleaseBundle` 真源）→ `NewAgentMarketplaceHandler`——即注入路径的端到端行为差分）。
- **随迁测试的机械适配（同 commit，TDD 锚定不变）**：`repository/agent_marketplace_test.go:238` `repoRoot` 三层改四层 `../../../..`（`internal/modules/agentcatalog/repository` + 4 层 = 仓库根，25a §4-②#4b 实跑口径）；`service/agent_marketplace_test.go` 8 处 `NewAgentMarketplaceService(...)` 调用点（:135/:152/:162/:174/:192/:272/:307/:327）追加第 5 实参 `MarketHostAdapters{BuildReleaseBundle: experts.BuildAgentReleaseBundle}`（继续锚定真源 builder，`:322` 期望值构造不动）；其余内容零改动。
- **新写（仅防副本漂移，无行为新增）**：`internal/modules/agentcatalog/repository/is_unique_violation_parity_test.go`（表驱动锚定 §4-①#1 副本语义；`remove_at: ib2` 随族收口删除）。
- **高风险差分（conventions §6）**：本面不在 framework:40 点名六面内，按 §14.3「HTTP 响应/错误码」「Worker 状态机、重试和幂等」执行：① 搬迁等价双跑（本节首项，38 用例 + 路由测试）；② 发布重试/幂等差分——`ReviewAndPublishTx` 的唯一冲突重试路径在搬迁前后由同批并发用例（repo 侧 `…ConcurrentPublishingUsesUniqueReleaseNumbers`、service 侧 `…ConcurrentApprovalFailureKeepsPeerCommittedBundle`、路由侧全生命周期用例）覆盖，模块本地 `isUniqueViolation` 的分类结果经 parity 表 + 并发用例双锚定；③ 注入位差分——宿主路由测试经残差绑定真源 builder 全链路执行，模块 service 测试注入同一真源，两者期望值同源（`experts.BuildAgentReleaseBundle`）。差分失败只修新实现，不改期望（spec §14.3）。
- **门禁（DAG gates 原文，不得替代）**：`sh -c "go build ./..."`；`go test -count=1 ./internal/modules/agentcatalog/...`；`make check-backend-architecture`；`make verify-module-moves`。加跑宿主三包与 router 目标测试确认 shim 链路（T1/T5 命令清单）。

---

## 6. 实施步骤（每 Task 一 commit；命令在 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/passb-b2-ac-market` 执行；严格按序）

### Task T1（25c.1）：基线锚定与前置门核验

- [ ] Step 1 固定基线：以协调者派发回填的 DAG `base_sha` 为准（conventions §9；须已含 25a `938087598` + 25b `5ed64d324` 模块树，见前置条件 1/2）。`git rev-parse HEAD` 记录工作树头。
- [ ] Step 2 门面存在性核验：前置条件 2 的 `test -f … && grep -q …` 命令链（预期 `FACADES-OK`）；缺失即停止并按 conventions §5 上报（引用 DAG 边 `b2-ac-market ← {b2-ac-definition, b2-ac-skills}`）。
- [ ] Step 3 基线门禁实跑并逐条记录退出码与关键输出：前置条件 4 全部命令（预期数值同 §前置条件 4 记录；模块包测试以派发基线复跑输出为准，须全 ok）。
- [ ] Step 4 随迁测试宿主侧预跑（旧实现基线 = §5 全部 38 用例 + 路由用例的 `-count=1 -v` 用例清单与 ok/FAIL 记录）——这是 T5 等价比对的「旧实现双跑」侧。
- [ ] Step 5 建 evidence/report 骨架：`mkdir -p docs/architecture/evidence/passb docs/architecture/passb/briefs docs/plans/passb/reports`，写入基线记录。
- [ ] Commit：`test(passb): passb b2-ac-market baseline characterization`

### Task T2（25c.2）：repository 批次搬迁（#1-#4）+ `isUniqueViolation` 模块副本 + 宿主 shim

- [ ] Step 1 逐文件执行 §2.2 两步：新建模块 repository 4 文件（内容 1:1，package `repository`；#1 的 :179 改调模块本地 `isUniqueViolation` 副本，副本函数体逐字复制 voice_session.go:289-300），原路径重写为 §4-⑤ 对应行 shim（文件头 `remove_at: ib2` 注释）。
- [ ] Step 2 随迁测试：`git mv` 4 个 `_test.go` 至模块 repository，package 行同步，内容零改动（`agent_marketplace_test.go:238` repoRoot 三层→四层除外）。
- [ ] Step 3 新写 `is_unique_violation_parity_test.go`（§5 新写项；表驱动：nil→false、`gorm.ErrDuplicatedKey`→true、含 `UNIQUE constraint failed`/`duplicate key value`/`23505` 标记的消息→true、普通错误→false）。
- [ ] Step 4 GREEN 复核：`go build ./...`（预期 0）；`go test -count=1 ./internal/modules/agentcatalog/repository ./internal/application/repository ./internal/application/service`（预期全 ok；宿主侧用例数与 T1 Step 4 基线一致——差异仅为随迁用例从宿主包转出）。
- [ ] Step 5 shim 精确性自检：`git diff $PASSB_BASE_SHA -- internal/application/repository/agent_marketplace.go internal/application/repository/expert_install.go internal/application/repository/published_skill.go internal/application/repository/published_expert.go` 逐行核对 = §4-⑤ 清单（仅别名/转发 + 注释），无业务语句残留。
- [ ] Commit：`refactor(agentcatalog): move marketplace/expert-install/published repositories into module`

### Task T3（25c.3）：service 批次搬迁（#5-#6）+ `MarketHostAdapters` 注入 + 宿主 shim

- [ ] Step 1 新建 `market_host_adapters.go`（§4-③，含 nil fail-fast）；逐文件执行 §2.2 两步：#5 零注入（import 改 `acrepo`）；#6 的 :60 改 `s.adapters.BuildReleaseBundle(...)`、import 删 `experts` 增无（注入后生产文件零 agentruntime import）、`repository.` 改 `acrepo.`；原路径两文件重写为 §4-⑤ 对应行 shim（#6 的残差构造器绑 `experts.BuildAgentReleaseBundle` 真源）。
- [ ] Step 2 随迁测试：`git mv` 两测试至模块 service（package 行同步）；`agent_marketplace_test.go` 8 处构造点（§2.1 #6 行列明坐标）追加 adapters 实参（绑真源 builder）；`tenant_skill_market_service_test.go` 零改动。
- [ ] Step 3 GREEN 复核：`go build ./...`（预期 0）；`go test -count=1 ./internal/modules/agentcatalog/... ./internal/application/service ./internal/application/repository ./internal/router -run '…基线用例集…'` 与全包（预期全 ok；`internal/router` 的 `routes_agent_marketplace_test.go` 通过即 shim 兼容 + 注入真源链路的回归证明）。
- [ ] Step 4 模块 import 纪律自检：`grep -rn "agentruntime\|internal/application/service\|internal/handler" internal/modules/agentcatalog/service/agent_marketplace.go internal/modules/agentcatalog/service/tenant_skill_market_service.go internal/modules/agentcatalog/service/market_host_adapters.go`（预期零命中）。
- [ ] Step 5 shim 精确性自检（同 T2 Step 5，两文件）。
- [ ] Commit：`refactor(agentcatalog): move agent release and tenant skill market services into module`

### Task T4（25c.4）：handler 搬迁（#7）+ `marketTenantID` 本地化 + 宿主 shim

- [ ] Step 1 执行 §2.2 两步：模块 handler `agent_marketplace.go` 1:1 随迁（`marketrepo.`→`acrepo.`、`marketservice.`→`acatsvc.`、`sandboxConfigTenantID` ×4 → `marketTenantID`，§4-④）；原路径重写为 `type AgentMarketplaceHandler = acathandler.AgentMarketplaceHandler` + `var NewAgentMarketplaceHandler = acathandler.NewAgentMarketplaceHandler`（§4-⑤）。
- [ ] Step 2 GREEN 复核：`go build ./...`（预期 0）；`go test -count=1 ./internal/router -run 'TestTenantAgentMarketplace'`、`go test -count=1 ./internal/modules/agentcatalog/... ./internal/handler`（预期全 ok；路由面零漂移）。
- [ ] Step 3 差集自检（conventions §1.2）：`PASSB_BASE_SHA=<派发基线>`；`git diff --name-only $PASSB_BASE_SHA...HEAD | sort` 逐条对照 §2.3 白名单（7 模块生产 + 6 随迁测试 + 1 parity 测试 + 7 shim 原路径 + 3 文档）——预期差集为空；任何超表文件出现即回滚该步。
- [ ] Commit：`refactor(agentcatalog): move agent marketplace http handler into module`

### Task T5（25c.5）：差分证据收口 + Integration Brief + 实施报告 + 节点门禁

- [ ] Step 1 双跑差分：T1 Step 4 用例清单 vs 终态同命令输出，逐用例比对结论写入 evidence「等价双跑」章节（conventions §6 格式：用例清单/双跑输出/比对结论/命令与退出码）；§5 高风险差分三项结论落盘。
- [ ] Step 2 节点 gates 全跑并记录（DAG 四条 + 通用门禁）：`go build ./...`；`go test -count=1 ./internal/modules/agentcatalog/...`；`make check-backend-architecture`（预期 `total=633 | redis=23 lite=23 | hooks=58 | modules=16`、0 violations）；`make verify-module-moves`（预期 `OK (16 manifests verified)`——原路径 shim 保留使 legacy `os.Stat` 全通过）。
- [ ] Step 3 定稿 `docs/architecture/passb/briefs/b2-ac-market.md`：① IB2 装配变更申请（7 个 shim 消费方切换步骤与删除顺序：container/router 改引模块包 → 删 shim；`isUniqueViolation` 第 4 副本与 parity 测试的族收口登记，conventions §7.1）；② §4-② 推迟台账 6 行 + 解除条件与裁定请求（expert.go 三 helper、upload_limit 抽取计划、experts/skillhub re-export、ErrAgentNotFound 同包化排序约束）；③ §2.1 拆分确认请求（13 文件逐条列示，供 `25-agentcatalog-program.md` 确认）；④ 计数零漂移声明与 contracts.yaml 回写申请（`agentcatalog.marketplace-service`/`tenant-skill-market-service` consumers 路径更新归 barrier）。
- [ ] Step 4 写 `docs/plans/passb/reports/b2-ac-market.md`（conventions §1.1/§1.2 格式：每条命令原文+退出码+输出摘录、变更文件 vs owned_files 逐条核对、差分指针、未完成项如实列出）。
- [ ] Step 5 DAG `status`/`head_sha`/`task_ids` 回填属协调者（conventions §9），实施者不写 DAG。
- [ ] Commit：`docs(passb): passb b2-ac-market parity evidence and integration brief`

---

## 7. 集成与回滚边界

- **集成**：本节点分支一次一支审后合并（framework:28）；IB2 在 25a→25b 之后接入本支（framework:152），按 Brief 串行执行：container/router 消费点改引模块包 → 删除 7 个 shim 文件与 parity 测试（manifest 行处置由协调者按 schema LOCKED 统一裁定）→ 推迟批次按 §4-② 裁定在对应 barrier 窗口执行（expert.go 系随 25a handler 批次 2；skill/expert 市场服务面随 B3 re-export 裁定）。shim 删除后宿主 `internal/{application,handler}` 不保留任何 25c 面转发声明（framework:29 终态）。
- **回滚**：spec §13 提交隔离——每 Task 独立 commit、零 integrator 文件触碰，任一 Task 失败 `git revert` 该 commit 即恢复（「新建模块文件 + 重写 shim」均为普通文件变更，revert 语义干净），无跨节点残留；Task 间顺序耦合仅 T3 依赖 T2 的 `acrepo` 包、T4 依赖 T2/T3 的哨兵落位（按序执行即无隐藏耦合）。门禁失败且不可修复时按 conventions §5 blocked 上报，禁止改断言/删测试/扩例外/复制实现/伪造证据。

## 8. 必须删除的 legacy/alias/例外（本节点口径）

- **既有例外行删除：0**（exception-ledger 105 条中无本面行，本会话 grep 实证）；**既有别名义务：0**（`moves/agentcatalog.yaml` `alias_obligations: []`）。
- **本节点产出的过渡 shim = §4-⑤ 的 7 个原路径文件**：删除点 = IB2（Brief 逐符号列明消费方切换步骤后由集成工程师删除）；`is_unique_violation_parity_test.go` 与模块本地 `isUniqueViolation` 副本：删除点 = IB2 族收口（conventions §7.1，收口为单一实现后本副本与测试同删）。
- **推迟批次残留**：6 文件留宿 + 其既有测试（`skill_market_service_test.go` 676 行、`tenant_expert_market_service_test.go` 538 行，及 3 个 handler 测试 `skill_market_test.go` 461 行、`tenant_expert_market_test.go` 241 行、`tenant_skill_market_test.go` 283 行），删除/搬迁点 = 各自行所属 barrier（§4-② 表），由 Brief 移交，不由本节点删除。
- **禁止**：B5 前残留「unused but harmless」兼容包（framework:41）；IB2 后本面不得仍有宿主包内 marketplace 符号转发。

## 9. 独立验收标准（全部满足才算完成；审查者逐条核验）

1. DAG 四条 gates + 通用门禁在分支 HEAD 全部退出码 0，命令原文与输出摘录落盘 evidence（无替代命令）：`go build ./...`=0；`go test -count=1 ./internal/modules/agentcatalog/...` 全 ok；`make check-backend-architecture` 计数 `literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16` 零漂移、0 violations；`make verify-module-moves` `OK (16 manifests verified)`。
2. `git diff --name-only $PASSB_BASE_SHA...HEAD | sort` 与 §2.3 白名单差集为空（7 模块生产 + 6 随迁测试 + 1 parity 测试 + 7 shim 原路径重写 + 3 文档）。
3. 模块新文件与原文件的 diff 仅 package/import/§4 列明的注入点与本地化 helper 行差异（`agent_marketplace.go:60` 一处调用点、4 处 `marketTenantID` 调用点、`acrepo.`/`acatsvc.` 前缀）；shim 文件仅别名/转发 + 注释；`marketplace-service`/`tenant-skill-market-service`/`agent-version-service`/`skill-market-service` 冻结签名逐字不变。
4. 等价双跑：§5 的 38 用例 + 路由用例搬迁前后逐用例一致（证据在 evidence §等价双跑）；并发发布/审批用例（唯一冲突重试路径）在搬迁后经模块本地 `isUniqueViolation` 副本仍绿。
5. parity 测试（T2 Step 3）通过且覆盖 nil/`gorm.ErrDuplicatedKey`/三驱动标记/非唯一错误分类。
6. 模块生产文件零 `internal/modules/agentruntime/**`、零 `internal/application/service`、零 `internal/handler` import（T3 Step 4 grep 核验）；architectureguard forbidden-import 0（含在验收 1 的 make 目标内）。
7. Brief 含 §4-② 全部 6 行推迟台账（符号@文件:行可复核）+ 反向义务核对 + 拆分确认请求 + shim 删除清单与 `isUniqueViolation` 族收口登记。
8. 宿主包残留引用编译成立且无新例外行；`internal/container/**`、`internal/router/**`、`internal/modules/agentcatalog/module.go`、`go.mod`、`go.sum`、`migrations/`、`tools/**`、四份治理 YAML、`docs/architecture/moves/*.yaml`、推迟 6 文件、25a/25b 已搬文件与残差在 diff 中零出现（`git diff --name-only` 核验）。

## 10. 升级路径（conventions §5）

- 门禁不可能通过（如未列入 §4 的隐藏消费点）→ 停在当前任务，报告记录未过项/根因/复现命令/建议裁定，DAG `status=blocked` + notes 上报；禁止删测试/扩例外/塞 common/复制实现。
- 需改冻结契约签名（marketplace-service 等）或需新增 architectureguard 豁免 → ADR/Spec 修订 + 串行契约任务（framework:26），不在本节点现场改判。
- 上游门面未落地类阻塞（25a/25b 模块树不在派发基线）→ 引用 DAG 边与 conventions §5 尾条，不自行实现上游门面。

## 11. 计划自检记录（撰写会话 2026-09-24，worktree `codex/passb-b2-ac-market`@`e586552d1`）

- **Spec 覆盖**：§4.4 薄别名过渡（shim+remove_at+删除点）、§5.4 市场面所有权（§2.1 批次=release/review/listing/发布记录）、§4.2 端口方向（§4-③ 使用方注入位）、§13 提交隔离与回滚（§6/§7）、§14.2/14.3 纯移动与差分（§5/§6）、§15 API 最小化（未导出接口保持未导出、注入位最小单字段）、§16（无新增包级可变状态）、§17.2 对应 IB2/b5 义务（§8/§4-②）。零覆盖缺口。
- **无占位符**：全部文件路径、符号、行号、签名、命令、预期输出均为本会话实读（13 文件全文 + expert.go/upload_limit.go/sandbox_skill.go/voice_session.go/planversion.go/experts 签名/interfaces 全文 + container/router/routes 消费方 grep + 25a/25b 分支 worktree 实测）或实跑（前置条件 4 全部命令）；无法当场决断的（upload helper 抽取、experts/skillhub re-export、deferred 件搬迁时点、矩阵子行回写）一律显式登记为裁定请求/前置门（§4-②、§1.2/1.5），未以 TBD 掩盖。
- **类型一致**：`BuildReleaseBundle` 字段签名与 `experts.BuildAgentReleaseBundle`（agent_release.go:64-69）逐参数/逐返回一致且全为公共类型；shim 别名经 §4-⑤ 消费方清单逐符号核对（含接口别名过 container.go:301 适配闭包、未导出名跨包别名 `TenantSkillMarketCatalogStore`/`TenantSkillMarketInstaller`、哨兵 `errors.Is` 同一性、`AgentMarketplaceHandler` 字面量/构造兼容）；`CatalogInstallResult` 经 25b 搬迁在模块 service 同包直引（`tenant_skill_catalog.go` 定义，25b 分支实测）；`handler/agent_marketplace.go` 哨兵跨层引用 module→module 合法、无环（§2.3 论证）；`repoRoot` 四层深度按 25a §4-②#4b 同口径推算并写入 T2 验证步骤。
- **跨任务接口一致**：§4-⑤ shim 清单 = T2/T3/T4 Step 1 执行面 = T5 Brief 切换清单三方同一来源；§4-③ 注入位 = T3 Step 1 落地 = T3 Step 2 测试适配同源（真源 builder 绑定）；T2 产出 `acrepo` 哨兵/接口 → T3 #5/#6、T4 #7 消费，顺序即依赖；测试期望从 T1 Step 4 基线记录参数化引用而非重写；差集核对统一使用派发 `base_sha`。
- **基线实跑**：前置条件 4 全部命令撰写者已跑并如实记录（`e586552d1`：build 0；repo 21 用例 ok 7.864s；service 17 用例 ok 0.875s；router 1 用例 ok 1.093s；模块包 no test files；`literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16`、0 violations；`modulemove: OK (16 manifests verified)`）；派发基线复跑为 T1 义务。
