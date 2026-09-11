# T03 旧浏览器会话一次性导入证据（2026-09-11）

## 范围

本证据覆盖 React Web 启动时对 Vue 时代 `weknora_*` 凭证/偏好的兼容导入，不代表真实 OIDC provider、浏览器登录或生产后端验收。

实现提交：`9e5f970`（`feat: add one-time legacy browser session import`）与 `f3a4f38`（`fix: tolerate unavailable browser storage reads`）。

## 行为证据

- 首次启动从旧 `weknora_token`、`weknora_refresh_token`、选中空间和已知 UI 偏好构造 React state。
- 先写 `weknora_react_legacy_fallback_v1` 回退快照，再写 `weknora_react_session_v1`，最后写 `weknora_react_legacy_import_v1=complete`。
- 完成标记存在时，React 只读取版本化记录；即使旧 token/空间随后变化，也不会重新采纳旧值。
- 正式记录损坏时读取 durable fallback；旧键不删除，以便仍受支持的 Vue/Lite 回退 artifact 继续工作。
- `persistBrowserCredential` 同步更新 React state 与旧键，登出清除旧 bearer 键并把 React state 置为 anonymous。
- 存储读取被浏览器禁用或抛异常时，导入降级为 anonymous，不让启动阶段因 `localStorage.getItem` 抛错而白屏。
- Embed 仍是独立 profile；transport 的 refresh 条件要求 bearer credential 与 refresh token，因此 Embed 不会触发主账号 refresh。

## 验证命令

| 命令 | 结果 |
|---|---|
| `pnpm exec tsx --test apps/web/src/platform/legacy-session.test.ts apps/web/src/platform/credentials.test.ts` | 7/7 passed，exit 0 |
| `pnpm test:web` | 64/64 passed，exit 0 |
| `pnpm typecheck:web` | exit 0 |
| `pnpm build:web` | exit 0，Vite production bundle generated |
| `node scripts/check-react-boundaries.mjs` | exit 0 |
| `git diff --check` | exit 0 |

测试按 TDD 先记录了缺少导出时的 RED 失败，再实现最小导入/回退逻辑并完成 GREEN 验证。

## 尚未覆盖

真实浏览器 cold start、OIDC provider 回调、跨浏览器版本升级、存储配额/禁用存储，以及真实后端刷新并发仍属于 T03 的 `review` 阶段开放项。
