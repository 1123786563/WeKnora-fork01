# SP14 轻项收割（Onyx 平台运营面对齐收官）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 套餐四页挂路由+升级入口、API key 文档入口（含两处计划外 bug 修复：悬空 /docs 链接、platform-api-keys 分区不可达）、per-user 默认模型偏好+解析链（GAP-MATRIX P-3/P-10/P-11）。

**Architecture:** 后端仅三处小改（system/info 加 swagger_enabled、UserPreferences 加 default_model、PUT 偏好请求扩展）；模型解析链走纯前端（resolveChatModelChip 插用户默认层）；四页挂 platformRoute 子路由照 analyticsRoute 模式，AdminCommercialPage 的 operator capability 用 isSystemAdmin 投影（后端仍权威）。

**Tech Stack:** Go（gin+GORM）、React 19 + TanStack Router、node:test。

**Spec:** `docs/superpowers/specs/2026-09-19-onyx-platform-parity-design.md` 第 4 节 SP14 部分；GAP-MATRIX P-3/P-10/P-11。

## Global Constraints

- 成功 `gin.H{"success":true,"data":...}`；错误 `c.Error(errors.NewXxx)`。
- 无新迁移（UserPreferences 是 JSONB 单列、system/info 是响应字段）。
- 新路由经 `g.apiKeyGroup`（若涉及）；billing 路由的授权后端已有（RequireExplicitCommercialCapability 等），前端只做 affordance。
- 前端测试 node:test；commit 前 porcelain 核对显式路径 + `git diff --cached --stat` 确认（**controller 42aab87d 教训**），绝不 add -A。

---

### Task 1: 套餐接线（路由三页 + general 套餐卡片 + 升级入口）

**Files:**
- Modify: `apps/web/src/router.tsx`（billingRoute/checkoutRoute/billingAdminRoute 三个 lazy+createRoute+addChildren，照 analyticsRoute 模式；`scopeController` 从 deps 取——grep 它在 router 里的现有传递）
- Modify: `apps/web/src/App.tsx`（无需动——re-export 已有；若 router lazy import 路径直连 commercial/ 目录则跳过）
- Modify: `apps/web/src/settings/GeneralPreferencesPanel.tsx`（加 `client` prop + 顶部套餐卡片：`client.commercial.summary()` 显示 plan_name/paid_until/可用额度 + "升级/续费"按钮 navigate 到 `/platform/billing/checkout`（orderId 空串新建）；summary 失败静默隐藏整卡——照 UsagePanel 预算卡模式）
- Modify: `apps/web/src/settings/SettingsPage.tsx`（general 分支传 client）
- Modify: i18n 五 locale（billing 相关键）
- Test: 纯函数（卡片格式化）+ SettingsPage 既有测试不破

路由设计：
- `/platform/billing` → BillingPage（client+scopeController）
- `/platform/billing/checkout` → CheckoutPage（orderId=URL query `order` 或空串）
- `/platform/billing/admin` → AdminCommercialPage（`capability={operator: scopeRuntime.isSystemAdmin()}`——UI affordance，无列表数据时页面自身 blocked-env 提示已有）
- RefundPage 不单独挂路由（refundId 宿主注入语义，由 billing 页列表链接带 query 进 checkout 同款模式或本期不挂——**裁定：本期不挂**，RefundPage 留组件库待真实退款流程接入，报告写明）

### Task 2: API key 文档入口 + 两处 bug 修复

