# Happy 管理与原生交付 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐移动管理能力并以真实后端、两端原生及升级回退证据验收。

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

## 任务边界

H27 管设备偏好与无障碍，H28 管Agent配置，H29 管内容编辑，H30 管数据源，H31 管组织空间，H32 管系统配置，H33 管受控终端，H34/H35管真实闭环与交付。管理表单消费原迁移T08/T09/T15/T16/T17的共享facade；缺失的端点/DTO只在共享层补一次，不能从Vue组件复制请求与token逻辑。

每项管理任务先以该任务的一个垂直子行为完成RED→GREEN→真机演示；同文件责任下的其余行为逐个重复该循环并分别提交。账本按子行为记录，不因一个表单通过就验收整个管理域。

## 管理域的独立评审单元

H28–H32 是汇总编号，实际实施和review以以下子任务为单位；每个子任务只修改其screen/控制器、对应共享facade及同名测试，独立RED→GREEN、原生验证和commit。读取时同时带入父任务给出的接口/测试代码，禁止把一整行管理域交给单个实现者批量完成。父任务只有全部子任务通过才accepted。

| 子任务 | 独立交付 / 原生文件（位于 `sources/weknora/management/`） | 共享来源 / 必须失败的业务断言 |
|---|---|---|
| H28.1 | `AgentEditorScreen.tsx` 基本信息创建/编辑 | agents facade；保存403保留草稿，不显示成功 |
| H28.2 | `AgentModelScreen.tsx` 模型/检索资源绑定 | agents.update；更换model_id不删除已有知识/MCP配置 |
| H28.3 | `AgentToolsScreen.tsx` MCP/Skill选择 | agents.update；已撤销工具不可从缓存重新选择 |
| H28.4 | `AgentActionsScreen.tsx` 复制/删除 | agents.copy/remove；复制产生新ID，取消删除不发送DELETE |
| H29.1 | `KnowledgeManagerScreen.tsx` KB增改删 | knowledgeBases facade；403/409不删本地行 |
| H29.2 | `KnowledgeFoldersScreen.tsx` 文件夹/标签/批量 | 原T07契约；部分失败逐项保留，不全量清选择 |
| H29.3 | `FaqEditorScreen.tsx` FAQ编辑 | 原T08契约；保存失败保留问题/答案及顺序 |
| H29.4 | `WikiEditorScreen.tsx` Wiki编辑 | wiki facade；409不覆盖另端版本 |
| H29.5 | `WikiVersionsScreen.tsx` 历史与revert | wiki revisions/revert；目标revision不可换为latest |
| H29.6 | `KnowledgeGraphScreen.tsx` 图谱浏览/操作 | 原能力矩阵与图谱API；未授权节点点击不得取详情 |
| H30.1 | `DataSourceScreen.tsx` 建连/凭据测试 | datasource facade；掩码不可写回，失败不显示连接有效 |
| H30.2 | `DataSourceResourcesScreen.tsx` 资源选择 | datasource.resources；分页切换不丢已选资源ID |
| H30.3 | `DataSourceSyncScreen.tsx` 同步/pause/resume | datasource facade；503不乐观标已暂停 |
| H30.4 | `KnowledgeSettingsScreen.tsx` 高级检索/解析 | 原T09契约；修改单字段不清其他配置 |
| H31.1 | `WorkspaceScreen.tsx` 切空间/无空间 | auth.switch-tenant；旧响应不可写新scope |
| H31.2 | `MemberScreen.tsx` 成员/角色 | 原T16契约；降权后再次写入得到服务端拒绝 |
| H31.3 | `InvitationScreen.tsx` 邀请/撤销/加入 | 原T16契约；已使用/过期链接不加入空间 |
| H31.4 | `OrganizationScreen.tsx` 共享组织 | 原T16契约；退出/审批按后端角色约束执行 |
| H32.1 | `ModelSettingsScreen.tsx` 模型配置与debug | `/models`及`/:id/debug`；debug失败不报可用 |
| H32.2 | `StorageParserScreen.tsx` 存储/解析 | `/system/storage-engine-check`、`parser-engines`；密钥write-only |
| H32.3 | `McpSkillScreen.tsx` MCP/Skill管理 | `/mcp-services`及原Skill接口；状态改变后重新校验权限 |
| H32.4 | `MemorySettingsScreen.tsx` Memory与运行参数 | `/memory/settings`、原T17；离线保存不显示成功 |
| H32.5 | `ApiKeyScreen.tsx` API Key创建/撤销 | `/tenants/:id/api-keys`或系统独立端点；创建secret仅显一次 |
| H32.6 | `QueueAuditScreen.tsx` 队列/审计 | `/system/admin/runtime/queues`与audit；Viewer不得执行队列动作 |
| H32.7 | `IntegrationScreen.tsx` IM/Embed配置 | `/agents/:id/im-channels`、`embed-channels`；凭证不跨channel复用 |
| H32.8 | `SystemAdminScreen.tsx` 用户/系统设置 | `/system/admin`实际接口；空间Admin无系统写权限 |

