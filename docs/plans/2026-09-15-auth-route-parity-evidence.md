# Vue/React 认证与公开路由审查证据

日期：2026-09-15。范围：登录、注册、OIDC 回调、`next`、匿名保护路由和工作空间 onboarding。未使用有效凭据或业务写操作。

## 基线与证据边界

- Vue 参考源码：`frontend/src/router/index.ts`、`frontend/src/views/auth/Login.vue`、`frontend/src/App.vue`。
- React 范围：`apps/web/src/auth/**`；路由判断只作审查依据，未修改 `apps/web/src/routes.tsx`。
- 已有配对采集：`artifacts/browser-evidence-20260915/results.json`（Vue `5180`、隔离 React worktree `5181`，14:15）。其中匿名 `/register` 为 Vue 标题“登录”、React 标题“创建账户”；`/platform/apps` 两端均回到登录页，但 React 保留 `next`。
- 当前工作树复核：临时启动当前 `apps/web` 于 `5191`，匿名访问 `/register` 在等待异步认证配置后显示登录表单；`/platform/apps?tab=connections` 回到 `/login?next=%2Fplatform%2Fapps%3Ftab%3Dconnections`。该端口未使用有效凭据。
- 采集脚本在 300ms 等待下可能早于嵌套认证页完成渲染；因此当前 `/register` 结论以源码和额外等待 1.2s 的浏览器检查为准，不把早期空白 DOM 当产品差异。

## 明确差异与修复

1. React 原先无条件把 `/register` 渲染为注册表单；Vue 的 `/register` 默认仍是登录卡，只有有效邀请 token 且注册策略允许时才切换注册。React 现在以 `GET /api/v1/auth/config` 的 `registration_mode` 和有效邀请共同决定；登录页的“创建账户”使用显式内部模式参数，保留自助注册入口。
2. Vue 消费 `#oidc_error` 和 `oidc_error_description`，清理 hash 后回到登录并显示错误；React 原先只消费 `oidc_result`。React 现在清理回调、暂存一次错误消息并在登录页显示，同时保留成功回调的现有安全 `next` 规则。

## 未修改/未宣称

- `next` 仍只接受单斜杠绝对路径，拒绝 `https://` 与 `//`；未发现需改动的源码差异。
- 当前任务未修改通用路由守卫、权限 capability/system-admin 表达或 Vue 的受保护页面；现有匿名证据不足以证明已认证角色矩阵，故不臆测运行时冲突。
- 未执行有效账户登录、有效邀请兑换、真实 OIDC Provider 往返、已认证 tenant/permission 变体、Wails/iOS/Android。

## 验证

- `node --import tsx --test src/auth/auth-state.test.ts src/routes.test.ts`（`apps/web`）：15 passed, 0 failed。
- `node artifacts/browser-evidence-20260915/collect-public-route-evidence.mjs`：两端匿名公开/保护路由 HTTP 200；Vue `/register` 为登录，现工作树 React `/register` 经异步配置后为登录。
- `node artifacts/browser-evidence-20260915/collect-protected-route-evidence.mjs`（当前 React `5191`）：三种 viewport 的匿名保护路由均回登录，`next` 保留；无有效认证。
- `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit`：未通过，失败来自现有未授权范围变更/依赖状态（`src/documents/upload-confirm.test.ts` 的 `multimodalEnabled` 类型、`packages/ui` 未使用 `@ts-expect-error`），不是本次认证文件诊断结果。
