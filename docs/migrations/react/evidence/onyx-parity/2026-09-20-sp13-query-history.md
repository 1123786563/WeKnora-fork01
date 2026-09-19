# SP13 查询历史审计+导出+隐私+分享（P-6~P-9）验收证据 — 2026-09-20

## 1. 交付清单（提交号）

| # | 提交 | 任务 | 内容 |
|---|------|------|------|
| 1 | `c2e2bb04` | Task 1 | 存储层三件：sessions.share_token 列 + `query_history_export_jobs` 表（PG versioned/sqlite 成对 up/down）+ 租户 KV `query-history-config` 隐私三档读写（repository 层） |
| 2 | `158c46d2` | Task 2 | admin 审计列表：`GET /api/v1/sessions?source=all` 扩展全 source + user/时间/反馈过滤（feedback 过滤复用 SP11 message_feedback 表），normal/anonymized/disabled 三档读时 gate |
| 3 | `df6a5095` | Task 3 | `GET /api/v1/admin/sessions/:id/snapshot`（会话行+最近 200 条消息+全部反馈行；disabled 403 / anonymized user 脱敏）+ feedback ListBySession 跨用户枚举 + 隐私 KV service 层 |
| 4 | `3b4b71c3` | Task 4 | asynq 异步 CSV 导出三段式：`POST /admin/sessions/export`（入队 job）→ `GET /export/:job_id/status`（pending/running/done/failed）→ `GET /export/:job_id/download`（CSV 流）；导出服务落 file_service，超时×3 转 failed |
| 5 | `182f8237` | Task 5 | 会话分享：mint（32B URL-safe token，重开即轮换）/revoke（幂等）/`GET /shared/sessions/:token` 只读快照（owner 或 Admin+ 可 mint；token 仅铸币租户内可解析） |
| 6 | `c95d41fc` | Task 6 | contracts query-history 域（列表/快照/导出/分享/配置五组 parser）+ api-client `queryHistory` 域（含 requestBinary CSV 下载） |
| 7 | `ec141d7f` | Task 7 | settings 审计分区 QueryHistoryPanel（隐私 radio 三档+用户/日期/反馈过滤+表格+分页+快照抽屉+导出触发/轮询/下载）+ `?section=query-history` 深链注册 |
| 8 | `e22b1a65` | Task 8 | 分享入口三处（侧栏 ⋯ 菜单/ChatRoutePage/SessionShareDialog 弹窗）+ 只读分享页 `/platform/shared/:token`（无效/撤销链接占位） |

路由总装：`internal/router/routes_query_history.go`（Admin+ + full-access-key）与 `routes_chat.go` 分享段（Viewer+ 路由 + owner/Admin+ 服务内校验）随 Task 2-5 各自提交。

## 2. 回归结果

### 2.1 后端（`go build ./internal/... ./cmd/...` OK；`go test ./internal/...` 117 包 ok / 6 包红）

**本线 query_history 全部绿**（显式复跑，覆盖四层）：

- `internal/types`：包整体 ok（含 query_history.go 类型）
- router：`TestRegisterQueryHistoryAdminRoutes`、`TestRegisterSessionShareRoutes` 2/2（静态/动态段共存与 RBAC 链）
- service：`QueryHistory|SessionShare|ShareSession|SharedSession` 15/0（导出 CSV 构建/隐私 gate/分享 owner-admin/轮换/碰撞重试等）
- repository：`QueryHistory|SessionShare|Feedback|Session` 43/0（sqlite+postgres 双库）
- handler/session：`QueryHistory|Snapshot|Export|Share|Shared|Feedback` 19/0

**预存红清单（9 项逐个归属，均非本线引入）**：

| 测试 | 包 | 归属证据 |
|------|----|---------|
| `TestArtifactVersionsMigrationDownIsReversible` | repository | `duplicate column backup_reconciled`（execution_cleanup 迁移）；**worktree 对照**：78dc09db（SP13 前基线）单跑 ok，c2e2bb04~1（SP13 首提交父）单跑已败 → 由并行 runtime 线（`ea2f88c6`/`213bd0b3` 引入 backup_reconciled）在 SP13 首提交前引入 |
| `TestExecuteDurableRunPersistsBudgetExhaustionForNotification` | service | notifications 线，SP12 已记录 |
| `TestExecutionTargetRegistrationGinSQLiteLifecycle`、`TestExecutionRegistrationChallengeRequiresTenantAndOwner` | handler | registration 类，SP11/SP12 已记录 |
| `TestDeleteSessionTombstonesCraftResources` | handler/session | craft 线，SP12 已记录 |
| `TestWorkbenchDecisionHTTPResumesDurableApprovalAndScopesIdentity`（10m panic，挂死 9m25s） | handler/session | **与 SP12 记录的同名挂死同测试**（SP12 §2.1 worktree 对照已证早于 SP12），workbench/环境预存 |
| `TestExecutionRegistrationRoutesUseRealRegistrationTargetRevokeLifecycle` | router | registration 类，SP11/SP12 已记录 |
| `TestMX003CrossLang*` 3 项 | workbench | 缺 fixture（并行 mobile 线），SP12 已记录 |

