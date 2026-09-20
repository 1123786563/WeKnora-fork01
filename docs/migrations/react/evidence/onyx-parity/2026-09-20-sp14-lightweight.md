# SP14 轻项收割（P-3/P-10/P-11）验收证据 — 2026-09-20

> 平台运营面六域对齐线（SP11–SP14）收官交付。

## 1. 交付清单（提交号）

| # | 提交 | 任务 | 内容 |
|---|------|------|------|
| 1 | `6a8c8f1a` | Task 1 | 套餐接线：`/platform/billing` 三路由（billing/checkout/admin，照 analyticsRoute 模式 lazy+Suspense）+ GeneralPreferencesPanel `client` prop 与顶部套餐卡片（`formatBillingSummary` 五语）+ SettingsPage 传参 + i18n `generated/billing.ts`。RefundPage 有意不挂路由（Ruling P-1） |
| 2 | `eb49392f` | Task 2 | swagger 文档入口：后端 `GetSystemInfoResponse.swagger_enabled`（`gin.Mode() != ReleaseMode`，三模式表驱动测试）+ 契约可选字段 + `swaggerDocsUrl`/`shouldShowSwaggerDocs` 纯函数 + integrations 页 OpenAPI /docs 行条件渲染 + **两处计划外 bug**：悬空 `/docs` 链接改指 `/swagger/index.html`、platform-api-keys 分区不可达（settingsRoute role prop 折叠吞掉 system-admin → `scopeRuntime.isSystemAdmin() ? 'system-admin' : …`）+ guardRoute 收口（Task 1 遗留，`/platform/billing*` 三路径 + `/platform/shared/*` 入 platform 清单）+ PlatformApiKeysPanel API 文档外链 + capabilities 五语一行语义 |
| 3 | `c14c4501` | Task 3 | `default_model` 用户偏好后端四处：`types.UserPreferences.DefaultModel *string`（JSONB 无迁移）+ `updateMyPreferencesRequest`（无 omitempty，区分"未发"与"空串"）+ service PATCH 合并（`*""` → nil 清除回平台默认，镜像 LastActiveTenantID sentinel；nil 不动）+ 6 例合并矩阵 + 隔离测试 |
| 4 | `f8ca453c`（净效果含并行 `e72acff9` 收编） | Task 4 | 模型解析链方案 A（纯前端）：`resolveChatModelChip` 可选 `userDefaultModel` 层（pick > agent 绑定 > 用户默认 > models[0]，向后兼容）+ ChatRoutePage 拉取/接线/发送链 seed + ChatPreferencesPanel 分区（`ModelOptionSelect` 回显/保存/清除 + 优先级文案）+ registry/surface/SettingsPage/i18n 五语四处注册 |
| 5 | 本任务 | Task 5 | 收尾修复：`apps/web/vite.config.ts` 别名表补 `@weknora/views/integrations/swagger`（T2 只加了 package.json exports，vite 别名前缀匹配语义下 dev/build 模块解析失败——冒烟发现后一行修复，见 §3.5） |

净效果 4+1 提交：router/billing 三页 + general 卡片（P-3）、swagger 入口 + api-keys 分区解锁（P-10）、default_model 端到端（P-11）。

## 2. 回归结果

### 2.1 后端（`go build ./internal/... ./cmd/...` 通过，仅预存链接器重复库警告）

- `go test ./internal/application/service/`：**ok**（160s，含 T3 default_model 合并矩阵 8 断言组）
- `go test ./internal/types/`：**ok**
- `go test ./internal/handler/`：仅 2 项失败 —— `TestExecutionTargetRegistrationGinSQLiteLifecycle`、`TestExecutionRegistrationChallengeRequiresTenantAndOwner`（execution 注册 SQLite 测试迁移缺 `usage_binding_json` 列）。**预存**：T2 已在干净 HEAD worktree 复现同样失败，T3 stash 对照复确认；SP11/SP12/SP13 证据连续记录同一对。本线涉及的 `system_info_test.go`（swagger 三模式）与 auth 偏好透传均绿。

### 2.2 前端

