# Onyx 平台运营面对齐设计（用量/套餐/分析/查询历史/OpenAPI/对话偏好）

> 状态：设计已与用户逐节确认（2026-09-19）。对应 ROADMAP SP11–SP14，差距基线见
> [GAP-MATRIX.md](../../onyx-parity/GAP-MATRIX.md) 第三部分（P-n 条目）。
> 与 craft/connectors 线（SP1–SP10）相互独立，可并行。

## 1. 背景与目标

以 Onyx（`/Users/wuyongjun/trea/onyx`）为参照，把其平台运营面六大功能域对齐进 WeKnora：
用量（usage）、套餐（plans/billing）、分析（analytics）、查询历史（query history）、
OpenAPI 操作（API key）、对话偏好设置（chat preferences）。

## 2. 已确认的关键决策

| 决策点 | 结论 |
|---|---|
| 复用边界 | **语义对齐 + 基建复用**：对齐 Onyx 功能语义/页面形态/API 能力面，但复用 WeKnora 已有等价基建（commercial 套餐、API key、chat session），不照抄 Onyx 表结构与 API 路径，不造并行系统 |
| 范围 | 六域全部规划进 ROADMAP；首期做三大缺口（SP11 分析、SP12 用量、SP13 查询历史），轻项收割（SP14）穿插 |
| 会话分享形态 | **租户内登录分享**（token + 登录 + 同租户校验），不做 Onyx 式公开匿名链接 |
| 聚合管道架构 | **方案 B**：用量走写时日桶表（Onyx 同构 upsert 累加）；分析走读时实时 SQL 聚合（Onyx 同构，无独立分析表） |
| 成本计价 | 复用 `internal/commercial/usage.go` 微积分费率体系，单位 micro-credit，不引入 Onyx 的美分 |
| 图表库 | recharts（与 Onyx 同库，新前端依赖） |

## 3. 六域差距基线（盘点结论）

| 域 | WeKnora 现状 | 定性 | 本设计动作 |
|---|---|---|---|
| 用量 | craft/commercial 双账本完备；普通聊天仅 `messages.usage` JSONB；无按用户/模型/日聚合 API | 🟡 | SP12 新建聚合层 |
| 套餐 | commercial 子系统完备（计划版本/订阅/订单/退款/支付宝微信）；前端组件已写好**未挂路由** | 🟡 | SP14 纯接线 |
| 分析 | 无 feedback 表、无 query/活跃用户聚合 API、前端无图表库 | ❌ | SP11 全新建 |
| 查询历史 | 用户侧 CRUD/搜索/fork 完整；admin 仅渠道会话（api/embed/IM）可见；无分享 | 🟡 | SP13 补 admin 审计+分享 |
| OpenAPI/API key | 两级 key + capabilities + swagger 完备 | ✅ | SP14 小补文档入口 |
| 对话偏好 | 四级偏好体系存在；无 per-user 默认模型/温度 | 🟡 | SP14 扩展 |

## 4. 子项目设计

### SP11 · 反馈 + 分析（P-4/P-5）

**数据模型**（新表 `message_feedback`）：
- `id` PK、`tenant_id`、`user_id`（FK users）、`message_id`（FK messages）、`session_id`（冗余，便于聚合）、
  `rating`（`like` / `dislike`）、`comment`（text, nullable）、`created_at`、`updated_at`
- 唯一约束 `(message_id, user_id)`：同一用户对同一消息只保留一条，重复提交为更新

**反馈 API**（挂现有聊天路由组）：
- `POST /api/v1/messages/:id/feedback` — body `{rating, comment?}`，upsert
- `DELETE /api/v1/messages/:id/feedback` — 移除
- 权限：消息所在会话的 owner（或 admin）

**分析聚合 API**（实时 SQL，`internal/handler/analytics.go`，统一鉴权 owner/admin，`start`/`end` query 参数，缺省回看 30 天）：
- `GET /api/v1/admin/analytics/queries?start&end` — 按日聚合：每日用户消息数（=query 数，对齐 Onyx 口径）、like 数、dislike 数
- `GET /api/v1/admin/analytics/users?start&end` — 按日活跃用户（当日发过消息的不同 user 数）
- `GET /api/v1/admin/analytics/channels?start&end` — 按日按 `session.source` 会话数（web/api/embed/feishu/wechat/slack…，对应 Onyx OnyxBot 图的 IM 语义）
- `GET /api/v1/admin/analytics/agents/:agentId?start&end` — 按 agent 的每日消息数与独立用户数
- 实现于 `internal/application/repository/analytics.go`：对 `sessions`/`messages`/`message_feedback` 按 `date(time_created)` 分组聚合；确认/补齐 `(tenant_id, time_created)` 索引
- 无独立分析表、无定时任务（Onyx 同构）

