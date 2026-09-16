# 2026-09-14 负路径第四批：T-3 刷新失败重登录 / T-4 预览文案 / 网络错误文案层（S00）

依据 2026-09-14-review-rows-audit.md 负路径第三批登记的两项 React 侧待修复（T-3、T-4），外加 live 对照中新发现并修复的共享文案层缺口。

## T-3[低→已修复] 401 refresh 失败后的重登录

- **Vue 基线**：frontend/src/utils/authRefresh.ts:104-150 — refresh 失败（或无 refresh token）时 clearAuthStorage() + redirectToLogin()（已在 /login 或 /embed/* 时跳过），用户落在登录页而非静默受损会话。
- **React 修复前**：refresh-coordinator 只清凭据，无导航、无提示，用户停留在失效页面。
- **修复位置**：新增 apps/web/src/auth/relogin.ts（shouldReloginAfterRefreshFailure + reloginAfterRefreshFailure，可注入 pathname/assign 便于测试）；main.tsx 的 transport refresh 回调在失败时清 scope/session 并导航 /login 后原样上抛。
- **测试**：apps/web/src/auth/relogin.test.ts 4/4（守卫分支：/platform/* → 跳转、/login → 仅清理、/embed/* → 仅清理）。
- **Live 验证**（.parity-tools/negpath4-auth.cjs，真实后端 :8080，parity 账号，只读——仅污染浏览器本地会话）：双端以损坏的 access+refresh token 打开 /platform/knowledge-bases，最终 URL 均为 /login（React 修复前不跳转）。截图 screenshots/negpath4-20260914/a1-refresh-failure-{vue,react}.png。

## T-4[低→已修复] 组织预览失败 fallback 文案

- **Vue 基线**：OrganizationList.vue:867-882 — 业务失败（HTTP 200 + success:false）fallback 为 organization.invite.invalidCode；异常（网络/抛错）fallback 为 organization.invite.previewFailed；有 message 时优先展示服务端文案（如「无效的邀请码/Invalid invite code」）。
- **React 修复前**：两处 catch（OrganizationsPage.tsx:313 URL 深链预览、:379 手输预览）统一 fallback invalidCode。
- **修复位置**：两处 fallback 改为 organization.invite.previewFailed（对齐 Vue catch 分支）；服务端 message 优先级由 errorText 保持不变。边界说明：React api-client 对 HTTP-200+success:false 抛内部解析错误（preview.success must be true），与 Vue 的 invalidCode 分支在该罕见无 message 场景存在残余差异，登记不阻塞（live 中死码实际返回 4xx + 服务端 message，双端展示一致）。

## 新发现并修复：网络错误文案层缺失

- **Live 发现**（negpath4 B 格）：中断 organizations/preview 请求后，Vue 显示「网络错误，请检查您的网络连接」，React 泄漏原生 Failed to fetch。
- **Vue 基线**：frontend/src/utils/request.ts:137-139 — 无 response 的失败 reject 为 t(error.networkError)，全局生效。Vue locale 集分歧：en-US/ja-JP 无该键，fallbackLocale=zh-CN（frontend/src/i18n/index.ts:26 + resolveDefaultLocale.ts:4），故 Vue 在 en-US/ja-JP 也渲染 zh-CN 文案。
- **修复位置**：
  - packages/i18n/src/generated/errorMessages.ts：error.networkError ×5 locale，en-US/ja-JP 值取 Vue 实际渲染结果（zh-CN 文案，注释说明），保持 React 键集一致；
  - apps/web/src/platform/http.ts：withNetworkError 包装 send/sendBinary/sendStream/sendMultipartFile——TypeError（网络层失败）转为 ApiError（code NETWORK_ERROR，message 取 error.networkError），AbortError/ApiError/其它错误原样透传；locale 取 localStorage locale 键（与 readInitialLocale 同源），缺省 zh-CN。
- **测试**：apps/web/src/platform/http.test.ts 新增 3 例（默认 locale 文案逐字节、存储 locale=en-US 仍渲染 Vue fallback 值、非网络错误不改写）。
- **Live 验证**：修复后 B 格双端 errorSample 均为「网络错误，请检查您的网络连接」，逐字一致。截图 screenshots/negpath4-20260914/b1-preview-failure-{vue,react}.png。

## 门禁

| 层 | 结果 |
|---|---|
| relogin.test.ts / http.test.ts 新增 | 4/4 · 3/3（http 全文件 13/13） |
| test:shared / test:web | 444/444 · 856/856 |
| typecheck:web / typecheck:shared | 0 错误 · 0 错误 |

## 处置结论

- T-3：已修复（React 侧），live 双端行为一致 → negpath 第三批 T-3 关闭。
- T-4：已修复（fallback 键对齐），残余 HTTP-200 业务失败边界登记不阻塞 → 关闭。
- 网络错误文案层：新缺口已修复，live 逐字一致 → 关闭。
