# Happy 原生壳与身份 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可运行的 Happy 原生壳、产品登录、隔离作用域与移动网络端口。

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

## 文件责任与输入

H01 管来源与完整库存，H02 管原生壳与构建，H03/H04 管身份，H05 管作用域，H06 管流传输。仅 H02 修改 workspace 和移动构建入口；其他任务需要导出新模块时修改各自包 exports，不重写根配置。

### H01：建立可验证的 Happy 来源和交互库存

**依赖：** 已读总计划/规格；隔离工作区已建立。**交付：** 引入范围可复现，遗漏交互会使检查失败。

**Files:**
- Create: `scripts/happy/check-inventory.mjs`、`scripts/happy/check-inventory.test.mjs`
- Create: `docs/migrations/happy/source-manifest.json`、`interaction-matrix.json`、`upstream/LICENSE`
- Read: Happy `sources/app/`、`sources/-session/SessionView.tsx`、`sources/components/`、`sources/sync/ops.ts`、`package.json`、`pnpm-lock.yaml`、`patches/`

**Interfaces:** `checkInventory(rows, routePaths): void`；每行 `{id, route, source, interaction, destination, service, task, status}`，status 仅 `pending|accepted|blocked`。产出来源记录 `{commit, files:[{source,target,sha256,license}], dependencies, patches}`。

