# Happy 统一 Agent 会话 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付无需选择知识库的 Agent 会话，以及发送、审批、追加和可恢复的原生交互。

**Architecture:** Happy 原生交互通过移动适配层消费共享 contracts/api-client/domain；产品身份、会话与权限仍由 WeKnora 后端管理。本册只实现下列任务的责任范围，沿用总计划的文件归属和依赖。

**Tech Stack:** Expo 55、React Native 0.83.1、React 19.2.0、TypeScript、pnpm、Vitest/Node test，服务增量沿用 Go/Gin 与现有数据库。

**Spec:** [已确认方向与细化规格](../specs/2026-09-10-happy-agent-mobile-design.md)；[总计划、接口归属与执行规则](2026-09-10-happy-agent-mobile.md)。所有新增路径、类型和端点均是实施目标，不代表当前已经存在。

## Global Constraints

- 移动不导入 DOM `ui/views` 或 Web `core`。
- REST/SSE/文件端点只在共享 SDK 定义；原生 transport 承担 POST、header、增量读取和取消。
- 会话可跨轮切换 Agent。发送时记录实际 Agent；历史按消息的 Agent 标识展示，不能随当前选择回写旧消息。
- 前端能力声明用于展示，不授予权限。
- 未知状态先对账，禁止自动重复付费执行。
- 只有矩阵全部验收或用户明确变更范围，才能声称完整保留 Happy 移动能力。
- Happy 基线 `ac64b9b4677870f7b7a9eacfd0780959229717f1`，源文件从仓库归档引入；保留许可记录，不引用相邻工作目录。
- 全部全局约束还包括总计划 Global Constraints；执行者必须同时读取。每项运行证据区分 fixture、真实后端、原生运行。

---

## 文件与接口责任

H07 定义 Agent DTO；H08 定义会话/消息 DTO；H09 是唯一 Happy Message 投影；H10/H11 分别拥有发送与恢复；H12/H13 使用共享命令。避免每个页面再维护一份 session store。每个共享 facade 消费现有 `WeKnoraClient['request']`，在包 `index.ts` 导出。H08 的产品消息不继承 Happy 解密消息类型。

### H07：Agent 列表、详情和能力展示

**依赖：** H05。**Files:** Create `packages/contracts/src/mobile/agent.ts`、`packages/api-client/src/agents/index.ts`、`index.test.ts`；Create `apps/mobile/sources/weknora/agents/AgentPicker.tsx`、`capabilities.ts`。

**Interfaces:** `AgentSummary {id:string; name:string; preset:string; mode:string}`；`createAgentsApi(request).list():Promise<AgentSummary[]>`、`.get(id:string):Promise<AgentSummary>`；能力 `Capability = {enabled:true}|{enabled:false;reason:string}`，先按实际配置映射，不虚构 capability API。

