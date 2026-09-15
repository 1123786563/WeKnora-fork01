# Vue / React protected-route Playwright evidence — 2026-09-15

本轮使用 Chromium Playwright 在 Vue `http://127.0.0.1:5180` 与 React `http://127.0.0.1:5181` 运行同一组匿名场景，locale 为 `zh-CN`，viewports 为 `1440x900`、`1024x768`、`390x844`。原始结果位于 [`protected-route-results.json`](./browser-evidence-20260915/protected-route-results.json)，采集时间为 `2026-09-15T14:47:41.981Z`。

## 场景矩阵

| 场景 | Vue | React | 判定 |
|---|---|---|---|
| 匿名 `/platform/apps` | HTTP 200，最终 `/login` | HTTP 200，最终 `/login?next=%2Fplatform%2Fapps` | 差异：React 保留 `next` |
| 匿名 `/platform/apps?tab=connections` | 最终 `/login` | 最终 `/login?next=%2Fplatform%2Fapps%3Ftab%3Dconnections` | 差异：React 保留完整 query |
| 匿名未知 `/platform/apps/not-a-route` | 最终 `/login` | 最终 `/login?next=%2Fplatform%2Fapps%2Fnot-a-route` | 同一 guard 分支的 query 差异 |
| 匿名 `/onboarding/workspace` | 最终 `/login` | 最终 `/login` | 对齐 |
| 无效匿名登录 | `POST /api/v1/auth/login` 401，显示 `Invalid email or password` | 同 | 对齐 |
| 初始化认证 API | `auto-setup` 403；`config` 与 `oidc/config` 200 | 同 | 本轮对齐 |

上述路由结果在三种 viewport 均稳定。各 viewport 的截图保存在：

- Vue：`browser-evidence-20260915/vue-{desktop,laptop,mobile}-protected-login.png`
- React：`browser-evidence-20260915/react-{desktop,laptop,mobile}-protected-login.png`

## 判定与边界

本轮没有修改业务路由代码。React 的 `apps/web/src/routes.tsx` 与 `routes.test.ts` 已把“保留完整原始 URL 到 `next`”写成明确契约；Vue 当前运行态则丢弃该 query。由于没有有效账号，无法验证登录后 `next` 是否完成回跳，也无法仅凭 URL 选择哪一方应被改写。因此本轮按 evidence-only 提交，不把该差异升级为修复结论。

没有使用有效凭据、成功认证、租户/权限 fixture 或业务 mutation；HTTP 200 仅代表 SPA 文档请求成功。该证据不覆盖 authenticated route/state parity、forbidden/tenant variants、Wails、iOS 或 Android。

## 可复现命令

```sh
EVIDENCE_OUTPUT_DIR="$PWD/artifacts/browser-evidence-20260915" \
  pnpm --dir apps/web exec node \
  ../../artifacts/browser-evidence-20260915/collect-protected-route-evidence.mjs
```
