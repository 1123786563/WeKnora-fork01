# b2-ac-market Integration Brief — Pass B IB2

> 状态：**定稿**（T5 Step 3，2026-09-25；提交 `docs(passb): passb b2-ac-market parity evidence and integration brief`）。
> 分支：`codex/passb-b2-ac-market`，节点终态头 `53646cb10`；对齐后任务基线 `c0ddf768a`（Ruling 2026-09-24-WAVE-DEP-BASELINE：merge 25a `ad53c2185` + merge 25b `c0ddf768a`；基线对齐发生于 IB2 之前，IB2 正式职责不变）。
> 计划：`docs/plans/passb/25c-marketplace.md`；证据：`docs/architecture/evidence/passb/b2-ac-market.md`；报告：`docs/plans/passb/reports/b2-ac-market.md`。
> 交付范围：7 生产文件 + 6 随迁测试 + 1 parity 测试 + `market_host_adapters.go` 注入位迁入 `internal/agentcatalog/{repository,service,handler}`；7 原路径过渡 shim（`remove_at: ib2`）；6 文件按裁定请求推迟留宿（§2）。节点 gates 全绿、633/23+23/58/537 计数零漂移。

## 1. IB2 装配变更申请：7 个 shim 的消费方切换与删除顺序

**顺序纪律**：先改消费方 import 至模块包（编译绿）→ 再删 shim 文件 → 最后删测试装置（§3）。全部切换与删除均在 IB2 窗口单写（conventions §3 集成工程师独占面）。

### 1.1 消费方切换清单（行号为分支终态 `53646cb10` 实测）

| # | shim（原路径，删后归零） | 切换前消费方（宿主包引用点） | 切换动作 |
|---|---|---|---|
| 1 | `internal/application/repository/agent_marketplace.go`（接口别名 + `NewAgentMarketplaceRepository` var + 6 哨兵 var：DigestMismatch/PointerConflict/NotFound/InvalidDecision/ReviewConflict/VersionAgentMismatch） | `container.go:300`（Provide 构造器）、`:301`（接口适配闭包形参）；`routes_agent_marketplace_test.go:51` | import 改 `acrepo "…/modules/agentcatalog/repository"`，`repository.` 前缀改 `acrepo.` |
| 2 | `internal/application/repository/expert_install.go`（接口别名 + 构造器 var） | `container.go:295`、`:813`、`:844`（Provide 形参与闭包参数）；推迟件 `skill_market_service.go:49`、`tenant_expert_market_service.go:43`（字段类型，随推迟件搬迁窗口处置，非 IB2 必做） | 同上 |
| 3 | `internal/application/repository/published_skill.go`（接口别名 + 构造器 var） | `container.go:296`、`:829` | 同上 |
| 4 | `internal/application/repository/published_expert.go`（接口别名 + 构造器 var） | `container.go:297`、`:843`；推迟件 `tenant_expert_market_service.go:42` | 同上 |
| 5 | `internal/application/service/agent_marketplace.go`（4 参残差构造器绑真源 `experts.BuildAgentReleaseBundle` + 3 哨兵 var：InvalidInput/StaleDigest/MissingDependency） | `container.go:425-426`（Provide 闭包）；`routes_agent_marketplace_test.go:59` | **切换即注入**：消费方改调模块 5 参构造器并自传 `acatsvc.MarketHostAdapters{BuildReleaseBundle: experts.BuildAgentReleaseBundle}`（宿主 `experts` import 合法）；宿主测试同理 |
| 6 | `internal/application/service/tenant_skill_market_service.go`（2 个未导出接口跨包别名 + 4 参转发构造器） | `container.go:832`（闭包传参，实参经 `repository.PublishedSkillRepository` 别名满足——该形参切换后改 `acrepo.PublishedSkillRepository`） | 同上；未导出接口别名 `TenantSkillMarketCatalogStore`/`TenantSkillMarketInstaller` 消费方仅本闭包，改由模块包内类型满足 |
| 7 | `internal/handler/agent_marketplace.go`（`type AgentMarketplaceHandler` 别名 + `var NewAgentMarketplaceHandler`） | `router.go:100`（字段）、`:396`（注册行）；`routes_agent_marketplace.go:21`、`:40`；`container.go:429`；`routes_agent_marketplace_test.go:61` | import 改 `acathandler "…/modules/agentcatalog/handler"` |