- [ ] **Step 1：写无 KB custom Agent 仍可列出的测试。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createAgentsApi } from './index.ts';
test('custom agent does not need a knowledge base',async()=>{
 const api=createAgentsApi(async()=>({success:true,data:[{id:'a',name:'General',config:{agent_type:'custom',mode:'smart-reasoning'}}]}));
 assert.equal((await api.list())[0].preset,'custom');
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/api-client/src/agents/index.test.ts`。
- [ ] **Step 3：按 handler fixture 归一化 DTO 并连接选择器。**

```ts
export interface AgentSummary {id:string;name:string;preset:string;mode:string}
export function parseAgent(v:unknown):AgentSummary {
 if(!v || typeof v!=='object')throw new Error('INVALID_AGENT');
 const a=v as Record<string,unknown>, c=a.config as Record<string,unknown>|undefined;
 if(typeof a.id!=='string'||!a.id||typeof a.name!=='string')throw new Error('INVALID_AGENT');
 return {id:a.id,name:a.name,preset:typeof c?.agent_type==='string'?c.agent_type:'custom',mode:typeof c?.mode==='string'?c.mode:'normal'};
}
```

facade 使用 GET `/api/v1/agents`、GET `/api/v1/agents/:id`，严格检查 success/data。选择器保留 Happy 切换入口和模型菜单，移除 `claude|codex` 决定产品可选 Agent 的限制；显式区分 product Agent 与 H24 remote provider。权限被撤销时清空选择并展示后端错误，不能选择本地缓存绕过授权。

- [ ] **Step 4：GREEN。** 同一命令，增加非法 envelope、删除的 Agent、403 和空列表用例；真机选择无 KB Agent，无空间时引导加入空间而非显示所有 Agent。
- [ ] **Step 5：提交。** stage 本任务文件与 exports；`git commit -m "feat(mobile): expose authorized product agents"`。

### H08：会话、历史与每轮 Agent 归属

**依赖：** H07。**Files:** Create `packages/contracts/src/mobile/conversation.ts`、`packages/api-client/src/sessions/index.ts`、`index.test.ts`；Create `apps/mobile/sources/weknora/conversation/session-list.ts`、`SessionListScreen.tsx`。

**Interfaces:** `ProductMessage {id:string; role:'user'|'assistant'; content:string; agentId:string|null; createdAt:number}`；`ProductSession {id:string;title:string}`；`SessionPage {items:ProductSession[];total:number;page:number;pageSize:number}`。`createSessionsApi(request)` 产出 `list(page,pageSize)`, `get(id)`, `create(title)`, `rename(id,title)`, `remove(id)`, `pin(id,pinned)`, `history(id,beforeTime?:string)`；history 返回 `ProductMessage[]`。

- [ ] **Step 1：写历史 Agent 不受当前选择影响的测试。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseProductMessage } from './index.ts';
test('message captures its own agent',()=>{
 const msg=parseProductMessage({id:'m',role:'assistant',content:'answer',agent_id:'old',created_at:'2026-09-10T00:00:00Z'});
 assert.equal(msg.agentId,'old');
 assert.equal(msg.id,'m');
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/api-client/src/sessions/index.test.ts`。
- [ ] **Step 3：实现严格产品 DTO 与实际路由。**

```ts
export function parseProductMessage(v:unknown):ProductMessage {
 if(!v||typeof v!=='object')throw new Error('INVALID_MESSAGE');
 const m=v as Record<string,unknown>;
 const createdAt=typeof m.created_at==='string'?Date.parse(m.created_at):NaN;
 if(typeof m.id!=='string'||(m.role!=='user'&&m.role!=='assistant')||typeof m.content!=='string'||!Number.isFinite(createdAt))throw new Error('INVALID_MESSAGE');
 return {id:m.id,role:m.role,content:m.content,agentId:typeof m.agent_id==='string'?m.agent_id:null,createdAt};
}
```

将接口 type 从 contracts 导入。使用 `/sessions?page=&page_size=` 的实际 pagination envelope；创建 `{title}` 无 KB 字段；历史 `/messages/:session_id/load?limit=...&before_time=...` 按实际 envelope 解析。扩展思考/工具/附件时只在 contracts 添加可选字段，不丢弃其原始语义。列表的 Agent 筛选不可直接传只支持 IM 的参数；首版仅使用后端现有可靠筛选，不对未加载页做伪全量搜索。

- [ ] **Step 4：GREEN 与列表交互。** 增加两页去重、相同时间戳边界、空标题、rename失败回滚、删除后返回列表、pin/unpin 与403测试。记录真实历史顺序与 Vue 同一会话一致。
- [ ] **Step 5：提交。** stage 本任务路径及 exports；`git commit -m "feat(mobile): connect product sessions and historical attribution"`。

### H09：把 Happy 消息展示从全局 sync 解耦

**依赖：** H08。**Files:** Create `apps/mobile/sources/weknora/conversation/project-message.ts`、`project-message.test.ts`、`ConversationProvider.tsx`；Modify `apps/mobile/sources/components/ChatList.tsx`、`sources/-session/SessionView.tsx`。

**Interfaces:** `projectMessage(message:ProductMessage):Message`，Message 是 Happy `sources/sync/typesMessage.ts`；`ConversationView {messages:Message[];hasMoreOlder:boolean;isLoadingOlder:boolean;loadOlder():Promise<void>}` 由新 provider 提供。Agent 归属单独保存 `Record<messageId,agentId|null>`，不能塞入 Happy 的 provider 名称。

- [ ] **Step 1：测试产品消息稳定投影、ID 不由数组位置生成。**

```ts
import {expect,test} from 'vitest';
import {projectMessage} from './project-message';
test('projection keeps message identity across pagination',()=>{
 expect(projectMessage({id:'m',role:'assistant',content:'你好',agentId:'a',createdAt:1}))
  .toEqual({kind:'agent-text',id:'m',localId:null,text:'你好',createdAt:1});
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/conversation/project-message.test.ts`。
- [ ] **Step 3：实现投影并抽出原有列表的 provider seam。**

```ts
import type {ProductMessage} from '@weknora/contracts';
import type {Message} from '@/sync/typesMessage';
export function projectMessage(m:ProductMessage):Message {
 return {kind:m.role==='user'?'user-text':'agent-text',id:m.id,localId:null,text:m.content,createdAt:m.createdAt};
}
```

ChatList 增加可选 `view:ConversationView`；原 Happy remote wrapper 仍可从旧 store 取值，产品 wrapper 显式传 view。不要用条件 hook：两个 wrapper 分开，底层展示接 props。保留 ChatListInternal 的倒序虚拟列表、顶部 inset、底部 composer 避让、滚动按钮和 tool renderer。工具投影按 Happy ToolCall 的 state/permission 字段映射，不能把未批准工具显示成 completed。

- [ ] **Step 4：GREEN 与原生视觉对照。** 测试用户/助手/思考/运行中工具/待审批至少5种消息；真机长列表加载更早消息不跳到底部、键盘打开不会遮挡输入。对照 H01 固定基线记录差异，不只验证输出对象。
- [ ] **Step 5：提交。** stage 本任务路径；`git commit -m "refactor(mobile): inject product conversation into Happy presentation"`。

### H10：发送、发送中输入和停止

**依赖：** H06/H09。**Files:** Create `packages/api-client/src/chat/commands.ts`、`commands.test.ts`；Create `apps/mobile/sources/weknora/conversation/send.ts`、`send.test.ts`；Modify `SessionView.tsx` 的 AgentInput 接线。

**Interfaces:** `SendInput {sessionId:string;agentId:string;query:string;mode:'knowledge'|'agent';knowledgeBaseIds:string[]}`；`buildSend(input):ClientRequest`；`buildStop(sessionId,messageId):ClientRequest`。`sendOnce(input,open):Promise<void>` 的 open 是 `(request:ClientRequest)=>Promise<void>`，允许任务在测试中记录请求。生命周期状态 `idle|sending|streaming|reconciling|failed`，后端完成状态由事件决定。

- [ ] **Step 1：测试非 KB Agent 请求和停止目标。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {buildSend,buildStop} from './commands.ts';
test('agent send does not require a knowledge base',()=>{
 const r=buildSend({sessionId:'s',agentId:'a',query:'hello',mode:'agent',knowledgeBaseIds:[]});
 assert.equal(r.path,'/api/v1/agent-chat/s');
 assert.equal((r.body as {agent_id:string}).agent_id,'a');
 assert.deepEqual(buildStop('s','m').body,{message_id:'m'});
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/api-client/src/chat/commands.test.ts`。
- [ ] **Step 3：实现命令与发送状态门。**

```ts
import type {ClientRequest} from '../client.ts';
export function buildSend(i:SendInput):ClientRequest {
 if(!i.query.trim()||!i.agentId)throw new Error('INVALID_SEND');
 return {method:'POST',path:`/api/v1/${i.mode==='agent'?'agent-chat':'knowledge-chat'}/${encodeURIComponent(i.sessionId)}`,
  headers:{accept:'text/event-stream','content-type':'application/json'},
  body:{query:i.query,agent_id:i.agentId,agent_enabled:i.mode==='agent',knowledge_base_ids:i.knowledgeBaseIds,channel:'mobile'}};
}
export function buildStop(sessionId:string,messageId:string):ClientRequest {
 if(!messageId)throw new Error('STOP_MESSAGE_REQUIRED');
 return {method:'POST',path:`/api/v1/sessions/${encodeURIComponent(sessionId)}/stop`,body:{message_id:messageId}};
}
```

AgentInput 的 onSend 从现有 ref 读取文本；同步关闭同一请求的发送门，捕获当时 Agent/空间/附件；失败保留草稿。发送超时置 reconciling，不置 idle 后自动发送。停止必须等后端确认或重新读取状态，取消 HTTP reader 只影响订阅。仍允许用户编辑下一轮草稿，不能因当前流更新覆盖非受控输入。

- [ ] **Step 4：GREEN。** 命令测试加双击只有一次 open、stop 403 不显示已停止、切 Agent 后进行中的输入 Agent 不变、网络断开不重复 POST。原生录制发送→增量→停止→再发一次。
- [ ] **Step 5：提交。** stage 本任务路径；`git commit -m "feat(mobile): send and stop agent turns without duplicate execution"`。

### H11：断线/后台恢复与历史对账

**依赖：** H10。**Files:** Create `packages/domain/src/mobile/recovery.ts`、`recovery.test.ts`；Create `packages/api-client/src/sessions/recovery.ts`；Create `apps/mobile/sources/weknora/conversation/reconcile.ts`、`reconcile.test.ts`。

**Interfaces:** `RecoverySnapshot {activity:'running'|'terminal'|'unknown';messageId:string|null}`；`recoveryAction(snapshot):'subscribe'|'history'|'wait'`；`buildContinue(sessionId):ClientRequest` GET `/api/v1/sessions/continue-stream/:id`。snapshot 由已核实后端会话状态映射，不由本地 spinner 推断。

- [ ] **Step 1：网络未知状态绝不触发生成。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {recoveryAction} from './recovery.ts';
test('unknown execution waits rather than resubmits',()=>{
 assert.equal(recoveryAction({activity:'unknown',messageId:null}),'wait');
 assert.equal(recoveryAction({activity:'running',messageId:'m'}),'subscribe');
 assert.equal(recoveryAction({activity:'terminal',messageId:'m'}),'history');
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/domain/src/mobile/recovery.test.ts`。
- [ ] **Step 3：实现保守恢复。**

```ts
export interface RecoverySnapshot {activity:'running'|'terminal'|'unknown';messageId:string|null}
export function recoveryAction(s:RecoverySnapshot):'subscribe'|'history'|'wait' {
 if(s.activity==='terminal')return 'history';
 if(s.activity==='running'&&s.messageId)return 'subscribe';
 return 'wait';
}
```

AppState active/网络恢复只启动一个 reconcile；先加载最新消息，再读实际活跃状态，运行中连接 continue-stream。按 messageId 合并历史与本地临时消息，只有实际稳定 event ID 才跨订阅去重；无稳定 cursor 时用历史快照替换已落盘部分，不能仅按文本去重丢掉合法重复内容。连续状态查询失败展示“状态未确认/重试查询”，重试按钮只查询；用户明确新发才创建新轮。

- [ ] **Step 4：GREEN 与场景回放。** 上述测试与 reconcile 测试覆盖断线前已完成、仍运行、503、旧 generation、无 event ID、重复完成事件；断网/后台30秒后回来，服务端只产生一个助手轮次。存在查询能力缺口则记录后端契约任务，不能硬猜 idle。
- [ ] **Step 5：提交。** stage 本任务路径；`git commit -m "feat(mobile): reconcile interrupted sessions before reconnecting"`。

### H12：工具审批、问题卡片和 MCP OAuth

**依赖：** H09/H10、现有 T12。**Files:** Create `packages/api-client/src/chat/interactions.ts`、`interactions.test.ts`；Create `apps/mobile/sources/weknora/conversation/interaction-controller.ts`、`interaction-controller.test.ts`；Modify Happy 工具权限卡与 SessionView。

**Interfaces:** `ApprovalInput {pendingId:string;decision:'approve'|'reject';modifiedArgs?:Record<string,unknown>}`；`buildApproval(input):ClientRequest`；UI 适配 Happy approved/denied 到该 wire enum，绝不传 bypassPermissions。`resolveOnce(id,submit):Promise<void>` 使用 pendingId 单飞，submit 返回 Promise<void>；OAuth facade 另封装实际 resolution/cancel DTO。

- [ ] **Step 1：测试 wire enum 和租户外拒绝状态。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {buildApproval} from './interactions.ts';
test('Happy approval is mapped to product command',()=>{
 assert.deepEqual(buildApproval({pendingId:'p',decision:'reject'}),
  {method:'POST',path:'/api/v1/agent/tool-approvals/p',body:{decision:'reject'}});
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/api-client/src/chat/interactions.test.ts`。
- [ ] **Step 3：实现命令及 UI pending 状态。**

```ts
import type {ClientRequest} from '../client.ts';
export function buildApproval(i:ApprovalInput):ClientRequest {
 return {method:'POST',path:`/api/v1/agent/tool-approvals/${encodeURIComponent(i.pendingId)}`,
  body:{decision:i.decision,...(i.modifiedArgs?{modified_args:i.modifiedArgs}:{})}};
}
```

以 `internal/handler/mcp_service.go` 的 body 为权威补 modified_args 校验；按钮双击共享同一 Promise，HTTP 成功后再更新状态。另端已处理时刷新 pending，不制造第二次批准；403 保持不可操作并提示。提问卡的答案协议必须按实际后端人工交互事件验证，未知 question 类型只读且保持缺口。OAuth 使用系统浏览器并沿现有 pending ID resolution，校验 callback/state，取消调用 cancel，不把普通问题回答塞进审批 decision。

- [ ] **Step 4：GREEN。** 测试重复点击、已处理、403、modified_args 非对象、OAuth取消和切空间后返回；真机完整批准/拒绝/OAuth成功各一条。
- [ ] **Step 5：提交。** stage 本任务文件及实际修改的工具卡；`git commit -m "feat(mobile): resolve tool interactions through product authority"`。

### H13：运行中追加、队列与建议问题

**依赖：** H12。**Files:** Create `packages/api-client/src/chat/steer.ts`、`steer.test.ts`、`suggestions.ts`；Create `apps/mobile/sources/weknora/conversation/steer-controller.ts`、`steer-controller.test.ts`；Modify AgentInput/建议问题展示的产品 wrapper。

**Interfaces:** `SteerInput {sessionId:string;query:string;steerId:string;messageId:string;delivery:'inject'|'after'}`；`buildSteer(i):ClientRequest`。`SteerResult {status:'queued'|'already_injected'|'new_run';steer_id?:string}` 严格按后端额外字段扩展；only `new_run` 明确响应允许转普通发送，503 不允许。

- [ ] **Step 1：写稳定队列 ID 和期望轮次断言。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {buildSteer} from './steer.ts';
test('steer pins the intended assistant turn',()=>{
 const r=buildSteer({sessionId:'s',query:'继续',steerId:'q',messageId:'m',delivery:'inject'});
 assert.equal((r.body as {expected_assistant_message_id:string}).expected_assistant_message_id,'m');
 assert.equal((r.body as {steer_id:string}).steer_id,'q');
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/api-client/src/chat/steer.test.ts`。
- [ ] **Step 3：实现命令和持久队列 ID。**

```ts
import type {ClientRequest} from '../client.ts';
export function buildSteer(i:SteerInput):ClientRequest {
 return {method:'POST',path:`/api/v1/sessions/${encodeURIComponent(i.sessionId)}/steer`,body:{
  query:i.query,steer_id:i.steerId,expected_assistant_message_id:i.messageId,delivery:i.delivery,channel:'mobile'}};
}
```

按 `frontend/src/api/chat/steer.ts` 封装 list/remove/promote，同一 steer ID 的 SSE 可以早于 HTTP，应合并状态，已注入不可重新变 pending。建议问题读取当前 message ID 的 suggestions；点击使用 H10 同一发送门，记录现有 suggestion attribution，不新建运行旁路。

- [ ] **Step 4：GREEN。** 覆盖事件先到、HTTP503、new_run、切 Agent 后旧队列不换归属、删除待发、promote、重复建议点击；原生运行中 inject/after 各验证一次。
- [ ] **Step 5：提交。** stage 本任务文件；`git commit -m "feat(mobile): support steer queue and contextual suggestions"`。
