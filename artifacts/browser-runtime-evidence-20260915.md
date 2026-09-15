# Browser/runtime evidence ledger — 2026-09-15

本注记只记录当前运行环境与已有 browser artifacts；不修改业务代码，也不把 HTTP 200、Vite 页面可达、构建或静态测试当作 Vue→React parity 验收。

## 当前环境探测

探测时间：2026-09-15 21:49（Asia/Shanghai；HTTP `Date` 为 13:49 UTC）。命令为 `lsof -nP -iTCP:<port> -sTCP:LISTEN` 与 `curl -sS -D - --max-time 5`。

| 目标 | 结果 | 可确认的事实 | 证据边界 |
|---|---|---|---|
| React `127.0.0.1:5181` | PASS（HTTP 200） | 返回 Vite React HTML，`<title>WeKnora</title>`，挂载 `/src/main.tsx` | 仅证明开发服务器和 HTML shell 可达；不证明页面状态、视觉/交互 parity 或后端链路 |
| Vue `127.0.0.1:5173` | BLOCKED-ENV（连接失败） | 当前无监听进程，`curl` 返回 code 7 | 按用户指定端口无法进行 Vue 对照；不得以其他端口结果替代 |
| Vue 实际监听 `127.0.0.1:5180` | PASS（HTTP 200） | `frontend` Vite 进程的 cwd 为 `/Users/wuyongjun/trea/WeKnora-fork01/frontend` | 这是当前发现的 Vue 端口，不等于 `:5173`；可作为已有 artifacts 的解释，不改变 `:5173` blocked 结论 |
| 后端 `127.0.0.1:8080/` | PASS（HTTP 401） | 服务响应 `Unauthorized: missing authentication`，带 `X-Request-Id` | 只证明端口/HTTP 服务存在及匿名请求被拒绝；没有认证业务、租户、权限或 mutation 证据 |
| 后端 `:8080/api/v1/health`、`:8080/api/v1/config` | PASS（HTTP 401） | 均返回匿名未授权 | 不能把 401 写成 health/config 业务成功；认证身份和可用业务 fixture 缺失 |

进程归属补充：`:5181` 的 Vite cwd 是 `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient/apps/web`；`:5180` 的 Vite cwd 是当前仓库 `frontend`。因此本次检查没有把共享 worktree 的 React 运行态误写成当前主工作树的服务态。

## 已有 browser artifacts（可复核，但不是本轮新截图）

最新已有记录为共享 worktree 中的 `artifacts/browser-evidence-20260915-run4/results.json`，`capturedAt=2026-09-15T12:38:07.461Z`，viewport `1355x776`，locale `zh-CN`：

- Vue 使用 `http://127.0.0.1:5180/login`，HTTP 200，最终 URL 保持 `/login`；未认证访问 `/platform/apps` 重定向 `/login`。记录了 `/api/v1/auth/auto-setup` 的 403。
- React 使用 `http://127.0.0.1:5181/login`，HTTP 200，未认证访问 `/platform/apps` 重定向到 `/login?next=%2Fplatform%2Fapps`。记录了 `auth/auto-setup`、`auth/config`、`auth/oidc/config` 的 404。
- 两端 artifacts 包含公开登录页 DOM 文本、输入框/按钮和部分 computed style，以及 `vue-login-public.png`、`react-login-public.png`。
- `evidenceBoundary` 明确排除 authenticated user data、permission variants、real mutations、Wails、iOS、Android。

这些 artifacts 可支持“公开登录渲染/匿名保护路由守卫的局部 browser evidence”，不能支持“整体 parity accepted”。截图也只能用于发现视觉差异，不能替代同账号、同租户、同状态的逐页交互验收。

## Browser tooling 状态

本轮尝试使用 `browser-use` 和 CUA 获取新的 DOM/浏览器态：

- `browser-use --doctor`：Chrome 与 daemon 可用，但 active browser connections 为 0。
- `browser-use` 执行失败于环境依赖 `ModuleNotFoundError: No module named 'oci'`。
- CUA `getState()` 返回 browsers inventory 错误：`Unable to load browser request-header policy`，没有可操作 browser tab。

因此本轮没有伪造“fresh browser PASS”，也没有重演旧截图；新增的当前结论仅依赖端口/HTTP 探测，浏览器交互新增证据为 `BLOCKED-ENV`。

## 状态汇总与下一步门槛

| 证据层 | 状态 |
|---|---|
| React `:5181` HTTP shell | PASS（仅可达性） |
| Vue 指定 `:5173` | BLOCKED-ENV（无监听） |
| Vue 实际 `:5180` HTTP shell | PASS（仅可达性） |
| 后端 `:8080` 匿名 HTTP | PASS（服务存在，业务请求未授权） |
| 既有公开登录 browser artifact | PARTIAL / 可复核 |
| 本轮 fresh browser DOM/interaction | BLOCKED-ENV |
| authenticated route/state parity | BLOCKED-ENV / 未验证 |
| real backend mutation、权限负例、租户隔离 | BLOCKED-ENV / 未验证 |
| Wails、iOS、Android 逐功能 parity | 未验证；历史 build/launch 记录不替代本轮 acceptance |

后续要升级为 parity evidence，至少需要：Vue 与 React 都有可操作的浏览器连接；使用同一认证身份/租户；逐页覆盖 loading、empty、error、forbidden、editing、submitting、success、failure；并对真实后端请求、权限负例、mutation 结果和截图分别留存。构建、类型检查和 HTTP 200 仍只能作为 supporting evidence。
