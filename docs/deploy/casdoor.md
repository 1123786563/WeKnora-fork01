# Casdoor SSO 部署指南

WeKnora 通过 Casdoor 提供 OIDC 单点登录（web 端授权码流程）与微信小程序静默登录通道（服务端 ROPC + Casdoor 自动建号）。本文覆盖 Casdoor 一次性初始化、WeKnora 侧环境变量、小程序侧配置，以及交付前的手动联调清单。

前置事实（与实现对应，排障前先核对）：

- Casdoor 由 `docker-compose.yml` 的 `casdoor` 服务提供：镜像 `casbin/casdoor:latest`，宿主端口 `${CASDOOR_PORT:-8000}:8000`，SQLite 数据持久化在宿主 `./config/casdoor/`（挂载为容器 `/conf`，含 `app.conf` 与 `casdoor.db`）。
- 容器网络内 Casdoor 的地址恒为 `http://casdoor:8000`（compose 服务名），与宿主端口映射无关。
- web 端 OIDC 与小程序静默登录共用同一组 Casdoor 应用凭据（`OIDC_AUTH_CLIENT_ID/SECRET`，即 `weknora-app` 应用的 Client ID/Secret）。

---

## 1. Casdoor 初始化（一次性，UI 操作）

### 1.1 启动并首次登录

```bash
docker compose up -d casdoor
```

浏览器打开 `http://localhost:8000`。若本机 8000 已被其他容器占用，在 `.env` 里设 `CASDOOR_PORT=18000` 后 `docker compose up -d casdoor`，改用 `http://localhost:18000` 访问（本文其余步骤同）。

使用内置账号 `admin` / `123` 登录。Casdoor 强制首次登录修改密码——改成一个强密码并妥善保存，这个 `admin` 是 Casdoor 的全局管理员，日常仅用于后台运维，不参与 WeKnora 登录链路。

### 1.2 建组织 `weknora`

顶部菜单 **Organizations → Add**，Name 填 `weknora`，其余默认即可。

### 1.3 建应用 `weknora-app`（核心）

**Applications → Add**：

| 字段 | 值 | 说明 |
| --- | --- | --- |
| Organization | `weknora` | 必须选刚建的组织 |
| Name | `weknora-app` | 即应用名 |
| Redirect URLs | `http(s)://<host>/api/v1/auth/oidc/callback` | **必须与浏览器访问 WeKnora 的地址精确一致**（协议+域名+端口）。本地默认 `http://localhost/api/v1/auth/oidc/callback`；改过 `FRONTEND_PORT`/用真实域名时对应调整，可填多行 |
| 授权类型（Grant types） | **勾选 `Password`（资源所有者密码凭证，ROPC）** | 必勾。小程序静默登录与服务账号管理认证都走 `grant_type=password`，未勾选时静默登录会报 `casdoor ropc: no access token` |

保存后进入应用详情，记录 **Client ID** 与 **Client Secret**（后者只展示一次，丢了就重新生成）。

### 1.4 建服务账号 `svc_weknora`

WeKnora 后端通过该账号调用 Casdoor 管理 API（`/api/add-user` 建号、`/api/set-password` 重置密码），需要 admin 权限：

1. **Users → Add**（组织选 `weknora`）：Name 填 `svc_weknora`，设一个强密码（即后续 `CASDOOR_ADMIN_PASSWORD`）。
2. 编辑该用户，将其加入 `weknora` 组织的 **admin** 角色（或在 Roles 里把 `weknora` 的 admin 角色授予该用户）。

注意：ROPC 登录时后端会把用户名拼成 `组织/用户名`（`weknora/svc_weknora`），因此该账号**必须建在 `weknora` 组织内**，不能放在内置的 `built-in` 组织。

### 1.5 建一个联调用真人账号

同样在 `weknora` 组织下建一个普通用户（如 `alice`），用于第 4 节的 web SSO 全流程联调。

---

## 2. WeKnora 环境变量（`.env`）

app 容器通过 `env_file: .env` 整体注入，以下键无需在 compose 里逐条声明。全部修改后执行 `docker compose up -d app` 生效。

