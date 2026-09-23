# 25a — Agent 定义/版本/人格/专家/子代理/收藏（b2-ac-definition）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 agentcatalog 模块 55 个 legacy 文件总盘中归属 25a 的「定义/版本/人格/专家/子代理/收藏」面迁入 `internal/modules/agentcatalog/{repository,service,handler}`，交付本节点可物理搬迁批次与全部断链推迟件登记（Integration Brief），全程 `go build ./...` 绿、既有测试全绿、633/23+23/58/537 计数零漂移、行为不变。

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §4.4（两遍迁移模型/薄别名过渡）、§5.4（Agent Catalog 所有权）、§11–§17.2（Pass B 图景）

---

## 0. 事实源指针（全部为本计划撰写会话实读/实证）

| 事实源 | 用途 |
|---|---|
| `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` | §4.4 薄别名过渡规则（「确需过渡时只能保留无业务逻辑的薄别名或 adapter，并记录删除阶段」）；§5.4 agentcatalog 所有权边界（「不拥有运行状态」） |
| `docs/plans/2026-09-23-backend-modularization-pass-b-framework.md` | :138-142（25a=custom agent/versions/persona/expert/subagent/favorites；25b=tenant skill catalog/install/verify/reaper；25c=marketplace；「25a/25b may run in parallel after shared immutable-version contracts freeze」）；:29（生产文件随迁 `_test.go`；host 包不保留转发性业务声明）；:32（计数 parity） |
| `.superpowers/sdd/passb/conventions.md`（主 checkout，git-ignored） | §1 派发契约（只改 owned_files、跨 owner 调用点上报、TDD、随迁测试）；§2 门禁；§3 禁改清单；§4 提交规范；§5 升级契约；§6 高风险差分；§7 package-private 耦合处理（§7.1 isUniqueViolation 族 barrier 收口；§7.3 所有权重裁升级；§7.4 去方法化裁定）；§8 计数基线；§9 DAG 字段仅协调者可写 |
| `docs/plans/passb/execution-dag.json` 节点 `b2-ac-definition` | owned_files（manifest 25a 行 + `internal/modules/agentcatalog/**` definition/version 面）、required_contracts、notes（BLOCKED 已解除，见 §1）、gates 四条 |
| 同文件 `package_private_couplings.pairs` | agentcatalog 9 条 owner 对（→conversation 7 符号 10 点、→commercial 3 符号 5 点、→knowledge 3 点、→workbench 2 点、→airesource 2 点、→execution 1 点；conversation/agentruntime→agentcatalog agentRequiresRerankModel/skillsForRun） |
| `.worktrees/passb-int/docs/architecture/passb/ownership-matrix.yaml` | `module: agentcatalog` 55 行（`plan: 25-agentcatalog-program`，无 25a 子行拆分，见 §2 拆分确认）；`integration_owner: ib2`、`delete_barrier: ib2`；aliases 段无 agentcatalog 行 |
| `.worktrees/passb-int/docs/architecture/passb/contracts.yaml` | agentcatalog 9 条 frozen 契约：`agent-version-service`（AgentVersionService 三方法）、`custom-agent-service`（CustomAgentService 九方法）、`facade`、`routes`（11 入口）、`lifecycle`（startTenantSkillReaper→25b 面）、`workers`（空）、`marketplace-service`/`skill-market-service`/`tenant-skill-market-service`（25c 面） |
| `.worktrees/passb-int/docs/architecture/passb/exception-ledger.yaml` | 105 条中 importer 为 agentcatalog 55 legacy 文件的行数 = **0**（本会话脚本比对实证）；`docs/architecture/passb/event-catalog.yaml` 无 agentcatalog 事件（agentrun.* 归 agentruntime） |
| `docs/architecture/moves/agentcatalog.yaml` | 55 条 legacy_files、`alias_obligations: []`、11 路由入口、1 生命周期挂点（startTenantSkillReaper）、`forbidden_shared_files` |
| `docs/plans/passb/10-identity.md` §3.1/§3.2/§3.4、`12-commercial.md` §4.1/§5.4、`13-execution.md` :61/:65 | 已评审通过的同型机械先例：导出+宿主薄 shim；正向 insoluble（Go import 环）推迟搬迁；宿主 var/type 别名兼容层；guard forbidden-import 断链文件原样保留+登记推迟 |
| 本会话 grep/AST 扫描实证 | §4 耦合全量表的每一行（扫描脚本：三宿主包非测试 .go，剥离注释/字符串后匹配未导出顶层 func、方法声明/接收者、包级 var/const/type 引用） |

**节点状态（2026-09-23）**：DAG notes 记载的 `BLOCKED（前置 b0 阻塞）` 已解除——目标 worktree `execution-dag.json` 当前实测：`b0: done`（B0.1–B0.6）、`ib1: done`、`b2-ac-definition: in_progress`。本计划撰写基线 = `codex/passb-integration` 分支头 `8c45a8815`。

