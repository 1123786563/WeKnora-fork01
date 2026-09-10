# T18 Embed、IM 与外部集成运行证据（2026-09-11）

## 环境边界

- 使用隔离 Lite Go 服务：`DB_DRIVER=sqlite`、临时 SQLite 数据库、临时文件目录、无生产数据/凭证。
- 服务监听 IPv6 `::1:8080`；现有本机 IPv4 `127.0.0.1:8080` 被另一个进程占用，因此探针显式使用 `http://[::1]:8080`。
- Lite 服务启动成功；已知环境警告仍存在：无 Redis 时 Embed session token exchange 返回不可用、SQLite 缺少 FTS5、Lite migration 未包含 `tenant_skills` 表。

## 真实后端结果

以临时 owner 登录并使用 `X-Tenant-ID: 1`：

| 旅程 | 结果 | 证据 |
|---|---|---|
| `GET /api/v1/auth/me` | HTTP 200 | React 运行时可从 `data.tenant` 读取当前空间；响应没有主用户秘密字段 |
| `GET /api/v1/embed-channels` | HTTP 200 | 初始空列表，之后返回创建的 Embed 渠道 |
| `GET /api/v1/im-channels` | HTTP 200 | 初始 `data: null`，之后返回创建的 IM 渠道摘要 |
| `POST /api/v1/agents/builtin-quick-answer/embed-channels` | HTTP 201/业务 success | 创建 `allowed_origins=https://shop.example.test` 的 Embed 渠道；明文 publish token 仅出现在一次性创建响应，未写入 React 管理列表 |
| `POST /api/v1/agents/builtin-quick-answer/im-channels` | HTTP 200/业务 success | 创建 Feishu 渠道；管理列表不依赖凭证明文 |
| `PUT /api/v1/embed-channels/:id` | HTTP 200 | 使用完整非秘密字段更新 Embed 名称；随后列表保留 origin、欢迎语、限流、开关等字段 |
| `POST /api/v1/im-channels/:id/toggle` | HTTP 200 | 渠道从 enabled 切换为 disabled |
| `GET /api/v1/embed/:id/config` + `Authorization: Embed ...` | HTTP 200 | 匿名 Embed token 可读取公开配置；没有 Bearer 或 Cookie |
| Embed exchange + 允许 origin | HTTP 503 | 服务端明确返回 `session tokens unavailable`，对应 Lite/no-Redis 能力边界；未伪造访客会话成功 |
| Embed exchange + `https://evil.example.test` | HTTP 403 | 服务端明确返回 `origin not allowed` |
| `PUT /api/v1/tenants/1/api-principal-config` | HTTP 200 | Owner 将 Principal 模式设置为 `signed_token`；响应只返回 `has_hmac_secret=true` |
| `POST /api/v1/tenants/1/api-principal-test-token` | HTTP 200 | 生成 900 秒短期外部用户 token；记录了 header 名称和 TTL，不记录 token 内容 |

本次未将真实第三方 IM 回调、外部模型、有效 SSE 问答或真实代理 iframe 记为通过：隔离 Lite 没有平台凭证/模型，也因 session token 服务不可用无法完成匿名访客聊天。

## React 静态/契约证据

- `pnpm test:shared`：151/151，通过；含 API Principal 路由和 Embed 更新字段保护测试。
- `pnpm test:web`：61/61，通过。
- `pnpm typecheck:shared`、`pnpm typecheck:web`、`pnpm build:web`：退出码 0。
- `pnpm test:embed`：3/3，通过；`pnpm typecheck:embed`、`pnpm build:embed`：退出码 0。
- `node scripts/check-react-boundaries.mjs`、`git diff --check`：退出码 0。

## 发现并修复的更新契约风险

后端 Embed `PUT` handler 的结构体更新语义会把未提供的字段变成零值；React 之前若只发送 `{name}` 会清空 origin/welcome/config。管理页现在从服务端列表构造完整非秘密更新 payload，并在缺少 origin allowlist 时 fail-closed；secret/webhook secret 不进入编辑 payload。该行为由 `packages/views/src/integrations/form.test.ts` 覆盖。

结论：T18 的独立 Embed 构建、匿名凭证隔离、origin 拒绝、管理 CRUD、IM toggle、Principal 配置/短 token 与可执行 SSE playground 已有实现和隔离后端证据；第三方代理、有效 Embed session/SSE、IM 平台回调、viewer/admin 负向矩阵仍保持 `review`。