这些新screen是各子任务明确的Create文件；测试路径统一同目录同basename改为 `.test.ts`（组件原生验证另记录），不要把上述新增文件误当成当前已有页面。每个子任务消费父任务的shared facade/权限接口，输出对应用户交互；额外发现的路由必须加入H01库存并分配独立子任务，不能被“其他设置”一项吞掉。

### H27：设置、主题、语言与无障碍

**依赖：** H05。**Files:** Create `apps/mobile/sources/weknora/management/preferences.ts`、`preferences.test.ts`、`SettingsScreen.tsx`；Modify引入的Happy `sources/text/`与主题provider；Create `docs/migrations/happy/accessibility-matrix.md`。

**Interfaces:** `DevicePreferences {theme:'system'|'light'|'dark';language:string}`；`parsePreferences(value:unknown,languages:string[]):DevicePreferences`。账号偏好用现有user API，设备主题/字号不覆盖服务端账号配置。

- [ ] **Step 1：无效本地设置有确定的回退。**

```ts
import {expect,test} from 'vitest';
import {parsePreferences} from './preferences';
test('invalid stored preference does not break startup',()=>{
 expect(parsePreferences({theme:'neon',language:'bad'},['zh-CN','en-US']))
  .toEqual({theme:'system',language:'zh-CN'});
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/management/preferences.test.ts`。
- [ ] **Step 3：实现偏好解析和Happy设置接线。**

```ts
export function parsePreferences(v:unknown,languages:string[]){
 if(!languages.length)throw new Error('LANGUAGES_REQUIRED');
 const x=(v&&typeof v==='object'?v:{}) as Record<string,unknown>;
 const theme=x.theme==='light'||x.theme==='dark'?x.theme:'system';
 return {theme,language:typeof x.language==='string'&&languages.includes(x.language)?x.language:languages[0]};
}
```

保留Happy主题、动画、原生菜单和响应式布局；翻译合并采用命名空间，WeKnora现有六种语言的产品文案必须有去向，不用硬编码中文按钮。网络/权限错误是可读文案。给交互按钮accessibilityLabel/role，pending状态正确disabled，屏幕阅读顺序先正文后操作；不要为视觉相同固定字体缩放上限。

- [ ] **Step 4：GREEN与设备检查。** 测试偏好重启/账号切换；iOS VoiceOver/Android TalkBack、系统大字体、暗色、横屏、平板、安全区、键盘和长内容逐项记录。
- [ ] **Step 5：提交。** stage本任务文件；`git commit -m "feat(mobile): preserve preferences and accessible native layout"`。

### H28：Agent、模型绑定、MCP和Skill配置

**依赖：** H07、原T15共享契约。**Files:** Create `apps/mobile/sources/weknora/management/agent-draft.ts`、`agent-draft.test.ts`、`AgentEditorScreen.tsx`；Extend `packages/api-client/src/agents/index.ts`、`index.test.ts`。

**Interfaces:** `AgentDraft {name:string;description:string;config:Record<string,unknown>}`；`patchAgentConfig(original:AgentDraft,patch:Record<string,unknown>):AgentDraft`。CRUD facade `create(input)`, `update(id,input)`, `copy(id)`, `remove(id)`沿实际Agent DTO；不发送后端不允许用户编辑的owner/is_builtin字段。

- [ ] **Step 1：修改模型不能丢失MCP/Skill或未知配置。**

```ts
import {expect,test} from 'vitest';
import {patchAgentConfig} from './agent-draft';
test('editing one binding preserves unrelated agent configuration',()=>{
 const a={name:'A',description:'',config:{model_id:'old',skill_names:['s'],custom_field:1}};
 expect(patchAgentConfig(a,{model_id:'new'}).config).toEqual({model_id:'new',skill_names:['s'],custom_field:1});
 expect(a.config.model_id).toBe('old');
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/management/agent-draft.test.ts`。
- [ ] **Step 3：实现不变草稿和分步编辑。**