---

## 1. 前置条件（实施前逐条核验，任一不满足即按 conventions §5 上报，不得现场改判）

1. **调度前置**：`b0`、`ib1` DAG status 均为 `done`（已实测满足）。
2. **25a/25b 并行前提（framework:142）**：共享 immutable-version 契约已冻结——`contracts.yaml` `agentcatalog.agent-version-service` 等 9 条 `stability: frozen`（B0.4 Step 5 产物，b0 已 done）。已满足。
3. **基线门禁绿**（conventions §1.2，在未改动工作树上实跑并留证，见 T1）：
   - `go build ./...` 退出码 0；
   - `go test -count=1 ./internal/application/repository ./internal/application/service ./internal/handler ./internal/modules/agentcatalog/...` 全部 ok；
   - `make check-backend-architecture` 退出码 0 且 `total=633 | redis=23 lite=23 | hooks=58`；
   - `make verify-module-moves` 退出码 0（`modulemove: OK (16 manifests verified)`）。
4. **写权边界认知**（conventions §1.2/§3）：本节点只写 §3 列出的文件；`internal/router/router.go`、`internal/container/container.go`、`internal/bootstrap/**`、`router/task.go`、`sync_task.go`、`migrations/`、`go.mod`、`go.sum`、四份治理 YAML、`tools/architectureguard/check.go` 的 `importExceptions`、`internal/modules/agentcatalog/module.go` 门面（归 IB2 装配）一律禁写；**禁止新增任何例外行**（conventions §5）。
5. **B1 实施缺位风险声明**（本会话实证）：`git show --stat f66dfb188`、`ca38afb7e` 显示 B1-CM/B1-EX 分支「实施产出止于计划文档，无生产代码变更」。因此 `internal/modules/commercial/repository/model_usage.go`（导出版）与 `internal/modules/execution/service`（`ResolveTenantSandboxForConfig`）**当前不存在**（已实测：宿主 `internal/application/repository/model_usage.go:31/80/101` 仍为原未导出实现；`internal/modules/execution/service/` 目录不存在）。受影响的推迟件处置见 §4-②/④——凡依赖「IB1 已导出」的消费任务一律带存在性前置门，缺失即 blocked 上报，不得自行实现导出（conventions §5「不自行实现上游门面」）。

---

## 2. 文件清单与写所有权

### 2.1 25a/25b/25c 拆分（55 总盘 → 22/20/13）

ownership-matrix 55 行全部为 `plan: 25-agentcatalog-program`（无 25a 子行）。本计划按 framework:138-140 的三类边界 + 代码实证把 25a 子集固化为下表 22 文件；**该拆分表随 T5 Brief 提交协调者在 `25-agentcatalog-program.md` 中确认**（矩阵行级拆分回写属 barrier/协调者，conventions §3）。

**25a（22 文件，本节点 scope）：**

| 层 | 文件（`git mv` 源 → 目标） |
|---|---|
| repository（6） | `internal/application/repository/{agent_share,agent_version,custom_agent,tenant_disabled_shared_agent,tenant_subagent,user_resource_favorite}.go` → `internal/modules/agentcatalog/repository/` |
| service（9） | `internal/application/service/{agent_browser_preferences,agent_service,agent_share,agent_version,custom_agent,expert_service,expert_skills,subagent_service,user_resource_favorite}.go` → `internal/modules/agentcatalog/service/` |
| handler（7） | `internal/handler/{agent_version,custom_agent,expert,persona,shared_agent_access,subagent,user_resource_favorite}.go` → `internal/modules/agentcatalog/handler/` |

归属证据要点：`agent_share.go:40` 定义 `agentRequiresRerankModel`（ask 裁定归本节点导出）；`tenant_disabled_shared_agent.go`/`shared_agent_access.go` 与 agent share 访问面一体（`handler/custom_agent.go` 消费 `h.disabledRepo.ListDisabledOwnAgentIDs`）；`expert_skills.go` 为专家-技能绑定解析（定义面）；skill/marketplace/reaper 各文件按 framework:139-140 归 25b/25c。

**25b（20）**：service 17 个 `tenant_skill_{admin,bundle,catalog,effective,env_declare,files,install,progress,reaper,remove,runtime_verify,service,source,steer,stop,transcript,verify}.go` + repository `tenant_skill.go` + handler `{skill_catalog,skill_handler}.go`。
**25c（13）**：repository `{agent_marketplace,expert_install,published_expert,published_skill}.go` + service `{agent_marketplace,expert_market_source,skill_market_service,tenant_expert_market_service,tenant_skill_market_service}.go` + handler `{agent_marketplace,skill_market,tenant_expert_market,tenant_skill_market}.go`。
（22+20+13=55，与矩阵行数一致。）

