# Onyx ↔ WeKnora 能力差距矩阵（craft + connectors + 平台运营面）

> **任务**：以 Onyx 仓库为参照，盘点 craft 功能面、连接器（connectors）体系与平台运营面（用量/套餐/分析/查询历史/OpenAPI/对话偏好）在 WeKnora fork 的现状差距，作为增量对齐的分期决策输入。
> **对照维度说明**：`docs/upstream-parity/LEDGER.md` 对照的是 Tencent/WeKnora 上游；本矩阵对照 **Onyx**，是独立维度，条目编号不与该台账混用。craft/appconnector 在该台账中属"fork 独有域（不适用上游对照）"，本矩阵补上 Onyx 这一参照系。

## 基线

| 项 | 值 |
|---|---|
| Onyx 参照 | `/Users/wuyongjun/trea/onyx`（工作副本，2026-09-19 盘点） |
| WeKnora fork | `/Users/wuyongjun/trea/WeKnora-fork01`（main） |
| 盘点方式 | 双侧打开代码逐项核对（非目录名推断）；craft 28 项、connectors 30 项 + 逐源清单 |

## 定性图例

✅对齐 ｜ 🟡部分 ｜ ❌缺失 ｜ 🔷有意不同（产品/架构形态差异，是否采纳需产品决策） ｜ ⛔不适用

---

# 一、Craft 功能面（28 项）

## 1.1 会话与交互

| # | 能力 | Onyx 实现要点 | WeKnora 现状 | 定性 | 备注 |
|---|------|--------------|--------------|------|------|
| C-1 | 会话 CRUD/恢复 | `backend/onyx/server/features/build/session/api.py:87/105/164/320/357`（list/create/get/delete/restore）；BuildSession `db/models.py:6353` | `internal/handler/session/craft.go:118-119/127/145`（create/list/view/restore），restore=快照版本回滚；lifecycle 以 TombstoneSession 跟随通用 session 删除 `internal/application/service/craft_lifecycle.go:313,620` | 🟡部分 | 无 craft 会话级 delete/rename 端点；"restore"语义不同（Onyx 恢复运行中会话 vs WeKnora 快照回滚） |
| C-2 | 发消息/交互回合/SSE | `session/messages.py:80`、`interactive_turns/api.py:108/128`（turns SSE） | `handler/session/craft.go:132`（POST craft/runs）+ 通用 run SSE `agent_run.go:131`、`router/routes_chat.go:75`（Last-Event-ID 续读）；前端 `apps/web/src/features/craft/routes.tsx:63-81` | ✅对齐 | 复用 agent_runs 持久事件流 |
| C-3 | 中断/steer | `session/messages.py:289` interrupt + `session/interrupt_signal.py:26`（跨副本 Redis fence） | 通用 cancel `agent_run.go:272` + durable steer `handler/session/steer.go:556,360`；craft 专用验证式 Stop 已挂生产路由 `POST /sessions/:session_id/craft/runs/:run_id/stop`（`handler/session/craft_interaction.go:71`，routes_chat.go 装配）+ workbench 停止入口与 stopping 轮询 | ✅对齐（SP1） | SP1 完成：路由+执行器注入+api-client+前端停止入口 |
| C-4 | 会话消息持久化 | build_message 表逐 packet 落行 `db/models.py:6665` | 无等价表；user/assistant 消息对 `internal/application/repository/agent_run.go:116` + 事件流 + C05 快照链 `internal/craft/snapshot.go:75` | 🟡部分 | 模型不同：Onyx 逐 packet，WeKnora 消息对+事件回放 |

