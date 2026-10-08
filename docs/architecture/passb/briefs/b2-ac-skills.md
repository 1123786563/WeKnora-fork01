# Integration Brief — b2-ac-skills（25b 租户 Skill 目录/安装/运行时验证/reaper → agentcatalog）

> 交付对象：IB2（第二集成屏障，按 25a→25b→25c 汇总切换，framework:152）。barrier 只从已评审 Brief 切换共享装配（framework:103）。
> 节点分支：`codex/passb-b2-ac-skills`；Brief 定稿于 T5（2026-09-24）。全部行号锚点在终态 HEAD 实测（grep/Read）。
> 差分证据：`docs/architecture/evidence/passb/b2-ac-skills.md`；实施报告：`docs/plans/passb/reports/b2-ac-skills.md`。
> 本面交付形态：20 个生产文件已迁入 `internal/modules/agentcatalog/{repository,service,handler}`（4 条 execution→agentcatalog 符号 + skillsForRun 已在新包导出化）；宿主旧路径仅存残差/占位，编译与全部既有测试绿；**装配未切换**（container/routes 仍指宿主残差，禁改文件零改动）。

## (a) 残差全删清单（前置：(b)(c)(d) 切换完成且差分绿）

**生产残差/占位 20 个**（宿主旧路径，全部 `remove_at: ib2`）：

| # | 文件 | 形态 |
|---|---|---|
| 1 | `internal/application/service/tenant_skill_service.go`（160 行） | 残差：类型别名族 + 构造器旧参转发 + 4 符号 1:1 委托（:148-160）+ `init()` 注册（RegisterReservedEnvNames/RegisterBundleParsers）+ §2.5 未列消费点承接（keyedMutex 等） |
| 2 | `internal/application/service/tenant_skill_effective.go`（29 行） | 残差：`skillsForRun` 包装 → `acatsvc.SkillsForRun` |
| 3 | `internal/application/repository/tenant_skill.go`（22 行） | 残差：同名重写转发 |
| 4 | `internal/handler/skill_handler.go`（44 行） | 残差：同形未导出接口 + `type SkillHandler = acathandler.SkillHandler` + `NewSkillHandler` 1:1 转发 |
| 5 | `internal/handler/skill_catalog.go` | 纯注释占位 stub（零业务声明） |
| 6–20 | `internal/application/service/tenant_skill_{admin,bundle,catalog,env_declare,files,install,progress,reaper,remove,runtime_verify,source,steer,stop,transcript,verify}.go`（15 个） | 纯注释占位 stub（manifest legacy 路径存在性占位，逐文件 0 业务行，framework:29 合规） |

**测试装置 2 个**：

| 文件 | 依据 | 删除时点 |
|---|---|---|
| `internal/application/service/tenant_skill_export_parity_test.go` | 差分装置（计划 §1 节点产出） | `remove_at: ib2`（(b)(c)(d) 切换后残差 1:1 断言失去对象） |
| `internal/application/service/tenant_skill_testsupport_test.go` | Ruling 2026-09-24-TEST-SUPPORT-SHIM（宿主孤儿测试装置垫片，不进生产编译） | IB2/25c 先到者，最迟 B5（裁定原文） |

注：计划 §1 曾拟残差文件名 `tenant_skill_residual.go`（新）；T3 实际采用 `tenant_skill_service.go` 同名重写形态（与 T2 repository 残差同模式，已审）。IB2 删除时以本表为准。

## (b) 调用点改指导出名（宿主消费方文件，IB2 执行；本节点禁改零改动）

| 调用点（终态实测行号） | 现状（经残差 1:1） | 切换为 |
|---|---|---|
| `internal/application/service/tenant_sandbox_config.go:1134` | `skillSnapshotNamePrefix(...)` | `acatsvc.SkillSnapshotNamePrefix(...)` |
| `internal/application/service/tenant_sandbox_config.go:1135` | `snapshotsNotFromOtherConfig(...)` | `acatsvc.SnapshotsNotFromOtherConfig(...)` |
| `internal/application/service/tenant_sandbox_config.go:1137` | `matchSnapshotByName(...)` | `acatsvc.MatchSnapshotByName(...)` |
| `internal/application/service/user_env.go:275`、`:381` | `validateUserEnvName(...)` | `acatsvc.ValidateUserEnvName(...)` |
| `internal/application/service/session_agent_qa.go:357` | `skillsForRun(...)` | `acatsvc.SkillsForRun(...)`（首参 `PinnedConfigReader` 窄接口；`*SessionSandboxPinner` 已满足——`Read(ctx, sessionID)` 签名一致） |
| `internal/application/service/expert_skills.go:36`、`:43` | `installedSkillLister`（宿主别名） | `acatsvc.InstalledSkillLister` |

说明：若 13-execution 搬迁在 IB2 前已落地，`tenant_sandbox_config.go`/`user_env.go` 调用点改按其 Brief 的注入端口形式，本表导出名即其注入缺省真源。`user_env.go` 切换后，残差 `init()` 中 `RegisterReservedEnvNames(skills.InjectedSandboxEnvVars())` 注册通道随 (a) 删除；`WEKNORA_SKILL_` 注入保留名名单的归属由 execution 面按契约任务决定（计划 §8(b) 原文）。