### 2.2 本节点物理搬迁批次（Go 语义与 guard 实证裁定的结果）

**批次 1（本节点搬迁，8 生产 + 3 测试）**——判定标准（逐文件实证，见 §4）：(a) 搬入模块树后不产生新的跨模块内部包 import（`tools/architectureguard/check.go:1175-1219` 只扫 `internal/modules/**`，模块 X import 模块 Y≠X 的任何包即 diagnostic，且实施者禁新增例外）；(b) 不调用定义在「本搬迁后将因宿主兼容层 import 本模块而形成 Go import 环」的宿主包未导出符号；(c) 其具体类型未被留宿文件以断言/方法声明方式引用。

| 文件 | 随迁测试（framework:29） |
|---|---|
| `repository/agent_share.go` | `repository/agent_share_source_test.go`（自包含 sqlite 内存库，已核无宿主测试基建依赖） |
| `repository/tenant_subagent.go` | `repository/tenant_subagent_test.go`（自包含） |
| `repository/tenant_disabled_shared_agent.go` | （无既有测试） |
| `repository/user_resource_favorite.go` | （无既有测试） |
| `service/agent_browser_preferences.go` | （无既有测试） |
| `service/agent_version.go` | `service/agent_version_test.go`（contracts.yaml `agentcatalog.agent-version-service.characterization_tests` 指定件；`repoRoot` 于该文件 :35 自算，`../../..` 深度在新位置不变，自包含） |
| `service/user_resource_favorite.go` | （无既有测试） |
| `handler/user_resource_favorite.go` | （无既有测试） |

**批次 2（14 文件，本节点不搬，逐文件推迟登记 + 裁定请求，见 §4-②）**：service `{agent_service,agent_share,custom_agent,expert_service,expert_skills,subagent_service}`、repository `{agent_version,custom_agent}`、handler `{agent_version,custom_agent,expert,persona,shared_agent_access,subagent}`。推迟不是 scope 缩水，而是与已批准先例同型的物理约束（10-identity §3.2 推迟 8/27 文件；13-execution:65 同律）。每个推迟件的解除条件、解除后动作、受益 barrier 在 T5 Brief 中逐条登记。

### 2.3 本节点产出新文件（owned 产出物）

| 文件 | 用途 | 删除点 |
|---|---|---|
| `internal/application/repository/agentcatalog_passb_compat.go` | 4 个构造器 var 别名（§4-③表） | IB2（集成工程师按 Brief 切换 container 后删） |
| `internal/application/service/agentcatalog_passb_compat.go` | 2 个构造器 var 别名（§4-③表） | IB2 |
| `internal/handler/agentcatalog_passb_compat.go` | 1 type 别名 + 1 构造器 var 别名（§4-③表） | IB2 |
| `docs/architecture/evidence/passb/b2-ac-definition.md` | 证据（基线/门禁/等价/差分登记） | 永久留存 |
| `docs/architecture/passb/briefs/b2-ac-definition.md` | Integration Brief（推迟台账+裁定请求+compat 删除清单） | 随 IB2 执行归档 |
| `docs/plans/passb/reports/b2-ac-definition.md` | 实施报告（conventions §1.1/§1.2 格式） | 永久留存 |

方向合法性依据：12-commercial.md:302 已裁定「宿主包 → 模块包 import 方向合法：architectureguard forbidden-import 仅扫描 `internal/modules/**` 内文件」。本节点 compat 层全部是宿主→模块方向；批次 1 搬迁文件对宿主包的残余 import（`service/agent_version.go` 的 `internal/application/repository` 接口类型引用）同为合法方向，且经环检测：`module/service → host/repository → module/repository`（compat 别名）不构成环（`module/repository` 不 import `module/service`）。

---

## 3. 禁改清单（违反即节点失败）

conventions §3 全文适用。特别强调本节点实证涉及的：`internal/router/**`（`routes_agent.go:22/:61/:76` 等签名引用 `*handler.XxxHandler`，由 §4-③ type 别名保持编译，本节点不改其一个字节）、`internal/container/container.go`（`:294/:298/:299/:304/:306/:307/:423/:430/:800` 等 Provide 行原样）、`internal/handler/{knowledge,knowledgebase,rbac_lookups,sandbox_config}.go`（他 owner/推迟件）、`internal/application/service/{session_*,kbshare,agent_run_graph,agent_capabilities,craft_delegate,subagent_delegate,tenant_skill_*,model_usage...}`（他 owner 文件）、`internal/modules/agentcatalog/module.go`（零逻辑骨架，门面实现归 IB2）、`tools/**`、治理 YAML 四份。

---

## 4. 耦合与断链面全量表（本会话扫描实证；符号/行号均可复核）

### ① 批次 1 文件的全部对外依赖（迁入后合法，无需处置）

