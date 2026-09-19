# SP12 用量聚合（P-1/P-2）验收证据 — 2026-09-19

## 1. 交付清单（提交号）

| # | 提交 | 任务 | 内容 |
|---|------|------|------|
| 1 | `43b8be8a` | Task 1 | user_usage 日桶表（PG versioned 000160 / sqlite 000081，up/down 成对）+ repository（AddUsage upsert 累加 + 两级聚合查询）；唯一键 `uq_user_usage_dims (tenant_id,user_id,window_start,model,flow)` + 读索引 `idx_user_usage_tenant_window` |
| 2 | `54f37272` | Task 2 | 计价管道 internal/usage（`WEKNORA_USAGE_RATES` env JSON → ModelRates，微积分 cost，半值四舍五入；未配费率成本 0，parse 失败降级不计费只告警） |
| 3 | `af33e632` + `1c9fd69b`（并发带入） | Task 3 | chat 写入点：UsageRecorderService（service/usage_recorder.go + 6 测试）+ completeAssistantMessage 接线（tenant/user 归因传入 qa.go 全部 3 个完成路径 + stop watcher）；af33e632 为并行会话提交，内容与 SP12 Task 3 计划一致 |
| 4 | `afd25ea9` | Task 3 fix1 | 并发双计窗口关闭：`usageRecordOnce sync.Map LoadOrStore(messageID)` 一次性 gate + `completeMsgMu` 串行化消息落库（评审修复） |
| 5 | `9548d95f` | Task 4 | craft 写入点：craft record 事务内 fold 进日桶（owner_id 归因；correction 事实不重放日桶，宁可少计；revision 单调校验） |
| 6 | `839d68b6` | Task 5 | 三端点 + 路由（`GET /usage/me` Viewer+chat-key、`GET /admin/usage/by-user` 与 `GET /admin/usage/export` CSV Admin+/full-access-key；routes_usage.go 带 RBAC 论证注释） |
| 7 | `4d7616e9` | Task 6 | contracts usage 域（usage.ts + parsers 数组契约）+ api-client usage 域（usage/index.ts） |
| 8 | `1a3f1f1c` | Task 7 | settings 用量分区（UsagePanel：日期窗口 + 模型行 + 合计 + 预算卡，usage-panel.test.tsx） |
| 9 | `5e0f000d` | Task 8 | analytics 用量 tab（全员表 + 导出 CSV 入口，usage-tab.test.tsx；与图表共用时间范围） |

## 2. 回归结果

### 2.1 后端（`go build ./internal/... ./cmd/...` OK；`go test ./internal/...` 102 包 ok / 8 包红）

**本线 usage 全部绿**（显式复跑，覆盖四层）：

- `internal/usage`：包整体 ok（费率解析/微积分计价/半值舍入）
- service：`TestRecordChatTurn{MapsDimensions,CacheFallback,SkipsInvalidInput,CostFromInjectedRates,PropagatesRepoError,NilReceiverSafe}` 6/6
- repository：`TestUserUsage{AddAccumulates,Aggregations}` + `TestCraftUsageStore{AppendCountsPhysicalAttemptsOnce,CorrectionAppendsRevisionNeverOverwrites,CrossTenantSameCallIDNeverMixes,RejectsUntrustedFacts,FoldsFactsIntoUserUsageDailyBuckets,UnknownFirstRevisionKeepsBucketAtFirstObservation}` 8/8（sqlite+postgres 双库）
- handler：`TestUsageHandler_*` 9/9（scope/tenant 门、空数组 `[]` 契约、分页 clamp、CSV 表头、500 映射、非法时间参数）
- session 写入点：`TestCompleteAssistantMessage{RecordsChatUsage,UsageSkips,UsageErrorDoesNotBlock}` + `Test{BuildStreamResponsePromotesUsageOnCompleteEvents,BuildStreamResponseLeavesUsageNilWhenAbsent,StreamEventUsageSurvivesJSONRoundTrip}` + `TestCraftUsageHTTP{AccessMatrix,ShapeHidesMoneyAndSecrets,RouteAbsentWithoutRegistration}` 全绿

**预存红清单（逐个归属，均非本线引入）**：