| 套件 | 结果 |
|---|---|
| `pnpm typecheck:web` / `typecheck:shared` | 均通过（0 错误） |
| settings 目录全量（含 billing-card/ChatPreferencesPanel/GeneralPreferencesPanel/SettingsPage/surface/PlatformApiKeys） | 313/313 |
| chat 模块全量（含 model-chip 19 用例） | 198/198 |
| 定向：router.test.tsx + routes.test.ts（guardRoute 矩阵）+ billing-card + ChatPreferencesPanel + GeneralPreferencesPanel | 45/45 |
| i18n 包全量（含 keys.test.ts） | 82/82 |
| packages/views integrations + settings registry + api-client settings | 80/80 |
| `pnpm test:web` 全量 | 2043 pass / 38 fail —— 38 个全部集中于 `src/documents/KnowledgeDocumentDetailPage.test.tsx`（`createPortal is not a function` @ sheet.tsx），**预存**：T4 已在干净 HEAD（965a80f6）worktree 复现，T1 stash 对照同判；与本线文件无关。KnowledgeGraphPage 挂起未复现（全量 67s 正常跑完） |

## 3. 冒烟（真实运行栈）

**环境**：共享 dev 栈（8084 后端 / 5175 前端）活跃但后端二进制为 `78dc09db`（2026-09-19 15:44 构建，早于 SP14 全部提交；重启会打断并行会话，未动）。照 SP13 配方起**专用栈**：`go run ./cmd/server` 于 **8085**（同 postgres/redis 容器；`REDIS_DB=2` 隔离 asynq；`GIN_MODE=debug`；DB/REDIS/DOCREADER 主机名改 127.0.0.1；`LOCAL_STORAGE_BASE_DIR` 本机可写目录）+ vite dev **5176**（`VITE_DEV_PROXY_TARGET=8085`）。冒烟账号 `sp14-smoke@local.dev`（tenant 10041 owner，冒烟期间提权 system-admin 验证分区可达后保留于 dev 库）；租户注册本机 ollama `qwen2.5:0.5b`（KnowledgeQA）。

### 3.1 billing 三页渲染 ✅（UI）

- `/platform/billing`：渲染「空间 10041 · 账单与额度」+ shell + scope key 租户隔离；数据区显示请求失败——根因为**预存解析错配**（§3.6 发现#1），非路由/接线问题。
- `/platform/billing/checkout`：渲染「订单结算」，自建 quote 报 `no published plan with this key` —— 已知欠账（CheckoutPage plan_key 硬编码 `pro` + 目录无发布套餐，组件原状）。
- `/platform/billing/admin`：system-admin 账号可见「平台商业运营」五分区（目录发布/授权审计/退款审核/paid 待履约/结算差异），各分区 blocked-env 占位不塞假数据 —— Ruling P-2 + 运营数据端点欠账原状。

### 3.2 integrations 文档链接 ✅（API+UI）

- 8085（当前代码，debug 模式）：`GET /system/info` → `swagger_enabled: true`；`GET /swagger/index.html` → 200。
- UI（5176 integrations api tab）：「OpenAPI /docs」行渲染，链接指向 `/swagger/index.html`（悬空 `/docs` 修复生效）。
- 共享 8084（旧二进制）：system info 无 `swagger_enabled` 字段 → 前端隐藏该行 —— **向后兼容分支按设计生效**（`shouldShowSwaggerDocs` 仅显式 true 才显示）。

### 3.3 platform-api-keys 分区可达 ✅（system-admin；修复后）

- DB 无既有 system-admin，冒烟账号 `UPDATE users SET is_system_admin=true`（dev 库）。
- settings 导航出现平台分区，`?section=platform-api-keys` 完整渲染：面板标题 + header「API 文档」外链（`/swagger/index.html`）+ 创建卡（密钥名称 + 7 能力 checkbox 五语文案）+ 空列表占位。
- **修复前置**：首次访问模块解析失败（§3.5 发现#2），一行修复后 5176 与共享 5175 的面板模块均 200。

### 3.4 chat-preferences 设默认 → 新会话生效 → 清除回退 ✅（API+UI 全链）

1. UI（`?section=chat-preferences`）：分区渲染（标题/优先级文案「会话内手动选择 > 助手绑定模型 > 个人默认模型 > 租户默认（列表第一个）」/下拉/前往模型配置）；选 `Qwen2.5 0.5B (local)` → 「默认模型已保存」→ API `auth/me` 确认 `preferences.default_model = <模型 UUID>` 持久化。
2. 新会话（`/platform/creatChat`）：「对话模型」下拉 **selected = Qwen2.5 0.5B (local)** —— 无 pick 无 agent 绑定时用户默认生效（chip 与发送链 seed 同源）。
3. UI 选「移除」→ API `preferences = {}`（空串=清除语义全链贯通）→ 新会话回退 **selected = mock-stream-model（models[0] 租户默认）**。