**切换后核验**：`go build ./...` 绿 + `make check-backend-architecture`（`total=633` 零漂移，路由入口签名不变）+ `routes_agent_marketplace_test.go` 原样绿（HTTP 面等价锚点）。

### 1.2 `isUniqueViolation` 族收口登记（conventions §7.1）

- 本节点在 `internal/agentcatalog/repository/agent_marketplace.go` 落地**第 4 份副本**（函数体逐字复制 `voice_session.go:289-300`；`gorm.ErrDuplicatedKey` + `UNIQUE constraint failed`/`duplicate key value`/`23505` 三标记）。全仓四份 = `voice_session.go:289`（40-workbench 留宿）、`internal/application/service/resource.go:358`（11-airesource）、`internal/modules/commercial/repository/commercial/planversion.go:323`（12-commercial 模块本地）、本副本。
- 防漂移锚：`internal/agentcatalog/repository/is_unique_violation_parity_test.go`（8 子用例表驱动，`remove_at: ib2`）。
- **IB2 收口义务**：族收口为单一实现（建议收口位由协调者裁定，候选：共享 errors/util 包或最早属主导出）；收口后**同窗删除**本副本 + parity 测试。收口前四副本语义漂移风险由各处既有测试与 parity 表双向锚定。

### 1.3 删除顺序总表（IB2 执行序列）

1. §1.1 表 7 行消费方 import 切换（container/router/宿主测试）→ 编译绿；
2. 删 7 个 shim 文件（§1.1 原路径列）；
3. 删 `is_unique_violation_parity_test.go` + 模块副本（随 §1.2 族收口，或族收口未就绪时保留副本只删 shim——以协调者裁定为准）；
4. §3 测试装置三项按各自 remove_at 处置；
5. 终态断言：宿主 `internal/{application/repository,application/service,handler}` 不再含任何 25c 面转发声明（framework:29）；`make verify-module-moves` 的 legacy `os.Stat` 通过与否取决于 manifest legacy 行处置（schema LOCKED，manifest 行删除由协调者统一裁定，本 Brief 不申请改 manifest）。

## 2. 推迟台账（6 文件；符号@文件:行 为计划撰写会话与实施轮实证，行号以对齐基线 `c0ddf768a` 为准）

