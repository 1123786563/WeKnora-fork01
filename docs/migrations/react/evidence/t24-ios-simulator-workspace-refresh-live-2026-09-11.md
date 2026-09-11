# T24 iOS Workspace refresh follow-up

日期：2026-09-11  
分支：`codex/react-multiclient`  
设备：iPhone 17 Pro Simulator，iOS 26.5  
运行时：当前 worktree Metro，`EXPO_PUBLIC_API_BASE_URL=http://127.0.0.1:18082`；隔离 Lite 服务和临时 SQLite 数据目录仅用于本次验证。

## 问题与修复

原生 Workspace 路由把整个 `useMobileRuntime()` 返回对象放进 `useCallback` 依赖。Provider 每次状态更新都会创建新的 context value，导致 `auth/me` 加载回调不断变化，Workspace 页面反复发起请求并长期显示加载状态。凭证从 SecureStore 恢复时还只恢复了 active tenant，没有恢复 memberships，因此直接打开管理页时角色可能显示为 unavailable。

本次修复：

- `refreshWorkspaces` 使用稳定的 `createSingleFlight` 回调，同一时间只允许一个 `auth/me` 请求；
- Workspace 路由只依赖稳定的 `refreshWorkspaces`，不再依赖整个 runtime 对象；
- bearer 凭证恢复完成且没有缓存 memberships 时，Provider 自动补拉一次 `auth/me`；
- 新增 memberships hydration 条件和 single-flight 回归测试。

## 验证

先按 TDD 验证回归测试确实能捕获缺失实现：

```text
node --import tsx --test apps/mobile/src/platform/workspace.test.ts
exit 1；原有 3 项通过，新增 hydration/single-flight 2 项失败（缺少对应 helper）
```

补充最小实现后，当前完整移动测试与类型检查：

```text
node --import tsx --test 'apps/mobile/src/**/*.test.ts'
36/36 passed，exit 0

pnpm --filter @weknora/mobile typecheck
exit 0

git diff --check
exit 0
```

## 原生实时证据

1. 现有隔离 Lite 账号在真实 iOS 原生宿主中保持已认证状态，进入 Knowledge bases 后点击 `Workspace`。
2. 页面在等待 5 秒后稳定显示 `mobilet24's Workspace · owner · Current`，没有继续显示加载指示器。
3. 返回 Knowledge bases → Manage → Workspace API keys，页面显示 `Workspace role: owner`，并显示真实后端返回的 `No API keys returned.` 空态；这证明恢复后的 memberships 已可用于角色判定。
4. 此次验证未记录账号密码、Bearer token 或临时数据库内容；隔离服务/数据仅用于本地探针。

## 证据边界

- 这是 iOS Simulator 的真实原生宿主、Metro 和隔离 Lite 后端证据，不是 mock 或仅 Expo export 证据。
- Android 没有可用设备或模拟器，Android 原生运行时仍未验收。
- 没有把这次 follow-up 误记为完整 T24 接受；Wails Windows/Linux、部署环境、完整浏览器/角色矩阵以及 Android 原生运行仍是开放项。