## 1.2 沙箱

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| C-5 | 沙箱模型 | 每用户沙箱，Docker/K8s 双 manager，四态状态机 `db/models.py:6443` | 每 **session** 沙箱，docker/cube(MicroVM)/e2b 三后端 `internal/sandbox/sandbox.go:14-29`；无 K8s、无 DB 状态机（绑定表+锁+CAS） | 🟡部分 | 粒度与生命周期管理方式不同；WeKnora 以绑定锁等价覆盖治理目标 |
| C-6 | 空闲回收+快照休眠+恢复 | SLEEPING 判定+S3 快照归档+reconcile `session/sandbox_lifecycle.py:111/174/495`、`sandbox/snapshot_manager.py:34` | Dormancy 规则（须 verified snapshot 才许 pause）`internal/craft/lifecycle.go:125`；sweep 六态 `internal/application/service/craft_lifecycle.go:543,749`；快照恢复 RestoreIdempotent `handler/session/craft.go:609` | 🟡部分 | 语义覆盖，无 S3 归档休眠管道；恢复=重放而非 pause/resume 实例 |
| C-7 | 沙箱内 agent 运行时 | opencode-serve HTTP，版本随镜像浮动 | 固定版 OpenCode **1.18.4** 协议锁 `internal/agent/opencode/protocol.go:14,32`；RuntimeDigest 钉死身份 `internal/craft/snapshot.go:48` | 🔷有意不同 | WeKnora 版本治理更严 |
| C-8 | provision 轮询/生命周期事件 | reserve/reconcile/POLL_INTERVAL 轮询 `session/sandbox_lifecycle.py:345/495/654` | createAndBind 同步绑定+分布式锁 `internal/sandbox/session_lifecycle.go:421,568`；Cube 模板构建有轮询 `internal/sandbox/cube_remote_client.go:404` | 🟡部分 | 以绑定 CAS+锁替代轮询状态机 |

## 1.3 产物与预览

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| C-9 | Artifact upsert | content_hash 变更检测+版本 bump `db/artifact.py:20`、`db/models.py:6503` | 不可变版本模型：VersionKey 三元 hash `internal/craft/version.go:23`、Publish 幂等 `:36-51` | 🔷有意不同 | WeKnora 旧下载永不漂移，更严 |
| C-10 | Web 应用实时预览反代（HMR WS） | `webapp_proxy.py:167/235-280`、webapp-info/download `session/api.py:678/700` | 无运行中反代；W02 受控静态预览：5min ticket+隔离 origin+CSP 全套 `internal/craft/preview.go:20-126`、`internal/handler/session/craft_preview.go:55-168` | 🔷有意不同 | WeKnora 只服务已发布 pinned 字节，明确拒绝代理任意 host/port；无 HMR |
| C-11 | 文档/PPTX/表格预览导出 | export-docx 即时转换 `session/api.py:609`、pptx-preview 转图 `:651` | 三 kind 原生契约：document/slides/spreadsheet 各自 deliverable+机器门 `internal/craft/document.go:33-35`、`slides.go:30-34`、`spreadsheet.go:31,159` | 🔷有意不同 | 生成与校验在沙箱 skill 内同轮完成 |
| C-12 | 文件树浏览/上传进沙箱 | list_directory `session/api.py:520`、upload `sandbox/base.py:508` | ListSessionFiles 仅 agent 工具/内部用 `internal/sandbox/session_manager.go:621`；**无 HTTP 文件树端点**；上传=inputs 关联（见 C-17） | 🟡部分 | 缺"实时沙箱工作区目录树"的 UI/API |

## 1.4 知识库集成

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| C-13 | 沙箱内检索（company-search CLI+PAT） | 内置技能+出口代理注入 PAT `onyx/skills/built_in.py:181,221` | C01 staged 知识包（64KiB 上限）`internal/craft/knowledge.go:20` + 执行期受控检索工具（服务器侧 ACL 复核、8 次上限）`internal/application/service/craft_knowledge_tool.go:17-36` | 🔷有意不同 | WeKnora 明确不放凭据进子执行，服务器侧代理检索 |
| C-14 | skills 推送 | SKILLS_MOUNT_PATH 运行时 mount 推送 `onyx/skills/push.py:53` | C06 技能版本 pin（name+digest）`internal/craft/skill.go:22,39`；物化进镜像 `internal/sandbox/sandbox.go:206` | 🟡部分 | 版本治理比 Onyx 强；无"变更后推送进已建沙箱"通道 |
| C-15 | AGENTS.md 指令模板渲染 | `sandbox/util/agent_instructions.py:104`（模板占位填充） | **无**（全仓零匹配）；委托 prompt 仅 goal+input_refs `internal/agent/tools/craft_delegate.go:35-38` | ❌缺失 | |
| C-16 | craft 内 MCP | `sandbox/util/mcp_config.py:35/78`（available_in_craft 过滤+凭据写 opencode.json） | **无**（craft 链路内零匹配；主 agent MCP 服务不接入 craft） | ❌缺失 | C06 明确 skill/manifest 不携带权限 |
| C-17 | 会话附件上传进沙箱 | PromptAttachment→attachments/ `session/manager.py:1119` | POST craft/inputs+内容寻址+20 文件/100MiB 上限 `internal/handler/session/craft.go:131`、`internal/craft/input.go:26,62`；前端上传→轮询→关联 `apps/web/src/features/craft/routes.tsx:415-444` | ✅对齐 | |
| C-18 | User Library（跨会话文件库） | `user_library/api.py:151/179/283/448/482` + 专用连接器走索引 | **无** | ❌缺失 | craft inputs 是 per-workspace |
| C-19 | 知识引用管控 | chat 侧 citation 体系 | kc_ 引用 id+document 门强制真实引用 `internal/craft/knowledge.go:85`、`document.go:65,145`；点击经 ACL 复解析 `routes.tsx:171-183` | 🔷有意不同 | WeKnora 更严（产物级引用门） |

