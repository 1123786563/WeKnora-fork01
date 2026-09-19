# SP11 反馈+分析（P-4/P-5）验收证据 — 2026-09-19

## 1. 交付清单（提交号）

| # | 提交 | 任务 | 内容 |
|---|------|------|------|
| 1 | `c9310a09` | Task 1 | message_feedback 表（sqlite 000079 / pg versioned 000158）+ repository（Upsert/Remove/ListMine） |
| 2 | `722ec67c` | Task 2 | 反馈提交/撤销/回显 API（service 所有权校验 + handler + routes） |
| 3 | `ce42a1ae` 打包携带 | Task 3 | 四组读时聚合查询（repository/analytics.go + interfaces/analytics.go）；该提交为并行会话整树提交，analytics 文件核验零偏差（同内容平行分支提交 `56bf720a`） |
| 4 | `66964201` | Task 4 | admin 分析聚合端点与路由（handler/analytics.go + routes_analytics.go） |
| 5 | `ae47e26b` | Task 5 | contracts：analytics 与 feedback 契约类型 + parsers |
| 6 | `a7361af8` | Task 6 | api-client analytics/feedback API（并行分支同内容提交 `ccf97831`） |
| 7 | `3c248161` 打包携带 | Task 8 | 消息气泡 like/dislike（message-list.tsx 乐观更新 + chat-copy + page 接线）；并行整树提交，内容核验零偏差 |
| 8 | `3726f8c2` | Task 9 | /platform/analytics 分析仪表盘（recharts 四图 + 日期过滤 + RBAC 门） |
| 9 | `b0783bf2` | Task 9 fix1 | 图表 label 键改用 chart-series 映射（unique_users raw-key 回退消除） |
| 10 | `524fe997` | Task 10 冒烟发现 | 纯日期 end_time 含当日全天 + 空结果返回 `[]`（详见 §4） |

Task 7（embed 不接线）按计划为"不做"项：按钮仅在 web 宿主接线。

## 2. 回归结果

### 2.1 后端（`go build ./cmd/... ./internal/...` OK；`go test ./internal/...` 97 包 ok / 9 包 FAIL，共 18 个测试红）

**本线全部绿**（`go test -run "Feedback|Analytics" -v`，覆盖 repository/service/handler 三层）：

- repository：`TestAnalyticsAggregations`（sqlite+postgres 双库）、`TestFeedbackUpsertIsIdempotentPerUser`（sqlite+postgres）
- service：`TestFeedbackServiceSubmitEnforcesOwnership`、`TestFeedbackServiceSubmitRecordsCallerScope`、`TestFeedbackServiceRemoveAndList`
- handler：`TestAnalyticsHandler_*` 6 项 + 冒烟修复新增 3 项（`DateOnlyEndCoversWholeDay`/`TimestampedEndNotExtended`/`EmptyRowsRenderEmptyArray`）+ `TestFeedbackHandler_*` 4 项

注：`go build ./...` 在 `apps/mobile/build/ios/.../libwebp`（并行会话的 Conduit 快照 vendored iOS 产物）因本机缺 `swig` 失败，与本线无关；`./cmd/... ./internal/...` 构建通过。

**预存红清单（逐个归属，均非本线引入）**：

| 测试 | 包 | 归属证据 |
|------|----|---------|
| `TestExecutionDispatchSQLiteMigrationHead` | repository | 已知硬编码 57：c9310a09~1 时 migrations/sqlite 已到 000078（漂移早于 SP11），SP11 的 000079 仅 +1（现 79） |
| `TestSQLiteMigrationsCreateVersionedSchema` 等 6 项 SQLite 迁移测试 | database | 同类硬编码计数（71 vs 79）+ dirty-at-66；并行迁移漂移预存 |
| `TestAgentRunToolCallProjectsDurableRunID` | agent | workbench/paseo 线（文件最后改动 a085c912/f6104706） |
| `TestExecuteDurableRunPersistsBudgetExhaustionForNotification` | service | notifications 线（29713103） |
| `TestExecutionTargetRegistrationGinSQLiteLifecycle`、`TestExecutionRegistrationChallengeRequiresTenantAndOwner` | handler | 已知 router/registration 类（404 生命周期） |
| `TestDeleteSessionTombstonesCraftResources` | handler/session | craft 线（0697597e o03 tombstone） |
| `TestExecutionRegistrationRoutesUseRealRegistrationTargetRevokeLifecycle` | router | 已知 registration 类 |
| `TestEveryContextKeyDeclaresACloneDecision` | types | `CredentialVersion` key 由 w22 线 261a4143 引入，SP11 未触碰 const.go/context_clone.go |
| `TestMX003CrossLang*` 3 项 | workbench | 缺 fixture `tests/mobile-v2/fixtures/mx-003-crosslang.json`（并行 mobile 会话） |
| `TestCreateTargetIfTrustedConcurrentCredentialRotation` | repository | 复跑出现（SQLite "database is locked" 负载抖动，paseo 域并发测试） |

### 2.2 前端