## (c) container.go 接线（禁改文件，IB2 单写；行号为终态实测）

| 行 | 现状 | 切换为 |
|---|---|---|
| `internal/container/container.go:293` | `must(container.Provide(repository.NewTenantSkillRepository))` | `acarepo.NewTenantSkillRepository` |
| `internal/container/container.go:566` | `must(container.Provide(service.NewTenantSkillService))` | `acatsvc.NewTenantSkillService` + `HostAdapters` 装配（11 位：ResolveConfigManager/InstallShellExecutor/SessionUserID/SkillManifestParser/FrontmatterVersionParser/InstallerToolNames/StreamContentForToolResult/SanitizeToolResultForClient/SanitizeAgentStepsForStorage/OnDemandInstallerPath/UniqueNonEmptyStrings；除 SessionUserID 外必填。七个 agentruntime 位在 IB2 改绑对端导出真源或维持残差闭包，按 33/35 面 Brief） |
| `internal/container/container.go:702` | `must(container.Invoke(startTenantSkillReaper))` | 调用形态不变；:2406 `startTenantSkillReaper` 参数类型 `*service.TenantSkillService` → `*acatsvc.TenantSkillService`；cleanup 注册名 `"TenantSkillReaper"`（:2413）不变 = `agentcatalog.lifecycle` 契约（startTenantSkillReaper，container.go:702/2406）不变 |
| `internal/container/container.go:802` | `return handler.NewSkillHandler(s, s)` | `acathandler.NewSkillHandler(s, s)` |

## (d) routes_agent.go RegisterSkillRoutes

`internal/router/routes_agent.go:76`：参数 `*handler.SkillHandler` → `*acathandler.SkillHandler`；或改走 `internal/modules/agentcatalog/module.go` 的 `RegisterRoutes`——IB2 按 25a→25b→25c 汇总裁定（framework:152）。现状宿主残差别名下 `router_api_key_capabilities_test.go:448`、`routes_skill_market_test.go:31` 零改动编译（T4 实测）。

## (e) contracts.yaml 回写（barrier 义务，本节点零改动——T3 越权改动已整体回滚，见 evidence §5.4）

1. `agentcatalog.facade/routes/lifecycle` 符号路径改指新包（`acatsvc`/`acathandler`）。
2. `agentruntime.agent-engine/.agent-service` 的 characterization_tests 中 `tenant_skill_install_test.go` 改新路径。
3. **`make check-passb-readiness` 当前红（10 条诊断 = IB2 收口清单，已登记 execution-ledger.md 2026-09-24 小节）**：
   - consumer-unrecorded（7）：`agentcatalog.custom-agent-service`、`agentruntime.agent-engine`、`agentruntime.agent-service`、`airesource.model-service`、`airesource.storage-backend-resolver`、`conversation.session-service`、`conversation.stream-manager`×2——新包 `internal/modules/agentcatalog/service/tenant_skill_{service,install,transcript}.go` 引用未登记；
   - consumer-vanished（3）：`agentruntime.agent-engine` → 宿主 `tenant_skill_install.go`（占位）、`conversation.stream-manager` → 宿主 `tenant_skill_transcript.go`（占位）/`tenant_skill_service.go`（残差）；
   - 收口动作：随 (a) 残差/占位删除与装配切换，把记录路径改指新包文件（stream-manager 的宿主残差 service.go 行随删除移除）；修后该 gate 应绿。

## (f) DAG 回写确认

1. `b2-ac-skills.required_contracts` 第 2 条「execution→agentcatalog 4 条未导出调用导出化落地」：已落地——`MatchSnapshotByName`/`SkillSnapshotNamePrefix`/`SnapshotsNotFromOtherConfig`/`ValidateUserEnvName` 新包导出名实测命中 4（evidence §4 T5 行），残差 1:1（§10.9 核验），parity 逐例等价（evidence §3 + §2 终态复跑 5/5 PASS）。
2. `conversation→agentcatalog skillsForRun` pair：已收口——`acatsvc.SkillsForRun` 导出（首参 `PinnedConfigReader` 化），宿主 `skillsForRun` 包装残差 1:1，`session_agent_qa.go:357` 消费零回归（宿主 service 全量 ok）。

## (g) 例外台账（如实口径，替代计划 §9 撰写时点文本）

- 本节点经 **Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY**（协调者授权，独立 commit 8978183b3）**新增 8 条**：`exc-0106..0113`（exception-ledger.yaml 105→113，`remove_at: ib2`，精确 file→package 豁免 `internal/modules/agentcatalog/**` → `internal/execution/sandbox`；tools/architectureguard/check.go importExceptions `PassBTask: B-agentcatalog` 同步）。**IB2 随对端端口化删除该 8 条并同步 pass-a-acceptance.md 计数**（conventions §8 三方一致）。
- 除上述裁定新增外：零删除、零通配、无其他 exc 提案（framework:38）。
- 无新增长期别名：全部残差接缝（含 `RegisterReservedEnvNames`/`RegisterBundleParsers` 导出接缝）随 (a) 删除；`SkillHandler` 别名等由 IB2 装配切换取代（IB2 后本面不得仍有宿主包内 Skill 符号转发，计划 §9 禁止项）。