## 1.5 外部应用与审批

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| C-20 | ExternalApp（OAuth/凭据/策略） | ExternalApp `db/models.py:7222`、凭据 `:7302`、Skill 关联 `:7198` | **无**（craft 侧） | ❌缺失 | 注意：WeKnora 的 appconnector 体系（见第二部分）已覆盖同类概念，缺的是 craft 沙箱↔appconnector 的桥 |
| C-21 | ActionApproval+approvals API | `approvals/api.py:120/144/201`（live/decision/session-grant） | C02 交互决定面：interactions+decide `internal/handler/session/craft_interaction.go:65-66`；args hash 绑定+revision CAS `internal/craft/decision.go:59` | 🔷有意不同 | Onyx=主 agent 写操作审批+粘性授权；WeKnora=子执行 question/permission（approve-once，无粘性） |
| C-22 | OAuth 回调页 | `external_apps/oauth.py:84/134` + 前端回调页 | **无**（craft 侧；appconnector 有自己的 OAuth） | ❌缺失 | |

## 1.6 任务与治理

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| C-23 | ScheduledTask 定时任务 | CRUD+run-now+Celery+预授权目标 `scheduled_tasks/api.py:397-558`、`db/models.py:6756/6853/6934` | **无**（craft 内零匹配） | ❌缺失 | |
| C-24 | 限流/预算 | 429 token/cost turn-budget | 调用次数制 BudgetGrant+原子 AuthorizeCall `internal/craft/budget.go:46-62`、`internal/application/service/craft_budget.go:275`；BUDGET_STOPPED 402/403 `internal/handler/craft_model_gateway.go:562-575`；O01 usage 账本 | 🔷有意不同 | 次数+商业化预留制 vs token/cost 限流 |
| C-25 | LLM 网关归属判定 | `craft_gateway.py:10`（权限判定） | O02 受控模型网关已生产挂载：`router/router.go:236-237` 挂 `POST/GET /api/v1/craft/model-gateway/v1/*`，cmg1 HMAC 短时凭据签发面 `routes_chat.go:160-166`，fail-closed secret，上游凭据仅服务器凭据仓（`internal/handler/craft_model_gateway.go`） | ✅对齐（SP1） | SP1 完成；G4 commercial_reservations owner 列已补（PG 000158/sqlite 000079） |
| C-26 | 三层功能开关 | 部署 ENABLE_CRAFT/PostHog + 工作区默认 + 用户级 `utils.py:156` | 仅部署级 `internal/container/container.go:2184-2202`（WEKNORA_CRAFT_ENABLED+WEKNORA_CRAFT_KINDS）；gate 快照随 API 下发 | 🟡部分 | 无工作区/用户级；kind 级开关是 WeKnora 特有 |
| C-27 | 管理后台三页 | admin craft access/apps/preferences | **无**（apps/web/src/administration/ 无 craft 内容） | ❌缺失 | |
| C-28 | onboarding/LLM 设置引导 | craft onboarding + LLM 偏好 | 无 craft 专用引导；模型取租户默认 `internal/application/service/craft_session.go:840` | ❌缺失 | |

## 1.7 Craft 反向差距（WeKnora 已有、Onyx 没有）

1. 三产物 kind 契约与机器门（D01-D03：文档四门/公式白名单/CSV 消毒/页图契约）
2. 不可变版本+内容身份体系（W01：凭据形状文件名拒绝进入产物）
3. 隔离 origin 受控预览（W02：一次性 ticket+CSP/COOP/CORP 全套）
4. 持久化交互决定流（C02：args hash 防批 A 授权批 B、CAS、410 gone、"answer 永不等于 approve"）
5. 崩溃恢复程序（C04：reuse/collect/observe/wait 四路由，unknown 永不自动重发）
6. 恢复快照链前缀校验（C05：双摘要+runtime digest 匹配）
7. 生命周期 sweep 六态+四类不可回收+byte-day 计量（O03）
8. usage 账本（O01/O04：unknown 不折零、corrected 修订链）
9. 技能内容 pin+manifest 权限词扫描拒绝（C06）
10. cmg1 HMAC 短时执行凭据体系（O02，待挂载）
11. 沙箱交互终端 WebSocket（每分钟复检 token/user/ownership）
12. Cube(Tencent MicroVM)/E2B/Docker 三后端+模板目录
13. ReleaseFacts 十轴机器证据门
14. 幂等提交意图（request_id 前端冻结重试）
15. durable run steer 注入语义（普通消息永不充当决定）

