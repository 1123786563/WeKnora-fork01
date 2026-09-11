# T24 iOS Simulator cold SecureStore restoration — 2026-09-11

## Environment

- Source: isolated worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`.
- Native host: `com.weknora.mobile`, iPhone 17 Pro Simulator, iOS 26.5, UDID `5EECD8BB-4B4A-473C-85C3-7841329FDF3C`.
- Build: `expo run:ios --device 'iPhone 17 Pro' --no-bundler --configuration Release`; the current worktree app built successfully with 0 errors and was installed on the simulator. The embedded Hermes bundle was present in `WeKnora.app/main.jsbundle`.
- Backend: isolated Lite server on `127.0.0.1:18082`; no production database, provider, or credential was used.
- Test identity: temporary owner `ioscold_20260911@example.test`; the password was entered only into the simulator and is not recorded here.

## Live sequence

1. Created the temporary owner through `POST /api/v1/auth/register`, receiving HTTP `201`.
2. Entered the temporary identity in the native iOS login form and tapped `Sign in`.
3. The app navigated to the native `knowledge/index` route and rendered the real Lite response `No knowledge bases available.`; the authenticated header exposed `Workspace`, `Manage`, and `Sign out`.
4. Terminated the native process with `xcrun simctl terminate ... com.weknora.mobile`, launched it again with `xcrun simctl launch ... com.weknora.mobile`, and waited six seconds. No login form or password prompt was shown.
5. After the cold relaunch, the same knowledge-base route rendered again. Opening `Workspace` fetched and displayed `ioscold_20260911's Workspace, owner · Current`.

## Evidence boundary

- This is real iOS native-host + SecureStore persistence + isolated Lite evidence, not a mock or Expo export-only check.
- The cold restart proves the bearer session and workspace context were restored without re-entering credentials; the role/current marker proves the restored session hydrated the server-owned membership context.
- The test account and isolated server data are temporary. No token or password is stored in the repository.
- This closes the previously open iOS cold SecureStore-only proof, but does not change T20/T24 from `review`: Android SSE/chat, broader refresh/network/provider flows, complete role/tenant matrix, deployment/OS coverage, and other listed release gates remain open.
