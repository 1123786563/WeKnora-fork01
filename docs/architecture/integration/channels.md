# Integration Brief — channels 模块（Pass A 后）

> 读者：Integration Agent（IA1）与 Pass B worker。本文是 Task A4 搬迁后 channels
> 模块对外集成面的事实快照。范围事实源：`docs/architecture/moves/channels.yaml`；
> 行为与结构在 Pass A 中未改变，仅包路径变化。

## 1. 已搬迁包（from → to）

| 旧路径（Pass A 别名保留） | 新路径 |
|---|---|
| internal/im | internal/modules/channels/im |
| internal/im/dingtalk | internal/modules/channels/im/dingtalk |
| internal/im/feishu | internal/modules/channels/im/feishu |
| internal/im/mattermost | internal/modules/channels/im/mattermost |
| internal/im/qqbot | internal/modules/channels/im/qqbot |
| internal/im/slack | internal/modules/channels/im/slack |
| internal/im/telegram | internal/modules/channels/im/telegram |
| internal/im/wechat | internal/modules/channels/im/wechat |
| internal/im/wecom | internal/modules/channels/im/wecom |
| internal/im/yunzhijia | internal/modules/channels/im/yunzhijia |

99 个文件（含全部 `_test.go`）纯改名搬迁；别名包为无逻辑转发，Pass B `B-channels` 删除。

## 2. 公共回调（匿名，平台签名自验）

注册入口：`RegisterIMRoutes — internal/router/routes_agent.go:284`

- `GET  /api/v1/im/callback/:channel_id` → `handler.IMHandler.IMCallback`（internal/handler/im.go）
- `POST /api/v1/im/callback/:channel_id` → 同上

契约：IM 平台自带签名验证，**不走全局鉴权中间件**。回调内流程（im.go:375-440）：
查 channel 行并校验 enabled → `adapter.HandleURLVerification`（如飞书/钉钉握手）→
`adapter.VerifyCallback`（各平台签名）→ `adapter.ParseCallback` → 异步 `HandleMessage`。
`VerifyCallback` / webhook 事件载荷是**外部契约，不可改语义**。

## 3. Embed 公共路由（发布 token 鉴权，非匿名）

注册入口：`RegisterEmbedPublicRoutes — internal/router/routes_agent.go:220`
组前缀 `/api/v1/embed/:channel_id`，鉴权 `middleware.EmbedAuth(embedService, tenantService, redis)`。
共 **19** 条，全部由 `internal/handler/embed_channel.go` 承接：

`POST /exchange`、`GET /config`、`GET /suggested-questions`、`GET /chunks/:chunk_id`、
`POST /sessions`、`POST /knowledge-chat/:session_id`、`POST /agent-chat/:session_id`、
`GET /messages/:session_id/load`、`POST /sessions/:session_id/stop`、
`GET|POST /sessions/:session_id/messages/:message_id/suggestions`、
`POST /sessions/:session_id/suggestion-events`、`POST /sessions/:session_id/events`（webhook 事件中继）、
`POST /sessions/:session_id/mcp-oauth-resolutions/:pending_id`（+ `/cancel`）、
`POST /sessions/:session_id/mcp-services/:id/oauth/authorize-url`、
`GET  /sessions/:session_id/mcp-services/:id/oauth/status`、
`POST /sessions/:session_id/tool-approvals/:pending_id`、`GET /files`（租户内文件回源）。

## 4. 已鉴权管理路由（per-file 计数）

### Embed 渠道管理 — `RegisterEmbedChannelRoutes — routes_agent.go:261`（共 9 条）
承接文件 `internal/handler/embed_channel.go`（9 条）：
- `/agents/:id/embed-channels`：`POST`（Admin）、`GET`（Viewer）
- `/embed-channels`：`GET`（Viewer）、`GET /:channel_id`（Viewer）、`PUT /:channel_id`（Admin）、
  `DELETE /:channel_id`（Admin）、`POST /:channel_id/rotate-token`（Admin）、
  `POST /:channel_id/preview-session`（Viewer）、`GET /:channel_id/stats`（Viewer）

### IM 渠道管理 — `RegisterIMChannelRoutes — routes_agent.go:297`（共 8 条）
- 承接文件 `internal/handler/im.go`（6 条）：`/agents/:id/im-channels` `POST`(Admin)/`GET`(Viewer)；
  `/im-channels` `GET`(Viewer)、`PUT /:id`(Admin)、`DELETE /:id`(Admin)、`POST /:id/toggle`(Admin)
- 承接文件 `internal/handler/wechat_qrcode.go`（2 条）：`/wechat/qrcode`、`/wechat/qrcode/status`
  （均 Admin —— 扫码成功会把个人微信绑定到租户）

全部经 `apiKeyGroup(..., apiKeyManageChannels(apiKeyFullAccess()))` 包裹。

## 5. 身份 / 会话绑定

**IM 侧**（internal/modules/channels/im）：
- 外部平台用户按 `(tenant_id, channel_id, platform 用户标识)` 归一为 userKey；
  `withIMIdentity(ctx, tenantID, channelID, msg)`（service.go:438）把身份注入请求上下文。