---

# 二、Connectors 体系（30 项）

## 2.1 框架与接口

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| K-1 | 接口分层 | Load/Poll/Slim/OAuth/Event/Checkpointed/Resolver/Hierarchy `connectors/interfaces.py:120-343` | Connector（FetchAll/FetchIncremental）+可选 FullSyncWithCursor/Streaming 三档 `internal/datasource/connector.go:11-118` | 🔷有意不同 | WeKnora 多出资源选择器（ListResources/ResolveResourceAncestors）这一 Onyx 没有的交互层 |
| K-2 | Document 统一模型 | sections（Text/Image/Tabular）+owner+权限 `connectors/models.py:196` | FetchedItem（Content []byte 文件流模型，复用 KB 解析管线）`internal/types/datasource.go:313` | 🟡部分 | 无 owner/权限字段、无 Section 类型；有 IsDeleted+子树协调字段 |
| K-3 | 凭据体系 | EncryptedJson+动态续期+Redis 分布式锁 `connectors/credentials_provider.py:17,82` | AES-256-GCM 静态加密 `internal/types/datasource.go:531,590`；凭据子资源 API `internal/handler/datasource_credentials.go` | 🟡部分 | 无 token 轮换回写与分布式锁 |
| K-4 | cc_pair 多对多绑定 | Connector×Credential 多对多+access_type+auto_sync_options `db/models.py:912,946` | DataSource 单表 1:1 内聚（配置+凭据+目标 KB+调度）`internal/types/datasource.go:65-104` | 🔷有意不同 | 换简单失凭据复用/多凭据同源/access_type |
| K-5 | 注册表与懒加载 | 57 条懒加载 `connectors/registry.py:14`、`factory.py:40` | 进程内 map，启动全量构造 11 实例 `internal/datasource/connector.go:121`、`internal/container/container.go:2017`；元数据注册表钉死同一 11 源集，registry==implementation 由对齐断言测试锁定（`connector_registry_test.go`） | ✅对齐（SP1） | Go 饥饿式合理；SP1 清除 6 条幽灵元数据（github/google_drive/onedrive/web_crawler/slack/imap），常量保留供 SP7-9 复用 |
| K-6 | 创建前实连校验 | `factory.py:151` validate_ccpair_for_user | Connector.Validate 三路径触发 `internal/application/service/datasource_service.go:1415,1433`、`internal/handler/datasource.go:309,272` | ✅对齐 | WeKnora 无 perm-sync 附加校验（本就无 perm sync） |

## 2.2 索引管道

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| K-7 | 定时调度 | celery beat 20s 扫描 refresh_freq | robfig/cron 按 SyncSchedule+双层去重（DB+asynq TaskID）+商业门控 `internal/datasource/scheduler.go:27,157-208` | ✅对齐 | WeKnora 额外有预算/套餐暂停门 |
| K-8 | 手动触发 run-once | `/admin/connector/run-once` | POST /datasource/:id/sync（ForceFull/max_items）`internal/handler/datasource.go:427` | ✅对齐 | |
| K-9 | 增量 checkpoint | FileStore 存 checkpoint JSON | DB 存 SyncCursor+流式分页持久化+fence 校验+失败保留 cursor+attempt>0 续传 `internal/application/service/datasource_service.go:1183-1205,847-857,1144` | ✅对齐 | 机制不同语义等价 |
| K-10 | 两阶段管道 | docfetching→docprocessing 批次协议+FileStore 暂存 | sync worker 抓取→asynq document process 异步索引（单文档粒度）`datasource_service.go:1455-1542`、`knowledge_create.go:252` | 🟡部分 | 实为两阶段但无 batch/attempt 协议、无跨 attempt 批次重放 |
| K-11 | IndexAttempt 状态机 | 状态+total/completed_batches+heartbeat+stall+取消标记 `db/models.py:2555-2649` | SyncLog（running/success/partial/failed/canceled）+计数+流式实时回写进度 `internal/types/datasource.go:145-180` | 🟡部分 | 无 batch 级进度/心跳/stall 检测/取消标记（有 Pause+删源取消排队） |
| K-12 | 失败处理与修复 | ConnectorFailure+错误分页 API+targeted reindex（Resolver 重抓） | 逐项失败不中断+稳定 i18n 错误码+partial 状态 `internal/types/datasource.go:452`、`datasource_service.go:927-940`；**无 targeted reindex** | 🟡部分 | 修复手段缺：只能等下次增量或 ForceFull |
| K-13 | pruning/deletion_sync | SlimConnector 独立 prune 周期任务 | IsDeleted+SyncDeletions 随每次同步（增量事件+全量对账双路）+子树清扫 `datasource_service.go:1003-1070,1611` | ✅对齐 | WeKnora 有独有附件子树清扫协议 |
| K-14 | 层级浏览 HierarchyNode | 持久化源结构镜像+周期抓取+前端浏览器 | 选择期 ListResources/ResolveResourceAncestors `connector.go:28,40`；**索引后文档树浏览无**（文档平铺进 KB） | 🟡部分 | 层级只用于建源时资源选择 |