| 文件 | 依赖 | 证据 |
|---|---|---|
| `repository/agent_share.go` | `internal/types`、`internal/types/interfaces` | 文件 import 块（本会话读取） |
| `repository/tenant_subagent.go` | `internal/types` | 同上 |
| `repository/tenant_disabled_shared_agent.go` | `internal/types`、`interfaces.TenantDisabledSharedAgentRepository`（构造器 :16-17） | 同上 |
| `repository/user_resource_favorite.go` | `internal/types`、`interfaces`（构造器 :17 `NewUserResourceFavoriteRepository(db *gorm.DB) interfaces.UserResourceFavoriteRepository`） | 同上 |
| `service/agent_browser_preferences.go` | `internal/types` 仅 | 同上 |
| `service/agent_version.go` | 宿主 `repository.AgentVersionRepository` 接口类型（定义于 `repository/agent_version.go:28`，该 repo 文件属批次 2 留宿）+ `apperrors`/`logger`/`types`/`interfaces` | 同上；方向合法无环（§2.3） |
| `service/user_resource_favorite.go` | `interfaces.UserResourceFavoriteRepository`；哨兵 `ErrFavoriteInvalidType`/`ErrFavoriteEmptyID`（:15-16） | 同上 |
| `handler/user_resource_favorite.go` | 哨兵消费 `service.ErrFavoriteInvalidType/ErrFavoriteEmptyID`（:70/:107/:135，全仓非测试消费者仅此文件，已 grep 实证）→ 搬迁后改引 `agentcatalogservice.ErrFavoriteInvalidType/ErrFavoriteEmptyID`（模块内 service↔handler 跨包，guard 允许 target==owner）；构造器 :27 `NewUserResourceFavoriteHandler(svc interfaces.UserResourceFavoriteService) *UserResourceFavoriteHandler` | 同上 |

### ② 批次 2 推迟件登记表（T5 Brief 的核心正文；每行 = 解除条件 + 解除后动作）

