# jira-todo-mcp — WeKnora 自托管只读 Jira 示例插件

这是一个可独立部署的远程 MCP 插件示例：提供单一工具 `search_my_week_issues`，
查询**当前授权成员本人**本周（`due >= startOfWeek()`）未解决的 Jira 待办事项。
它是 WeKnora「自行托管插件」体系的参考实现，演示完整的插件契约：

- **MCP 目录公开、执行鉴权**：未认证的 `initialize` / `ListTools` 可用（空间管理员
  粘贴清单地址做安装前核验时无需个人授权）；`CallTool` 必须携带成员个人
  OAuth Bearer token，否则返回 `401` +
  `WWW-Authenticate: Bearer resource_metadata=<base>/.well-known/oauth-protected-resource`
  （RFC 9728，WeKnora 客户端据此自动发起 OAuth 流）。
- **工具 schema 固定无参数**：输入 schema 恒为
  `{"type":"object","properties":{},"additionalProperties":false}`——模型无法传入
  `token` / `user_id` / `jql` / `url` 等任何参数，JQL 由服务端固定模板构造。
- **简化 OAuth 2.0 授权码服务器**（内置）：PKCE `S256` + 动态客户端注册
  （`POST /register`）+ RFC 8414/9728 元数据。成员在授权页输入 Jira 邮箱与
  API token，经 `/rest/api/3/myself` 验证后签发一次性授权码；凭据仅驻留内存，
  **不落盘、不写日志、不回显**。
- **自托管清单**：`GET /manifest.json` 每次请求由代码动态序列化（digest 由
  `internal/modules/plugins.ToolSchemaDigest` 计算，源码不写死字面量）。

## 工具

| 名称 | 说明 |
| --- | --- |
| `search_my_week_issues` | 查询授权成员本周未解决的 Jira 待办；只读（`read_only: true`），需个人授权（`requires_personal_auth: true`） |

输出为文本行数组，每行格式（缺失字段省略对应段）：

```
[{key}] {summary} · 状态 {status} · 截止 {due} · {url}
```

服务端固定 JQL（不接受任何输入拼接）：

```
assignee = currentUser() AND resolution = Unresolved AND due >= startOfWeek() AND due < startOfWeek("+1w") ORDER BY due ASC
```

## 自托管部署

### 1. 环境变量

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `PLUGIN_JIRA_BASE_URL` | ✅ | Jira REST API base URL（如 `https://your-company.atlassian.net`）。**缺失时服务拒绝启动（fail-closed），不猜测默认值。** |
| `PLUGIN_BASE_URL` | ❌ | 本服务对外可达的 base URL（如 `https://plugins.your-company.com`）。用于 OAuth 元数据、`WWW-Authenticate` 与清单端点；缺省用监听地址拼接。 |
| `PLUGIN_LISTEN_ADDR` | ❌ | 监听地址，默认 `:8020`。 |

> 本服务对 Jira 的出站请求统一走 SSRF-safe HTTP client（仅 http/https，拒绝
> 环回/私有/保留地址）。自托管内网 Jira 需经 `SSRF_WHITELIST` 显式放行。

### 2. 构建/运行

```bash
# 仓库根目录
go build -o jira-todo-mcp ./examples/plugins/jira-todo-mcp

PLUGIN_JIRA_BASE_URL="https://your-company.atlassian.net" \
PLUGIN_BASE_URL="https://plugins.your-company.com" \
PLUGIN_LISTEN_ADDR=":8020" \
./jira-todo-mcp
```

成员个人的 Jira API token（[创建指引](https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/)）
只在授权页运行时输入，或经你自行管理的密钥服务注入——本仓库的源码、示例与
测试不包含任何可用凭据字面量。

### 3. 清单地址

部署完成后，清单 URL 即：

```
https://<你的部署主机>/manifest.json
```

空间管理员在 WeKnora「插件」设置中粘贴该地址即可预览并安装。`manifest.json`
仓库副本仅是部署参考（endpoint 为占位符）；运行时以 `/manifest.json` 路由的
动态输出为权威——`input_schema_digest` 与 `content_digest` 均由代码实时计算。

## 开发者约定：版本端点稳定性

清单描述**一个**版本；已发布版本的 MCP 端点（`/mcp`）与清单 URL 必须在该版本
生命周期内保持可用。发布新版本时：把新版本部署在不同路径或不同实例上
（例如 `https://plugins.example.com/v1.1.0/`），旧版本端点继续运行，已安装旧
版本的空间不受影响，直到管理员显式审阅并接受新版本。

## 信任边界声明

WeKnora 平台在安装前核验的是**可观察的目录**（`ListTools` 的工具集与 schema
digest）；它无法、也不承诺锁定远端服务的内部代码行为。运行时的保障来自：
工具快照核验（目录/schema 漂移即阻断）+ 写工具默认关闭 + 成员审批闸门 +
本示例这样的最小只读面。部署第三方插件前请自行审查其源码与出站行为。

## 与 OAuth 相关的端点集

| 端点 | 用途 |
| --- | --- |
| `GET /.well-known/oauth-protected-resource` | RFC 9728：`{"authorization_servers": ["<base>"]}` |
| `GET /.well-known/oauth-authorization-server` | RFC 8414：`authorization_endpoint` / `token_endpoint` / `registration_endpoint` / `grant_types: ["authorization_code","refresh_token"]` |
| `POST /register` | RFC 7591 动态客户端注册，返回 `client_id`；注册文档必须携带至少一个绝对 http(s) `redirect_uris`（`/authorize` 只接受与注册值精确匹配的 `redirect_uri`，RFC 6749 §3.1.2.3） |
| `GET /authorize` | 授权表单（Jira 邮箱 + API token → 验证 `myself` → 302 `redirect_uri?code=...&state=...`，PKCE `S256`；凭据输错可用同一 `state` 重试，`state` 与授权码 10 分钟有效） |
| `POST /token` | `grant_type=authorization_code` 换 access token；`refresh_token` 续期。access token 有效期 1 小时（与 `expires_in` 一致），refresh 轮换后旧 access token 在其有效期内继续可用 |

内置 OAuth 实现是**内存态教学级实现**（单实例、无持久化）：适合示例、测试与
小规模自托管评估。多实例或生产场景请置于专用 OAuth 提供方之后（或在单实例
反向代理后运行），并自行评估会话存储与吊销需求。

## 测试

```bash
go test ./examples/plugins/jira-todo-mcp/ -v
```

测试全部使用内嵌 fake Jira（`httptest`），不访问真实 Atlassian 服务。