## 2.3 连接器广度

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| K-15 | 云盘类 | google_drive/box/dropbox/s3/r2/gcs/oci/egnyte/sharepoint/onedrive | feishu_drive/lark_drive 实装；google_drive/onedrive **仅元数据声明** | 🟡部分 | 西文公有云盘全缺；飞书云盘独有 |
| K-16 | Wiki 类 | confluence/notion/outline/gitbook/bookstack/coda/guru/document360/mediawiki/wikipedia 等 | confluence/notion/yuque/腾讯 ima 实装 | 🟡部分 | 主流通用 Wiki 已对齐；yuque/ima 为 WeKnora 独有 |
| K-17 | 代码类 | github/gitlab/bitbucket | gitlab 实装；github 仅声明 | 🟡部分 | github/bitbucket 缺 |
| K-18 | 聊天/邮件类 | slack/teams/discord/zulip/zoom/gmail/outlook/imap/fireflies | imap/slack 仅声明未实现 | ❌缺失 | Onyx 大幅领先 |
| K-19 | 客服 CRM | zendesk/freshdesk/hubspot/salesforce/gong 等 | 无 | ❌缺失 | |
| K-20 | 网页爬虫 | web（recursive/single/sitemap 三模式） | web_crawler（Sitemap）仅声明；有单页 URL 导入与 browser 基建，无调度型爬虫连接器 | ❌缺失 | |
| K-21 | 中国生态 | 无 | feishu/lark/双 drive/dingtalk/yuque/ima 全实装（7 个） | 🔷反向领先 | |
| K-22 | ingestion API/user_file/craft_file | 公开直推 API `server/onyx_api/ingestion.py:101`；user_file 聊天临时文件索引；craft FILE_SYSTEM 处理模式 | 无连接器直推 API（KB 上传 API 语义替代但不属连接器体系） | ❌缺失 | |

## 2.4 权限与 EE

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| K-23 | 文档权限同步 | `ee/onyx/external_permissions/` 15+ 源 doc_sync+周期任务+检索后过滤 | 无（同步进 KB 的文档无源侧 ACL，隔离单位=租户/KB） | ❌缺失 | "导入即共享给 KB 成员"模型 |
| K-24 | 外部用户组同步 | `ee/.../external_group_syncing` | 无 | ❌缺失 | |
| K-25 | AccessType.SYNC | PUBLIC/SYNC 按源权限控可见 `db/enums.py:280` | 无对应概念 | ❌缺失 | |

## 2.5 前端

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| K-26 | 配置向导 | CONNECTOR_CONFIGS schema 驱动 50+ 源 `web/src/lib/connectors/connectors.tsx:177` | React form.ts VUE_* 常量 + Vue 分步向导 `apps/web/src/data-sources/form.ts:142`、`frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue:499`；confluence/dingtalk 创建入口两套前端均已补（fields/guides/i18n + connectorDefs） | ✅对齐（SP1） | 均声明式 schema；SP1 补齐 confluence/dingtalk 前端入口 |
| K-27 | cc_pair 详情页 | IndexAttemptsTable+错误弹层+stage metrics+重索引 UI | 同步日志抽屉+partial 失败样本 `apps/web/src/data-sources/`（log-state.ts/card.ts） | 🟡部分 | 无 attempt/stage 级明细、无错误全列表分页、无单点重索引 |
| K-28 | 状态总览页 | indexing/status 聚合视图+过滤 | DataSourcesPage 列表卡片+appconnector sync-status `internal/handler/app_connector_sync.go:44` | 🟡部分 | 无跨源索引健康度聚合视图 |