- `pnpm test:shared`：**872/875**。3 红 = `packages/views/src/guides/use-kb-detail-guide-trigger.test.tsx` 全部 `createRoot is not a function`（guides 域环境问题，并行会话领域）。
- `pnpm test:web`：**1838/1877**。39 红 = 38 项 `apps/web/src/documents`（并行 documents 会话已知）+ 1 项 `apps/web/src/platform/global-command-palette-live-search.test.tsx`（debounce 计时抖动；**单独复跑 7/7 全绿**，判定 flake）。
- 本线显式复跑全绿：`apps/web/src/analytics/*.test.ts` 4/4；`packages/contracts/src/analytics.test.ts` + `packages/api-client/src/analytics/*.test.ts` + `packages/views/src/chat/message-list.test.tsx` 12/12；`message-list.test.tsx` 单独 5/5（含 4 处 feedback 断言）。

## 3. 冒烟（Playwright + 真实后端）

环境：共享 dev 栈（postgres/redis/docreader 容器健康）；Go 后端按 `.env` 惯例跑在 **8084**（brief 所记 8082 被并行 Onyx 会话的 expo 占用）；React dev 5175（/api 代理至后端）；mock LLM `:18090`（`dev:mock-llm` 已运行，模型 `builtin-llm-mock`/mock-stream-model）。冒烟账号 `sp11-smoke@local.dev`（tenant 10005 owner）。

### 3.1 消息反馈全生命周期 ✅

1. 聊天页发送消息 → mock 流式回复渲染
2. 助手气泡工具栏出现 `赞`/`踩`；点 `赞` → `POST /api/v1/messages/{sid}/{mid}/feedback` `{"rating":"like"}` → **200**，DB `message_feedback` 落行（rating=like）
3. 刷新页面 → `GET .../feedback/mine` 200 → `赞` 变为 `撤销评价`（**回显**）
4. 点 `撤销评价` → `DELETE .../feedback` → **200**，DB 行删除、`/feedback/mine` 返回 `[]`，按钮回到 `赞`/`踩`
5. 重新点赞保留数据供分析图消费

### 3.2 /platform/analytics 四图 ✅

- 四区全渲染：查询趋势（查询数/点赞/点踩）、活跃用户、渠道分布（web 桶）、Agent 维度（空态文案）
- DOM 断言 `.recharts-wrapper,.recharts-surface` = **10** 个图表元素；无 `data: expected an array` 报错
- 日期过滤：改窗口点 `应用` → 三端点带新 `start_time/end_time` 重拉（网络断言）；空窗口无报错
- 导航入口「数据分析」对 owner 可见

### 3.3 非 admin 入口不可见

未做浏览器双人冒烟（需第二账号+邀请流）；由既有单测覆盖：settings registry「nav filtering by role hides admin-only... from a viewer」+ AnalyticsPage `canView`（owner/admin）门测试。连同 Task 9 已记录的 RBAC 不对称遗留见 §5。

截图（本目录）：`sp11-chat-feedback-rated.png`（已赞回显态）、`sp11-analytics-dashboard.png`（默认窗口四图含当日数据）。

## 4. 冒烟发现并修复的缺陷（`524fe997`）

- **现象**：默认窗口三图全空且前端报 `data: expected an array`；接口返回 `{"data":null,"success":true}`。
- **根因一（边界语义）**：仪表盘默认窗口 `[T-30, T]` 以纯日期传 `end_time`，后端解析为当日 0 点作 `[from, to)` 上界 → 当天数据全部排除。
- **根因二（契约违反）**：GORM `Scan` 零行留下 nil 切片 → JSON `data:null`，而 api-client contracts parsers 契约要求数组（Go 侧测试对 `null` 反序列化为 nil 不报错，故单测未拦截）。
- **修复**：`parseAnalyticsRange` 纯日期 end +24h（含当日全天，带时间戳的不加）；四端点 `analyticsRows` nil→`[]` 兜底；补 3 个回归测试。
- **复验**：接口默认窗口返回当日真实数据（queries=1/likes=1、active_users=1、channels web=2），空窗口返回 `[]`；页面四图渲染+过滤重拉通过。

## 5. 已知遗留（deferred minors）

- IM 渠道会话在渠道分布图落 web 桶（sessions 无 source 物理列，owner 派生三桶；细分需 join im_channel_sessions）
- Task 2 `:id` 兜底分支无执行测试；纯空白 start_time 绕过 400（继承 parseFilterTime 惯例）
- Task 8 快速连点并发在途 UI 态覆盖（乐观更新已知局限）；copy/showAgentToast deps
- Task 9 RBAC 不对称（systemAdmin 见导航但页面按租户角色占位）；导航门 `canSeeAdminSessionSources` 与 `canViewChannelSessions` 的等价假设
- 预存失败（非本线）：`TestExecutionDispatchSQLiteMigrationHead`（硬编码 57 vs 实际 79，漂移早于 SP11）、documents 域前端 39 失败（并行会话在途）、guides 域 3 失败（createRoot 环境问题）
- 提交归属：Task 3/6/7/8 的改动被外部并行整树提交（`56bf720a`/`ce42a1ae`/`ccf97831`/`3c248161`/`5989f2f3`）打包带入，内容均核验零偏差；如需按任务归位历史需人工 rebase
- 冒烟环境 fixture 欠账：新建租户未自动播种已配置的 builtin-quick-answer 智能体（`config.model_id=""` 触发发送就绪门阻断），冒烟时手工为 tenant 10005 播种（复制 10002 行、model_id=builtin-llm-mock）；该问题属共享 dev 环境数据面而非 SP11 交付物