```ts
export function patchAgentConfig(a:AgentDraft,patch:Record<string,unknown>):AgentDraft {
 return {...a,config:{...a.config,...patch}};
}
```

分别完成基本信息保存、模型选择、知识绑定、MCP选择、Skill选择、复制/删除6个小循环，每个循环写自己的提交/失败断言。字段白名单按现有Config；保留未知字段仅用于相同资源的更新兼容，不允许用户注入服务端保护字段。内置Agent与非owner由后端决定可编辑性，403保留草稿不伪成功。测试preset切换的默认值不能默默覆盖用户明确修改的配置。

- [ ] **Step 4：GREEN与真实配置。** shared facade断言真实PUT路径/DTO、内置/owner/Admin差异；真机修改一个Agent并发起会话确认实际模型/工具生效，不能只看表单保存提示。
- [ ] **Step 5：提交。** stage本任务文件；`git commit -m "feat(mobile): edit agent bindings with preserved configuration"`。

### H29：知识管理、FAQ与Wiki版本编辑

**依赖：** H14、原T06/T07/T08。**Files:** Create `apps/mobile/sources/weknora/management/wiki-save.ts`、`wiki-save.test.ts`、`WikiEditorScreen.tsx`、`FaqEditorScreen.tsx`、`KnowledgeManagerScreen.tsx`；Extend现有 `packages/api-client/src/wiki/pages.ts`及知识facade需要的缺口。

**Interfaces:** `saveVersioned<T>(save:()=>Promise<T>):Promise<{kind:'saved';value:T}|{kind:'conflict'}>`；现有wiki facade update/revert使用实际version字段；FAQ/KB/folder/tag输入由已存在DTO定义，不用一个任意JSON表单替代原功能。

- [ ] **Step 1：版本冲突不能显示保存成功。**

```ts
import {expect,test} from 'vitest';
import {saveVersioned} from './wiki-save';
test('conflict preserves the edit flow',async()=>{
 expect(await saveVersioned(async()=>{throw {status:409}})).toEqual({kind:'conflict'});
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/management/wiki-save.test.ts`。
- [ ] **Step 3：实现冲突结果与原生内容编辑。**

```ts
export async function saveVersioned<T>(save:()=>Promise<T>):Promise<{kind:'saved';value:T}|{kind:'conflict'}>{
 try{return {kind:'saved',value:await save()}}catch(e){
  if(e&&typeof e==='object'&&'status' in e&&e.status===409)return {kind:'conflict'};throw e;
 }
}
```

按顺序分独立小循环交付：KB创建/编辑/删除；文件夹/标签/批量操作；FAQ创建/编辑/删除；Wiki读取/编辑/历史/指定版本revert。每个实际UI步骤写到对应screen的测试用例和原生脚本，不把API已存在当页面完成。冲突保留本地文本、提供重新加载和对照，用户再次提交前不得无条件覆盖；批量操作列出逐项失败，不全量标成功。

- [ ] **Step 4：GREEN。** 409/403/404/离线保存、回退旧版本、批量部分失败；两台客户端编辑同一Wiki制造真实冲突。旧T23矩阵中的复杂图谱若未实现仍明确未验收，不能被Wiki编辑通过覆盖。
- [ ] **Step 5：提交。** 按上述小循环精确提交screen/facade/test；最终 `git commit -m "feat(mobile): complete versioned knowledge editing flows"` 仅在仍有本任务改动时使用。

### H30：数据源、同步和高级知识配置

**依赖：** H29、原T09。**Files:** Create `apps/mobile/sources/weknora/management/datasource-draft.ts`、`datasource-draft.test.ts`、`DataSourceScreen.tsx`、`KnowledgeSettingsScreen.tsx`；Reuse `packages/api-client/src/datasource.ts`。

**Interfaces:** `credentialPatch(value:string|undefined):{credential?:string}` 是UI草稿规则，真正wire credential对象通过后端DTO映射；undefined表示不变，用户显式清除必须使用后端支持的清除命令，不能把掩码发回。

- [ ] **Step 1：掩码不作为新凭据发送。**

```ts
import {expect,test} from 'vitest';
import {credentialPatch} from './datasource-draft';
test('masked secret is never written back',()=>{
 expect(credentialPatch(undefined)).toEqual({});
 expect(()=>credentialPatch('********')).toThrow('MASKED_CREDENTIAL');
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/management/datasource-draft.test.ts`。
- [ ] **Step 3：实现write-only凭据输入和同步控制。**

