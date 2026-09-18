# api-facts.md · WeKnora Go 后端已核验 API 事实（移动端视角）

核验日期：2026-09-18（只读源码调研，子代理产出、主线复核关键项后使用）。基础路径 `/api/v1`，gin（internal/router/router.go:232）。

## 1. 认证（internal/router/routes_auth_tenant.go:206-236）
- POST `/auth/login` `{email,password}` → `{success,message,user,active_tenant,memberships,token,refresh_token}`（internal/handler/dto/auth.go:6-14）
- POST `/auth/refresh` `{refreshToken}` → `{success,access_token,refresh_token}`
- GET `/auth/me`、PUT `/auth/me/preferences`（last_active_tenant_id）、POST `/auth/logout`、POST `/auth/switch-tenant`
- OIDC：GET `/auth/oidc/config|url|callback|start`、POST `/auth/oidc/exchange`、POST `/auth/mobile/exchange`（`{code,state,redirect_uri,code_verifier}`）；/auth/oidc/url 支持 code_challenge PKCE

## 2. 空间/成员
- GET/POST `/tenants`；`/tenants/:id/members`；个人邀请收件箱 GET `/me/invitations`、`/pending-count`、POST `/:inv_id/accept|decline`
- 切换空间：POST `/auth/switch-tenant` 或 `X-Tenant-ID` header（internal/middleware/auth.go:302-318）

## 3. 会话/消息（internal/router/routes_chat.go）
- `/sessions` POST/GET（列表 `{sessions,total,...}`）、`/:id` GET/PUT/DELETE、`/:session_id/generate_title|stop|steer|pin`、`/:session_id/attachments` POST
- GET `/messages/:session_id/load`、POST `/messages/search`
- 发送：POST `/knowledge-chat/:session_id`、POST `/agent-chat/:session_id`；请求体字段（internal/handler/session/types.go:47-66）：`query, knowledge_base_ids, agent_enabled, agent_id, web_search_enabled, mcp_service_ids, skill_names, mentioned_items, images(base64), attachment_uploads, attachment_ids, channel`
- 断线续流：GET `/sessions/continue-stream/:session_id?message_id=`

## 4. Agent（routes_agent.go）
GET `/agents`（含 builtin）、GET `/agents/:id`、`/suggested-questions` 等。

## 5. Run/执行/审批（routes_workbench.go；agent_run.go）
- POST `/workbench/executions` body `{session_id,agent_id,target_id,workspace_ref,request_id,text,budget_upper}`（internal/application/service/workbench/admission.go:26-34）→ 202 `{data:{run_id,request_id,status}}`
- GET `/workbench/executions/:run_id`、`/snapshot`、`/events`(SSE)、`/requests/:request_id`（对账）、`/:run_id/interactions`、POST `/interactions/:id/decisions`、POST `/:run_id/commands`（cancel/steer）
- 交互 kind（internal/workbench/interaction.go:13-17,37-41）：`tool_approval(approve/reject)`、`budget(extend)`、`recovery(retry/provide_result/terminate)`
- 会话内 Run：GET `/sessions/:id/runs/:run_id[/events|/decisions|/cancel]`；决策体 `{pending_id,decision_id,tool_call_id,expected_revision,action,args_hash,resource_ref,reason,result}`
- `/execution-targets` GET/POST、`/execution-workspaces/:id`
- MCP 工具审批：GET `/mcp-services/:id/tool-approvals`、PUT `.../:tool_name`；会话内 POST `/agent/tool-approvals/:pending_id`；OAuth POST `/agent/mcp-oauth-resolutions/:pending_id[/cancel]`

## 6. SSE 帧格式（关键）
- 聊天流（POST knowledge-chat/agent-chat）：帧 `event: message`，data=types.StreamResponse：`id,response_type,content,done,knowledge_references,session_id,assistant_message_id,tool_calls,data,usage,finish_reason`；response_type 枚举含 answer/references/thinking/tool_call/tool_result/error/reflection/session_title/agent_query/complete/stop/artifacts_pending/tool_approval_required(_resolved)/mcp_oauth_required(_resolved)/memory_recalled/steer/user_message_injected/context_compacted/install_prompt（internal/types/chat.go:215-300）
- Run 事件流（agent_run.go:131-232）：游标 `Last-Event-ID` 头或 `?after=`；每帧 event: 为事件 seq 数字；`event: run` `{run_id,session_id,status,wait_reason,revision,epoch,seq,capabilities,pending_id}`；15s `event: keepalive` `{"seq":N}`；`event: error` `{code,message,seq}`；游标过期 HTTP 409 `{"error","code":"cursor_expired"}`；`?once=1` 单页
- Workbench 事件流（workbench_read.go:164-236）：同 Last-Event-ID/seq watermark/409 cursor_expired

## 7. 鉴权
- 三通道：embed session / `Authorization: Bearer <JWT>` / `X-API-Key`（middleware/auth.go:119-190）；无 cookie 会话
- JWT access 24h、refresh 7d（application/service/user.go:1080-1099）
- 401 `{error:"Unauthorized: ..."}`、403 非空间成员（auth.go:176-178,249-250）

## 8. 错误 envelope（三形态都要兼容）
- 统一 `{success:false,error:{code,message,details}}`（middleware/error_handler.go:25-32）
- auth 401/403 裸 `{error:"string"}`
- workbench 启动错误 `{success:false,error:"string"}`

## 9. 知识/连接器/用量
- 知识：`/knowledge-bases` CRUD、POST `/:id/file`（multipart 字段 `file`）、`/url`、`/manual`、`/knowledge/:id` download/preview
- 连接：`/apps/installations`、`/apps/catalog`、`/apps/connections` GET/POST、`/:id/revoke`、`/:id/authorization-attempts`、`/apps/oc/actions/prepare`、`/apps/actions` prepare/get/`/:id/approve|execute`
- 用量：GET `/commercial/summary|plans|usage|orders`、POST `/commercial/quotes`、`/commercial/tasks/:id/budget/extend`

## 10. OpenAPI
`docs/swagger.yaml` / `docs/swagger.json`（swaggo，284 path）存在。

## 11. 移动端缺口（差异，见 decisions D-04/05/06/07）
- 无 `/workbench/overview`、无通用通知/push inbox、无统一四类交互 kind、无实时语音端点。
