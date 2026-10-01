# T01 #31 — Android 活体链证据（2026-10-02 04:50–04:58 CST）

branch `codex/issue30-t63-closure` @ `8042218c2`（HEAD 后端二进制）+ 本轮本地未提交修复（见下）。
环境：AVD test36（android-36 google_apis arm64，`-writable-system`）→ nginx TLS :8443（mkcert，192-168-3-33.nip.io）→ 后端 :8084（HEAD）；Casdoor :18000（org weknora / app weknora-app / t01live）。

## 三目标

1. **app 以授权 HTTPS origin 启动**：`app-deployment-login-authorized-origin.png` — deployment-login 预填 `https://192-168-3-33.nip.io:8443`（origin 校验门通过）。
2. **真实 OIDC 重定向登录**：`01-casdoor-authorize-login-page.png`（Chrome 授权页）→ t01live 真实认证 → 后端签发一次性 code（`weknora://oidc?code=…&state=…`）→ `02-oidc-redirect-resolver.png`（scheme 回跳系统 Resolver；当时的第二个候选项 `com.weknora.app` 为早期遗留 dev 构建，已卸载）→ app `POST /api/v1/auth/mobile/exchange` 200、token 283 字节（logcat 见 t01-live-run.txt）。
3. **登录后能力读面**：`03-post-login-permission-prompt.png` → `04-post-login-home-surface.png`（SIGN OUT / NEW TASK / ASK KNOWLEDGE / TASK OFFICE 面）→ `05-post-login-tasks-read-surface.png`（Tasks 读面）。后端侧带票 `/auth/me` 200、`/system/capabilities` 200（t01-live-run.txt）。

## 模拟器 CA 信任（双层）

- **系统 store**（覆盖 RN/OkHttp 等 Conscrypt 栈）：`-writable-system` 启动 → `adb remount` → mkcert rootCA (`f160f0cf.0`) 装入 `/system/etc/security/cacerts`；API 36 真实信任源在 conscrypt apex → 对 init/zygote64/webview_zygote/chrome_zygote 四个 mount namespace 各做 `nsenter --mount` + tmpfs 覆盖 `/apex/com.android.conscrypt/cacerts`（remount/avbctl 均报 "Device must be bootloader unlocked"，此为绕法）。
- **用户 store**（覆盖 Chrome 133 自带 Chrome Root Store，系统 store 追加对其无效）：Settings UI 安装 mkcert rootCA（Trusted credentials → User 出现 "mkcert development CA"，`03` 前的探针即靠它）。首次 Chrome 探针 `ERR_CERT_AUTHORITY_INVALID` → 用户 store 安装后 Chrome UA 请求到达 nginx（t01-live-run.txt）。

## 本轮本地未提交修复（真缺陷，均待 owner 裁决）

1. `internal/middleware/auth.go` — noAuthAPI 缺 `/api/v1/auth/mobile/exchange`（web 版 `/oidc/exchange` 在列）：全局认证中间件把移动端 OIDC 交换一律 401，HEAD 上移动 OIDC 结构性不可用。
2. `apps/mobile/src/task-office/native-boundary/task-entry.ts` — 动态 `require(name)` 被 Metro 拒绝（"Invalid call at line 59"），整个 app bundle 不可构建；已改为字面量 switch。
3. `apps/mobile/src/app/task-office-state.test.ts` 位于路由目录（expo-router require.context 吞入 `node:test`）→ bundle 失败；本轮临时移出取证，现已原样恢复（根治应迁出 `src/app/`）。
4. `apps/mobile/src/composition.ts` — Hermes 无 `globalThis.crypto`/`TextEncoder`/`btoa`，OIDC PKCE 必抛 `OIDC_RANDOM`/`OIDC_CRYPTO`（iOS 轮 task-2 的「beginOidc 静默失败」真因）；expo-crypto 垫片补缺。
5. `apps/mobile/src/adapters/credential-store.ts` — SecureStore 键含 `encodeURIComponent` 的 `%`，带端口 origin（`%3A8443`）必炸 "Invalid key"；`%`→`.`。
6. `internal/application/service/user.go` — Casdoor(sqlite) 并发写下 JWKS 序列化中途断流（实测 200 + 1 字节 body `{`）→ "JWKS document contains no keys" 间歇失败；400ms 单次重试兜底（含 T01DEBUG 日志，需回收）。
7. Casdoor 供给面（非仓库代码）：`weknora-app`（克隆自 app-built-in）引用 `cert-built-in` 但库中不存在 → 登录报 "The cert \"cert-built-in\" does not exist"；已建 admin-owned cert（自签 RSA-2048）。另 Casdoor discovery 广告容器内 `:8000` 端口 → 后端显式 `OIDC_AUTH_TOKEN_ENDPOINT/_USER_INFO_ENDPOINT/_JWKS_URI`（:18000）+ `OIDC_AUTH_ISSUER_URL=http://localhost:8000`（对齐 Casdoor 签名的 iss claim）。

## 残差

- 首页 `TASK_OFFICE_BACKEND` 行显示错误态（RETRY/LOAD HOME）；后端未见 /task API 调用，疑 client 侧，未阻塞三目标。
- `02` 截图中的 Resolver 双选项含已卸载的 `com.weknora.app`（早期遗留 dev 构建，`expo-dev-launcher`，也抢 `weknora://` scheme；卸载后回跳直连本 app）。
- Custom Tab 复用旧 state 页会导致 `AUTH_RETURN`（本轮每轮先 force-stop Chrome 规避）；根因疑在 expo-web-browser 会话复用，未深究。
- 凭据仅经 `~/.t01-live-creds.env`（600）环境变量引用，未入库未打印；本目录无任何秘密。