```bash
# ========== Casdoor / OIDC SSO（web 端） ==========
OIDC_AUTH_ENABLE=true
# 容器网络内地址：后端用它做 discovery，token/userinfo/jwks 都从这里解析。
# 与宿主端口映射无关（CASDOOR_PORT=18000 时它仍写 8000）。
OIDC_AUTH_ISSUER_URL=http://casdoor:8000
# 浏览器可达地址：授权端点只有浏览器会访问，必须显式覆盖成宿主端口。
# 本机 Casdoor 跑在 8000：http://localhost:8000/login/oauth/authorize
# 本机 Casdoor 跑在 18000：http://localhost:18000/login/oauth/authorize
# 生产：https://sso.example.com/login/oauth/authorize
OIDC_AUTH_AUTHORIZATION_ENDPOINT=http://localhost:18000/login/oauth/authorize
# 登录按钮上的展示名（默认 OIDC）
OIDC_AUTH_PROVIDER_DISPLAY_NAME=Casdoor
OIDC_AUTH_CLIENT_ID=<weknora-app 的 Client ID>
OIDC_AUTH_CLIENT_SECRET=<weknora-app 的 Client Secret>
# true 时 web 登录页隐藏账密/注册表单，只剩 SSO 按钮；false 或不设则表单与 SSO 按钮共存
OIDC_AUTH_SSO_ONLY=true

# ========== 微信小程序静默登录通道 ==========
# 必须与微信小程序后台的 AppID 一致，否则 code2session 报 invalid appid
WECHAT_MP_APP_ID=wx1234567890abcdef
WECHAT_MP_APP_SECRET=<小程序 AppSecret>
# 加密每个微信用户 Casdoor 服务密码的 AES 密钥（取 sha256 后用作 AES-256-GCM）。
# 必填强随机值：openssl rand -hex 32。留空时启动不报错（见下方“启动校验”），
# 首次静默登录即 503；弱值则任何知道该值的人都能解开存量密文。
# 一经使用请固定保存：换值 = 存量密文全部解不开（会触发自动重置兜底自愈，
# 但每次都多一轮 Casdoor 重置，且历史绑定关系失效重建）。
WECHAT_MP_SECRET_KEY=<openssl rand -hex 32 生成>

# ========== Casdoor 服务账号（建号/重置密码） ==========
# 容器网络内地址（同 issuer 逻辑，与宿主端口无关）
CASDOOR_ADMIN_BASE_URL=http://casdoor:8000
CASDOOR_ADMIN_ORG_NAME=weknora
CASDOOR_ADMIN_USERNAME=svc_weknora
CASDOOR_ADMIN_PASSWORD=<svc_weknora 的密码>

# ========== SSRF 白名单（本地/私网部署必须） ==========
# 后端所有 OIDC 端点与 CASDOOR_ADMIN_BASE_URL 的出站请求都过 SSRF 校验：
#   - `casdoor` 解析到 Docker 网桥私网 IP，未加白会被“resolves to restricted IP”拦截；
#   - `localhost` 是受限主机名，授权端点写 http://localhost:18000/... 时未加白会报
#     “OIDC authorization endpoint failed SSRF validation”。
# 注意：不要写进 SSRF_WHITELIST_EXTRA——compose 对它有内置默认值（searxng,qdrant,…），
# 自定义会整体替换默认值。写 SSRF_WHITELIST 会与之合并。
SSRF_WHITELIST=casdoor,localhost
```

可选键（一般无需设置）：`OIDC_AUTH_DISCOVERY_URL`（默认由 issuer 拼 `/.well-known/openid-configuration`）、`OIDC_AUTH_TOKEN_ENDPOINT` / `OIDC_AUTH_USER_INFO_ENDPOINT` / `OIDC_AUTH_JWKS_URI`（默认由 discovery 填充）、`OIDC_AUTH_SCOPES`（默认 `openid profile email`）、`OIDC_USER_INFO_MAPPING_USER_NAME/EMAIL`（默认 `name`/`email`）。

### 2.1 issuer 与 authorization_endpoint 的双地址机制

这是 Casdoor 集成最容易出错的一处，原理如下：

- **服务端地址（issuer 一族）**：token 交换、userinfo、JWKS、discovery 都由 app 容器发起，走 Docker 内部网络，因此 `OIDC_AUTH_ISSUER_URL=http://casdoor:8000` 与宿主端口映射无关。discovery 文档只会填充**留空**的端点；一旦你显式写了某个端点，显式值优先。
- **浏览器地址（authorization_endpoint）**：授权端点是全链路中唯一由**浏览器**访问的 URL。若不显式覆盖，discovery 会把它填成 `http://casdoor:8000/login/oauth/authorize`——浏览器解析不了 `casdoor` 这个容器名，登录会直接打不开授权页。所以必须显式设为浏览器可达地址（本机 `http://localhost:<CASDOOR_PORT>/login/oauth/authorize`，生产用真实域名）。