```ts
export function credentialPatch(v:string|undefined):{credential?:string}{
 if(v===undefined)return {};if(v==='********')throw new Error('MASKED_CREDENTIAL');return {credential:v};
}
```

分开完成连接器创建/测试凭据、资源选择、同步状态与pause/resume、高级检索/解析配置4个循环。沿现有SDK，无密钥持久草稿；显示后端同步失败原因与时间，网络失败不宣称paused。权限被撤销后停止轮询并清理敏感表单。

- [ ] **Step 4：GREEN。** 测试未修改secret、显式更换、测试连接失败、pause503、资源分页、切空间；受控连接器真实同步一份测试文档并在检索中验证。
- [ ] **Step 5：提交。** stage本任务路径；`git commit -m "feat(mobile): manage data sources and knowledge settings"`。

### H31：空间、组织、成员、邀请与角色

**依赖：** H05、原T16。**Files:** Create `packages/api-client/src/mobile/membership.ts`、`membership.test.ts`；Create `apps/mobile/sources/weknora/management/membership.ts`、`membership.test.ts`、`WorkspaceScreen.tsx`、`OrganizationScreen.tsx`。

**Interfaces:** `Membership {tenantId:string;role:string;active:boolean}`；`canEnter(memberships:Membership[],tenantId:string):boolean`；产品membership/organization facade复制实际请求契约，不从前端role枚举产生后端授权。

- [ ] **Step 1：撤销成员不能进入缓存空间。**

```ts
import {expect,test} from 'vitest';
import {canEnter} from './membership';
test('revoked membership is not a valid navigation target',()=>{
 expect(canEnter([{tenantId:'t',role:'owner',active:false}],'t')).toBe(false);
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/management/membership.test.ts`。
- [ ] **Step 3：实现入口守卫和分步管理。**

```ts
export interface Membership {tenantId:string;role:string;active:boolean}
export function canEnter(rows:Membership[],id:string):boolean{return rows.some(r=>r.tenantId===id&&r.active)}
```

分别交付空间切换/无空间引导、成员查看/角色变更、邀请创建/撤销/加入、组织加入/退出/审批四个子循环。变更角色/移除成员后刷新权限和资源；后台仍执行owner/admin限制，客户端隐藏按钮不是权限证明。显示组织共享关系与空间身份，不把组织当统一计费账户。

- [ ] **Step 4：GREEN。** 同一命令及shared facade契约；Viewer/Admin/Owner三个真实身份测试允许/拒绝路径，角色中途降级、邀请过期、最后Owner约束按后端反馈呈现。
- [ ] **Step 5：提交。** stage本任务路径；`git commit -m "feat(mobile): manage workspace membership through product permissions"`。

### H32：系统后台、模型/存储/解析、Memory和集成配置

**依赖：** H31、原T15/T16/T17/T18。**Files:** Create `apps/mobile/sources/weknora/management/admin-entry.ts`、`admin-entry.test.ts`、`SystemAdminScreen.tsx`、`RuntimeSettingsScreen.tsx`；Extend相应shared facade，逐项路径记录 `docs/migrations/happy/admin-capabilities.json`。

**Interfaces:** `adminEntry(user:{isSystemAdmin:boolean},role:string):{system:boolean;workspace:boolean}` 是展示规则，具体workspace role值按当前后端语义映射；`AdminCapabilityRow {id,source,route,role,test,evidence,status}` 由库存记录全部配置面板，不能只记录screen大类。

- [ ] **Step 1：空间管理员不是系统管理员。**

```ts
import {expect,test} from 'vitest';
import {adminEntry} from './admin-entry';
test('workspace admin does not gain system administration',()=>{
 expect(adminEntry({isSystemAdmin:false},'admin')).toEqual({system:false,workspace:true});
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/management/admin-entry.test.ts`。
- [ ] **Step 3：实现分离入口，按能力行逐个交付。**

```ts
export function adminEntry(user:{isSystemAdmin:boolean},role:string){
 return {system:user.isSystemAdmin===true,workspace:role==='admin'||role==='owner'};
}
```

按原路由/设置库存拆成独立提交：模型配置与测试；存储/解析；MCP/Skill管理；Memory/运行参数；API Key创建/撤销；队列/审计；IM/Embed集成。每个子循环分别记录字段、真实读/写路由、保护角色、失败断言与原生入口。API Key仅一次显示并允许用户显式复制，页面离开清内存，不落日志/截图；私有配置按服务端write-only字段处理。禁止用一个Web后台链接勾选所有原生管理功能。