| # | 推迟文件 | 硬阻塞（符号 @ 定义文件:行 ← 调用点，本会话实证） | 约束类别 | 建议裁定（供协调者/IB2） |
|---|---|---|---|---|
| 1 | `service/agent_service.go` | `resolveSandboxForExecution`@`session_sandbox_pin.go:146`（调用点 agent_service.go ×2）、`sessionSandboxFileStore`@`session_attachment_staging.go:28`、`sessionSandboxShellExecutor`@:38、`sessionSandboxInstallShellExecutor`@:49（各 ×1）；`archiveMatchesSHA`@`tenant_skill_bundle.go`（25b，调用点 :685 ×1） | 调用宿主他 owner 未导出符号 → 搬入后需 import 宿主 service 包，而宿主 service 必将含 compat 别名 import 本模块 → **Go import 环**（10-identity §3.2 同律物理不可行） | conversation 4 符号由集成工程师提前导出（DAG notes 已列裁定通道，跨 owner 写走 Brief）；`archiveMatchesSHA` 无需导出——25b 搬迁后与本项目标包同包，**排序约束：agent_service.go 搬迁排在 25b 合并之后（IB2 内串行）** |
| 2 | `service/agent_service.go`（附加） | `*agentService` 方法声明扩散：`prepareAgentCapabilities`@`agent_capabilities.go`（33-agentruntime-engine）、`registerCraftDelegateTool`@`craft_delegate.go`（41-craft）、`sessionSandboxInputStore`/`stageSessionAttachments`/`resolveSessionAttachmentURLs`@`session_attachment_staging.go`（35-conversation，3 方法）、`registerSubagentDelegateTool`@`subagent_delegate.go`（33）；具体类型断言 `s.agentService.(*agentService)`@`agent_run_graph.go`（33） | 方法接收者/类型断言跨 owner（DAG 符号级分析只计顶层 func，未含方法——**本会话新发现，超出 DAG notes 清单**，按 conventions §7.4 类别上报） | §7.4 去方法化裁定（集成工程师执行）或推迟至 B3/B4 对应文件搬迁后随 IB3/IB4 收口；两案均登记 Brief，由协调者择一 |
| 3 | `service/agent_share.go` | `applyTenantRoleCap`@`kbshare.go:70`（22-knowledge-retrieval，K2 留宿）← 调用点 :297/:356/:419 ×3 | 同 #1 import 环 | 提前导出走 Brief；或按 conventions §7.3 上报所有权重裁（org 角色封顶语义是否属 knowledge 由协调者裁） |
| 4 | `service/custom_agent.go` | `var _ installerAgentSource = (*customAgentService)(nil)`@`tenant_skill_install.go`（25b 留宿）——对 25a 具体类型的断言 | 具体类型被留宿文件引用 | 排序约束：25b 合并后（两文件同入 `internal/modules/agentcatalog/service`）即可搬，IB2 串行；消费 `repository.ErrCustomAgentNotFound`（:162 等 5 点，定义 `repository/custom_agent.go:13` 批次 2 留宿）随同文件搬迁自然同包 |
| 5 | `repository/custom_agent.go` | `customAgentModelUsageBindings`@宿主 `model_usage.go:31`（调用点 :94）、`scopeCustomAgentsByModelID`@:80（:72,:87）、`scopeCustomAgentsBySandboxConfigID`@:101（:119,:136）——commercial 属主（12） | 同 #1 import 環 | **消费 IB1 已导出 model_usage 绑定族**（ask 裁定）：`internal/modules/commercial/repository/model_usage.go` 的 `CustomAgentModelUsageBindings/ScopeCustomAgentsByModelID/ScopeCustomAgentsBySandboxConfigID`（签名冻结于 12-commercial.md §4.1:184-188，逐字搬迁体）。**前置门**：该导出文件须已落地（B1-CM 实施合入）；缺失即 §5 blocked 上报，不得自行实现或复制 |
| 6 | `repository/agent_version.go` | `isUniqueViolation`@`voice_session.go:289`（40-workbench 留宿）← 调用点 :89 ×1 | 同 #1 import 环 | conventions §7.1：isUniqueViolation 多属主重复 helper 族（现存 `voice_session.go:289` 与 `resource.go:358` 两份）由 IB2 收口为单一实现；收口位置（裁定 canonical home 后）本文件改引之。备选：推迟至 IB4 workbench 搬迁 |
| 7 | `handler/agent_version.go` | `sandboxConfigTenantID`@`sandbox_config.go:92`（13-execution 留宿，B1-EX 未实施）← 调用点 ×3 | 同 #1 import 环 | execution 模块搬迁落地时其 handler 面导出；或提前导出走 Brief |
| 8 | `handler/custom_agent.go` | `pickUserDisplayName`@`knowledgebase.go:663`（22-K2 留宿）← 调用点 ×1；`rbac_lookups.go`（10-identity 推迟件，留宿）在 `*CustomAgentHandler` 上声明方法（rbac_lookups.go:49 起） | 同 #1 + 方法接收者跨 owner（§7.4） | identity 侧 §7.4 去方法化裁定（10-identity.md:79 已登记同一边）；K2 搬迁/导出走 Brief |
| 9 | `handler/expert.go`、`service/expert_service.go`、`service/expert_skills.go` | import `internal/modules/agentruntime/agent/{experts,skills}`；`service/expert_skills.go` 另引用 `installedSkillLister` 类型@`tenant_skill_effective.go`（25b） | **guard forbidden-import**：搬入模块树后成为 agentcatalog→agentruntime 内部包 import（check.go:1210-1215 diagnostic），实施者禁新增例外 | 13-execution.md:65 同型裁定：agentruntime 根门面 re-export 所需符号，或推迟至 B3 R 面搬迁协同收口（IB3）；`installedSkillLister` 随 25b 同包化自然解除 |
| 10 | `service/subagent_service.go`、`handler/subagent.go` | import `internal/modules/agentruntime/agent/subagents` | 同 #9 | 同 #9 |
| 11 | `handler/persona.go` | import `internal/modules/agentruntime/agent/persona` | 同 #9 | 同 #9 |
| 12 | `handler/shared_agent_access.go` | import `internal/modules/agentruntime/agent/tools`、`internal/modules/policy/access`；消费 `service.ErrAgentShareNotFound/ErrAgentSharePermission/ErrAgentNotFoundForShare`（宿主 service，定义于批次 2 的 `agent_share.go:22-24`） | 同 #9 + #1 | 同 #9；哨兵消费随 #3 的 agent_share.go 搬迁转模块导出后改引 |

**批次 2 的反向义务（当前留宿故不断链；搬迁时由对应 barrier 执行，全部登记 Brief）**：`agentRequiresRerankModel`（agent_share.go:40）← `session_agent_qa.go:133`（35）+ `agent_run_graph.go:207`（33）——**ask 指定的本节点导出义务，因 agent_share.go 本体推迟至 IB2，导出+宿主一行委托 shim（10-identity §3.1 模式）随其 IB2 搬迁执行**；`skillsForRun`（tenant_skill_effective.go:35）归 25b；`shared_agent_access.go` 三函数 `resolveSharedAgentForRequest`/`filterKnowledgeBasesForSharedAgent`/`filterKnowledgeByAgentScope` ← `handler/knowledge.go:1607,1667,2185`（24-K4）+ `handler/knowledgebase.go:516,533`（22-K2）共 6 点；`expert.go` 的 `decodeExpertInstantiateBody`/`expertServiceError`/`maxExpertAgentNameLen` ← `handler/skill_market.go:219` + `handler/tenant_expert_market.go:186,202` 等（25c）共 6 点。