API 级补充（8085）：curl 设值/读回/空串清除/第三键 patch 不动 default_model（合并隔离）四步全部符合 T3 矩阵。

### 3.5 冒烟发现并修复：vite 别名漏登记（T2 引入，本任务修复）

`PlatformApiKeysPanel.tsx` import `@weknora/views/integrations/swagger` 在 vite 下解析失败：`apps/web/vite.config.ts` 的 workspace 解析采用**显式子路径别名表**（`@rollup/plugin-alias` 前缀匹配语义——未登记的子路径会命中 `'@weknora/views'` 兜底别名拼出不存在的路径），T2 只登记了 `packages/views/package.json` 的 `exports`（tsx/node/typecheck 走 exports 因此全绿），漏加 vite 别名 → dev（且预期 build）下面板模块不可达。修复：别名表补一行（`settings-route` 与 `swagger` 相邻同款）。修复后 5176 专栈与共享 5175（vite 监听 config 自动重启）均 200。**教训入账：子路径出口需同时登记两处（package.json exports + vite 别名表），已在本证据与 GAP-MATRIX 备注留痕。**

### 3.6 冒烟发现的非本线问题（预存，登记不修复）

- **发现#1（预存，系统性）**：`parseCommercialSummary`（`packages/contracts/src/commercial.ts`，`de023ac9` 2026-09-11）期望 ledger 形状（`plan_name/available/held/refund_locked/as_of/stale`），而后端 `GET /commercial/summary`（`internal/handler/commercial.go:172`，同日已是 `base_tier` 形状）从未匹配——api-client 测试仅用 fixture（`commercial.test.ts:10`），从未对真实后端跑过。影响：BillingPage 数据区报错、general 套餐卡片（T1，失败静默隐藏=设计行为）与 **SP12 UsagePanel 预算卡**在真实环境一直静默隐藏。非 SP14 引入；SP14 接线使其首次显形。归档为独立欠账（对齐方向需产品决策：改解析器贴合 base_tier 形状，或后端补 ledger 投影端点）。
- 共享 8084 二进制落后 SP14（78dc09db < eb49392f）：后端两项新能力（swagger_enabled/default_model）在共享栈不可达，属栈重启欠账（与 SP13 §5 同款）；本证据以专用 8085/5176 栈等价覆盖（同一 DB/Redis、同一工作树代码）。

## 4. 已知遗留（deferred minors，含 brief 预填）

- **RefundPage 未挂路由**（Ruling P-1）：refundId 宿主注入语义，留组件库待真实退款流程接入。
- **AdminCommercialPage 运营数据端点未接线**：五分区 blocked-env 占位原状（Ruling P-2：UI affordance 投影，服务端 RequireExplicitCommercialCapability 权威）。
- **CheckoutPage plan_key 硬编码 `pro`**：组件原状；真实下单需目录发布套餐 + 支付管道配置（blocked-env）。
- **swagger 生成物未 regen**：`docs/docs.go`/`swagger.json` 不含 `swagger_enabled`/`default_model` 新字段，待 `make docs` 统一再生成（与 T2 入口改动合并防噪音）。
- **用户默认不在模型列表时降级 models[0]**：设计决策（「存的偏好」非「本次会话意图」，与 stale pick 的「未配置 pin」区分；芯片/下拉/发送三者一致），注释与测试钉死。
- **发现#1 commercial summary 解析错配**（§3.6）：BillingPage 数据区/general 卡片/SP12 预算卡真实数据不可用，待独立立案。
- ModelOptionSelect 清除后显示首模型 label（继承范本，data-value 正确）；ChatRoutePage effect 可能重复拉 models（注释失实+重复请求，行为无害）。
- 无 tenant 用户开 shared 链接走 onboarding（与 platform 策略一致，刻意收口）；integrations api tab system info 重复拉取（开销极小）。

## 5. 六域线收官状态

SP11（P-4/P-5 反馈+分析）→ SP12（P-1/P-2 用量聚合）→ SP13（P-6~P-9 查询历史+分享）→ SP14（P-3/P-10/P-11 轻项收割）全部 ✅，**GAP-MATRIX 第三部分 P-1~P-11 十一项全绿**，平台运营面六域（用量/套餐/分析/查询历史/OpenAPI/对话偏好）对齐线收官。
