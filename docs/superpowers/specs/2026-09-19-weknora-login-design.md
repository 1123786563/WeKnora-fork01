# Conduit（apps/mobile）WeKnora 账号登录与服务端会话同步 — 设计稿

- 日期：2026-09-19
- 状态：已与用户逐节确认，待转入实施计划
- 落点：`apps/mobile`（直接开发，不走 conduit 上游双仓同步）
- 关联：`apps/mobile/docs/WEKNORA.md`（WeKnora Direct 适配器 v1）、`apps/mobile/PROVENANCE.md`

## 背景与目标

Conduit 已通过 WeKnora Direct 适配器（`lib/features/direct_connections/services/weknora_adapter.dart`）支持 API-Key 直连聊天与模型发现。本设计把"凭据来源"从手填 API Key 升级为**账号密码登录**，并让手机端与 WeKnora 网页端共享**同一份服务端会话**。

## 已确认决策

| 决策点 | 选择 |
|---|---|
| 产品形态 | C：登录页作入口 + 复用 Direct Connection 架构（不另起炉灶） |
| 会话列表 | 服务端会话同步：手机与网页看到同一份历史，新会话双向同步 |
| 历史会话 | 可继续聊（不止只读） |
| 开发落点 | 直接改本仓 `apps/mobile` |

## 总体架构

新增 feature 模块 `lib/features/weknora/`：

```
lib/features/weknora/
├── account/    WeKnoraAccountService（登录/刷新/登出）+ WeKnoraLoginPage + riverpod providers
└── sessions/   服务端会话同步（列表拉取 upsert、消息 load 映射、session id 回填）
```

- 依赖方向：`weknora/` → 复用 `direct_connections` 的 client pool 与 WeKnoraAdapter + `core/persistence`；chat/workspace 通过 riverpod provider 消费。
- **不改**：Direct Connection 架构、聊天管线主体、其他 provider 适配器、`direct_http_client` 请求层。
- 登录后一切 WeKnora 请求（模型发现、会话、聊天）仍走已验证的 Direct 通道，仅凭据来源变为 JWT。

## 账号与登录层

### 存储

secure storage 新键 `weknora_account_v1`：`baseUrl, email, userId, displayName, accessToken, refreshToken`。

### 登录流程与 profile 镜像

1. 登录页提交 → `POST /api/v1/auth/login`（`{email, password}`）→ 得到 `access_token`、`refresh_token`、用户信息。
2. 存入 `weknora_account_v1`。
3. **创建或更新**一条 WeKnora Direct Connection profile：`adapterKey=weknora`、`baseUrl`、`apiKeyAuthMode=bearer`、`apiKey=accessToken`、名称如 `WeKnora`。Bearer 注入复用 `direct_http_client.dart` 现有逻辑（`Authorization: Bearer ${profile.apiKey}`），请求层零改动。

### JWT 刷新（单飞）

- 任何 WeKnora API 调用前本地解码 access token 的 `exp`；剩余有效期 < 60 秒即视为过期（对冲设备时钟偏差）。
- 过期 → 互斥锁内调 `POST /api/v1/auth/refresh`（`{refreshToken}`；后端**轮换**返回新的双令牌）→ 更新存储，并**镜像写回 profile.apiKey**。
- 并发请求只触发一次刷新，其余等待结果复用。
- 刷新失败（refresh token 失效/网络 401）→ 清除账号存储 → 发出 auth-expired 事件 → 路由跳回登录页。

### 登录页 UI 与入口

- `WeKnoraLoginPage`（复用 `lib/features/auth/views` 现有页面风格）：服务器地址、邮箱、密码、登录按钮、内联错误（凭据错误 / 服务器不可达）、可显示密码。
- 入口一：首启 `BackendChooserPage` 增加"WeKnora"选项 → 登录页。
- 入口二：设置页新增账户区（邮箱、服务器地址、**退出登录**）。
- 已登录用户冷启动直达聊天页（现有路由逻辑照旧）。

### 登出

`POST /api/v1/auth/logout`（尽力而为，失败不阻塞）→ 清 `weknora_account_v1` → **禁用**（不删除）WeKnora profile → 本地会话列表数据保留。

## 服务端会话同步层（方案 1：远端会话直连，不做本地镜像）

### 列表同步

- 登录成功后与抽屉打开/下拉刷新时拉 `GET /api/v1/sessions?limit=50`（v1 只取第一页）。
- 以 `Conversation.metadata['weknoraSessionId']` 为键 **upsert** 本地 Conversation：同步 `title`、`updatedAt`、agent 标记；手机上尚未关联服务端的新对话不受影响。
- 网页端新建会话在下次刷新时出现；网页自动标题更新同步到本地。

### 消息加载

- 打开远程会话时拉 `GET /api/v1/messages/:session_id/load`，映射为 ChatMessage：query→user 消息、answer→assistant 正文、thinking→reasoning、references→Sources 块。
- **内存缓存（会话打开期间），不落库**——离线不可读是方案 1 的明确取舍，后续如需离线再加缓存层，架构不堵死。
- 实施第一天先 curl 实测 load 端点返回结构，将字段映射以单测锁定（与既有 StreamResponse 帧结构可能不同）。

