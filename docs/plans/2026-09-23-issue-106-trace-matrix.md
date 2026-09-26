# Issue #106 追踪矩阵：Spec 条款 → Issue → Task → 测试/证据

日期：2026-09-23。范围：`docs/specs/2026-09-23-self-hosted-plugins-spec.md`（已批准）。
任务编号见 `docs/plans/2026-09-23-issue-106-00-index.md` 任务总表；测试/证据列为**计划规定的验收测试**，执行者在 T20 时把实际通过的测试函数名与 evidence 键回填确认。

## A. #106 十条验收边界（任务书确认，与 Spec Solution/Implementation/Testing Decisions 一致）

| # | 验收边界（摘要） | Spec 依据 | Issue | Task | 测试/证据 |
| --- | --- | --- | --- | --- | --- |
| B1 | 清单交付 + 管理员安装前预览核验（清单、版本端点、实际工具目录、schema、scope、读写分类、授权需求） | US7-9；ID49-50 | #108 | T01-T03 | `TestValidateManifest*`、`TestFetchRejectsNonHTTPAndPrivateManifestURLs`、`TestBuildVerifiedSnapshotRejectsMismatch`、`TestPreviewPersistsFingerprintAndExpiry`、handler `TestPreviewManifestHandler`（含 ReturnsPreview happy path）；T20 `TestPlugin106Acceptance/B1_清单交付与管理员安装前预览核验`（evidence `B1`：非管理员 403 / 字段齐备 / digest 口径 / 受限地址 / 声明不符） |
| B2 | 安装固定版本；差异展示+手动接受；升级失败不破坏旧版 | US10/12/13/24；ID51 | #110/#114/#115 | T06/T14/T16 | `TestConfirmInstallation*`、`TestDiffSnapshotsCoversAllDimensions`、`TestPreviewUpgradeDoesNotTouchInstallation`、`TestAcceptUpgradeIdempotentAndFingerprintGuard`、`TestAcceptUpgradeFailureKeepsOldVersion`、`TestUpgradeVisibleFromMemberConversation`、`TestTwoSpacesDifferentVersions`；升级重抓来源=`plugin_installations.manifest_url`（T06 落列，迁移测试断言）；T20 `TestPlugin106Acceptance/B2_安装固定版本+差异+手动接受+失败保旧`（evidence `B2`：固定 1.0.0 / diff / 失败保旧 / 接受 2.0.0） |
| B3 | 运行时以已接受快照为核验基准，阻止未接受的工具扩张/漂移 | US25；ID52 | #116 | T09/T17 | `TestRegisterMCPToolsFiltersPluginToolsToAcceptedSnapshot`、`TestRegisterMCPToolsRejectsSchemaDrift`、`TestDiffLiveAgainstSnapshot`、`TestCheckDriftPersistsDetectedState`、`TestResolveDriftRebasesSnapshot`、`TestDriftBlocksFromMemberConversation`、`TestDriftScopedToSingleInstallation`；T20 `TestPlugin106Acceptance/B3_运行时以已接受快照阻断漂移`（evidence `B3`：发现被阻 / detected+明细 / resolve 重定基） |
| B4 | 成员发现已安装插件；个人授权按空间/成员/插件连接隔离 | US16/17/19/26；ID53 | #110/#112 | T07/T11 | `TestListInstallationsTenantScoped`、`TestGetInstallationRejectsForeignTenant`、`TestGetMyConnectionStatus*`、`TestMemberConnectionsIsolatedAndRevocable`、`TestExpiredTokenGuidesReauthorization`；T20 `TestPlugin106Acceptance/B4_成员发现+个人授权隔离`（evidence `B4`：发现 / A-B 凭据隔离 / 撤销只影响本人 / 跨空间 not found） |
| B5 | 新写工具默认关闭；逐项启用+成员审批；拒绝/超时不派发 | US14/15/22；ID54 | #117/#118 | T18/T19 | `TestUpgradedWriteToolStaysDisabledAndZeroExternalWrites`、`TestMemberAuthorizationDoesNotEnableWriteTool`、`TestListInstallationToolsShowsDisabledReason`、`TestWriteApprovalApproveDispatchesOnce`、`TestWriteApprovalRejectTimeoutDisableZeroWrites`、`TestReadOnlyToolsDoNotPromptApproval`；T20 `TestPlugin106Acceptance/B5_写工具默认关闭+审批零写入`（evidence `B5`：默认关 / 拒绝零派发 / 超时零派发 / 审批卡带参数原文） |
| B6 | 停用、撤销、过期、候选不可达等状态正确行为 | US11/18/23/24；ID53 | #110/#112/#113/#114/#115 | T06/T11/T13/T14 | `TestSetInstallationStateSyncsService`、`TestUninstalledOrDisabledOrForeignTenantRejected`、`TestMemberConnectionsIsolatedAndRevocable`（撤销分支）、`TestExpiredTokenGuidesReauthorization`、`TestPreviewUpgradeCandidateUnreachable`、`TestAcceptUpgradeFailureKeepsOldVersion`、`TestJiraFailureModesDoNotFabricate`；T20 `TestPlugin106Acceptance/B6_停用撤销过期候选不可达`（evidence `B6`：停用不可见+旧 ref 拒绝 / 撤销 / 过期引导 / 候选不可达保 v1） |
| B7 | Jira 纵向案例：本人本周待办、来源链接、不接受模型令牌/任意 URL、服务端限定时间范围 | Solution 第 13 行；ID56；US20/21 | #109/#113 | T04/T05/T13 | `TestListToolsIsPublicAndSchemaIsFixed`、`TestTwoMembersIsolated`、`TestJiraErrorsSurfaceWithoutFabrication`、`TestEmptyWeekIsEmptySuccess`、`TestToolRejectsExtraneousArguments`、`TestJiraTodoVerticalEndToEnd`、`TestModelCannotInjectAccountParameters`；T20 `TestPlugin106Acceptance/B7_Jira纵向案例`（evidence `B7`：本人本周+来源链接 / B 隔离 / 注入拒绝零外呼） |
| B8 | 无账号工具独立使用，仍受空间启停与工具策略约束 | US5；ID53/54 | #111 | T09/T10 | `TestAgentCallsNoAccountPluginToolEndToEnd`、`TestRegisterMCPToolsNilGuardKeepsLegacyBehavior`、`TestUninstalledOrDisabledOrForeignTenantRejected`（停用分支）；T20 `TestPlugin106Acceptance/B8_无账号工具独立能力`（evidence `B8`：discover/describe/call 成功 / 停用后隐藏） |
| B9 | 手工 MCP 路径兼容；旧工具不因迁移被自动视为已批准插件版本 | US27；ID55 | #110 等 | T06/T07/T09/T18 | `migration_pg_integration_test`（000190.down 后手工服务仍在；**注**：该测试的 harness 存量顺序缺陷见 T20 执行记录 concerns，T20 以 `accMigrationDownKeepsManual` 复述同一契约）、`TestManualMCPServiceUnaffectedByInstallations`、`TestManualMCPServiceCoexists`、`TestDriftManualServiceNotGoverned`、`TestManualDefaultSemanticsUnchanged`、`TestRegisterMCPToolsNilGuardKeepsLegacyBehavior`；T20 `TestPlugin106Acceptance/B9_手工MCP兼容`（evidence `B9`：同会话共存 / 缺行=启用（读写均然）/ 000190.down 后手工行幸存） |
| B10 | 清单抓取/发现/执行遵守既有网络、租户、凭据边界；open-connector 遵守 ADR-0001 | ID57 | #108/#109（横切） | T01/T02/T04 | `TestFetchRejectsNonHTTPAndPrivateManifestURLs`、handler `TestPreviewManifestHandler`（file:// 分支）、`ConfirmInstallation` 重核、示例服务凭据仅 env；T20 `TestPlugin106Acceptance/B10_网络与租户边界`（evidence `B10`：预览/升级受限地址拒绝 / 漂移核验 fail-closed 零持久化 / 跨空间隔离） |