**前端**：
- 新路由 `/platform/analytics`（owner/admin 守卫，TanStack Router 挂 `router.tsx` routeTree）：
  日期范围选择器（默认 30 天）+ 四组 recharts 图（query 数+活跃用户合并视图、反馈、渠道分布、agent 维度）+ admin 用量 tab（SP12 交付后并入）
- api-client 新域 `packages/api-client/src/analytics/`（`createAnalyticsApi`，进 `client.ts` 装配，contracts 加类型）
- chat 消息气泡组件加 like/dislike 按钮（含已反馈态高亮；embed 渠道首期不展示）

### SP12 · 用量聚合（P-1/P-2）

**数据模型**（新表 `user_usage`，Onyx `user_usage` 语义同构、字段适配）：
- 维度列：`tenant_id`、`user_id`（nullable，系统调用置空）、`window_start`（date 日桶，UTC）、`model`、`flow`（`chat` / `craft` / `search`）
- 度量列：`input_tokens`、`output_tokens`、`cache_read_tokens`、`cache_write_tokens`（bigint）、`cost_microcredits`（bigint，微积分）
- `created_at` / `updated_at`
- 唯一索引 `(tenant_id, user_id, window_start, model, flow)`，写入用 upsert 原子累加（`ON CONFLICT ... DO UPDATE SET ... + excluded`）

**写入点（双写）**：
1. 普通聊天：`internal/handler/session/agent_stream_handler.go` 落 `Message.Usage` 的同一处
2. craft：`internal/application/repository/craft_usage.go` 的 UsageFact Append 处同步 upsert（flow=`craft`）
- 计价：复用 `internal/commercial/usage.go` 的 `PriceVersionRates`（micro-credit、big.Rat）；无费率模型的调用记 0
- 幂等要求：消息重试/流式重放不得重复累加——以消息完成落库为唯一触发点（与 `Message.Usage` 落库同事务），craft 侧沿用 UsageKey 去重后的 Append

**API**（`internal/handler/usage.go`）：
- `GET /api/v1/usage/me?start&end` — 当前用户按日/模型聚合（token 四列 + 成本）
- `GET /api/v1/admin/usage/by-user?page&page_size` — admin 全员分页（用户/模型/日汇总）
- `GET /api/v1/admin/usage/export` — CSV 流式导出
- 预算余量不新做接口：前端复用已有 `GET /api/v1/commercial/summary` 组合展示

**前端**：
- 用户级：settings 新分区 `usage`（`/platform/settings?section=usage`，注册进 `packages/views/src/settings/registry.ts`）：时间范围选择 + 按模型 token/成本表 + 预算余量卡片（读 commercial summary）
- admin 级：`/platform/analytics` 页内 usage tab（复用同一 api-client 域）
- api-client 新域 `packages/api-client/src/usage/`

### SP13 · 查询历史审计 + 会话分享（P-6~P-9）

**admin 审计 API**：
- `GET /api/v1/admin/sessions?user_id&start&end&feedback&page&page_size` — 把现有 admin 渠道会话视图（现仅 source=api/embed/IM）扩展为**全部 source、可按用户/时间/反馈类型（like/dislike/any）过滤**的分页列表
- `GET /api/v1/admin/sessions/:id` — 完整会话快照（消息 + 知识引用 + 反馈明细）
- CSV 异步导出三段式（复用 asynq 基建）：`POST /api/v1/admin/sessions/export`（返回 request_id）→ `GET /api/v1/admin/sessions/export/status?request_id` → `GET /api/v1/admin/sessions/export/download?request_id`（流式 CSV）
- **隐私三档**：租户级设置 `query_history_mode`（`normal` / `anonymized` / `disabled`）；`anonymized` 在审计 API 响应中把用户邮箱/ID 脱敏为固定占位；`disabled` 时审计 API 返回 403 且 `/platform/settings` 审计分区隐藏。存储放 tenants 表新 JSONB 字段（跟随现有租户配置模式）

