# React main 与 Vue 路由/隐藏入口/历史 URL/设置子项清单

审查日期：2026-09-15（静态源码审查）  
工作目录：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`

## 范围与证据边界

本清单只审查 React Web 主入口及其路由分派、Vue Router 与设置抽屉的入口契约；不修改业务代码、Vue 代码或移动端代码。证据来自以下源码和已有测试：

- React：`apps/web/src/main.tsx`、`apps/web/src/routes.tsx`、`apps/web/src/routes.test.ts`、`apps/web/src/settings/SettingsPage.tsx`、`apps/web/src/settings/ModelSettingsPanel.tsx`。
- Vue：`frontend/src/router/index.ts`、`frontend/src/views/settings/Settings.vue`、`frontend/src/views/settings/ModelSettings.vue`、相关知识库/集成入口调用点。
- 共享设置/集成契约：`packages/views/src/settings/registry.ts`、`packages/views/src/integrations/settings-route.ts`、`packages/views/src/integrations/registry.ts`。

这是静态证据，不等同于浏览器、Wails、后端权限或移动端验收。

## 1. 入口与路由分派

| 入口/状态 | Vue 基线 | React 当前实现 | 结论 |
|---|---|---|---|
| 根路径 | `/` redirect 到 `/platform/knowledge-bases`（`frontend/src/router/index.ts:49-52`） | `routeRedirect('/')` 到同一目标（`apps/web/src/routes.tsx:76-79`） | 对齐 |
| 登录/注册 | `/login`、`/register` 共用 Login 组件（`frontend/src/router/index.ts:54-69`） | `RouteMatch.kind=login`，`main.tsx` 渲染 Login/注册模式（`apps/web/src/routes.tsx:36-37`、`apps/web/src/main.tsx:281-333`） | 对齐 |
| 无租户 onboarding | `/onboarding/workspace`，允许认证用户但跳过 tenant gate（`frontend/src/router/index.ts:72-75`、`335-348`） | 同路径 guard 与独立渲染（`apps/web/src/routes.tsx:154-156`、`apps/web/src/main.tsx:226-228`） | 对齐 |
| 组织邀请 | `/join?code=` 转 `/platform/organizations?invite_code=`（`frontend/src/router/index.ts:78-88`） | `routeRedirect` 保留同一 query 转换（`apps/web/src/routes.tsx:83-87`） | 对齐 |
| 知识库历史入口 | `/knowledgeBase`、`/knowledgeBase/:kbId` 及 platform 变体 | `resolveRoute` 支持 `/knowledgeBase`、`/knowledgeBase/:id`、`/platform/knowledge-bases/:id`，并读取 `tab`、`slug`、`knowledge_id`（`apps/web/src/routes.tsx:47-67`） | 对齐；query 状态由页面消费 |
| 文档/wiki/FAQ/知识库设置 | Vue 由知识库页面及内部状态承载；历史 URL 仍可进入 | React 显式分派 document、wiki、FAQ、knowledge settings（`apps/web/src/routes.tsx:47-63`、`apps/web/src/main.tsx:234-241`） | 对齐 |
| Chat | Vue `creatChat`、KB `creatChat`、`chat/:chatid`（`frontend/src/router/index.ts:159-175`） | React 支持 KB `creatChat`、`/platform/creatChat`、`/platform/chat/:id`，并将 `/platform/chat`、`/creatChat` 作为兼容入口（`apps/web/src/routes.tsx:64-72`、`apps/web/src/main.tsx:264-265`） | 对齐并扩展历史兼容 |
| Embed | Vue 主 SPA 明确不承载 Embed，使用 `embed-main.ts`（`frontend/src/router/index.ts:59`、`frontend/src/embed-main.ts:1-26`） | React 主入口对 `/embed/*` 只显示“必须使用独立入口”，独立 `apps/embed/src/main.tsx` 承载真实 Embed | 入口边界正确；不可把 `/embed/*` 当普通受保护页面 |
| Apps | 当前 Vue Router 清单没有对应 platform apps 子路由 | React 支持 catalog/connections/authorization/action（`apps/web/src/routes.tsx:40-45`、`apps/web/src/main.tsx:256-257`） | React 额外能力；需保持为独立 route kind |

## 2. 历史 URL、隐藏/兼容入口与守卫

| URL | React 解析/行为 | Vue 基线/预期 | 证据结论 |
|---|---|---|---|
| `/platform` | 认证、租户检查后跳 `/platform/knowledge-bases` | Router redirect（`frontend/src/router/index.ts:97-101`） | 对齐 |
| `/platform/knowledge-search?q=x` | 转 `/platform/knowledge-bases?cmdk=x`（`apps/web/src/routes.tsx:79-81`） | Vue 同样把 `q` 转为 `cmdk`（`frontend/src/router/index.ts:126-134`） | 对齐 |
| `/platform/tenant` | 转 `/platform/settings`（`apps/web/src/routes.tsx:89`） | Vue child redirect（`frontend/src/router/index.ts:103-106`） | 对齐 |
| `/platform/integrations` | 保留 `tab`/`section`/`agentId` 等 query，规范化为 `/platform/settings?section=integration-<tab>`（`apps/web/src/routes.tsx:88`；共享实现 `packages/views/src/integrations/settings-route.ts:18-35`） | Vue 同一 redirect 与 `normalizeSettingsSection`（`frontend/src/router/index.ts:143-157`） | 对齐；集成内容由 Settings 内嵌渲染 |
| `/platform/administration` | 转 `/platform/settings?section=members`（`apps/web/src/routes.tsx:90`） | 当前 Vue Router 无同名 child，历史入口由 React 兼容层保留 | React 兼容入口，不能误报为 Vue 页面 |
| `/platform/system`、`/system/settings`、`/system/admins` | system-admin 通过后转 `section=system-global`；非 system-admin 回知识库（`apps/web/src/routes.tsx:91`、`apps/web/src/routes.tsx:171-175`） | Vue 同样是 system-admin 兼容 redirect（`frontend/src/router/index.ts:183-208`） | 对齐 |
| `/platform/system/queues` | system-admin 通过后转 `section=runtime-queues` | Vue 同一目标（`frontend/src/router/index.ts:205-208`） | 对齐 |
| `/platform/dev/markdown` | 仅 development 加入 route；生产解析为 not-found（`apps/web/src/routes.tsx:68`、`98-103`） | Vue 同样通过 `import.meta.env.DEV` 添加（`frontend/src/router/index.ts:212-218`） | 对齐 |
| `/platform/*` 未知路径 | 受保护前缀下先认证，再显示 not-found；普通未知路径允许公共 not-found（`apps/web/src/routes.tsx:164-169`） | Vue 未知 route 交由 Router fallback/应用层 | React 有明确 guard 证据；仍需浏览器确认最终文案 |

已有 `apps/web/src/routes.test.ts` 覆盖了上述历史路径、认证/租户/system-admin 分支、集成 query 保留及未知路径（例如 `:12-63`、`:107-163`、`:165-229`）。

## 3. 设置 section 与子项

### 3.1 Vue 导航基线

Vue `Settings.vue:359-402` 的 nav item 集合为：

`general`、`ollama`、`weknoracloud`、`models`、`websearch`、`chathistory`、`memory`、`vectorstore`、`parser`、`storage`、`sandbox`、`skills`、`mcp`、`system`、`system-global`、`runtime-queues`、`platform-api-keys`、`system-audit-log`、`userprofile`、`mymemory`、`envvars`、`tenant`、`members`，以及 `INTEGRATION_PREVIEW_ITEMS` 生成的 `integration-*` 项。导航按 account、workspace、models_runtime、integrations、data_extensions、system_administration、platform 分组（`frontend/src/views/settings/Settings.vue:404-456`）。

Vue 入口可由 route query 或 `uiStore.openSettings(section, subSection)` 触发；设置子菜单点击会把 `currentSection` 设为父项，把 `currentSubSection` 设为子项，并滚动到 `[data-model-type="..."]`（`frontend/src/views/settings/Settings.vue:479-490`）。模型类型是 `chat`、`embedding`、`rerank`、`vllm`、`asr`（`frontend/src/views/settings/ModelSettings.vue:307-320`）。知识库编辑器及资源设置会调用 `openSettings('models', 'vllm'|'asr'|'knowledgeqa')`、`openSettings('storage')`、`openSettings('parser')`、`openSettings('vectorstore')` 等，证据见相关 Vue 文件调用点。

### 3.2 React 对照

- React registry 在 `packages/views/src/settings/registry.ts:25-47` 注册基础 section、system-admin section 与 `integration-*`；`SettingsPage.tsx:315-323` 使用同一分组顺序。
- React 明确把 `retrieval` 放在 registry/section state 中，但通过 `NAV_HIDDEN_SECTIONS = new Set(['retrieval'])` 隐藏导航（`apps/web/src/settings/SettingsPage.tsx:325-329`）。这与 Vue 当前 navItems 不展示 retrieval 一致；它仍可作为深链 section 被读取和渲染，属于“隐藏但可达”的历史/内部入口。
- React 对 `models` 的 `subsection` query 做了兼容：`SettingsPage.tsx:80-86` 读取，`ModelSettingsPanel.tsx:102-113` 将 `chat|embedding|rerank|vllm|asr` 映射到类型过滤器；对应测试为 `apps/web/src/settings/model-settings.test.ts:50-57` 与 `apps/web/src/settings/SettingsPage.test.tsx:285-...`。
- React 选择 section 时以 `history.pushState` 写入 `?section=...`，集成能力不满足时以 `history.replaceState` 回退到 general，并清除非集成 section 的 `agentId`（`apps/web/src/settings/SettingsPage.tsx:129-140`、`174-178`；`packages/views/src/integrations/settings-route.ts:26-35`）。关闭设置时普通 section 使用 `history.back()`，system-admin section 回知识库（`SettingsPage.tsx:158-167`）。

## 4. 审查发现与后续验收边界

1. **P1：React dispatch 依赖启动时 route 快照。** `main.tsx:83` 只在模块初始化时调用一次 `resolveRoute`；`renderProtected` 后续仍使用该 `route`（`main.tsx:201-274`）。当前设计依赖 legacy route 通过 full navigation/reload 完成切换，并对 settings 的 `pushState` 由 SettingsPage 自己维护。若未来新增不 reload 的主页面 history transition，必须同步更新 route state，否则可能出现 URL 与渲染树不一致。本次不改代码，仅记录为验收约束。
2. **已关闭（实现层）：`knowledgeqa` 设置子项语义。** `models` 的五类子项可通过 `?section=models&subsection=<type>` 深链进入；Vue 的 `uiStore.settingsInitialSubSection` 触发的已知 `openSettings('models', 'knowledgeqa')` 调用已在 React 映射到 conversation/chat 模型子项，并由 `apps/web/src/settings/model-settings.test.ts` 覆盖。若未来新增非 URL 的 `openSettings(section, subSection)` 调用，仍需逐个补充对应 contract；本条不再作为当前 P1 阻塞。
3. **P2：`/platform/administration` 是 React 额外历史兼容入口。** 它不是当前 Vue Router 的显式 child，但 React 将其映射到 members。该行为已有测试（`apps/web/src/routes.test.ts:51-52`、`:158-162`），应在 parity ledger 中标为 compatibility，而不是要求 Vue 新增同名路由。
4. **P2：Embed 是双入口契约。** `apps/web/src/main.tsx` 对 `/embed/*` 的提示页不能被视为 Embed 功能缺失；真实 Embed 必须从 `apps/embed` 的独立构建入口验证。主 SPA 路由审查只证明隔离边界。

## 5. 2026-09-16 路由参数安全复核

- 对照 Vue 动态入口和 React `resolveRoute()` 时发现，Apps 授权与动作路径的非法 percent-encoding（例如 `/platform/apps/authorization/%E0%A4%A`）此前会返回 `apps` route，但 `id` 为 `undefined`，可能把坏参数交给页面层。
- React 现已在 `apps/web/src/routes.tsx` 对授权和动作动态段复用解码失败的 `not-found` 分支；`apps/web/src/routes.test.ts` 增加两条回归断言。提交：`ab86e359`。
- 验证：`pnpm exec tsx --test apps/web/src/routes.test.ts`（13/13）；`pnpm exec tsx --test frontend/src/config/settingsRoute.test.ts frontend/src/config/settingsAccess.test.ts`（7/7）；`pnpm typecheck:web`（exit 0）；`pnpm --filter @weknora/web test`（1172/1172）；`git diff --check`（exit 0）。
- 本节是静态源码/单元/类型/完整 Web 回归证据；不等同于认证浏览器、真实后端权限/变更、Wails 原生运行或 Embed 宿主验收。

## 6. 静态检查记录

本文件提交前执行：

```text
git diff --check -- docs/migrations/react/route-inventory-20260915.md
pnpm exec tsx --test apps/web/src/routes.test.ts apps/web/src/settings/model-settings.test.ts
pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit
```

检查结果与 commit 输出应记录在交付消息中；检查仅覆盖静态/单元层，不宣称浏览器、后端、Wails、Embed 独立构建或移动端验收通过。

## 7. 2026-09-16 当前快照审计补充

`route-parity.csv` 当前包含 59 条数据行，kind 分布为 page 18、settings-section 26、redirect 5、layout+redirect 1、dev-only 1、independent-entry 1、special-file-route 5、special-websocket 1、special-embed-route 1。CSV 没有显式 `row_id` 列；早期 baseline 中的 53/56 行数和 R 编号属于历史快照，不能作为当前唯一分母。

当前静态存在性检查还发现四个 `/platform/apps*` 行引用的 Vue `frontend/src/views/apps/*.vue` 文件不存在，另有 `embed.html` 行把两个路径合并在一个字段中。React Apps 页面存在并不补足 Vue authority 追溯。详细命令、路径和影响见 `artifacts/route-inventory-parity-ledger-audit-20260916.md`。在 authority 解析和显式 ID contract 完成前，这些项保持 review/open，不标记为 parity accepted。