## 2.6 治理

| # | 能力 | Onyx | WeKnora | 定性 | 备注 |
|---|------|------|---------|------|------|
| K-29 | 级联删除 | fence 撤销+文档清理任务集+监督 `connector_deletion/tasks.py:147` | 软删 ds+摘 cron+取消排队；**已同步文档不级联删除（残留）** `datasource_service.go:423` | ❌缺失 | metadata datasource_id 仍在，可手动清 |
| K-30 | 能力体检 | capability_checks runner+报表+回流 `connectors/capability_checks/` | 无（仅创建时一次性 Validate） | ❌缺失 | |

## 2.7 连接器逐源清单

| 源 | Onyx | WeKnora | 结论 |
|----|------|---------|------|
| Confluence | ✅ | ✅ 实装（Streaming+FullStreaming） | 等价 |
| Notion | ✅ | ✅ 实装（增量） | 等价 |
| GitLab | ✅ | ✅ 实装（增量+hierarchical） | 等价 |
| GitHub | ✅ | 仅元数据声明 `connector.go:240` | 缺失 |
| Bitbucket | ✅ | 无 | 缺失 |
| 飞书 wiki/Lark | ❌ | ✅ 实装 | WeKnora 独有 |
| 飞书/Lark 云盘 | ❌ | ✅ 实装 | WeKnora 独有 |
| 钉钉 | ❌ | ✅ 实装 | WeKnora 独有 |
| 语雀 | ❌ | ✅ 实装 | WeKnora 独有 |
| 腾讯 ima | ❌ | ✅ 实装 | WeKnora 独有 |
| RSS/Atom | ❌ | ✅ 实装 | WeKnora 独有 |
| Slack/Teams/Discord/Zulip/Zoom | ✅ | 无/slack 仅声明 | 缺失 |
| Gmail/Outlook/IMAP | ✅ | 仅 imap 声明 | 缺失 |
| Google Drive/Box/Dropbox/Egnyte | ✅ | 仅 google_drive 声明 | 缺失 |
| S3/R2/GCS/OCI | ✅ | 无 | 缺失 |
| SharePoint/OneDrive | ✅ | 仅 onedrive 声明 | 缺失 |
| Web 爬虫（三模式） | ✅ | 仅 web_crawler(Sitemap) 声明 | 缺失 |
| Jira/Linear/Asana/ClickUp | ✅ | 无 | 缺失 |
| Zendesk/Freshdesk/HubSpot/Salesforce/Gong | ✅ | 无 | 缺失 |
| Outline/Gitbook/Bookstack/Coda/Guru 等 15+ 长尾 Wiki | ✅ | 无 | 缺失 |
| 文件上传 | 连接器形态 | KB 上传 API（语义替代） | 形态不同 |
| ingestion API | ✅ | 无 | 缺失 |

**计数**：Onyx 57；WeKnora 实装 11（7 个中国生态 Onyx 全无）；重叠仅 Confluence/Notion/GitLab。**幽灵条目债**：github/google_drive/onedrive/slack/imap/web_crawler 六个声明了元数据但无实现；confluence/dingtalk 已实装但前端无创建入口。

## 2.8 Connectors 反向差距（WeKnora 已有、Onyx 没有）

1. 应用连接器操作体系（App/Installation/Connection/Action + AuthVersion 授权代际）`internal/appconnector/model.go`
2. 操作风险分级与审批链（四风险类+ActionDigest+预授权+unknown 只解析不重放）`internal/appconnector/action.go:17,134,163`
3. 飞书发送 Action（FS-01 契约+官方幂等 uuid）`internal/appconnector/feishu_send.go`
4. Notion 建页 Action `internal/appconnector/notion_create.go`
5. open-connector 共享运行时+独立管控进程（outbox+fence-lease、唯一持管理凭据）`docs/adr/0001-open-connector-shared-runtime.md`、`internal/connectorcontrol/worker.go`
6. MCP 适配层（工具 pinning+漂移检测+stdio 沙箱策略）`internal/appconnector/mcp_adapter.go`
7. 商业化/预算门控同步治理（暂停原因闭集+CanAdvanceCheckpoint+AuthVersion 游标绑定+取失败永不删本地）`internal/appconnector/sync.go:163,192`
8. 流式同步（fetch→ingest→checkpoint 交错、内存有界）`internal/datasource/connector.go:74-118`
9. 子树协调删除（ReplacesSubtree/SubtreeKeep 协议+瞬态失败防误删）`internal/types/datasource.go:347-401`
10. i18n 错误码（稳定 Code+Params 本地化）`internal/types/datasource.go:452`
11. 中国生态 7 连接器
12. 资源选择器协议（惰性层级加载+深层已选回显）`internal/datasource/connector.go:28,40`

