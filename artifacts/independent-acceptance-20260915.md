# React multi-client independent acceptance audit — 2026-09-15

工作目录：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`

## 结论

本轮不通过全量 parity acceptance。路由和组件入口已有较完整的静态映射，但仍存在认证后页面、真实 mutation/权限负例、特殊文件/WS、Wails/native 的未闭环证据；React Web 第二次 fresh 全量测试为 `1067 tests: 1063 passed, 4 failed`，所以不能把 build、截图或局部测试写成整体通过。

## 入口盘点

`docs/migrations/react/route-parity.csv` 当前 59 行数据，按 kind 为：page 18、settings-section 26、redirect 5、layout+redirect 1、dev-only 1、independent-entry 1、special-file-route 5、special-websocket 1、special-embed-route 1。Vue SPA 权威入口是 `frontend/src/router/index.ts`；React resolver 是 `apps/web/src/routes.tsx`。

已覆盖的入口类别：认证/引导、平台 shell、知识库列表与详情、Wiki/FAQ/Graph、Agent、组织、Chat、Settings、Embed、文件代理和 sandbox WS 均有 route/source 或 shared-view 静态映射。React 另保留兼容别名 `/knowledgeBase/:id/{documents,wiki,faq,settings}`、`/platform/chat`、`/creatChat` 以及 React-only `/platform/configuration`、`/platform/administration`；这些不是新增 Vue 路由，不能重复计数。

发现的追溯缺口：CSV 第 21–24 行的 `/platform/apps*` 四行分别引用 `frontend/src/views/apps/AppsView.vue`、`ConnectionsView.vue`、`AuthorizationView.vue`、`ActionView.vue`，但四个文件在当前 worktree 均不存在。`apps/web/src/apps/AppsPages.tsx`、`model.ts` 与 focused test 存在。应在后续审查中确认 Vue app surface 的真实历史/上游来源，再更新 authority 字段；本轮不改业务代码。2026-09-16 当前快照审计进一步确认 CSV 为 59 条数据行、没有显式 `row_id`，详见 `artifacts/route-inventory-parity-ledger-audit-20260916.md`。

## 当前未覆盖/未闭环清单

以下是“仍不能称为 parity accepted”的入口或状态，不代表 React 没有页面代码：

| 范围 | 当前缺口 | 证据状态 |
|---|---|---|
| `/`, `/join`, `/knowledgeBase`、platform redirects/aliases | 需同账号、同租户、刷新/回退/邀请 token 的浏览器行为闭环 | static/unit + 局部 browser；完整受保护 browser 未闭环 |
| `/platform`、KB list/detail、Agent、Organizations、Chat | loading/empty/error/forbidden/edit/submitting/success/failure 的同条件页面与交互对照仍不全 | static/unit/mock + 局部 browser；real backend/browser review |
| `/platform/settings` 与 26 个 settings rows | system-admin、viewer/admin/owner、capability unavailable、保存失败/冲突/secret redaction 及窄屏全矩阵未闭环；registry 中 models/members/mcp/sandbox/skills/system sections 仍是重点 | static/unit/mock；real backend/browser/Wails review |
| `/platform/integrations`、apps catalog/connections/authorization/action | provider/API key/SSE、授权轮询、动作审批成功/失败/权限负例以及 Vue app source authority 未闭环 | static/unit；authenticated browser/backend blocked or review |
| `embed.html` 与 `/api/v1/embed/:channel_id/files` | publish-token、allowed-origin、postMessage、受保护资源预览/过期/拒绝 | static/unit + limited browser；real visitor/backend blocked |
| `/files`、`/r/:token`、presigned、KB/message file proxy | 签名/过期、跨租户 403、真实下载 bytes/filename 与预览失败 | static/unit；fresh real permission evidence absent |
| sandbox terminal WS | ticket、input、resize、cancel、切租户后的旧连接隔离 | static/unit；real WS/backend evidence open |
| Wails / iOS / Android | 有历史或局部 build/launch/设备记录，但不是所有 route/state/真实链路的逐功能验收 | partial native/build；global acceptance open |

## Fresh verification record

| 检查 | 结果 | 分类与边界 |
|---|---|---|
| React route focused test | 12/12 pass | unit/static；只覆盖 resolver/guard，不证明页面 parity |
| React Web full suite, run 1 | 1062/1063 pass, 1 failed in `src/apps/AppsPages.test.tsx` | full suite red；focused AppsPages rerun 2/2 pass |
| React Web full suite, run 2 | 1067 total, 1063 pass, 4 failed | fresh full-suite red；Document detail 2、FAQ 2：header download/metadata、failed-save draft、FAQ example menu、tag clear affordance |
| `pnpm typecheck:web` | exit 0 | static/typecheck |
| `pnpm --dir frontend type-check` | exit 0 | Vue static/typecheck |
| `pnpm build:web` | exit 0 | build only；不等于 route/state/backend acceptance |
| public browser run 4 | Vue/React `/login` HTTP 200；两端 `/platform/apps` 未认证均回登录 | browser/public guard only；Vue auto-setup 403，React auth config/oidc endpoints 404 |

浏览器产物：`artifacts/browser-evidence-20260915-run4/results.json`、`vue-login-public.png`、`react-login-public.png`。该 run 使用 `1355x776`、`zh-CN`，没有 credentials、mock response、localStorage token 或写 API。

## 证据分层

- `static`：源码、route CSV、matrix、registry、配置和文件存在性检查。
- `unit`：React route focused test 与各模块测试；`mock` 仅说明测试 fixture，不是后端行为。
- `real backend`：本轮仅确认端口 `:8080` 可达；公开浏览器没有认证会话，因此没有业务成功、租户隔离或 mutation 证据。
- `browser`：本轮仅为公开登录/未认证保护路由，以及 DOM/computed-style/截图；截图用于发现差异，不能作为对齐验收。
- `Wails/native`：本轮未新增逐功能证据；历史 build/launch/设备记录按 partial 使用，不能覆盖全部 route/state。

## 保留的工作树风险

初始审计时存在大量他人 dirty changes；检查期间 `apps/web/src/apps/AppsPages.tsx` 后来显示为 deleted，且全量测试同时出现不同的失败集合。该变化未由本轮恢复或修改；后续应先确认并保留责任人的工作状态，再进行修复/复验。