- [ ] **Step 4：GREEN。** entry单测加跨身份后端负例；每个能力行有read成功、write成功或权限拒绝、重载后值一致三种证据。凭证缺失的外部集成保持blocked而不是mock accepted。
- [ ] **Step 5：提交。** 每个子循环只提交对应facade/screen/test；完成后更新能力矩阵与原T23摘要。

### H33：产品沙箱终端和受控复杂预览

**依赖：** H16、原T14。**Files:** Create `packages/api-client/src/sessions/terminal.ts`、`terminal.test.ts`；Create `apps/mobile/sources/weknora/resources/terminal-bridge.ts`、`terminal-bridge.test.ts`、`TerminalScreen.tsx`；Read `internal/handler/session/sandbox_terminal_bridge.go`。

**Interfaces:** `TerminalBridgeMessage={type:'resize';cols:number;rows:number}|{type:'input';data:string}`；`parseTerminalMessage(value:unknown):TerminalBridgeMessage`；SDK生成现有 terminal-ticket POST请求，ticket字段/WS URL来自handler实际响应，不把长期Bearer交给WebView。

- [ ] **Step 1：拒绝任意桥方法。**

```ts
import {expect,test} from 'vitest';
import {parseTerminalMessage} from './terminal-bridge';
test('web content cannot call arbitrary native methods',()=>{
 expect(()=>parseTerminalMessage({type:'openURL',url:'https://evil.test'})).toThrow('TERMINAL_MESSAGE');
});
```

- [ ] **Step 2：RED。** `pnpm --filter @weknora/mobile exec vitest run sources/weknora/resources/terminal-bridge.test.ts`。
- [ ] **Step 3：实现严格白名单。**

```ts
export function parseTerminalMessage(v:unknown):TerminalBridgeMessage {
 const x=(v&&typeof v==='object'?v:{}) as Record<string,unknown>;
 if(x.type==='input'&&typeof x.data==='string'&&x.data.length<=8192)return {type:'input',data:x.data};
 if(x.type==='resize'&&Number.isInteger(x.cols)&&Number.isInteger(x.rows)&&Number(x.cols)>0&&Number(x.rows)>0&&Number(x.cols)<=500&&Number(x.rows)<=500)return {type:'resize',cols:Number(x.cols),rows:Number(x.rows)};
 throw new Error('TERMINAL_MESSAGE');
}
```

受控本地HTML/xterm资源只加载明确来源，禁任意导航；持有短期ticket而非服务端secret，窗口关闭、切空间、ticket失效时关闭WS。产品sandbox terminal与H25远程machine terminal区分provider，不能混用路径或ticket。复杂图谱/Office预览按原T23决策允许受控WebView时采用同样来源/桥限制，但单独验收其用户操作。

- [ ] **Step 4：GREEN。** 测试过大输入、非法resize、任意方法、过期ticket/跨session/重放；受控沙箱运行 `pwd`、resize、中文输入、断线重连，确认退出无悬挂连接。
- [ ] **Step 5：提交。** stage本任务路径；`git commit -m "feat(mobile): expose scoped sandbox terminal bridge"`。

### H34：共享fixture与两条真实Agent链

**依赖：** H13/H17、可用测试后端/模型/工具。**Files:** Create `scripts/happy/run-live.mjs`、`run-live.test.mjs`；Create `docs/migrations/happy/fixtures/knowledge.json`、`general.json`、`native-scenarios.md`；Create `apps/mobile/sources/weknora/conversation/parity.test.ts`。

**Interfaces:** `Evidence{caseId:string;level:'fixture'|'backend'|'native';platform:'ios'|'android'|'none';passed:boolean;sourceSha:string;artifact:string}`；runner使用进程环境 `HAPPY_TEST_ORIGIN`、`HAPPY_TEST_TOKEN`、`HAPPY_TEST_KNOWLEDGE_AGENT`、`HAPPY_TEST_GENERAL_AGENT`，不能将token输出或写fixture。