推论：本机 Casdoor 从 8000 换到 18000 时，只改 `CASDOOR_PORT` 和 `OIDC_AUTH_AUTHORIZATION_ENDPOINT` 两处；`OIDC_AUTH_ISSUER_URL` 与 `CASDOOR_ADMIN_BASE_URL` 保持 `http://casdoor:8000` 不变。

### 2.2 启动校验行为（避免误判“配置没生效”）

- `OIDC_AUTH_ENABLE=true` 时启动即校验 Client ID/Secret 非空，且 discovery 或（authorization+token）端点至少一组可用，缺了 app 容器起不来。
- 微信通道的完整性校验在 `OIDC_AUTH_SSO_ONLY=true` 时**被跳过**：`WECHAT_MP_*`/`CASDOOR_ADMIN_*` 缺项不会阻止启动，而是在第一次静默登录时以 503（`WeChat login channel is not configured`）或 401 暴露。联调前请逐项核对第 2 节样例。
- 后端容器需要能出网访问 `api.weixin.qq.com`（code2session），内网隔离环境需放行。

---

## 3. 小程序侧

小程序构建期配置在 `apps/miniprogram/config/index.ts`，通过环境变量注入：

```bash
# 在 apps/miniprogram 下构建（dev:weapp 为 watch 模式，本地联调常用）：
WEKNORA_API_ORIGIN=https://weknora.example.com WEKNORA_WEAPP_APPID=wx1234567890abcdef npm run build:weapp
```

- `WEKNORA_API_ORIGIN`：WeKnora API 的固定 origin。生产必须是 HTTPS 且不带 `/api/v1` 后缀（微信要求小程序 request 域名为 HTTPS）；本地联调允许 `http://localhost:<端口>` 回环地址（config/index.ts 显式放行，仅限 localhost/127.0.0.1）。
- `WEKNORA_WEAPP_APPID`：写入 `project.config.json` 的 appid（缺省 `touristappid`，即微信开发者工具的体验用号）。**必须与服务端 `WECHAT_MP_APP_ID` 一致**——code 换 openid 时微信会校验 code 的签发 AppID 与后端提交的 appid/secret 匹配，不一致报 `invalid appid`。
- 微信小程序后台（mp.weixin.qq.com → 开发管理 → 开发设置 → 服务器域名）：**request 合法域名**须包含 `WEKNORA_API_ORIGIN` 的域名（HTTPS）。开发者工具里可勾选「不校验合法域名」做本地联调。

静默登录链路（无需用户操作）：启动进登录页 → `wx.login` 取 code → `POST /api/v1/auth/wechat/login`（公开路由，带限流）→ 后端 code2session 拿 openid → 首登自动在 Casdoor 建 `wx_<openid 前 8 位>` 用户（邮箱 `wx_<openid>@wechat.local`）→ ROPC 校验 → 下发 WeKnora 会话。失败时登录页保留「立即登录」按钮整页重试，不再渲染账密表单。

---

## 4. 手动联调清单

按序执行并逐项打勾。前两项需要 Casdoor 后台与微信开发者工具配合。

### 4.1 web SSO 全流程

1. `OIDC_AUTH_SSO_ONLY=true` 生效：打开 web 登录页，只看到一张含单个 SSO 按钮（文案含 `OIDC_AUTH_PROVIDER_DISPLAY_NAME`，如「使用 Casdoor 登录」）的卡片，账密/注册表单全部不可见。
2. 点击按钮 → 跳转 Casdoor 登录页（地址栏是浏览器可达的 authorization_endpoint，如 `http://localhost:18000/login/oauth/authorize`）。
3. 用 1.5 节建的真人账号（如 `alice`）登录 → Casdoor 回调 `http(s)://<host>/api/v1/auth/oidc/callback` → 自动回到 WeKnora 并进入工作台。
4. Casdoor 后台 Users（组织 `weknora`）与 WeKnora 成员列表中都能看到该用户（WeKnora 侧是 OIDC 首登自动建档）。

### 4.2 小程序静默登录（微信开发者工具）

