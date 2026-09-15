# Vue / React public-route runtime evidence — 2026-09-15

本轮只采集 artifacts 和运行时证据；没有修改业务代码、没有登录、没有发送 mutation，也没有使用 mock。采集脚本与结果位于 [`artifacts/browser-evidence-20260915`](./browser-evidence-20260915)。

## 可复现命令

```sh
EVIDENCE_OUTPUT_DIR="$PWD/artifacts/browser-evidence-20260915" \
  pnpm --dir apps/web exec node \
  ../../artifacts/browser-evidence-20260915/collect-public-route-evidence.mjs
```

固定条件：Vue `http://127.0.0.1:5180`、React `http://127.0.0.1:5181`、`zh-CN`、Chromium、viewport `1440x900`。结果文件的 `capturedAt` 为 `2026-09-15T14:15:29.737Z`。逐项差异见 [`vue-react-public-route-differences-20260915.md`](./vue-react-public-route-differences-20260915.md)。

## 结果摘要

| 检查 | Vue :5180 | React :5181 | 证据含义 |
|---|---:|---:|---|
| `/login` HTTP / 首屏 | 200 / 登录 | 200 / 登录 | 公开 shell 与渲染可达 |
| `/register` HTTP / 首屏 | 200 / 登录 | 200 / 创建账户 | 公开路由行为有差异；Vue 首个 heading 仍为“登录” |
| `/onboarding/workspace` | 200 → `/login` | 200 → `/login` | 未认证 onboarding 被登录守卫接管 |
| 未认证 `/platform/apps` | 200 → `/login` | 200 → `/login?next=%2Fplatform%2Fapps` | 两端均保护路由；query 保留策略不同 |
| `POST /api/v1/auth/auto-setup` | 403 | 404 | 认证初始化 API 状态不同 |
| `GET /api/v1/auth/config` | 200 | 404 | 认证配置 API 状态不同 |
| `GET /api/v1/auth/oidc/config` | 200 | 404 | OIDC 配置 API 状态不同 |

HTTP 200 仅说明 Vite 文档/导航请求完成，不代表业务成功。认证 API 状态来自浏览器真实 response 监听；未发送凭据，因此没有登录成功或 authenticated parity 结论。

## login input computed-style 差异

脚本选择 login 页面首个 text/email input，并同时保存 `loginInputComputedStyle` 与通用 `styles`。

| 属性 | Vue :5180 | React :5181 |
|---|---|---|
| color | `rgba(0, 0, 0, 0.9)` | `rgb(26, 26, 26)` |
| background | `rgba(0, 0, 0, 0)` | `rgba(0, 0, 0, 0)` |
| font-family | `-apple-system, "system-ui", Segoe UI, Roboto, ...` | `"PingFang SC", "Helvetica Neue", Helvetica, Arial, ...` |
| font-size / line-height | `15px / 24px` | `15px / 24px` |
| border | `0px none rgba(0, 0, 0, 0.9)` | `0px solid rgb(220, 220, 220)` |
| border-radius | `0px` | `0px` |
| padding | `0px` | `1px 0px` |
| content box | `374×24` | `400×24` |

截图：[Vue login](./browser-evidence-20260915/vue-login-1440x900.png)、[React login](./browser-evidence-20260915/react-login-1440x900.png)。两张截图均为本轮固定视口 fresh capture；它们用于显示/定位差异，不单独构成 parity 验收。

## 证据层级与边界

| 层级 | 状态 | 说明 |
|---|---|---|
| 进程/HTTP 可达性 | PASS | 两个 Vite 端口均在本轮运行前监听，公开路由返回 200 |
| 浏览器 DOM/截图 | PASS（公开页面局部） | Chromium、固定 viewport、真实页面渲染与 computed style |
| 匿名认证 API | PASS（状态观测） | 记录了真实请求的 200/403/404；不等同于认证业务成功 |
| 匿名保护路由守卫 | PARTIAL | `/platform/apps` 均回到 login；未证明登录后 `next` 完成导航 |
| authenticated route/state parity | BLOCKED-ENV / 未验证 | 无凭据、租户、权限矩阵和后端 fixture |
| full Vue→React parity | NOT CLAIMED | 公开页面不是全量 parity；未覆盖已认证页面、loading/empty/error/forbidden/editing/submitting/success/failure、真实 mutation、Wails、iOS、Android |

浏览器 console error 与 API 失败响应原样保存在 `results.json` 的每个 target 下，便于复核。脚本不把公开页面或构建状态提升为全量 parity 结论。
