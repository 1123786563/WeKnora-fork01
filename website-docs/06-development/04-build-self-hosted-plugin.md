# 自行开发并接入 WeKnora 插件

本指南说明**当前代码已经支持的路径**：开发者自行部署远程 MCP 服务，空间管理员把它登记为 MCP 服务，成员按需完成个人 OAuth 授权，Agent 在对话中调用工具。未来的“粘贴插件清单地址安装”见[产品设计](../../docs/specs/2026-09-23-self-hosted-plugins-design.md)；当前版本尚无清单安装、版本锁定和升级确认功能。

## 先确定一个可验证的工具

以 `jira_list_my_week_issues` 为例：当前成员在对话中问“我本周有哪些 Jira 待办？”，工具只查询**该成员**有权访问的 Jira 事项，返回事项标识、标题、状态、截止日期和可打开链接。工具输入可以限定项目和本周的时间范围，但**不能接受 Jira 账号 ID、访问令牌或任意目标 URL 作为模型参数**。服务端从已授权身份推导 Jira 用户，并在 Jira API 层执行权限检查。

开发者可使用任意支持远程 MCP 的 SDK 实现该服务。最小交付物是：

1. 一个可从 WeKnora 后端网络访问的 Streamable HTTP 或 SSE MCP 端点；不要交付本机 `stdio` 命令。当前 WeKnora MCP 客户端支持前两种传输，不支持 `stdio`。
2. `ListTools` 能发现稳定命名、带描述和 JSON Schema `inputSchema` 的只读工具；`CallTool` 对合法输入返回结构化、可读的结果，对未授权、超时和上游错误返回明确错误。
3. 用于成员个人授权的 MCP OAuth 2.0 流程。MCP 服务须提供 WeKnora 客户端能发现和使用的受保护资源与授权服务器元数据，支持授权码、PKCE、动态客户端注册及所需 scope。MCP 服务再将该成员的授权映射到其 Jira 账号。**仅在插件服务内部实现 Jira OAuth，未向 MCP 客户端暴露兼容授权流程，不足以触发 WeKnora 的个人授权体验。**
4. 由开发者维护的服务地址和运行环境。按已确认的产品目标，正式版本应有独立且稳定的端点；当前 WeKnora 只保存 URL，无法防止该 URL 背后的行为被直接替换。

一个可用于 `ListTools` 的输入 schema 示例：

```json
{
  "name": "jira_list_my_week_issues",
  "description": "读取当前已授权成员本周的 Jira 待办，不修改工单",
  "inputSchema": {
    "type": "object",
    "properties": {
      "project_key": { "type": "string", "description": "可选的 Jira 项目键" }
    },
    "additionalProperties": false
  }
}
```

本周的时区、起止日和 Jira 查询语义由服务端统一确定并写入工具说明；不要让模型传任意 JQL 代替受控查询。返回值至少包含匹配范围与每项事项的 Jira ID、标题、状态、截止日期和 URL；无结果时返回空列表，不虚构事项。

仓库中的 [`mcp-server/`](../../mcp-server/) 是 **WeKnora 对外提供 MCP 工具** 的参考实现，不是这个 Jira 插件的模板或现成 Jira 连接器。协议、认证和 Agent 接入的现状见 [MCP 功能说明](../03-features/08-mcp.md)。

## 本地验证顺序

1. 在插件服务自身测试工具发现、合法查询、无授权、授权过期、Jira 403、Jira 超时、空结果和跨用户隔离。用两个不同 Jira 成员账号验证相同输入不会串用凭据。
2. 从 WeKnora **后端部署环境**验证 MCP 端点可达，TLS 与 OAuth 元数据地址可访问。浏览器能访问并不能证明后端能访问。
3. 用空间管理员身份在“设置 → MCP 服务”新建服务，传输选择 `http-streamable` 或 `sse`，填入端点 URL，认证方式选择 OAuth。可以先保存连接，再补充 `usage_instructions`。
4. 运行“测试连接”，核对发现的工具名、描述与参数 schema；为服务填写用途说明，并检查工具启停与审批设置。
5. 在 Agent 配置中选择该 MCP 服务或工具。现有 Agent 的 `mcp_selection_mode` 可为 `all`、`selected`、`none`；若为 `none`，对话不会注册该工具。
6. 用尚未授权的普通成员提问，完成会话内 OAuth，再确认查询只返回本人的 Jira 事项；用第二位成员重复验证。最后撤销其中一位成员的授权，确认其调用需要重新授权且不影响另一位成员。

管理员也可以调用现有 API 完成第 3–4 步。下面只展示服务登记和工具检查的请求形状；`BASE` 是 WeKnora API 根地址，`TOKEN` 是具有空间管理员权限的 WeKnora 令牌，`SERVICE_ID` 来自创建响应：

```bash
curl -X POST "$BASE/api/v1/mcp-services" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"jira-my-issues","transport_type":"http-streamable","url":"https://plugins.example.com/jira/v1/mcp","auth_config":{"auth_type":"oauth"},"usage_instructions":"查询当前成员有权访问的 Jira 待办；不得用于修改工单。"}'

curl -X POST "$BASE/api/v1/mcp-services/$SERVICE_ID/test" \
  -H "Authorization: Bearer $TOKEN"

curl "$BASE/api/v1/mcp-services/$SERVICE_ID/tools" \
  -H "Authorization: Bearer $TOKEN"
```

如需临时禁用某个已发现的工具，管理员可调用 `PUT /api/v1/mcp-services/:id/tool-approvals/:tool_name`，提交 `{"enabled":false}`；需要逐次成员审批时提交 `{"require_approval":true}`。这只能处理**已知工具**，不能代替未来插件版本的“新写工具默认关闭”规则。

个人 OAuth 授权使用 `POST /api/v1/mcp-services/:id/oauth/authorize-url`，`redirect_uri` 必须指向 WeKnora 后端的 `/api/v1/mcp-oauth/callback`；完成后可用 `GET /api/v1/mcp-services/:id/oauth/status` 检查本人状态。具体请求、权限和返回值以 [MCP API 文档](../04-api/02-api-agent-mcp.md) 为准。

## 当前路径的产品边界

- 这一步登记的是 **MCP 服务**，不是可通过清单安装的版本化插件；管理员需手工配置地址、认证与 Agent 选择。
- 当前未知工具的策略默认是 `enabled=true`、`require_approval=false`。如果开发者在同一端点增加写工具，现有配置不会自动让它默认关闭。设计目标要求升级前冻结版本，并由管理员显式接受、逐项开启写操作；在该能力实现前，不要把写工具部署到已连接的只读端点。
- OAuth 令牌按成员身份隔离；空间安装和成员授权是两个不同步骤。当前 MCP 服务并不自动提供完整的“应用安装 → 成员可见 → 个人连接”产品流程。
- 如果插件只提供无需外部账号的计算或查询能力，可以省去个人 OAuth，但仍按空间管理员登记和 Agent 工具选择流程接入。