## B. Implementation Decisions 逐条（Spec 45-57 行）

| Spec 条款 | Issue/Task | 测试/证据 |
| --- | --- | --- |
| ID47 领域语言：插件=版本化分发单位；安装=空间接受版本；个人连接/操作沿用；清单、安装、个人连接独立状态 | 全部（CONTEXT.md 词条既有） | `types.PluginManifest`/`PluginInstallation`/`mcp_oauth_tokens` 三态分离（实现结构即证据）；T20 走查 |
| ID48 插件自行部署，WeKnora 作远程 MCP 客户端；不托管代码/不走本机进程 | #108-#118 | `examples/plugins/jira-todo-mcp` 独立部署 + `EndpointLister` 客户端核验；无进程托管代码 |
| ID49 清单 URL 安装；安装前校验清单/版本端点/实际工具目录；清单权限声明不构成执行授权 | #108 T01/T02 | B1 全部测试；写工具默认关闭（B5）证明声明≠授权 |
| ID50 清单最小字段；具体 JSON 由实施计划确定 | T01（协议固化，GAP-1） | `TestValidateManifest*`；协议定义 `internal/types/plugin.go` |
| ID51 空间安装固定已接受版本；升级审阅手动接受；旧端点可用约定；能力漂移阻断 | #110/#114/#115/#116 | B2/B3 全部测试 |
| ID52 已接受快照=运行时核验基准；新工具/移除/schema 变不自动采用；提示复审；跨空间不互相影响 | #116 | `TestDriftScopedToSingleInstallation`、B3 |
| ID53 成员可发现；未授权不执行；授权按空间/成员/插件连接隔离；停用阻止调用；个人撤销只影响本人 | #110/#111/#112 | B4/B6/B8 测试 |
| ID54 新增写工具默认关闭；逐项启用+成员审批；审批针对确定目标和输入；拒绝/超时不派发；无账号工具受启停与策略约束 | #117/#118 | B5 全部测试 + `TestAgentCallsNoAccountPluginToolEndToEnd`（无账号受启停） |
| ID55 复用现有 MCP 服务管理/元数据/OAuth/工具注册/启停/审批 + 产品层；手工兼容；迁移不自动视为已批准插件版本 | 全部（物化架构） | B9 测试；000190 up 零回填（migration 测试断言） |
| ID56 首案例个人 Jira 授权只读；从已授权身份确定账号；不接受模型传令牌/URL；服务端限定本周；返回标识/标题/状态/截止/来源链接 | #109/#113 | B7 全部测试 |
| ID57 网络与租户边界；open-connector 须遵守 ADR-0001；不迁入、不获得管理凭据 | 横切（T01/T02/T04） | B10；插件域零触碰 `MCPOAuthBindingStore`/open-connector 模块（diff 审查项） |