| # | 推迟文件 | 硬阻塞（符号 @ 定义处 ← 调用点） | 约束类别 | 建议裁定（供协调者/IB2/B3） |
|---|---|---|---|---|
| 1 | `internal/application/service/skill_market_service.go`（519 行） | `skillhub.Client/SkillsetClient` 接口与 `SkillSummary/SkillsetSummary/ZipFiles` 数据类型、`skillhub.New/NewCached/DefaultCacheTTL/ParsePackage/MarketExpertID` 与 7 哨兵（`ErrStaleOnly:142,150,315,356`/`ErrUnreachable:168`/`ErrUnsupportedRanking:172`/`ErrInvalidSkillsetSlug:175,386`/`ErrPackage:294,441,495,498`/`ErrSkillsetNotFound:383`/`ErrMarket:495`）@`agentruntime/agent/skills/skillhub`（import :24）；`experts.MaterializeSkillset:434/MarketInstallDir:450/MarketDataRoot:98/WriteMaterializedExpert:451`（import :23）；`uniqueNonEmptyStrings`@`session_knowledge_qa.go:638-648`（conversation，35b 留宿）← :197 | guard forbidden-import（agentruntime 内部包）+ 跨 owner 未导出符号（成环）；数据形状面宽（接口签名携带 skillhub/experts 数据类型，注入需逐字段视图映射） | 优先：B3 R 面对 `agentruntime/agent/experts`、`…/skills/skillhub` 根门面 re-export 消费符号；备选：串行契约任务按 25b HostAdapters 模式宽视图注入（含 `uniqueNonEmptyStrings` 注入位）；推迟期间宿主原样跑绿 |
| 2 | `internal/application/service/tenant_expert_market_service.go`（339 行） | `experts.MaterializePublishedAgent:125/PublishedAgentExport:125-129/PublishedSnapshotDir:138,299/WriteMaterializedExpert:139,313/TenantExpertID:233,298/LoadExpertDir:299/MaterializedExpert:306/MarketInstallDir:312`（import :27）；`ErrAgentNotFound`@`custom_agent.go:21`（25a 批次 2 留宿）← :116 | 同 #1 + 同层哨兵成环（与 `agent_version.go:107` 同型） | 排序约束：`custom_agent.go` 属 25a 批次 2（IB2 内 25b 合并后搬迁），同包化后哨兵恢复可见；experts 面同 #1；两者齐备前留宿 |
| 3 | `internal/application/service/expert_market_source.go`（32 行） | `ExpertSource`@`expert_service.go:51`（25a 批次 2 留宿，`:32 var _ ExpertSource`）；`experts.InstalledExperts:29` 与 `[]*experts.Expert`（import :6）；消费方 `container.go:785` | 同层接口成环 + 方法签名携带 agentruntime 数据类型（注入无法消除签名本身） | 随 `expert_service.go` 搬迁 + experts re-export 后同包化；期间留宿零改动 |
| 4 | `internal/handler/skill_market.go`（262 行） | `limitJSONBody`/`skillSourceJSONMaxBytes`@`upload_limit.go:33/:19`（platform，B0.3 裁定 4 不得认领）← :124/:218；`isRequestBodyTooLarge`@`upload_limit.go:55` ← :127/:221；`skillJSONRequestTooLargeError`@`sandbox_skill.go:458` ← :128/:222；`decodeExpertInstantiateBody:178`/`maxExpertAgentNameLen:23`@`expert.go`（25a 批次 2 留宿）← :219/:228；`sandboxConfigTenantID`@`sandbox_config.go:92` ← :135/:164/:183 | 模块 handler ← 宿主 handler 物理环（宿主 handler 已含 25a/25c shim import 模块包，反向即环） | expert.go 三 helper 随 25a handler 批次 2 搬迁同包化；upload_limit 四 helper 须等「独立消费者抽取计划」（B0.3 裁定 4 明文）产出共享位或冻结导出；齐备前留宿 |
| 5 | `internal/handler/tenant_expert_market.go`（218 行） | 同 #4 全部（:100-110/:185-195）+ `expertServiceError`@`expert.go:260`（体内引用宿主 `service.ErrExpertNotFound/ErrAgentNameRequired`）← :202；`sandboxConfigTenantID` ← :122/:141/:158/:200 | 同 #4 | 同 #4 |
| 6 | `internal/handler/tenant_skill_market.go`（209 行） | `limitJSONBody/skillSourceJSONMaxBytes` ← :102/:178；`isRequestBodyTooLarge` ← :104/:181；`skillJSONRequestTooLargeError` ← :105/:182；`sandboxConfigTenantID` ← :114/:133/:150/:189 | 同 #4（无 expert.go 直接依赖，仅 platform/sandbox 系） | 同 #4（upload helper 抽取裁定后可先行搬迁） |

**反向义务核对**（25a Brief 登记的消费面，本节点维持现状零改动）：`handler/skill_market.go:219` + `handler/tenant_expert_market.go:186,202` 对 expert.go 三 helper 的消费继续由宿主同包满足；`skill_market_service.go:33,216` 对 `service.CatalogInstallResult` 的引用经 25b 残差别名（`tenant_skill_service.go:48`）继续编译——IB2 删 25b 残差时须与 #1 推迟件处置联动收口。**推迟件及其测试删除/搬迁点 = 各自行所属 barrier（§2 建议裁定列），不由 IB2 强制清空。**

## 3. 测试装置台账（TEST-SUPPORT-SHIM 裁定族；治理 YAML 本节点禁写，暂登记于此）

| # | 装置 | 位置 | remove_at |
|---|---|---|---|
| 1 | `openRunTestDB` 最小装置副本（sqlite 分支逐字，省略 postgres 分支与 `seedRunFixtures`——9 用例无该子测试） | `internal/agentcatalog/repository/agent_marketplace_test.go` 文件内（随迁白名单文件内追加，未新建文件） | IB2 或宿主 `agent_run_test.go` 装置导出时（先到者） |
| 2 | `requireAppErrorStatus` 包内副本（与留宿 `skill_market_service_test.go:292` 逐字同体） | `internal/agentcatalog/service/tenant_skill_market_service_test.go:189-200` | #1 推迟件测试搬迁时同删（届时留宿定义随迁或抽取共享） |
| 3 | `fakePublisherNames` 宿主垫片追加（+26 行；留宿禁改测试 `tenant_expert_market_service_test.go:365` 断链补齐） | `internal/application/service/tenant_skill_testsupport_test.go`（25b 已建唯一垫片文件，本节点追加尾节） | 推迟件 `tenant_expert_market_service.go` 及其测试搬迁时（25a 批次 2/B3 窗口）随该测试同删 |