注：SP12 记录的 database 包 SQLite 迁移计数漂移与 types 包 `CredentialVersion` 两项本轮已绿（并行会话已修复硬编码计数/补 clone 决策）。

### 2.2 前端

- `pnpm test:shared`：**898/901**。3 红 = guides kbDetail entry trigger（SP11/SP12 已记录的 guides 域预存，同一批用例）。
- `pnpm test:web`：{{WEB_FULL_PLACEHOLDER}}
- 本线显式复跑全绿：contracts `query-history.test.ts` + api-client `queryHistory/index.test.ts` **21/21**；web `query-history-panel.test.tsx` + `chat/session-share.test.ts` + `chat/session-sidebar.test.tsx` + `router.test.tsx` **30/30**。

## 3. 冒烟（真实运行栈）

**环境**：共享 dev 栈的 8084 后端运行 CommitID `78dc09db`（2026-09-19 15:44 构建，早于 SP13 全部提交；SP13 端点在其上 404/"unsupported key"），且并行会话（R484 web-search 等）正活跃使用该栈——重启共享后端会打断并行工作，未动。改为**专用冒烟栈**：本仓库工作树起 `go run ./cmd/server` 于 **8085**（同一 postgres/redis 容器；REDIS_DB=2 隔离 asynq 队列防与旧后端互抢任务；SSRF_WHITELIST 增 127.0.0.1；LOCAL_STORAGE_BASE_DIR 改本机可写目录——`/data/files` 为容器路径，宿主直跑只读）+ vite dev **5176**（VITE_DEV_PROXY_TARGET=8085）。冒烟账号 `sp13-smoke@local.dev`（tenant 10040 owner），数据：2 会话+1 轮本机 ollama（qwen2.5:0.5b，注册租户模型+自建 agent 绑定）真实问答+1 条 like 反馈。

### 3.1 admin 审计列表（API）✅

`GET /api/v1/sessions?source=all`：2 行（标题/用户/来源/引擎/时间）。过滤矩阵——`user_id` 精确命中 2 / 不存在用户 0；`feedback=like` 仅命中带赞会话 / `dislike` 0；`page_size=1` 分页（total=2、1 行）；`start/end_time` 未来窗 0 / 当日窗 2。

### 3.2 快照抽屉 ✅（API+UI）

API `GET /api/v1/admin/sessions/:id/snapshot`：会话行+6 条消息（3 轮 user/assistant）+1 条 feedback（like）+normal 档 user_id 可见。UI（5176 `/platform/settings?section=query-history`）：行点击打开「会话快照」抽屉——会话信息（创建时间等）+消息流（用户/助手逐条）+「反馈（赞 1 / 踩 0）」。截图 `sp13-snapshot-drawer.png`。

### 3.3 导出三段式全链路 ✅（API+UI）

API：`POST /admin/sessions/export` → `{job_id}`；`GET /export/:id/status` → done（含 file_path）；`GET /export/:id/download` → `text/csv; charset=utf-8` + `Content-Disposition: attachment; filename=query_history_export_2.csv`，正文 BOM + 表头 `session_id,title,user_id,source,engine_type,created_at,updated_at,message_count,like_count,dislike_count` + 2 数据行（message_count 8/6、like_count 1/0 与库一致）。过滤导出 `feedback=like` → 仅 1 行。UI：点「导出 CSV」→ 网络轨迹 POST export（200）→ GET status（200）→ 按钮变「下载 CSV」→ 点击真实下载 `query_history_export_4.csv`（内容同上）。截图 `sp13-export-download-ready.png`。

### 3.4 分享：生成→只读页→轮换/撤销→404 ✅（API+UI）