## C. Testing Decisions 逐条（Spec 59-66 行）

| Spec 条款 | Task | 测试/证据 |
| --- | --- | --- |
| TD61 应用边界集成测试为主要验收 seam；受控远程 MCP 服务 + 两名成员；只断言对外可观察结果 | T10/T20 | `plugin_agent_integration_test.go`、`plugin_acceptance_test.go`（组装走真实 service/repo/manager/Gate，不测内部调用顺序） |
| TD62 覆盖安装预览与确认、清单和远程工具不一致、插件停用、只读 Jira 查询、未授权引导、两名成员凭据隔离、个人撤销和授权过期 | T02/T06/T11/T13 | B1/B4/B6/B7 各测试 |
| TD63 覆盖旧版继续可用、候选版本差异、管理员接受升级、候选不可达、远程工具漂移、新版新增写工具保持关闭 | T14/T16/T17/T18 | B2/B3/B5 各测试 |
| TD64 覆盖逐项启用写工具、成员批准、拒绝和超时；有记录的测试服务确认拒绝或超时后没有外部写入 | T18/T19 | `plugintest.Server.WriteCalls()` 计数断言（零写入） |
| TD65 覆盖无需账号的独立能力、空间成员可见性、跨空间隔离和原有手工 MCP 服务兼容性 | T07/T09/T10 | B8/B9 测试 |
| TD66 复用既有测试基础设施（MCP 元数据快照、OAuth 主体隔离、MCP 工具目录与启停、Agent 工具调用、AppConnector 远程定义 pin 与集成测试） | T01/T10/T11 | `sdkserver+httptest` 先例（mcp_catalog_integration_test.go）、`OCSchemaDigest` SHA-256 同族、oc_integration blocked-env 模式（`PLUGIN_TEST_DATABASE_URL`） |

## D. Spec Further Notes 前置项（81 行 → GAP-1…GAP-6 落实）

| 前置项 | GAP | Task | 证据 |
| --- | --- | --- | --- |
| 清单协议固化 | GAP-1 | T01 | `weknora.plugin/1` 类型 + `TestValidateManifest*` |
| 网络抓取限制 | GAP-2 | T01/T02 | `ValidateURLForSSRF` 前置 + `TestFetchRejectsNonHTTPAndPrivateManifestURLs` + SSRF 安全客户端 |
| 版本漂移处置 | GAP-6 | T17 | `CheckDrift`/`ResolveDrift` 闭环 + `TestResolveDriftRebasesSnapshot` |
| OAuth 身份映射 | GAP-4 | T11 | (tenant, principal, service_id=installation.service_id) 映射（不新建表）+ `TestMemberConnectionsIsolatedAndRevocable` |
| 数据迁移与回退 | GAP-3 | T02/T06 | 000189/000190 up/down + SQLite twin + `migration_pg_integration_test` 回退安全断言 |
| （裁决 GAP-5 授权链） | GAP-5 | T09/T19 | `approval.Gate` 链路裁决（index 总索引记录）+ T19 真实 Gate 验收 |

## E. Out of Scope 守护（Spec 68-74 行）

| Out of Scope | 计划如何守护 |
| --- | --- |
| 公开第三方插件市场/平台代管代码/上传代码包/执行任意进程 | 无任何市场/上传端点；安装仅消费开发者自托管清单 URL |
| 空间共用 Jira 账号和空间连接 | T11 仅 `connections/me`（个人）；无空间连接 API |
| 本机 stdio 插件执行 | `ValidateManifest` 拒绝 `stdio` transport（T01 测试 case） |
| 首个案例创建/修改/删除 Jira 工单 | 示例服务只读工具；写操作仅由受控测试服务验收（T18/T19） |
| 改造 WeKnora 对外 MCP Server 方向/强制迁移已有 MCP 服务 | `mcp-server/`、`cli/internal/mcp/server.go` 零改动（Global Constraints）；手工服务兼容断言（B9） |