**会话分享（租户内登录分享）**：
- `sessions` 表加列 `share_token`（varchar, nullable, unique, 随机不可猜测）
- `POST /api/v1/sessions/:id/share` — 生成 token 并返回完整链接；`DELETE /api/v1/sessions/:id/share` — 撤销（置 NULL）
- `GET /api/v1/shared/sessions/:token` — **登录态 + 分享者与查看者同租户**校验通过后返回只读快照（消息 + 引用，不含反馈者身份）；无权限返回 404（不泄露存在性）
- 只读快照复用现有会话消息读取序列化，不新增写路径

**前端**：
- admin：settings 新分区 `query-history`（表格：用户/时间/来源/反馈过滤 + 详情抽屉 + CSV 导出按钮与状态轮询）
- 用户：会话侧栏 `session-sidebar.tsx` 菜单加"分享"项 + 分享弹窗（展示链接、撤销按钮）
- 新只读路由 `/platform/shared/:token`（登录守卫内，无输入框）

### SP14 · 轻项收割（P-3/P-10/P-11）

**套餐接线**（后端零改动）：
- `BillingPage`/`CheckoutPage`/`RefundPage`/`AdminCommercialPage` 挂路由：`/platform/billing`、`/platform/billing/checkout`、`/platform/billing/refunds`（`router.tsx` routeTree + `routes.tsx` 守卫）
- settings general 分区顶部加当前套餐卡片（读 commercial summary）与升级入口
- **不引入** seats 概念与 Stripe（现有 commercial + 支付宝/微信已是等价物）

**API key 小补**：
- settings API key 分区（`PlatformApiKeysPanel` 与租户 key 面板）加 API 文档入口：链接 `/swagger/index.html`，仅非 release 模式显示（与后端路由开关一致）+ capabilities 权限说明文案

**对话偏好扩展**：
- `UserPreferences`（`internal/types/user.go`）加 `default_model` 字段（JSONB，无迁移成本）；`PUT /api/v1/auth/me/preferences` 已有，扩展请求结构体
- 前端 settings 新分区 `chat-preferences`：默认模型选择器（复用现有模型选择组件）
- 新会话模型解析链：**会话覆盖 > agent 默认 > 用户默认 > 租户默认**（对齐 Onyx 优先级；注入点=会话创建时 agent_config 初始化，未显式选择模型时填用户默认）

## 5. 代码组织约定

- 后端：每域 `internal/types/<domain>.go`（模型）+ `internal/application/repository/<domain>.go`（数据）+ `internal/handler/<domain>.go`（路由），路由注册进现有 `routes_*.go` 分组；DB 迁移跟随现有 migrations 机制
- 前端：api-client 按域建 `packages/api-client/src/<domain>/index.ts` 工厂 → `client.ts` 装配 → `src/index.ts` barrel；类型进 `packages/contracts`；页面挂 `router.tsx` routeTree 或 settings registry，遵循 `/platform` 前缀与守卫
- 鉴权：复用现有 middleware（登录态、租户、角色），不新增认证机制

## 6. 测试与验收

- **后端**：repository 单测（聚合 SQL 正确性是重点）+ handler 集成测试，遵循现有测试基建
- **前端**：vitest 组件测试；页面截图证据进 `docs/migrations/react/evidence/` 惯例目录
- **专项测试点**：
  - 日桶 upsert 幂等（同一消息完成重放不重复计数）
  - craft 与 chat 双写入点均进 user_usage，flow 维度正确
  - anonymized 模式脱敏；disabled 模式 403 + 分区隐藏
  - 分享 token 跨租户访问被拒（404）；撤销后旧链接失效
  - CSV 导出全链路（触发→轮询→下载内容正确）
  - 模型解析优先级：会话 > agent > 用户 > 租户
- **门禁**：沿用仓库现有 Makefile/CI 目标，不新增工具链

## 7. 明确不做（本线范围外）

- Onyx EE 目录代码的复制式移植（analytics/query-history 在 Onyx 属 EE，本设计为语义重写）
- Stripe、license/seats 体系（commercial 已覆盖）
- 公开匿名分享链接（安全模型决策：仅租户内登录分享）
- Onyx 的 token-rate-limits 预算体系（WeKnora 已有 BudgetGrant/commercial 预算预留，语义已覆盖）
- 用量 PDF 报表（CSV 已覆盖首期诉求）
- embed 渠道的消息反馈按钮

## 8. 实施顺序与依赖

SP11 → SP12 → SP13；SP14 独立可穿插。依赖关系：SP13 的审计详情需要 SP11 的
`message_feedback` 表与反馈数据；SP12 与两者无依赖。