### ③ 宿主兼容层（批次 1 的别名清单；全部 var/type 别名，零业务逻辑，spec §4.4 合规）

消费方全量扫描实证（本会话 grep；`internal/router/*_test.go` 零引用）：

| 宿主文件 | 别名（= 模块包同名符号） | 消费方 |
|---|---|---|
| `internal/application/repository/agentcatalog_passb_compat.go` | `NewTenantSubagentRepository`（container.go:294）、`NewAgentShareRepository`（:304）、`NewTenantDisabledSharedAgentRepository`（:306）、`NewUserResourceFavoriteRepository`（:307） | 仅 container.go |
| `internal/application/service/agentcatalog_passb_compat.go` | `NewAgentVersionService`（container.go:423 内联调用）、`NewUserResourceFavoriteService`（:430 Provide） | 仅 container.go |
| `internal/handler/agentcatalog_passb_compat.go` | type `UserResourceFavoriteHandler`（router.go:104 字段、routes_agent.go:61 形参）、var `NewUserResourceFavoriteHandler`（container.go:800） | router + container |

### ④ 冻结契约与本节点的兼容义务

- `agentcatalog.agent-version-service`（frozen）：`FreezeAgentVersion(ctx, tenantID uint64, actorID, agentID string) (AgentVersionView, error)`、`GetAgentVersion(ctx, tenantID, versionID string) (AgentVersionSnapshot, error)`、`ListAgentVersions(ctx, tenantID, agentID string) ([]AgentVersionView, error)`——签名逐字不变；特征化测试 `agent_version_test.go` 随迁即锚定。
- `agentcatalog.custom-agent-service`（frozen）：九方法签名不变（contracts.yaml :28）；其特征化测试清单中 `embed_channel_public_config_test.go`（36-channels 留宿）、`tenant_skill_install_test.go`（25b 留宿）、`rbac_lookups_test.go`/`custom_agent_api_key_scope_test.go`/`persona_test.go`（handler 宿主测试，随批次 2 对应生产文件处置）——批次 1 不触及。
- `agentcatalog.routes`（frozen）11 入口、633 路由总数：批次 1 仅涉 `RegisterUserFavoriteRoutes`（routes_agent.go:61）路径，经 type 别名签名不变，路由面零漂移（T4 用 `make check-backend-architecture` 断言 `total=633`）。
- `agentcatalog.facade`/`lifecycle`/`workers`：module.go 不写（IB2）；startTenantSkillReaper 归 25b 面不触及。
- 公共可观察行为：全部 HTTP 路由/状态码/错误哨兵语义（`errors.Is` 判等链）、收藏/子代理目录/共享代理禁用/版本冻结的存储行为不变；搬迁 = `git mv` + package/import 修复 + §4-③别名，零逻辑改动（spec §4.4 Pass B 纯移动纪律）。

---

## 5. 测试策略

- **复用现有（随迁即特征化）**：`agent_share_source_test.go`、`tenant_subagent_test.go`（6 用例，含幂等/租户隔离/软删除语义）、`agent_version_test.go`（contracts.yaml 指定特征化件，sqlite migration 全链路）。搬迁前后同用例双跑比对 = §6 等价证据。
- **新写**：无行为新增则不新增测试；若 T2/T3 搬迁中发现某批次 1 文件存在未被锚定的可观察行为分支（如 `agent_browser_preferences.go` 无任何既有测试），按 conventions §1.4 先在宿主包写特征化测试（锚定旧实现、宿主侧跑绿）再随迁——新测试文件随生产文件同 commit。
- **高风险差分（conventions §6）**：批次 1 无 framework:40 高风险面。本节点高风险面 = 共享代理 KB 列表/文档检索的可见性过滤（`resolveSharedAgentForRequest`/`filterKnowledgeBasesForSharedAgent`/`filterKnowledgeByAgentScope`，消费方 knowledge.go/knowledgebase.go）与 rerank 校验（`agentRequiresRerankModel`）——全部位于批次 2 留宿文件，**差分义务随推迟件登记转移至 IB2/IB3 执行窗口**（旧实现特征化 → 搬迁 → 同用例双跑 → 逐用例比对，证据写入 evidence 差分章节），T5 Brief 显式登记不得遗漏。
- **门禁**（DAG gates 原文，不得替代）：`go build ./...`；`go test -count=1 ./internal/modules/agentcatalog/...`；`make check-backend-architecture`；`make verify-module-moves`。加跑宿主三包测试确认 shim 链路（T1/T4 命令清单）。

---

## 6. 实施步骤（每 Task 一 commit；命令在 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/passb-b2-ac-definition` 执行）

### Task T1：基线锚定与预检