| 测试 | 包 | 归属证据 |
|------|----|---------|
| `TestExecutionDispatchSQLiteMigrationHead` | repository | 硬编码 57 vs 实际 83（漂移继续扩大：SP11 时 79）；brief 已列 |
| `TestSQLiteMigrationsCreateVersionedSchema` 等 7 项 | database | 同类硬编码计数 + dirty 基线，漂移预存（SP11 已记录） |
| `TestExecuteDurableRunPersistsBudgetExhaustionForNotification` | service | notifications 线，brief 已列 |
| `TestExecutionTargetRegistrationGinSQLiteLifecycle`、`TestExecutionRegistrationChallengeRequiresTenantAndOwner` | handler | 已知 router/registration 类（SP11 已记录） |
| `TestDeleteSessionTombstonesCraftResources` | handler/session | craft 线，brief 已列 |
| `TestExecutionRegistrationRoutesUseRealRegistrationTargetRevokeLifecycle` | router | 已知 registration 类（SP11 已记录）；注：router 包在全量跑中曾 `[build failed]`（`handler.ExpertHandler` 未定义）——并行 experts 会话在测试编译窗口内在途建文件所致，复跑编译通过、仅余此项预存红 |
| `TestEveryContextKeyDeclaresACloneDecision` | types | `CredentialVersion` key（w22 线 const.go 引入），SP12 未触碰 |
| `TestMX003CrossLang*` 3 项 | workbench | 缺 fixture `tests/mobile-v2/fixtures/mx-003-crosslang.json`（并行 mobile 会话） |
| session 包 10m panic（`TestWorkbenchDecisionHTTPResumesDurableApprovalAndScopesIdentity` 挂死 9m53s） | handler/session | **stash/worktree 对照法**：在 SP12 首提交的父提交（43b8be8a~1）临时 worktree 单跑该测试同样 240s+ 无结果 → 挂死早于 SP12，属 workbench/环境预存（两次全量复跑均复现，非负载抖动） |

### 2.2 前端

- `pnpm test:shared`：**881/884**。3 红 = `packages/views/src/guides/use-kb-detail-guide-trigger.test.tsx`（kbDetail entry trigger 3 项，createRoot 环境问题，SP11 已记录的 guides 域预存）。
- `pnpm test:web`：**1855/1893**。38 红全部位于 `apps/web/src/documents/KnowledgeDocumentDetailPage.test.tsx`（brief 已列的 documents 域预存，并行会话在途）；无任何 usage/analytics/settings 相关红。
- 本线显式复跑全绿：contracts `usage.test.ts` + settings `usage-panel.test.tsx` + analytics `usage-tab.test.tsx` + analytics 既有 `analytics-range.test.ts`/`chart-series.test.ts` 合计 **21/21**；api-client `usage/index.test.ts` **7/7**。

## 3. 冒烟（共享 dev 栈真实环境）

环境：共享 dev 栈活跃——后端 8084（go run，CommitID 与 HEAD 同步推进）、React dev 5175、postgres/redis 容器健康、本机 ollama（gemma4:latest/qwen2.5:0.5b）。mock LLM `:18090` 已按 `dev:mock-llm` 启动，但 `config/builtin_models.yaml` 与 `.env SSRF_WHITELIST` 均钉在旧 LAN IP `192.168.3.30`（本机现 IP 192.168.3.32），模型出站不可达；冒烟改用本机 ollama（在冒烟租户注册 `gemma4:latest` provider=ollama 模型）。

### 3.1 三端点 API 冒烟 ✅（冒烟账号 `sp12-smoke@local.dev`，tenant 10039 owner）

- `GET /api/v1/usage/me` → 200 `{"data":[],"success":true}`（空数组契约，SP11 修复的 null→[] 在本线同样成立）
- `GET /api/v1/admin/usage/by-user?page=0&page_size=50` → 200 空数组
- `GET /api/v1/admin/usage/export` → 200 `text/csv` + `Content-Disposition: attachment; filename=usage_export.csv`，正文 BOM + 表头 `user_id,model,flow,window_start,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,cost_microcredits`
- DB 核验：共享 postgres 上 `user_usage` 表已由 000160 建成（含 uq/idx 两索引），空表符合预期

### 3.2 settings 用量分区 ✅（真实页面）

`/platform/settings?section=usage`（共享浏览器已登录账号）：「用量统计」标题+说明+日期窗口（默认 T-30~T）+「应用」按钮渲染；网络断言 `GET /api/v1/usage/me?start_time=2026-08-20&end_time=2026-09-19` → 200 `{"data":[],"success":true}`；点「应用」重拉一次（请求重复出现）。空态显示「暂无内容」。截图 `sp12-settings-usage.png`。

### 3.3 analytics 用量 tab + CSV 下载 ✅（真实页面）

`/platform/analytics` → 「用量」tab 可切换渲染：全员说明文案 + 「导出 CSV」按钮 + 表格区（空态「暂无内容」）；网络断言 `GET /api/v1/admin/usage/by-user?page=0&page_size=50&start_time=...&end_time=...` → 200；点「导出 CSV」触发真实下载 `usage_export.csv`（表头与 §3.1 一致）。截图 `sp12-analytics-usage-tab.png`。

### 3.4 chat 一轮 → flow=chat 行 ⚠️ 欠账（环境阻断，根因已定位）