- [ ] **Step 1：写“缺少路由/重复能力/空去向不能通过”的失败测试。**

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { checkInventory } from './check-inventory.mjs';
test('inventory cannot silently omit a route', () => {
  assert.throws(() => checkInventory([], ['sources/app/(app)/index.tsx']), /unmapped/);
});
test('duplicate IDs are rejected', () => {
  const row = {id:'voice.start', route:'voice', source:'apiVoice.ts',
    interaction:'start', destination:'voice adapter', service:'voice', task:'H21', status:'pending'};
  assert.throws(() => checkInventory([row,row], ['voice']), /duplicate/);
});
```

- [ ] **Step 2：运行 RED。** `node --test scripts/happy/check-inventory.test.mjs`；预期新增模块缺失，随后应转为业务断言。
- [ ] **Step 3：实现库存校验并生成完整清单。**

```js
export function checkInventory(rows, routes) {
  const ids = new Set();
  for (const row of rows) {
    if (ids.has(row.id)) throw new Error(`duplicate ${row.id}`);
    ids.add(row.id);
    for (const key of ['id','route','source','interaction','destination','service','task']) {
      if (typeof row[key] !== 'string' || !row[key].trim()) throw new Error(`missing ${key}`);
    }
    if (!['pending','accepted','blocked'].includes(row.status)) throw new Error('invalid status');
  }
  for (const route of routes) {
    if (!rows.some(row => row.route === route)) throw new Error(`unmapped ${route}`);
  }
}
```

用 `git -C /Users/wuyongjun/trea/happy ls-tree -r --name-only ac64b9b4677870f7b7a9eacfd0780959229717f1 packages/happy-app/sources/app` 枚举路由；逐屏再拆输入、菜单、手势、设置、语音、通知、目标、旁支、远程操作。route 覆盖不是控件覆盖，必须对照 SessionView/ops 的每个用户动作。保存每个实际引入文件 SHA256；依赖闭包包括 happy-wire、构建插件和 patches，不记录账户配置或密钥。

- [ ] **Step 4：GREEN 与来源复核。** 重跑同一测试；用 `git show <固定提交>:<source>` 内容重算 manifest 的 hash，一次性列出缺失源。所有清单行保持 pending；没有原生截图不能 accepted。
- [ ] **Step 5：提交此交付。** `git add scripts/happy/check-inventory.mjs scripts/happy/check-inventory.test.mjs docs/migrations/happy/source-manifest.json docs/migrations/happy/interaction-matrix.json docs/migrations/happy/upstream/LICENSE`；`git commit -m "docs: freeze Happy source and interaction inventory"`。

### H02：引入原生壳，隔离产品数据入口

**依赖：** H01。**交付：** iOS/Android 开发构建打开保留 Happy 样式的壳，不连接上游生产服务。

**Files:**
- Create: `apps/mobile/`（仅 source-manifest 中明确列出的 Happy 文件）与 `packages/happy-wire/`
- Modify: `pnpm-workspace.yaml`、`package.json`、`pnpm-lock.yaml`
- Modify after import: `apps/mobile/package.json`、`app.config.js`、`metro.config.js`、`sources/app/_layout.tsx`
- Create: `apps/mobile/sources/weknora/platform/host.ts`、`host.test.ts`

**Interfaces:** `MobileHost {backend:'weknora'; origin:string}`；`createMobileHost(origin:string):MobileHost`。Web core 不在此依赖图。

- [ ] **Step 1：写禁止向任意上游地址自动连接的测试。**

```ts
import { expect, test } from 'vitest';
import { createMobileHost } from './host';
test('only explicit product origin is accepted', () => {
  expect(() => createMobileHost('')).toThrow('SERVER_REQUIRED');
  expect(createMobileHost('https://example.test/')).toEqual({backend:'weknora',origin:'https://example.test'});
});
```

- [ ] **Step 2：按 manifest 引入最小依赖闭包并运行 RED。** 从固定 git blob 写目标文件，不能 `cp` 可变工作树；移动包改名 `@weknora/mobile`。pnpm workspace 显式增加 `apps/mobile`、`packages/happy-wire`；保留 wire 原包名以便原组件编译。依赖安装前检查上游 postinstall/release 脚本，移除自动发布/生产环境默认值，保留有证据需要的原生 patches。使用根 pnpm 10.28.2 更新锁文件，不复制上游 lockfile 覆盖全仓库；保留 Expo55/RN0.83.1/React19.2.0 与 TS5.9.3，react-test-renderer 若使用须与 React 对齐。安装后运行 `pnpm --filter @weknora/mobile exec vitest run sources/weknora/platform/host.test.ts`，预期新增 host 模块缺失。
- [ ] **Step 3：实现 host 并接入壳的 provider。**

```ts
export interface MobileHost { backend:'weknora'; origin:string }
export function createMobileHost(origin:string):MobileHost {
  if (!origin.trim()) throw new Error('SERVER_REQUIRED');
  const url = new URL(origin);
  if (!['https:','http:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error('INVALID_SERVER');
  }
  return {backend:'weknora',origin:url.toString().replace(/\/$/,'')};
}
```

`_layout.tsx` 在没有产品身份时进入服务器/登录入口；不启动 Happy 的同步、分析、付费或语音连接。保留 SafeArea、键盘、主题、导航 provider；原有 remote 功能由 H24 显式连接后启用。Metro 保留上游 wasm/preact/inlineRequires 修正；新增 workspace 解析不得将 Web React 实例引入原生。

- [ ] **Step 4：GREEN 与开发构建。** 重跑测试，运行 `pnpm --filter @slopus/happy-wire build`、`pnpm --filter @weknora/mobile typecheck`、`pnpm --filter @weknora/mobile exec expo export --platform ios`、Android 同命令；随后 `expo run:ios` / `expo run:android` 验证壳。配置开发包 ID 为 `com.weknora.mobile.dev`，移除上游 EAS projectId/updates URL，生产签名在 H35 设置。记录两端启动、主题、返回导航和键盘截图。
- [ ] **Step 5：提交。** 按 manifest 的 target 逐个 stage；另外 stage 本任务配置、host 与测试；`git commit -m "feat(mobile): introduce Happy native shell with product host"`。执行前用 `git diff --cached --name-only` 排除上游密钥与其他任务文件。

### H03：共享登录、刷新与原生凭证

**依赖：** H02、原 T03 认证语义。**交付：** 真机登录及 401 单飞刷新，重启仍可恢复有效身份。

**Files:**
- Create: `packages/api-client/src/auth/login.ts`、`login.test.ts`
- Create: `apps/mobile/sources/weknora/auth/credentials.ts`、`credentials.test.ts`、`LoginScreen.tsx`
- Modify: `packages/api-client/src/index.ts`、移动登录路由；Read `frontend/src/api/auth/index.ts`、`internal/handler/dto/auth.go`

**Interfaces:** `parseLogin(value:unknown):{credential:BearerCredential;userId:string;tenantId:string|null}`；现有 `CredentialAdapter` 不变；`createCredentials(store:{get(k:string):Promise<string|null>;set(k:string,v:string):Promise<void>;remove(k:string):Promise<void>},key:string):CredentialAdapter`。

- [ ] **Step 1：测试登录与刷新 wire 字段差异、失败响应不落 token。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseLogin } from './login.ts';
test('login uses token and preserves tenantless user', () => {
  assert.deepEqual(parseLogin({success:true,token:'a',refresh_token:'r',user:{id:'u'},tenant:null}),
    {credential:{kind:'bearer',accessToken:'a',refreshToken:'r'},userId:'u',tenantId:null});
  assert.throws(() => parseLogin({success:false,token:'a'}), /INVALID_LOGIN/);
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/api-client/src/auth/login.test.ts`。
- [ ] **Step 3：实现归一化与凭证适配。**

```ts
import type { BearerCredential } from '../ports.ts';
export function parseLogin(value:unknown):{credential:BearerCredential;userId:string;tenantId:string|null} {
  if (!value || typeof value!=='object') throw new Error('INVALID_LOGIN');
  const v=value as Record<string,unknown>;
  const u=v.user as {id?:unknown}|undefined;
  const t=(v.active_tenant ?? v.tenant) as {id?:unknown}|null|undefined;
  if (v.success!==true || typeof v.token!=='string' || !v.token.trim() || typeof u?.id!=='string' || !u.id) throw new Error('INVALID_LOGIN');
  if (v.refresh_token!==undefined && (typeof v.refresh_token!=='string' || !v.refresh_token.trim())) throw new Error('INVALID_LOGIN');
  if (t?.id!==undefined && typeof t.id!=='string' && (typeof t.id!=='number' || !Number.isSafeInteger(t.id))) throw new Error('INVALID_TENANT_ID');
  if(t?.id!=null&&!/^[1-9]\d*$/.test(String(t.id)))throw new Error('INVALID_TENANT_ID');
  return {credential:{kind:'bearer',accessToken:v.token,...(typeof v.refresh_token==='string'?{refreshToken:v.refresh_token}:{})},
    userId:u.id,tenantId:t?.id==null?null:String(t.id)};
}
```

SecureStore 通过上述 store 注入，不在 Node 测试导入原生模块；key 以产品 origin 的编码标识分区，产品 token 和 Happy secret 分开。登录仅调用共享 facade 的 POST `/api/v1/auth/login`，刷新复用 `createRefreshCoordinator` 和 POST `/api/v1/auth/refresh`。401 最多重试一次已确认未执行的认证失败请求；网络未知结果不重发。LoginScreen 保留加载/错误、租户缺失引导，密码不进入日志、草稿或持久存储。

- [ ] **Step 4：GREEN 与身份竞态。** 同一测试命令，加 `credentials.test.ts`：重启 read、clear、两个 origin 隔离；现有 `auth.test.ts` 验证并发401只有一次 refresh，刷新失败清理凭证。真机验证登录、错误密码、过期刷新、退出重启。
- [ ] **Step 5：提交。** `git add packages/api-client/src/auth/login.ts packages/api-client/src/auth/login.test.ts packages/api-client/src/index.ts apps/mobile/sources/weknora/auth`，精确追加实际登录路由；`git commit -m "feat(mobile): connect product login and secure credentials"`。

### H04：OIDC、邀请和无空间入口

**依赖：** H03、原 T03 已核实 OIDC 回调规则。**交付：** 浏览器认证安全返回 App，邀请注册和无空间用户可继续。

**Files:**
- Create: `packages/domain/src/mobile/auth-return.ts`、`auth-return.test.ts`
- Create: `packages/api-client/src/auth/oidc.ts`、`invitations.ts`
- Create: `apps/mobile/sources/weknora/auth/AuthReturnScreen.tsx`、`InvitationScreen.tsx`
- Modify: `apps/mobile/app.config.js`、原生 auth 路由

**Interfaces:** `validateAuthReturn(expectedState:string,callback:string,expectedRedirect:string):URL`；OIDC facade `start(redirectURI:string)` 使用现有 `/auth/oidc/url`，exchange 的实际 code/state DTO 从 Go handler 冻结，不能假造现有移动 token 回调。

- [ ] **Step 1：跨 origin/错误 state 测试。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { validateAuthReturn } from './auth-return.ts';
test('auth callback cannot redirect to a foreign application', () => {
  assert.throws(()=>validateAuthReturn('s','evil://auth?state=s','weknora-dev://auth'), /AUTH_RETURN/);
  assert.throws(()=>validateAuthReturn('s','weknora-dev://auth?state=x','weknora-dev://auth'), /AUTH_RETURN/);
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/domain/src/mobile/auth-return.test.ts`。
- [ ] **Step 3：实现 callback 验证与系统浏览器接线。**

```ts
export function validateAuthReturn(state:string,callback:string,redirect:string):URL {
  const actual=new URL(callback), expected=new URL(redirect);
  if (!state || actual.protocol!==expected.protocol || actual.host!==expected.host || actual.pathname!==expected.pathname || actual.searchParams.get('state')!==state) throw new Error('AUTH_RETURN');
  return actual;
}
```

使用 Expo 系统浏览器 auth session；state 一次消费，取消/超时清理 pending。核对 `internal/router/routes_auth_tenant.go` 和 `internal/handler/auth.go` 的浏览器回调是否支持该 redirect；不支持时先写 handler 重定向白名单/一次性 code 的专项契约测试，再接原生，不允许把 access_token 放在任意跳转 URL。邀请 facade 移植现有 API 的 DTO，邀请失效/已使用反馈来自后端，无空间用户不进入 require-tenant 的页面。

- [ ] **Step 4：GREEN 与三条原生路径。** 同一测试命令；原生完成成功回调、用户取消、恶意回调不改变身份；邀请有效/失效与无成员身份各录一次，OIDC 服务缺失明确 blocked。
- [ ] **Step 5：提交。** stage 上述确切路径及包 exports；`git commit -m "feat(mobile): support verified OIDC and invitation entry"`。

### H05：作用域切换、缓存与草稿生命周期

**依赖：** H03。**交付：** 切空间/服务器/退出时旧结果不能污染新界面。

**Files:**
- Create: `apps/mobile/sources/weknora/platform/scope.ts`、`scope.test.ts`
- Create: `packages/domain/src/mobile/draft-key.ts`、`draft-key.test.ts`
- Modify: `apps/mobile/sources/app/_layout.tsx`、`packages/domain/package.json`

**Interfaces:** 复用 `createScopeController`；`draftKey(origin,userId,tenantId,sessionId):string`；`commitIfCurrent(scope,generation,apply):boolean`，scope 消费 `{isCurrent(g:number):boolean}`。

- [ ] **Step 1：写迟到结果与同会话不同空间草稿的测试。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { draftKey } from './draft-key.ts';
test('draft cannot cross tenants even for equal session IDs',()=>{
  assert.notEqual(draftKey('https://a','u','t1','s'),draftKey('https://a','u','t2','s'));
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/domain/src/mobile/draft-key.test.ts`。
- [ ] **Step 3：实现隔离 key 与结果门。**

```ts
export const draftKey=(origin:string,userId:string,tenantId:string|null,sessionId:string)=>
  JSON.stringify(['draft',origin,userId,tenantId,sessionId]);
export function commitIfCurrent(scope:{isCurrent(g:number):boolean},generation:number,apply:()=>void):boolean {
  if(!scope.isCurrent(generation)) return false;
  apply(); return true;
}
```

先让 scope advance/abort 旧请求，再关闭订阅、清理旧 scoped cache/草稿和 refresh generation，最后提交新身份与 provider；切空间用实际 `/auth/switch-tenant` 响应，不能仅更换本地 tenantId。AppState 返回只触发 H11 reconcile，不触发发送。为同一会话保存草稿时使用 key；退出清除该身份命名空间，不清他人或其他服务器的密钥。

- [ ] **Step 4：GREEN。** 同一命令，加移动 `scope.test.ts`：延迟 Promise 在 switch 后落地不调用 apply、旧 signal.aborted 为 true、logout during refresh 不写新 token。原生快速切空间后返回列表不得闪出旧资源。
- [ ] **Step 5：提交。** stage 本任务文件与 exports；`git commit -m "feat(mobile): isolate scoped state and late responses"`。

### H06：原生 REST/SSE transport 和跨 chunk 解析

**依赖：** H05、现有 T10 parser。**交付：** iOS/Android 上 POST SSE 可取消、增量显示且不破坏中文。

**Files:**
- Create: `packages/api-client/src/chat/transport.ts`、`transport.test.ts`
- Modify: `packages/api-client/src/chat/stream.ts`、`stream.test.ts`、`index.ts`
- Create: `apps/mobile/sources/weknora/platform/stream.ts`、`stream.test.ts`

**Interfaces:** `StreamTransport.open(request:HttpRequest,onBytes:(chunk:Uint8Array)=>void):Promise<void>`；SDK 构造 URL/headers/body，原生注入 `expo/fetch`。此端口不走返回 JSON 的 `HttpTransport.send`。

- [ ] **Step 1：写跨 CRLF 边界不会提前 dispatch 的测试。**

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createServerSentEventParser } from './stream.ts';
test('CRLF split across chunks is one newline',()=>{
 const rows:unknown[]=[]; const p=createServerSentEventParser(e=>rows.push(e));
 p.push('data: one\r'); p.push('\ndata: two\r'); p.push('\n\r'); p.push('\n');
 assert.deepEqual(rows,[{data:'one\ntwo'}]);
});
```

- [ ] **Step 2：RED。** `pnpm exec tsx --test packages/api-client/src/chat/stream.test.ts`。若已由并行任务修复则保留其实现，新增 UTF-8/abort 失败用例，不制造重复 parser。
- [ ] **Step 3：用保留末尾 CR 的扫描器修正 parser；原生流循环如下。**

```ts
import { fetch } from 'expo/fetch';
import type { StreamTransport } from '@weknora/api-client';
export const nativeStream:StreamTransport={async open(req,onBytes){
 const response=await fetch(req.url,{method:req.method,headers:req.headers,
   body:req.body===undefined?undefined:JSON.stringify(req.body),signal:req.signal});
 if(!response.ok) throw new Error(`STREAM_HTTP_${response.status}`);
 if(!response.headers.get('content-type')?.includes('text/event-stream')) throw new Error('STREAM_CONTENT_TYPE');
 if(!response.body) throw new Error('STREAM_BODY');
 const reader=response.body.getReader();
 try { while(true){const part=await reader.read(); if(part.done)break; onBytes(part.value);} }
 finally {await reader.cancel().catch(()=>{}); reader.releaseLock();}
}};
```

SDK 保有 `TextDecoder`，每次 `decode(chunk,{stream:true})`，结束 flush；parser 扫描原始 CR/LF，不先对每块全局 replace。abort 是本地断开，不当作后端停止；HTTP 错误转换为共享 ApiError 并保留 status，401 交 H03 处理，403 不自动重连。

- [ ] **Step 4：GREEN 与 native probe。** 同一共享测试及移动 stream 测试；构造中文 UTF-8 每个字节单独 chunk、无 body、HTML错误页、取消后迟到 chunk、正常结束 fixtures。真机接真实 SSE，记录首字节/增量/停止订阅，不仅 `expo export`。
- [ ] **Step 5：提交。** stage 本任务路径；`git commit -m "feat(mobile): add native streaming transport and boundary-safe parsing"`。