### 继续聊（精确绑定）

- 会话 `metadata['weknoraSessionId']` 存在时，发送管线往 `DirectCompletionRequest.parameters` 注入 `weknora_session_id`。
- **仅当目标 profile 的 `adapterKey == 'weknora'` 时注入**，防止参数泄漏进其他 provider 的请求体。
- 适配器见该参数即直接使用该会话（见下节），跳过前缀匹配与 `POST /sessions` 新建；同会话串行队列与 409 处理照旧。

### session id 回填（手机新建对话）

- 适配器保留现有 `_bindings` 表，新增只读方法 `boundSessionIdFor({profileId, priorUserQueries})`（复用现有前缀匹配函数）。
- UI 在一轮对话流结束后调用它，把查到的服务端会话 id 写入 `conversation.metadata['weknoraSessionId']`；此后该对话永远走精确绑定。
- 零共享接口改动（不新增事件类型、不改 `DirectCompletionRequest` 构造）。

## 适配器改动（小改）

`weknora_adapter.dart` 的 `startCompletion` 开头新增分支：

```
if (request.parameters['weknora_session_id'] 是非空字符串) {
  会话 id = 该值；跳过前缀匹配与新建；记入 _bindings；
}
```

其余逻辑（SSE 帧映射、stop、引用双路径、串行队列）不动；现有 15 个单测保持通过。

## 边界与错误处理

- 任意 WeKnora API 调用 401 → 单飞刷新后**重试一次**；再失败 → 清账号跳登录页。
- 网页端已删除的会话：手机打开时 load 返回 404 → 从本地列表剔除并提示。
- 手机端删除会话：v1 调 `DELETE /api/v1/sessions/:id`（实施时核实端点存在；若无则隐藏手机端删除入口，仅保留网页删除）。
- 网络错误：会话列表显示重试按钮；聊天流错误沿用现有展示。

## 模型发现（不改）

沿用适配器现有 listModels（kb/agent/model 三类命名空间 + 端点降级逻辑）。JWT 用户令牌权限下三端点均可访问；API-Key 能力门降级逻辑原样保留（两种凭据形态共存）。

## 测试计划

复用 direct_connections 现有 mock HttpClientAdapter 模式：

1. **account service**：登录成功/凭据错误/服务器不可达；刷新单飞（并发只刷一次）；exp 提前 60s 判过期；刷新后 profile.apiKey 镜像；refresh 失效清账号。
2. **sessions**：列表 upsert（新增/更新/网页删除剔除 404 路径）；消息 load 映射（query/answer/thinking/references）；回填写入 metadata。
3. **适配器**：`weknora_session_id` 参数精确绑定（不再 POST /sessions）；`boundSessionIdFor` 查询；既有 15 用例回归。

## 文件清单

**新增**：`lib/features/weknora/account/{weknora_account_service.dart, providers.dart, weknora_login_page.dart}`、`lib/features/weknora/sessions/{weknora_session_sync.dart, weknora_message_mappers.dart}` 及对应测试。

**修改**：`weknora_adapter.dart`（精确绑定 + boundSessionIdFor）、`backend_chooser_page.dart`（+WeKnora 选项）、设置页（+账户区/退出登录）、会话抽屉（远程列表接入 + 下拉刷新）、聊天发送管线（parameters 注入，仅 weknora profile）、`lib/l10n/app_*.arb` ×14。

## 明确不做（v1 范围外）

OIDC/微信扫码登录、注册与找回密码、离线消息缓存、会话历史分页加载（只拉最新 50 条）、多 WeKnora 服务器多账号、本地工具运行时接入。

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| `GET /messages/:sid/load` 字段与 StreamResponse 帧结构不一致 | 实施首日 curl 实测并以单测锁定映射 |
| 设备时钟偏差导致 exp 误判 | 提前 60 秒判过期；刷新失败兜底跳登录 |
| `DELETE /sessions/:id` 端点可能不存在 | 实施时核实；缺失则隐藏手机端删除入口 |
| 登录创建的 profile 被用户手动改动凭据 | 账户区展示"由登录管理"提示；刷新时以账号存储为准覆写 |

## 后端端点速查（均已在本仓 internal/handler 验证存在）

| 端点 | 用途 |
|---|---|
| `POST /api/v1/auth/login` | 登录，返回 access/refresh 双令牌 |
| `POST /api/v1/auth/refresh` | 轮换刷新双令牌 |
| `POST /api/v1/auth/logout` | 登出吊销令牌 |
| `GET /api/v1/auth/me` | 当前用户信息（可选用于账户区） |
| `GET /api/v1/sessions?limit=50` | 服务端会话列表 |
| `GET /api/v1/messages/:session_id/load` | 会话历史消息 |
| `DELETE /api/v1/sessions/:id`（待核实） | 删除会话 |