真实聊天轮完整跑通（ollama gemma4，quick-answer 管道，SSE answer/complete 正常、消息落库），但 `user_usage` 未出 chat 行。根因两层（均非 SP12 接线缺陷，SP12 写入点单测全覆盖）：

1. **上游 usage 填充缺口（预存）**：消息级 `usage` 仅在 tRPC-agent 路径（`agent/finalize.go turnUsage → AgentCompleteData.Usage → agent_stream_handler.go:737`）与 IM 路径（`im/service.go applyIMCompleteDataToMessage`）写入；web 默认 quick-answer 管道（`session_knowledge_qa.go`/`chat_pipeline/chat_completion_stream.go`）不落 message.Usage（本轮实测 DB assistant 消息 usage 列 NULL）→ completeAssistantMessage 的 `Usage != nil` gate（防双计设计）跳过。
2. **共享栈 tRPC 禁用**：`config/config.yaml agent.recovery.admission_enabled: false` → agent 模式请求报 "tRPC agent runs are disabled"，唯一有 usage 上游的路径在该环境不可跑。

### 3.5 craft 一轮 → flow=craft 行 ⚠️ 欠账（环境禁用）

`GET /api/v1/craft/sessions` 返回 `{"capabilities":{"allowed_kinds":["web"],"enabled":false}}`——craft 在共享 dev 栈关闭，无法产生 craft_usage_facts 供日桶 fold（该路径由 `TestCraftUsageStoreFoldsFactsIntoUserUsageDailyBuckets` 等 6 项双库单测覆盖）。

## 4. 冒烟发现的缺陷

- **发现（欠账级，非阻断）**：§3.4 第 1 条——quick-answer（web 默认模式）消息不带 usage，导致 P-1 chat 日桶在纯 web 快答场景无数据来源。归属上游「会话 token 用量统计」（af33e632 同代的管道填充缺口），不改变 SP12 写入点正确性（tRPC 路径与 stop 竞态路径正确记账）；建议后续任务在 quick-answer 管道 `response.Done` 处把 StreamResponse.Usage 落到 ChatResponse/消息。
- **终审更正（I-2，已修）**：本节初稿「IM 路径正确记账」表述失实——IM 路径（`im/service.go`）虽把 `AgentCompleteData.Usage` 写进 `msg.Usage`，但三处完成点直接 `UpdateMessage`，从未调用 usageRecorder，IM 渠道带 usage 的聊天轮此前不进 `user_usage`（漏计）。终审已在 IM 三处完成点（handleMessageStream 完成、runQA 正常完成、runQA 取消路径）补 `recordIMChatUsage`：usageRecorder 经 dig 注入 IM Service（构造参数追加），归属取 `withIMIdentity` 注入的 `PrincipalIMUser`（`<tenant>:<channel>:<platform>:<im user>`），模型取消息 ModelID 或 agent 绑定（并回写消息列），幂等用与 session Handler 同款 `usageRecordOnce` sync.Map 按消息 ID 一次性 gate，`internal/im/usage_record_test.go` 六项单测覆盖（归属/一次性 gate/无 usage/无 recorder/无模型跳过/取消上下文仍记账/记账失败 fail-soft）。
- 环境问题（非交付物）：`.env SSRF_WHITELIST` + `config/builtin_models.yaml` 钉死旧 LAN IP 192.168.3.30（本机已变更为 .32），mock LLM 不可达；修复需改共享 env/yaml 并重启共享后端，超出本任务权限，未动。租户自建 ollama 模型（空 base_url 走本机默认）可绕过。

## 5. 已知遗留（deferred minors）

- craft cost 列暂 0 防双算（Ruling P-2：commercial 管道已有计价，日桶不重复计）
- input/output 同价起步（分维费率留待真需要）；费率走 `WEKNORA_USAGE_RATES` env 配置而非面板（无 DB 费率表）
- correction 事实跳过日桶：宁可少计 + 差量补偿方案待决
- `usageRecordOnce` sync.Map 随消息 ID 缓慢增长（无淘汰；与家族风格一致，重启归零）
- 分页 page 0 基契约（Onyx 对齐）；offset 极端溢出未特判
- settings/analytics 加载与翻页竞态无 AbortController（沿用家族风格，最后写赢）
- analytics 用量表列合并展示（cache 列并入说明，完整明细走 CSV 导出）
- chat/craft 冒烟欠账（§3.4/§3.5）：需 tRPC admission 开启的环境或上游 quick-answer usage 填充补齐后复跑
- 预存红与并行漂移（非本线）：SQLite 迁移计数硬编码漂移（57 vs 83）、documents 域前端 38 红、guides 域 3 红、workbench hang（SP12 前基线复现）