**Files:**
- Modify: `internal/handler/system.go`（GetSystemInfoResponse 加 `SwaggerEnabled bool \`json:"swagger_enabled"\``= `gin.Mode() != gin.ReleaseMode`）+ 测试
- Modify: `packages/api-client/src/settings/index.ts`（SystemInfo 契约加 swagger_enabled 可选字段）
- Modify: `packages/views/src/integrations/page.tsx`（**修悬空链接**："OpenAPI /docs" → `/swagger/index.html`；据 swagger_enabled 条件渲染——SystemInfo 从哪拿？grep integrations 页有无 system info 调用，无则 api-client settings.systemInfo 方法补）
- Modify: `apps/web/src/router.tsx`（**修 platform-api-keys 不可达**：SettingsPage 的 role prop 在 `scopeRuntime.isSystemAdmin()` 时传 `'system-admin'`）
- Modify: `apps/web/src/settings/PlatformApiKeysPanel.tsx`（header 右侧加"API 文档"外链按钮 `/swagger/index.html` 同条件渲染 + capabilities 说明文案一行（每个能力的语义简述，五 locale COPY 扩展））
- Test: system.go 字段测试 + 前端纯函数（条件渲染判定）

### Task 3: 偏好后端（default_model 字段四处）

**Files:**
- Modify: `internal/types/user.go`（UserPreferences 加 `DefaultModel *string \`json:"default_model,omitempty"\``，照 BrowserSearchInstructions 风格）
- Modify: `internal/handler/auth.go`（updateMyPreferencesRequest 加 DefaultModel *string + patch 映射）
- Modify: `internal/application/service/user.go`（UpdateUserPreferences 合并分支：非 nil 覆盖、空串=清除置 nil——照 LastActiveTenantID 的 *0 sentinel 风格）
- Test: service 合并测试（设/改/清除/不影响他字段）

### Task 4: 偏好前端 + 模型解析链

**Files:**
- Create: `apps/web/src/settings/ChatPreferencesPanel.tsx`（默认模型行：ModelOptionSelect 复用照 PersonalMemoryPanel.tsx:70 范本——模型列表自拉 `client.configuration.models.list()` 过滤 KnowledgeQA 型、addModel 跳 models 分区；读写 `client.settings.preferences` get/update({default_model})；清除选项→update 空串）
- Modify: registry.ts（`{ key: 'chat-preferences', viewId: 'ChatPreferences', apiDomain: 'preferences', scope: 'user', minRole: 'viewer', operations: ['read','save'], ported: true }`）+ surface.ts + SettingsPage.tsx 分支
- Modify: `apps/web/src/chat/model-chip.ts`（resolveChatModelChip 插层：pick>agent>**userDefault（新参数）**>models[0]；userDefault 由调用方传入）+ `apps/web/src/chat/ChatRoutePage.tsx`（从 client.settings.preferences.get() 拉默认模型传给 chip 解析——拉失败/未设则维持现状；注意缓存避免每渲染拉取：useState+useEffect 一次）
- Test: model-chip 解析优先级纯函数测试（含新层）；ChatPreferencesPanel 纯函数
- i18n 五 locale

### Task 5: 回归、冒烟、证据与收官登记

- 后端/前端回归（本线全绿+预存判归属）
- 冒烟（共享栈）：/platform/billing 三页渲染、general 套餐卡片、integrations 文档链接正确、platform-api-keys 分区可达（system-admin 账号）、chat-preferences 设默认后新会话模型生效、清除后回退
- evidence `docs/migrations/react/evidence/2026-09-20-sp14-lightweight.md` + ROADMAP SP14 ✅ + GAP-MATRIX P-3/P-10/P-11 ✅ + **六域线收官声明**（P-1~P-11 全 ✅）
- 遗留预填：RefundPage 未挂路由（待真实退款流程）；AdminCommercialPage 运营数据端点未接线（blocked-env 原状）；CheckoutPage plan_key 硬编码 pro（组件原状）

---

## Self-Review 记录

- Spec 覆盖：P-3→Task 1；P-10→Task 2（含两处计划外 bug）；P-11→Task 3/4 ✓
- 一致性：default_model 的 snake_case JSON 在 Go/contracts/settings api 间对齐；resolveChatModelChip 新参数向后兼容（可选）
- 执行期确认点：scopeController 在 router deps 的取法、integrations 页 systemInfo 获取通道、models.list 的 KnowledgeQA 过滤字段名
- Ruling：RefundPage 本期不挂路由；AdminCommercial capability=isSystemAdmin 投影；模型解析链纯前端方案 A