- [ ] Step 1 记录 `PASSB_BASE_SHA=$(git merge-base origin/main HEAD)` 与 `git rev-parse HEAD`（预期 = `8c45a8815` 或协调者派发时给定值）入 evidence 骨架 `docs/architecture/evidence/passb/b2-ac-definition.md`（`mkdir -p docs/architecture/evidence/passb docs/architecture/passb/briefs docs/plans/passb/reports`）。
- [ ] Step 2 基线门禁实跑并逐条记录退出码与关键输出（§1.3 四条 + `go test -count=1 ./internal/application/repository ./internal/application/service ./internal/handler`）。预期：全绿、`total=633 | redis=23 lite=23 | hooks=58`。
- [ ] Step 3 随迁测试宿主侧预跑（旧实现基线）：`go test -count=1 -run 'TestGetShareByAgentIDAndSource|TestSubagent|TestFreeze|TestAgentVersion' ./internal/application/repository ./internal/application/service -v`，记录用例清单与 ok/FAIL——这是 T4 等价比对的「旧实现双跑」侧。
- [ ] Step 4 前置门复核：`test -f internal/modules/commercial/repository/model_usage.go && echo EXPORT-LANDED || echo EXPORT-MISSING`（预期当前为 `EXPORT-MISSING`，仅登记不阻塞批次 1；批次 2 #5 的执行门）。
- [ ] Commit：`test(passb): b2-ac-definition baseline characterization`

### Task T2：repository 批次搬迁 + 宿主兼容层

- [ ] Step 1 `git mv internal/application/repository/agent_share.go internal/modules/agentcatalog/repository/`（其余 `tenant_subagent.go`、`tenant_disabled_shared_agent.go`、`user_resource_favorite.go` 同）；`git mv` 两个测试文件同往。
- [ ] Step 2 包声明改 `package repository`，import 路径修复，零逻辑改动（diff 审查仅允许 package/import 行）。
- [ ] Step 3 新建 `internal/application/repository/agentcatalog_passb_compat.go`：4 个 var 别名（§4-③表），文件头注释注明「Pass B (25a) 过渡 shim — 消费方 container.go 由 IB2 切换后删除」。
- [ ] Step 4 RED→GREEN 复核：`go build ./...`（预期 0）；`go test -count=1 ./internal/modules/agentcatalog/repository ./internal/application/repository`（预期 ok，用例数与 T1 Step 3 一致）。
- [ ] Commit：`refactor(agentcatalog): move agent-share/subagent/favorite repositories to module`

### Task T3：service + handler 批次搬迁 + 哨兵重指向

- [ ] Step 1 `git mv` `service/{agent_browser_preferences,agent_version,user_resource_favorite}.go`、`service/agent_version_test.go`、`handler/user_resource_favorite.go` 至对应模块包；包声明/import 修复。
- [ ] Step 2 搬迁后的 `handler/user_resource_favorite.go` 哨兵改引 `agentcatalogservice "github.com/Tencent/WeKnora/internal/modules/agentcatalog/service"` 的 `ErrFavoriteInvalidType/ErrFavoriteEmptyID`（errors.Is 语义同一性不变：同一 var 实体）。
- [ ] Step 3 新建 `internal/application/service/agentcatalog_passb_compat.go`（2 var 别名）与 `internal/handler/agentcatalog_passb_compat.go`（1 type + 1 var 别名），注释同 T2 Step 3。
- [ ] Step 4 全量复核：`go build ./...`（预期 0）；`go test -count=1 ./internal/modules/agentcatalog/... ./internal/application/service ./internal/handler ./internal/router`（预期全 ok；`internal/router` 包编译即别名兼容的回归证明）。
- [ ] Step 5 差集自检（conventions §1.2）：`git diff --name-only $(git merge-base origin/main HEAD)...HEAD | sort` 逐条对照 §2 清单——预期差集为空；任何超表文件出现即回滚该步。
- [ ] Commit：`refactor(agentcatalog): move version/favorite service and favorite handler to module`

### Task T4：门禁全套 + 等价证据

- [ ] Step 1 DAG 四条 gates 逐条执行并记录：`go build ./...`；`go test -count=1 ./internal/modules/agentcatalog/...`；`make check-backend-architecture`（预期 `total=633 | redis=23 lite=23 | hooks=58`，0 violations）；`make verify-module-moves`（预期 `OK (16 manifests verified)`）。
- [ ] Step 2 等价比对：新位置重跑 T1 Step 3 用例清单，逐用例与宿主侧基线输出比对（用例数、ok/FAIL、关键断言），结论写入 evidence「等价双跑」章节；批次 1 无高风险差分义务（§5），高风险面推迟登记核对（对照 §4-②尾段清单逐条 presence 检查）。
- [ ] Step 3 evidence 文件补全：命令原文+退出码+输出摘录、变更文件 vs owned_files 核对结论、未完全满足项（如实列出，禁止省略）。
- [ ] Commit：`test(passb): b2-ac-definition parity evidence`

