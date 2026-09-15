# Vue / React 公共路由与登录逐项差异 — 2026-09-15

本文件是 fresh Playwright 运行的差异索引，原始数据位于 [`browser-evidence-20260915/results.json`](./browser-evidence-20260915/results.json)，采集时间为 `2026-09-15T14:15:29.737Z`。运行条件为 Chromium、`zh-CN`、`1440x900`，Vue `:5180`、React `:5181`；没有使用凭据、mock 或 mutation。

## 逐项结果

| ID | 检查 | Vue :5180 | React :5181 | 判定 | 证据/路径 |
|---|---|---|---|---|---|
| R-01 | `/login` 首屏 | HTTP 200；最终 URL `/login`；heading `登录` | HTTP 200；最终 URL `/login`；heading `登录` | 对齐（公开可达） | `results.json` targets.vue/react.publicRoutes；登录截图仅作定位 |
| R-02 | `/register` 首屏 | HTTP 200；最终 URL `/register`；首个 heading `登录` | HTTP 200；最终 URL `/register`；首个 heading `创建账户` | 明确差异 | `results.json` targets.*.publicRoutes[1]；路径 `/register` |
| R-03 | `/onboarding/workspace` 匿名访问 | HTTP 200；最终 URL `/login`；heading `登录` | HTTP 200；最终 URL `/login`；heading `登录` | 路由结果对齐 | `results.json` targets.*.publicRoutes[2]；未证明登录后 onboarding |
| R-04 | `/platform/apps` 匿名保护 | HTTP 200；最终 URL `/login` | HTTP 200；最终 URL `/login?next=%2Fplatform%2Fapps` | 明确差异 | `results.json` targets.*.unauthenticatedProtectedRoute；React 保留 `next` |
| R-05 | `POST /api/v1/auth/auto-setup` | 403 | 404 | 明确差异/需契约核对 | `results.json` targets.*.authenticationAPIResponses；均为匿名真实浏览器响应 |
| R-06 | `GET /api/v1/auth/config` | 200 | 404 | 明确差异/需契约核对 | `results.json` targets.*.authenticationAPIResponses；不等于认证成功 |
| R-07 | `GET /api/v1/auth/oidc/config` | 200 | 404 | 明确差异/需契约核对 | `results.json` targets.*.authenticationAPIResponses；不等于 OIDC 登录验收 |
| V-01 | 登录邮箱输入框 | 内容宽 `374px`；`border: 0px none`；`padding: 0px`；系统字体栈 | 内容宽 `374px`；`border: 0px solid`；`padding: 1px 0px`；PingFang/Helvetica 栈 | 明确低风险视觉定位差异 | `results.json` targets.*.publicRoutes[0].snapshot.loginInputComputedStyle |
| V-02 | 登录标题/语言按钮计算样式 | heading `line-height: normal`；语言按钮 `106.61×36px` | heading `line-height: 32px`；语言按钮 `104.61×38px` | 明确低风险视觉定位差异 | `results.json` targets.*.publicRoutes[0].snapshot.styles；截图不作为完整验收 |

## 可直接进入修复排查的路径

1. `/register`：核对 React route entry 与 register view 的首屏 heading、表单状态和返回登录交互是否遵循 Vue。当前证据只覆盖匿名首屏，不覆盖提交、校验、失败或成功。
2. `/platform/apps`：核对 React 未认证 guard 是否应保留 `next`，并与 Vue 的登录后回跳约定对齐。仅凭 URL 差异不能判断哪一方是目标行为。
3. `/api/v1/auth/config`、`/api/v1/auth/oidc/config`、`/api/v1/auth/auto-setup`：核对 React dev proxy/API client 是否与 Vue 使用同一后端路径。当前 React 的 404 可能是代理或运行时契约差异，不能直接归因于页面代码。
4. 登录输入框与语言按钮：优先检查共享 theme/reset/base-control 样式；这是低风险、可复现的定位项，但必须补 DOM 交互与多状态证据后才能判定 parity。

## 证据边界

- PASS 仅表示本轮 Playwright 观察到的公开路由加载、匿名 redirect、响应状态和 computed style。
- 未执行真实登录；没有可用于本轮的凭据/租户/权限矩阵，因此不报告 authenticated parity。
- 未覆盖 loading、empty、error、forbidden、editing、submitting、success、failure、真实 mutation、Wails、iOS、Android。
- 截图 [`vue-login-1440x900.png`](./browser-evidence-20260915/vue-login-1440x900.png) 与 [`react-login-1440x900.png`](./browser-evidence-20260915/react-login-1440x900.png) 只用于视觉差异定位，不能单独构成完整验收。
- 主采集脚本为 [`collect-public-route-evidence.mjs`](./browser-evidence-20260915/collect-public-route-evidence.mjs)，可用仓库主证据文档中的命令复现。