## 4. 拆分确认请求（13 文件逐条，供 `25-agentcatalog-program.md` 确认；ownership-matrix 56 行 `plan: 25-agentcatalog-program` 无子行，矩阵行级回写归 barrier/协调者）

**已搬迁（7，终态头 `53646cb10`）**：

1. `internal/application/repository/agent_marketplace.go` → `internal/agentcatalog/repository/agent_marketplace.go`（+ 模块本地 `isUniqueViolation` 副本，§1.2）
2. `internal/application/repository/expert_install.go` → 模块 repository 同名
3. `internal/application/repository/published_skill.go` → 模块 repository 同名
4. `internal/application/repository/published_expert.go` → 模块 repository 同名
5. `internal/application/service/tenant_skill_market_service.go` → `internal/agentcatalog/service/tenant_skill_market_service.go`
6. `internal/application/service/agent_marketplace.go` → `internal/agentcatalog/service/agent_marketplace.go`（+ `market_host_adapters.go` 注入位新文件，§4-③）
7. `internal/handler/agent_marketplace.go` → `internal/agentcatalog/handler/agent_marketplace.go`（+ `marketTenantID` 本地 helper）

**推迟留宿（6，§2 台账）**：`service/{skill_market_service, tenant_expert_market_service, expert_market_source}.go`、`handler/{skill_market, tenant_expert_market, tenant_skill_market}.go`。

（7 + 6 = 13，与 25a 计划 §2.1 的 25c 拆分表一致；随迁测试 6 件 + parity 测试 1 件同步入模块。）

## 5. 计数零漂移声明与 contracts.yaml 回写申请

- **计数声明**：分支终态 `make check-backend-architecture` = `literal=564 apiKeyRoute=69 handle=0 total=633 | redis=23 lite=23 | hooks=58 | modules=16`、`OK (0 violations)`；`make verify-module-moves` = `OK (16 manifests verified)`（原路径 shim 保留使 legacy `os.Stat` 全通过）——633/23+23/58/537 对基线**零漂移**（evidence §5）。
- **contracts.yaml 回写申请（归 barrier，本节点未写治理 YAML）**：
  - `agentcatalog.marketplace-service`（:78-89）：实现文件路径宿主 → `internal/agentcatalog/service/agent_marketplace.go`；consumers 中 `service/agent_marketplace.go` → 模块路径（container.go 消费行不变，IB2 切换后同步）。
  - `agentcatalog.tenant-skill-market-service`（:130-140）：实现/消费路径同步为模块 service。
  - `agentcatalog.agent-version-service`（:9-23）：consumers `service/agent_marketplace.go`/`handler/agent_marketplace.go` 两行路径更新（门面本体零改动，本节点只消费）。
  - `agentcatalog.skill-market-service`（:119-129）：实现文件推迟（§2#1），consumers 路径**暂不变**，随推迟件搬迁窗口处置。
  - `agentcatalog.routes` 4 入口（:112/:116-118）：经 shim 类型别名路由面零漂移，无需回写。
- **manifest 处置申请**：`docs/architecture/moves/agentcatalog.yaml` schema LOCKED（无「已搬迁」字段），本节点未写；IB2 删 shim 后的 legacy 行处置（删行 or 保留占位）由协调者按 25b 先例（`3a7395d37` 保留占位残差）统一裁定。

## 6. 集成顺序与回归义务（IB2）

- 按框架顺序 25a → 25b → **25c（本节点）** 串行接入（framework:152）；本分支对齐基线已含 25a/25b 全部产出（Ruling WAVE-DEP-BASELINE），IB2 正式合并仍须走完整三支序。
- 回归最小集：39 用例等价清单（evidence §3/§4）+ `go test -count=1 ./internal/... -timeout=25m`（barrier 门禁）+ 两 make + 计数奇偶三方一致（conventions §8）。
- 高风险差分三项结论已落盘（evidence §4.3，全通过）；IB2 汇总本阶段差分时直接引用。