1. `WEKNORA_API_ORIGIN` 指向本地 API（回环 http 即可），构建并打开小程序：启动即静默登录，无感进入首页。
2. Casdoor 后台出现 `wx_*` 用户（组织 `weknora`），WeKnora 侧对应 `wx_<openid>@wechat.local` 邮箱的账号。
3. 杀掉小程序进程重进：复登成功（后端用本地加密保存的服务密码走 ROPC，全程无感）。

### 4.3 禁用验证

1. Casdoor 后台把某个 `wx_*` 用户设为禁用（is forbidden）。
2. 小程序复登该账号：被拒（ROPC 失败，含重置兜底后仍失败），登录页给出错误并可重试——不应出现绕过 Casdoor 直接进站的情况。

### 4.4 回退路径

1. `.env` 改 `OIDC_AUTH_SSO_ONLY=false`（保持 `OIDC_AUTH_ENABLE=true`），`docker compose up -d app`：登录页恢复账密表单，SSO 按钮与表单共存，本地账号登录正常——后端账密接口从未改动。
2. 要完全关闭 SSO：再把 `OIDC_AUTH_ENABLE=false`。注意此时若 `.env` 里还残留部分 `WECHAT_MP_*`/`CASDOOR_ADMIN_*`（任一非空），启动校验会要求整套配齐——要么全清空，要么保持完整。

### 4.5 常见问题

| 症状 | 原因与处置 |
| --- | --- |
| 点 SSO 按钮报错 `OIDC ... endpoint failed SSRF validation`，或后端日志 `resolves to restricted IP` / `hostname localhost is restricted` | 未加 SSRF 白名单。`.env` 设 `SSRF_WHITELIST=casdoor,localhost`（生产只需 `casdoor`），重启 app 容器 |
| 授权页打不开（浏览器无法解析 `casdoor`） | `OIDC_AUTH_AUTHORIZATION_ENDPOINT` 没显式覆盖，被 discovery 填成了容器地址。按 2.1 节设为浏览器可达地址 |
| 授权后 Casdoor 报 redirect URL 不匹配 | `weknora-app` 的 Redirect URLs 与浏览器实际回调地址（协议/域名/端口）不一致，逐字符比对 |
| 小程序静默登录报 `casdoor ropc: no access token` | `weknora-app` 未勾选 Password（ROPC）授权类型；或 Client ID/Secret 与 `.env` 不一致；或用户不在 `weknora` 组织（ROPC 用户名按 `org/name` 拼接） |
| 建号/重置密码失败（`casdoor admin auth failed`） | `svc_weknora` 密码错误、未建在 `weknora` 组织、或没有 admin 权限——管理 token 取不到，后续管理调用全部失败 |
| 日志出现 `casdoor add-user: {"status":"error","msg":"Unauthorized operation"}` 或 `casdoor set-password: ...` | 管理 API 需服务账号 Bearer：后端先经 ROPC（`weknora/svc_weknora`）取管理 token，再以 **JSON body** 调 `/api/add-user`（owner/name/displayName/email/password/type），以 **userOwner/userName/newPassword 表单字段**调 `/api/set-password`（`?id=` 参数无效）；匿名或失效 token 会被 Casdoor ApiFilter 拒绝并返回 `Unauthorized operation` |
| 静默登录 503 `WeChat login channel is not configured` | `WECHAT_MP_APP_ID`/`WECHAT_MP_SECRET_KEY`/`CASDOOR_ADMIN_BASE_URL` 有空项（sso_only 部署启动不校验，缺项在请求时才暴露） |
| code2session 报 `invalid appid` | 小程序 appid（`WEKNORA_WEAPP_APPID`/工具项目 appid）与服务端 `WECHAT_MP_APP_ID` 不一致 |
| 8000 端口被占 | `.env` 设 `CASDOOR_PORT=18000`；同时把 `OIDC_AUTH_AUTHORIZATION_ENDPOINT` 改成 18000 地址。`OIDC_AUTH_ISSUER_URL` 与 `CASDOOR_ADMIN_BASE_URL` 不动（容器网络内仍是 `http://casdoor:8000`） |
| 换了 `WECHAT_MP_SECRET_KEY` 后小程序首登变慢/多一次重置 | 预期行为：存量密文解不开会触发「ROPC 失败 → 重置密码 → 重试 → 回写新密文」兜底自愈。密钥应固定，不要反复换 |
