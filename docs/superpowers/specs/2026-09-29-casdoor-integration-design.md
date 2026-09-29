# Casdoor 集成设计

日期:2026-09-29
状态:已确认(用户逐节审阅通过)

## 背景与决策

将 [Casdoor](https://github.com/casdoor/casdoor) 接入 WeKnora 作为统一身份认证入口。已确认的决策:

1. **完全替换**:所有登录走 Casdoor,本地账密登录不再作为用户入口
2. **无存量负担**:新部署,无真实用户数据需迁移
3. **软下线**:后端账密接口保留(可回退),前端不展示账密表单
4. **小程序静默登录**:`wx.login` + code2session,不走 webview
5. **范围仅新栈**:React web(`apps/web`)+ Taro 小程序(`apps/miniprogram`);Vue 旧栈与旧小程序不动
6. **部署**:Casdoor 加入本仓库 `docker-compose.yml`
7. **方案 A**:Casdoor 为唯一身份源,小程序经"服务密码"程序化登录(不改 Casdoor 源码)

## 1. 总体架构

Casdoor 是唯一登录入口;WeKnora 保留自有 JWT、用户表、租户体系,Casdoor 只负责身份证明。

| 通道 | 流程 | 现状 |
|---|---|---|
| Web | OIDC 授权码 → 自动建号 → WeKnora JWT | 已有,纯配置 |
| 小程序 | wx.login → code2session → Casdoor 建号/ROPC → WeKnora JWT | 新增后端通道 |
| App(未来) | OIDC + PKCE | 已有,不动 |

本地账密:后端接口保留(软下线),SSO 开启后前端不再展示。

## 2. Casdoor 部署(docker-compose)

- 新增 `casdoor` 服务,镜像 `casbin/casdoor`,复用现有 postgres(paradedb/pg17),独立 database `casdoor`,xORM 自动建表,与 WeKnora 的 GORM 无迁移耦合
- 挂载 `config/casdoor/app.conf`(数据源、origin、前端地址)
- 初始化步骤写入部署文档:
  - 组织 `weknora`
  - 应用 `weknora-app`:开启 Password Credentials Grant,回调 `http(s)://<host>/api/v1/auth/oidc/callback`
  - 服务账号 `svc_weknora`(admin,供 WeKnora 后端调管理 API 建号)

## 3. 后端改动(Go)

### 3.1 Web 通道配置(零代码)

`config/config.yaml` + env(`OIDC_AUTH_*`,见 `internal/config/config.go:600`):

```yaml
oidc_auth:
  enable: true
  issuer_url: http://casdoor:8000
  client_id: <weknora-app client id>
  client_secret: <secret>
  user_info_mapping:
    username: preferred_username
    email: email
```

Casdoor 建号即认领已有逻辑:`LoginWithOIDC` → `provisionOIDCUser`(`internal/application/service/user.go:561`),自动建号含随机密码与 `OidcOnlyLogin` 标记。

### 3.2 微信小程序静默通道

新端点 `POST /api/v1/auth/wechat/login`,入参 `{ "code": string }`,加入 `noAuthAPI` 白名单(`internal/middleware/auth.go:44`),注册于 `internal/router/routes_auth_tenant.go`。

流程:

1. 后端调微信 `code2session`(appid + app_secret)拿 `openid`(+unionid)
2. 查本地 `User.Profile.WeChatOpenID`(新增字段):
   - **无**:经 Casdoor 管理 API 建用户(`wx_<openid 前 8 位>`,邮箱占位 `wx_<openid>@wechat.local`);生成 64 字节**服务密码**;AES-GCM 加密落库;标记 `OidcOnlyLogin`
   - **有**:直接进入下一步
3. ROPC(`POST /api/login/oauth/access_token`,`grant_type=password`)向 Casdoor 换 access_token → 拉 userinfo → **复用** `LoginWithOIDC` 后半段(`resolveOIDCUserInfo` + 用户认领/签发,`user.go:547`)
4. 响应结构与现有 `/auth/login` 同构(access/refresh/memberships),小程序零适配成本

配置新增:

```yaml
wechat_mp:
  appid: <小程序 appid>
  app_secret: <secret>
```

Casdoor 服务账号凭据(组织名、用户名、密码)经 env 注入,用于管理 API 建号与密码重置。

**ROPC 失败兜底**:经管理 API 重置服务密码,重试一次。

### 3.3 登录模式端点

前端已调 `client.auth.oidcConfig()`(`apps/web/src/auth/LoginPage.tsx:131`,现有返回 `enabled` + `providerDisplayName`)。为其增加只读字段 `sso_only`(`oidc_auth.enable && auth.sso_only`,新配置项,默认 false),前端据此隐藏账密表单。

## 4. 前端改动

### 4.1 React web(apps/web)

登录页在 `oidc.enabled && sso_only` 时隐藏账密表单;SSO 按钮已有,其余零改动。

### 4.2 Taro 小程序(apps/miniprogram)

- `AuthPort` 增加 `wxLogin()`(`apps/miniprogram/src/core/auth.ts:5`)
- 启动时静默 `wx.login()` → 调新端点 → 存 `BearerCredential`(复用现有持久化与 `AuthCoordinator` 会话机制)
- 静默登录失败显示重试页;移除账密登录表单

## 5. 错误处理

- **Casdoor 不可达**:web 显示已有 `auth.oidcLoginFailed` 文案;小程序静默登录失败 → 重试页
- **code2session 失败**(code 过期等):返回明确错误码,小程序重新 `wx.login`
- **并发首登竞态**:Casdoor 建号冲突 / 本地关联写入冲突时幂等重查
- **服务密码加密密钥**:env `WECHAT_MP_SECRET_KEY`;解密失败视为密码失效 → 走管理 API 重置

## 6. 测试

- **后端单测**(mock code2session + Casdoor client):首登建号、复登、ROPC 失败兜底三条路径
- **React**:`login-page.test.tsx` 补 `sso_only` 渲染分支
- **小程序**:对齐现有渲染级测试 pattern
- **手动联调清单**:compose 起 Casdoor → web SSO 全流程 + 小程序静默全流程

## 明确不做(YAGNI)

- Casdoor 组织/角色与 WeKnora 租户/权限的同步映射
- App 端实现(后端 PKCE 通路已备)
- 存量用户迁移
- 修改 Casdoor 源码

## 关键文件索引

| 位置 | 内容 |
|---|---|
| `internal/config/config.go:600` | `OIDCAuthConfig`,新增 `wechat_mp`、`sso_only` |
| `internal/application/service/user.go:435-560` | OIDC 既有实现,wechat 通道复用其后半段 |
| `internal/middleware/auth.go:44` | `noAuthAPI` 白名单 |
| `internal/router/routes_auth_tenant.go:205` | 认证路由注册 |
| `internal/types/user.go:93` | User 模型,新增 `Profile.WeChatOpenID` |
| `docker-compose.yml` | 新增 `casdoor` 服务 |
| `apps/web/src/auth/LoginPage.tsx` | sso_only 隐藏账密表单 |
| `apps/miniprogram/src/core/auth.ts:5` | `AuthPort.wxLogin()` |