---

# 三、平台运营面（六域，P-n 条目）

> 2026-09-19 增补。设计定稿见 [docs/superpowers/specs/2026-09-19-onyx-platform-parity-design.md](../superpowers/specs/2026-09-19-onyx-platform-parity-design.md)，
> 总原则：**语义对齐 + 基建复用**（不照抄 Onyx 表结构/API 路径，复用 commercial/API key/chat session 等已有体系）。
> "→SPxx" 表示该项已排入 ROADMAP 对应子项目。

| # | 能力 | Onyx 实现要点 | WeKnora 现状 | 定性 | 备注 |
|---|------|--------------|--------------|------|------|
| P-1 | 用量日桶聚合 | `user_usage` 表（user×日×model×flow）upsert 累加 token/成本 `backend/onyx/db/models.py:6206`、tracing 采集 `tracing/processors/user_usage_processor.py` | craft/commercial 双账本（`craft_usage_facts`/`commercial_usage_facts`+微积分定价+预算预留）完备；普通聊天仅 `messages.usage` JSONB（`internal/types/message.go:383`） | ❌→SP12 | 缺按用户/模型/日的跨会话聚合层；成本复用 commercial 费率 |
| P-2 | 用户/admin 用量视图 | `/app/settings/usage`、`/admin/performance/usage` | 无任何用量页面（仅 craft 会话内用量面板 `packages/views/src/craft/usage.tsx`） | ❌→SP12 | 前端无图表库（随 SP11 引 recharts） |
| P-3 | 套餐/计费页 | `/admin/billing` 两档卡片+Stripe Checkout | commercial 后端全套（计划版本/订阅/订单/退款/支付宝微信 `internal/commercial/`）；前端组件已写好**未挂路由**（`apps/web/src/commercial/BillingPage.tsx` 等 4 个） | 🟡→SP14 | 纯接线；Stripe/seats 不引入（有意不同） |
| P-4 | 消息反馈 like/dislike | `chat_feedback` 表 `db/models.py:3607` | ✅ `message_feedback` 表（migration 000079/000158）+ 提交/撤销/回显 API + 消息气泡 like/dislike（乐观更新+回显），见 SP11 验收证据 | ✅ | 交付物：`c9310a09`/`722ec67c`/`3c248161`；[evidence](../migrations/react/evidence/onyx-parity/2026-09-19-sp11-feedback-analytics.md) |
| P-5 | 分析聚合+图表页 | `/api/analytics/admin/*` 实时 SQL 聚合+recharts（EE） | ✅ 四组读时聚合端点（queries/users/channels/agents）+ `/platform/analytics` recharts 四图（日期过滤+RBAC 门），见 SP11 验收证据 | ✅ | 交付物：`ce42a1ae`(Task3 文件)/`66964201`/`3726f8c2`/`b0783bf2`/`524fe997`；[evidence](../migrations/react/evidence/onyx-parity/2026-09-19-sp11-feedback-analytics.md) |
| P-6 | admin 查询历史审计 | `chat-session-history` 分页+完整快照（EE） | admin 仅渠道会话可见（source=api/embed/IM，`internal/application/service/session.go:394`）；web 会话按 user 隔离不可见 | 🟡→SP13 | 扩展现有 admin 视图到全 source+按用户过滤 |
| P-7 | 查询历史 CSV 导出 | Celery 三段式（触发/状态/下载） | 无 | ❌→SP13 | 用已有 asynq 替代 Celery |
| P-8 | 隐私三档开关 | NORMAL/ANONYMIZED/DISABLED | 无 | ❌→SP13 | 租户级设置 |
| P-9 | 会话分享 | PUBLIC 链接匿名只读 | 无 | ❌→SP13 | **有意收紧**：租户内登录分享（token+同租户校验），不做匿名公开链接 |
| P-10 | API 文档入口 | docs.onyx.app 外链 | 后端 swagger 有（非 release 模式 `internal/router/router.go:158`）；前端 API key 面板无文档入口 | 🟡→SP14 | 加 `/swagger` 入口+capabilities 文案 |
| P-11 | per-user 默认模型偏好 | `User.default_model` 等+设置页 `web/src/views/SettingsPage.tsx` | `user.preferences` JSONB 仅 3 项（`internal/types/user.go:24`）；无默认模型/温度 | ❌→SP14 | JSONB 扩展无迁移；解析链 会话>agent>用户>租户（对齐 Onyx 优先级） |