- [ ] **Step 1：fixture证据不能算真实验收。**

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import {isNativeEvidence} from './run-live.mjs';
test('fixture cannot satisfy native acceptance',()=>{
 assert.equal(isNativeEvidence({level:'fixture',platform:'ios',passed:true,artifact:'x'}),false);
 assert.equal(isNativeEvidence({level:'native',platform:'ios',passed:true,artifact:'x'}),true);
});
```

- [ ] **Step 2：RED。** `node --test scripts/happy/run-live.test.mjs`。
- [ ] **Step 3：实现证据分类与真实runner。**

```js
export function isNativeEvidence(e){
 return e.level==='native'&&['ios','android'].includes(e.platform)&&e.passed===true&&typeof e.artifact==='string'&&e.artifact.length>0;
}
```

runner只在直接执行时运行（模块被测试import不调用后端）。先检查四个环境变量、服务器健康与Agent可见性；创建隔离测试会话，再分别发知识问答/无KB custom Agent，增量采集SSE、执行真实审批、取历史与stop/reconnect，不调用mock fetch。每轮检查唯一助手messageId、Agent归属、最终状态和工具结果。fixtures使用脱敏实际事件构造，覆盖重复/乱序/无ID/中文切块；对比Web投影与移动投影的正文、工具、审批与引用，不要求DOM结构一致。

- [ ] **Step 4：GREEN与原生脚本逐项执行。** `node --test scripts/happy/run-live.test.mjs`；`node scripts/happy/run-live.mjs`；移动parity单测。原生脚本明确：登录→选Agent→发送→等首token→断网→联网→查无重复；第二轮审批拒绝；第三轮停止；切Agent查看旧消息归属；专业Agent附件与结果。iOS/Android分别记录设备、服务/模型版本、截图/录屏和messageId。清理仅runner创建的会话，不能批量删除用户会话。
- [ ] **Step 5：提交。** stage测试/脚本/脱敏fixtures，凭证与原始敏感日志不提交；`git commit -m "test(mobile): verify native product agent journeys"`。

### H35：全量交互保留、构建发布与回退

**依赖：** H01–H34所有适用交付通过；专项服务分支被阻塞则本任务不能整体accepted。**Files:** Create `scripts/happy/release-gate.mjs`、`release-gate.test.mjs`、`.github/workflows/mobile.yml`、`docs/migrations/happy/release-runbook.md`；Modify `apps/mobile/app.config.js`、`eas.json`（若选择EAS构建）、原交互矩阵和账本。

**Interfaces:** `releaseReady(rows,evidence):boolean`，要求完整inventory每项accepted且存在实际证据；evidence按H34 schema并附commit/device/version。CI发布产物不自动发布商店。

- [ ] **Step 1：任一能力未验收即拒绝完整发布声明。**

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import {releaseReady} from './release-gate.mjs';
test('blocked voice cannot disappear from release acceptance',()=>{
 assert.equal(releaseReady([{id:'chat',status:'accepted'},{id:'voice',status:'blocked'}],[]),false);
});
```

- [ ] **Step 2：RED。** `node --test scripts/happy/release-gate.test.mjs`。
- [ ] **Step 3：实现完整矩阵门和分平台CI。**

```js
export function releaseReady(rows,evidence){
 if(!rows.length||rows.some(r=>r.status!=='accepted'))return false;
 return rows.every(row=>['ios','android'].every(platform=>evidence.some(e=>
  e.caseId===row.id&&e.level==='native'&&e.platform===platform&&e.passed===true&&e.artifact)));
}
```

继续增加来源SHA一致、artifact真实存在、没有未映射route/interaction、移除行需批准记录等断言；门不能接受仅伪造的JSON布尔值。CI分shared/node、mobile typecheck/test、iOS构建、Android构建；根测试脚本必须包含新增mobile/shared目录。开发/生产包ID、scheme、签名和updates channel分开，生产标识经发行配置确认，不继承Happy EAS项目；禁debug证书/测试origin进入发布包。

- [ ] **Step 4：GREEN与安装升级。** `node --test scripts/happy/release-gate.test.mjs`；`pnpm test:shared`、`pnpm typecheck:shared`、`pnpm --filter @weknora/mobile typecheck`、`pnpm --filter @weknora/mobile exec vitest run`；iOS/Android原生安装运行、旧版升级保存用户数据、重新登录、token撤销、上一版客户端连接兼容后端。更新原生依赖不得仅OTA；回退应用与后台服务开关分别演练，迁移默认向前兼容，不通过删数据库回滚。商店提交/外部部署在实际执行时提供可审查产物后再按用户授权处理。
- [ ] **Step 5：提交与报告。** stage本任务文件及脱敏证据索引；`git commit -m "ci(mobile): gate native release on complete interaction evidence"`。输出核心通过/完整通过的明确区分、仍失败/blocked列表、构建产物与回退记录；原T20–T23只有相应H任务真正满足才改accepted。