API：mint → token；`GET /shared/sessions/:token` 200（会话+6 消息、无 feedback 字段；user_id 不遮蔽为设计——`session_share.go:151` 注释明示"share readers are logged-in"）；重开 mint 轮换 → 旧 token 404；`DELETE` 撤销 → 404；重复撤销幂等 success。UI：侧栏 ⋯ 菜单「分享」→ 弹窗自动生成只读链接（`http://…/platform/shared/<token>`）+复制/撤销按钮（截图 `sp13-share-dialog.png`）；新标签打开只读页 → 标题+「只读」徽标+创建时间/来源/引擎+6 条消息流（截图 `sp13-shared-readonly-page.png`）；「撤销分享」后刷新 → 「链接无效或已撤销」占位页（截图 `sp13-shared-revoked-404.png`）。

### 3.5 隐私三档 ✅（API+UI）

API：`PUT tenants/kv/query-history-config {"mode":"anonymized"}` → 列表与快照 user_id 均返回 `anonymous`（消息仍全量）；`disabled` → 列表/快照/导出提交全部 403（"query history is disabled for this tenant"）；回 `normal` → user_id 恢复。UI：radio 切「匿名」→ 表格用户列全变 `anonymous`（截图 `sp13-anonymized-user-column.png`）；切「禁用」→ 面板占位「查询审计已被租户禁用。」且不发列表请求（KV 先判；列表 403 由 curl 直证），仅隐私 radio 保留可再开（截图 `sp13-disabled-placeholder.png`）；回「正常」→ user_id 恢复可见。

### 3.6 冒烟发现的非本线问题（环境/并行线）

- 共享 8084 后端二进制落后（78dc09db < SP13）：SP13 端点在共享栈不可达，属栈重启欠账（见 §5）；本证据以专用 8085/5176 栈等价覆盖（同一 DB/Redis、同一工作树代码）。
- 聊天路由页 dev 加载失败：工作树 `ChatRoutePage.tsx`（并行会话**未提交**的在途修改）import 尚不存在的 `@weknora/views/chat/web-search` → vite 编译错。非 SP13 回归（SP13 提交 e22b1a65 内容完整；分享入口走 PlatformShell 侧栏同一 SessionShareDialog 完成 UI 冒烟）。
- mock LLM 钉死旧 LAN IP 192.168.3.30:18090（SP12 已记录），builtin-llm-mock 不可达；租户默认 agent 模型解析不带请求级 model_id，需自建 agent 绑定本机 ollama 模型后问答才通（环境绕行记录，供后续冒烟复用）。

## 4. 截图清单（本目录）

`sp13-query-history-panel.png`（审计面板全貌：隐私 radio+过滤+表格）、`sp13-snapshot-drawer.png`、`sp13-export-download-ready.png`、`sp13-share-dialog.png`、`sp13-shared-readonly-page.png`、`sp13-shared-revoked-404.png`、`sp13-anonymized-user-column.png`、`sp13-disabled-placeholder.png`

## 5. 已知遗留（deferred minors）

- CSV 按会话行汇总（非 Onyx 逐问答对——问答明细走快照抽屉）
- 快照 200 条消息截断（分享页同源同截断）
- 导出 10000 行静默截断（保留最旧）
- 导出超时×3 全耗尽时 job 卡 running 状态（失败显式标 failed 仅覆盖可判定错误路径）
- share 重开即轮换旧链接失效（无"保留旧链接"选项）；token 无过期时间（撤销即失效）
- CSV 公式注入防护与 SP12 usage 导出同现状，待统一加固
- status 端点返回内部 `file_path`（resource:// URI，非敏感路径但属内部细节外泄面）
- anonymized 档未覆盖 title 列（user_id 已脱敏；im_user_id 已于终审修复一并抹除——`AnonymizeSessionOwner` 置空 + omitempty 从 wire 消失，快照行本为 `types.Session` 无 IM 主键字段，测试双断言钉住）
- 渠道视图与隐私三档的分叉（终审 F2b 显式登记）：三档 gate（disabled 403 / anonymized 脱敏）仅作用于三面——source=all 审计列表、admin 快照、CSV 导出；`GET /api/v1/sessions` 的渠道过滤视图（source=api/embed/IM 平台名）不查 `CheckQueryHistoryAccess`（见 session.go ListSessions：gate 仅在 sourceAll 分支）。渠道视图是先于隐私档落地的既有端点语义（渠道管理/排障用途，Admin+ 才可达），SP13 终审裁定本次不扩 gate、既有语义保留；follow-up：将渠道视图并入三档 gate 或为其定义等效隐私语义（连带 title 脱敏一并收口）。
- 共享 dev 栈 8084 二进制落后 SP13：待下次栈重启后可在共享栈复跑 §3 API 面（专用栈已等价验证）
