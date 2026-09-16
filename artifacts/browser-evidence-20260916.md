# Vue / React 认证、匿名与设置 Portal 浏览器证据 — 2026-09-16

本轮只采集本地运行环境的浏览器证据和结果文件，没有修改业务代码，没有提交真实注册/登录或其他业务 mutation。既有工作树中的未提交改动已保留；本轮新增内容仅在 `artifacts/browser-evidence-20260916/`。

固定匿名条件：Chromium、`zh-CN`、Vue `http://127.0.0.1:5180`、React `http://127.0.0.1:5181`、后端 `http://127.0.0.1:8080`、`1440×900`。匿名结果见 [`anonymous-auth-results.json`](./artifacts/browser-evidence-20260916/anonymous-auth-results.json)，采集脚本见 [`collect-anonymous-auth-evidence.mjs`](./artifacts/browser-evidence-20260916/collect-anonymous-auth-evidence.mjs)。

## 结果摘要

| 场景 | Vue | React | 判定 |
|---|---|---|---|
| 匿名 `/login` | HTTP 200，最终 `/login`，首个 heading“登录” | HTTP 200，最终 `/login`，展示登录表单 | PASS：两端公开登录入口可渲染 |
| 匿名 `/register` | HTTP 200，URL 保留 `/register`，但首个 heading 仍为“登录”，展示“创建账户”按钮 | HTTP 200，URL 保留 `/register`，当前仍为登录表单并展示“创建账户”按钮 | PARTIAL：公开注册 URL 可达，但两端都未出现独立注册表单 |
| 匿名 `/platform/apps` | HTTP 200 → `/login` | HTTP 200 → `/login?next=%2Fplatform%2Fapps` | PASS：保护路由生效；query 保留策略不同 |
| 匿名 `/platform/settings`（设置 Portal 入口） | HTTP 200 → `/login` | HTTP 200 → `/login?next=%2Fplatform%2Fsettings` | PASS：设置入口受保护；Portal 未在匿名态渲染 |
| 匿名认证配置 | `auth/config` 200，`oidc/config` 200，`auto-setup` 403 | `auth/config` 200，`oidc/config` 200 | PASS/环境信号：配置可读；Vue auto-setup 被后端拒绝 |

HTTP 200 只代表 SPA 文档请求完成，不代表业务登录、注册或保护页面成功。

## 已有 session

使用浏览器现有 session 做了值脱敏探测：只记录 localStorage key 是否存在和 `GET /api/v1/auth/me` 状态，不记录 token 或身份字段。完整脱敏结果见 [`existing-session-results.json`](./artifacts/browser-evidence-20260916/existing-session-results.json)。该浏览器视口为 `1355×720`，Vue/React 两端一致。

| 场景 | 观测 | 判定 |
|---|---|---|
| session 存在性 | Vue 与 React 均有 `weknora_token`、refresh token、user、tenant、memberships；`auth/me` 均返回 200 | session/API 层 PASS |
| Vue `/platform/apps` | URL 保持目标路由，但页面 body 为空、无 heading | `blocked-env`：session API 可用但受保护 UI 空白 |
| Vue `/platform/settings` | URL 保持目标路由，但页面 body 为空、无 dialog | `blocked-env`：设置 Portal 未渲染 |
| React `/platform/apps` | 重放同一 session 后进入 `/onboarding/workspace`，提示仅支持邀请加入空间 | `blocked-env`：React 没有进入受保护 App 页面 |
| React `/platform/settings` | 同样进入 `/onboarding/workspace` | `blocked-env`：设置 Portal 未渲染 |

已有 session 截图：Vue [`apps`](./artifacts/browser-evidence-20260916/vue-existing-session-platform-apps.png)、Vue [`settings`](./artifacts/browser-evidence-20260916/vue-existing-session-platform-settings.png)、React [`apps`](./artifacts/browser-evidence-20260916/react-existing-session-platform-apps.png)、React [`settings`](./artifacts/browser-evidence-20260916/react-existing-session-platform-settings.png)。

## 匿名截图

每个目标均为 fresh anonymous context、`1440×900`、同一组路由；原始结果文件中的 `screenshot` 字段对应文件名。

- Vue：[`login`](./artifacts/browser-evidence-20260916/vue-rootlogin-1440x900.png)、[`register`](./artifacts/browser-evidence-20260916/vue-rootregister-1440x900.png)、[`apps guard`](./artifacts/browser-evidence-20260916/vue-rootplatform-apps-1440x900.png)、[`settings guard`](./artifacts/browser-evidence-20260916/vue-rootplatform-settings-1440x900.png)
- React：[`login`](./artifacts/browser-evidence-20260916/react-rootlogin-1440x900.png)、[`register`](./artifacts/browser-evidence-20260916/react-rootregister-1440x900.png)、[`apps guard`](./artifacts/browser-evidence-20260916/react-rootplatform-apps-1440x900.png)、[`settings guard`](./artifacts/browser-evidence-20260916/react-rootplatform-settings-1440x900.png)

## 证据边界

本轮可以确认：公开认证入口可达、匿名保护路由重定向、认证配置接口状态、已有 session 的存在性，以及已有 session 在两端的实际页面结果。

以下保持 `blocked-env`，没有提升为“已通过”：有效账号登录后的回跳、注册成功、authenticated `/platform/apps` 内容、设置 Portal 的打开/关闭与焦点行为、tenant/role/forbidden 变体、Wails、iOS、Android。阻塞原因是现有 session 在 Vue 受保护页面空白、在 React 进入 workspace onboarding，且没有额外创建账号/租户或修改后端 fixture 的授权。

## 可复现

```sh
EVIDENCE_OUTPUT_DIR="$PWD/artifacts/browser-evidence-20260916" \
  pnpm --dir apps/web exec node \
  ../../artifacts/browser-evidence-20260916/collect-anonymous-auth-evidence.mjs
```

运行前提：Vue dev server 在 `:5180`，React dev server 在 `:5181`，后端在 `:8080`。本轮实际监听并验证了这三个本地端口，其中认证配置请求来自浏览器 response 监听。