- `im_channels.bot_identity`（types.go:28）按平台+模式+凭据派生，全库唯一索引防重复绑定。
- 会话绑定：userKey → (sessionID, messageID) in-flight 映射（storeInflightMapping，
  service.go:1545）；`session_mode` 决定新会话/复用策略（BeforeSave 校验，types.go:118）。

**Embed 侧**（legacy_files，Pass B 归位）：
- 发布 token → channel：`embedChannelRepository.GetByPublishToken`
  （internal/application/repository/embed_channel.go:36）。
- 会话令牌签发/解析：`embedChannelService.IssueSessionToken / ResolveSessionToken`
  （internal/application/service/embed_session.go:35/55）；会话句柄签名
  `SignEmbedSessionHandle / VerifyEmbedSessionHandle`（同文件 :108/:119）。

## 6. 出站投递生命周期

**IM 回复**：`Adapter`（adapter.go:126）必须实现回复发送；可选能力接口
`StreamSender`（流式卡片）、`FullOutputProgressSender`、`FileDownloader`。
长连接适配器（WS/longconn）经 Redis 租约选主（tryAcquireWSLeader，service.go:1312），
单实例（Lite/无 Redis）回退本地状态。渠道配置变更经 Redis pub/sub
（publishChannelConfigChange，service.go:1130）触发 reloadChannelFromDB；
`Service.Stop()`（service.go:1002）优雅停机。文件出站走 FileService 上传后按平台发送。

**Embed Webhook 出站**：`DispatchEmbedWebhook`
（internal/application/service/embed_webhook.go:60）向渠道登记的 http(s) URL POST JSON；
URL 先经 `ValidateEmbedWebhookURL`（仅 http/https，:36）；
签名头 `X-WeKnora-Signature: sha256=HMAC(secret, body)`
（SignEmbedWebhookBody，:114）。**外部契约，不可改语义。**

## 7. Workers

`integration_points.workers: []` — channels 不注册任何 asynq/Lite 任务类型。

## 8. container.Invoke 生命周期挂点

- `registerIMService` — Invoke 于 internal/container/container.go:863（函数体 :2283）：
  注册 10 个 adapter factory（`wecom`、`feishu`、`lark`（feishu 双云）、`slack`、`telegram`、
  `dingtalk`、`mattermost`、`wechat`、`qqbot`、`yunzhijia`）→ `LoadAndStartChannels()`
  从 DB 拉起启用渠道 → 经 `ResourceCleaner` 挂进程级停机钩子。
- Provide：`imPkg.NewService`（container.go:862，15 依赖注入）、
  `handler.NewIMHandler`、`handler.NewEmbedChannelHandler`。

## 9. 配置键

`config.Config.IM`（internal/config/config.go:39，结构体 :300）：
`im.workers`（默认 5）、`im.global_max_workers`（0=不限，需 Redis）、
`im.max_queue_size`（50）、`im.max_per_user`（3）、`im.rate_limit_window`（60s）、
`im.rate_limit_max`（10）。解析于 `resolveIMConfig`（新路径 service.go:876）；
**平台凭据不入 config**，存 `im_channels.credentials`（JSONB，ParseCredentials）。

## 10. 禁改共享文件中的旧路径引用清单（Pass B 切换指南）

迁移后全仓 grep `WeKnora/internal/im`，仅剩以下引用：

| 文件:行 | 引用 | 性质 |
|---|---|---|
| internal/container/container.go:82 | `imPkg "…/internal/im"` | 禁改文件，Pass A 留别名 |
| internal/container/container.go:83-91 | `…/internal/im/{dingtalk,feishu,mattermost,qqbot,slack,telegram,wechat,wecom,yunzhijia}` | 同上 |
| internal/im/**/alias.go（10 个文件） | 别名包自身 | Pass B 整树删除 |

router（routes_agent.go / router.go / task.go / sync_task.go）**无**旧路径引用——
router 只依赖 handler。`internal/handler/custom_agent.go:12`、`im.go:10`、
`wechat_qrcode.go:6` 已在本次 Compile-Repair 切到新路径（仅 import 行）。

**切换指令（Pass B，B-channels）**：
1. 把 container.go:82-91 的 10 条 import 改为 `internal/modules/channels/im[/子包]`
   （`imPkg` 别名可保留或去掉，`feishu.RegionFeishu/RegionLark` 用新路径直引）；
2. `git rm -r internal/im`（10 个 alias.go，零逻辑，无其他引用方）；
3. 跑 `go build ./... && go test ./internal/modules/channels/... ./internal/container/... -count=1`。

## 11. Legacy files 导航（横向包内，Pass B 拆分）

- internal/application/repository/embed_channel.go — Embed channel (application/repository)
- internal/application/service/embed_channel.go — Embed channel (application/service)
- internal/application/service/embed_session.go — Embed session (application/service)
- internal/application/service/embed_webhook.go — Embed webhook (application/service)
- internal/handler/embed_channel.go — Embed channel (handler)
- internal/handler/im.go — Im (handler)
- internal/handler/wechat_qrcode.go — Wechat qrcode (handler)

模块骨架：internal/modules/channels/{README.md,module.go,legacy/README.md}（零逻辑 façade 占位，
`NewModule/RegisterRoutes/RegisterWorkers/Start/Stop` 契约见 module.go 注释）。