### Task T5：Integration Brief + 实施报告

- [ ] Step 1 写 `docs/architecture/passb/briefs/b2-ac-definition.md`：① IB2 装配变更申请（compat 三文件的切换与删除步骤、`RegisterUserFavoriteRoutes` 形参切换说明）；② §4-② 推迟台账 12 行 + 反向义务清单（含 conversation 7 符号中本项目 4 符号与 25b 3 符号的统一提前导出申请、agentRequiresRerankModel IB2 导出+宿主 shim 义务、25c 对 expert.go 三 helper 的消费处置）；③ 25a/25b/25c 拆分表确认请求；④ 计数零漂移声明。
- [ ] Step 2 写 `docs/plans/passb/reports/b2-ac-definition.md`（conventions §1.1/§1.2 格式：命令+退出码+输出摘录、owned_files 核对、差分指针、未完成项）。
- [ ] Step 3 上报协调者：DAG `status`/`head_sha`/`task_ids` 回填属协调者（conventions §9），实施者不写 DAG。
- [ ] Commit：`docs(passb): b2-ac-definition integration brief and deferral ledger`

---

## 7. 集成与回滚边界

- **集成**：本节点分支一次一支审后合并（framework:28）；IB2 按 Brief 串行切换 container/router 消费点、删除三个 compat 文件、执行推迟批次的导出/去方法化/搬迁（排序：B1-CM 实施 → repo/custom_agent.go；25b 合并 → agent_service.go/custom_agent.go(service)；guard 裁定 → expert/subagent/persona/shared_agent_access 系）。compat 文件删除后 `internal/{application,handler}` 宿主包不保留任何 agentcatalog 转发声明（framework:29 终态）。
- **回滚**：spec §13 提交隔离——每 Task 独立 commit、零 integrator 文件触碰，任一 Task 失败 `git revert` 该 commit 即恢复，无跨节点残留；Task 间无隐藏顺序耦合（T2/T3 可独立 revert）。门禁失败且不可修复时按 conventions §5 blocked 上报，禁止改断言/删测试/扩例外/复制实现/伪造证据。

## 8. 必须删除的 legacy/alias/例外（本节点口径）

- **既有例外行删除：0**（exception-ledger 无 importer 为 agentcatalog 55 文件的行，本会话脚本实证）；**既有别名义务：0**（`moves/agentcatalog.yaml` `alias_obligations: []`）；**agentcatalog 相关例外在本节点的删除义务：0**（exception-ledger 中消费 agentcatalog 的例外归各消费方计划，如 identity 的 rbac_lookups 推迟件）。
- **本节点产出的过渡 compat 三个文件**：删除点 = IB2（Brief 逐符号列明切换步骤）；**推迟批次残留**：14 文件留宿 + 其随宿测试，删除点 = 各自行所属 barrier（§4-② 表），由 Brief 移交，不由本节点删除。

## 9. 独立验收标准

1. DAG 四条 gates + §1.3 通用门禁在分支 HEAD 全部退出码 0，命令原文与输出摘录落盘 evidence（无替代命令）。
2. `git diff <base>...HEAD --name-only` 与 §2.2 批次 1 + §2.3 新文件清单差集为空。
3. 搬迁 diff 中零业务逻辑改动（仅 package/import/哨兵包路径行）；`agent-version-service`/`custom-agent-service` 冻结签名逐字不变。
4. `make check-backend-architecture` 计数 `633/23+23/58` 零漂移；`verify-module-moves` OK。
5. 等价双跑：批次 1 全部既有用例搬迁前后逐用例一致（证据在 evidence）。
6. Brief 含 §4-② 全部 12 行推迟台账 + 反向义务 + 拆分确认请求，行项可复核（符号@文件:行）。
7. 宿主包残留引用编译成立且无新例外行、无治理 YAML 改动、`internal/modules/agentcatalog/module.go` 零改动。

## 10. 计划自检记录

- **Spec 覆盖**：§4.4 薄别名过渡（compat 层+删除点）、§5.4 所有权边界（批次划分=定义/版本/人格/专家/子代理/收藏面）、§12 交付包、§13 回滚、§14.2 纯移动验证已各落节。
- **无占位符**：所有文件路径、符号、行号、命令、预期输出均为本会话实读/实跑或 ask 给定事实；无法当场决断的（guard 裁定、所有权重裁、B1 实施落地时点）一律显式登记为前置门/裁定请求，未以 TBD 掩盖。
- **类型一致**：compat 别名经消费方签名核对（§4-③ 表逐行给出 container.go:行）；哨兵重指向为同一 var 实体；`AgentVersionRepository` 接口宿主引用无环已论证（§2.3）。
- **跨任务接口一致**：T2/T3 别名清单 = §4-③ 表 = T5 Brief 切换清单三方同一来源；测试期望从 T1 基线记录参数化引用而非重写。