**API key 域结论**：WeKnora 两级 key（tenant/platform）+capabilities+swagger 已与 Onyx 语义等价（Onyx 的"服务账号合成 User+用户组"模型 vs WeKnora 的细粒度 capabilities 模型属有意不同），仅补文档入口，不重写。

---

# 四、总体结论与分期建议

## 4.1 总体定性

- **Craft**：WeKnora 已有强治理形态（不可变版本/恢复程序/usage 账本/发布门控，反向领先 15 项）；相对 Onyx 的缺口集中在**生态面**：定时任务、User Library、craft 内 MCP、AGENTS.md 模板、外部应用桥、admin 管理页、onboarding。另有**两处"已实现未接线"**（模型网关、Stop 路由）。
- **Connectors**：WeKnora 强在治理与写操作（审批/预授权/open-connector），弱在**广度**（11 vs 57，西文 SaaS 几乎全空）与**文档级权限**（EE 三件套全缺）。框架层 6 项中 4 项对齐/有意不同，管道层缺 attempt 级进度/心跳/定向重索引/级联删除。
- **平台运营面**：套餐后端与 API key 已是等价物（甚至更贴合国内支付）；真正缺口是**分析（全新）**、**用量聚合层（新建）**、**查询历史 admin 审计+分享（补齐）**三块，已排 SP11–SP14。
- **共同注意**：两个"有意不同"大项（webapp 实时预览反代、cc_pair 模型重构）改造成本高且与 WeKnora 现有形态冲突，建议默认**不追随**，除非有明确产品诉求。

## 4.2 缺失清单分组（建议优先级）

**P0 · 已实现未接线 + 低成本债**（快速收割）
- C-25 craft 模型网关生产挂载（router/container 装配 + G4 billing 缺口决策）
- C-3 CraftControlService.Stop 挂 HTTP 路由
- K-26 前端补 confluence/dingtalk 创建入口（后端已实装）
- K-5/逐源清单 清理六个幽灵连接器声明（或标记 Experimental）

**P1 · Craft 生态补齐**（Onyx 已验证的高价值功能）
- C-23 ScheduledTask 定时任务（CRUD+执行器+预授权）
- C-18 User Library（跨会话文件库）
- C-15 AGENTS.md 指令模板渲染
- C-27 admin 管理页 + C-28 onboarding
- C-16 craft 内 MCP（与 appconnector MCP 适配层打通）

**P1 · Connectors 治理补强**（框架层补洞）
- K-11 attempt 级进度/心跳/stall/取消
- K-12 targeted reindex（单点重索引）
- K-29 删源级联文档清理
- K-3 凭据动态续期

**P2 · Connectors 广度扩展**（按需求逐个立项）
- GitHub（已有声明+元数据）、Web 爬虫、IMAP/邮件、Google Drive、Slack…

**P2 · 平台运营面（SP11–SP14，设计已定稿）**
- SP11 反馈+分析（P-4/P-5）→ SP12 用量聚合（P-1/P-2）→ SP13 查询历史审计+分享（P-6~P-9）；SP14 轻项收割（P-3/P-10/P-11）穿插
- 详见 `docs/superpowers/specs/2026-09-19-onyx-platform-parity-design.md`

**P3 · 大改造/需产品决策**（🔷 项，默认不追随）
- C-10 webapp 实时预览反代（与 W02 受控预览冲突）
- C-20/22 craft↔外部应用 OAuth 桥（若做，建议桥接现有 appconnector 而非新造 ExternalApp）
- K-23/24/25 文档权限同步三件套（改变"导入即共享"模型，牵动检索过滤）
- K-4 cc_pair 多对多重构（当前 1:1 内聚够用）

## 4.3 后续流程

用户审阅本矩阵 → 圈定首期范围 → 按 brainstorming 流程对首个子系统走 方案对比 → 分节设计 → spec（docs/superpowers/specs/）→ writing-plans 实施计划。
