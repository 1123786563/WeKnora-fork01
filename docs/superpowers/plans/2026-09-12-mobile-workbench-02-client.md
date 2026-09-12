# 移动工作台 02：产品身份与原生会话 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Happy 页面真实消费 WeKnora 会话，支持安全登录、作用域隔离、原生流和杀进程恢复。

**Architecture:** 先建立产品会话依赖与本地恢复模型，再替换 Happy 页面对全局 sync 的读取。旧协议仅由边界适配器保留，任何未接通能力都明确不可用。

**Tech Stack:** Expo 55/RN 0.83.1、React 19.2.0、SecureStore、共享 SDK、node:test；原生组件测试采用与 React 19.2.0 一致的 renderer。

**Spec:** [技术架构](../specs/2026-09-12-mobile-ai-saas-workbench-architecture.md) §6、§8–10、§13；[总计划](2026-09-12-mobile-ai-saas-workbench.md)。执行者同时读取本册与规格。

## Global Constraints

- “采用 **Happy 移动底座 + WeKnora 产品与执行控制 + Paseo 远程编码执行接入 + AWS 示例多模态交互复用**。”
- “分批交付不取消原 Happy 交互保留要求，未交付能力继续保留在清单中。”
- “任何 Run、事件、附件、审批都通过服务端认证上下文和持久所有权解析空间。”
- “同一请求 ID 与相同参数重放返回原 Run；同 ID 不同参数返回冲突。”
- “取消、失败和超时不抹掉实际用量，也不意味着外部副作用已回滚。”
- “预算授权、工具操作批准和连接授权是三件独立的事。”
- “子 Run 共享父任务预算树，不能复制一份可消费余额；新增子执行仍需授权和准入。”
- 保持 Expo 55 / React Native 0.83.1 / React 19.2.0 与当前 pnpm 10.28.2 工作区；执行前验证锁文件。Go 1.26.0；SQLite 与 PostgreSQL 分别验收。
- 本册所有新增接口和文件是实施目标；缺少环境记 `blocked-env`，不能把静态代码、fixture 或测试跳过写为真实运行验收。
- 实施前创建隔离工作区并带入未提交规格；保留并行任务修改。只提交本任务文件；不得 `git add .`、擅自发布或改写旧台账通过状态。

---

## 文件与责任边界

`weknora/platform`负责身份作用域和网络；`domain/mobile`负责持久投影；`weknora/conversations`负责UI适配。认证/缓存代码不依赖 Happy TokenStorage；原 SessionView 仅逐个入口接入，不搬动无关文件。

### W07：产品会话作用域与刷新失效

**依赖：** W06。

**Files：**

- Create: `apps/mobile/sources/weknora/platform/product-session.ts`、`apps/mobile/sources/weknora/platform/product-session.test.ts`
- Modify: `apps/mobile/sources/weknora/auth/session.tsx`、`apps/mobile/sources/app/_layout.tsx`

**Interfaces：**

Consumes `createScopeController`、`createRefreshCoordinator` 与 CredentialAdapter（现有共享包）。Produces `createProductScope(initial:{origin:string;userId:string|null;tenantId:string|null})`，方法 `switchTo(next):void`、`capture():{generation:number;signal:AbortSignal}`、`accept(generation:number):boolean`、`logout():void`；React Provider 为本 scope 持有 SDK 和刷新实例。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createProductScope } from './product-session.ts';
test('switching tenant aborts previous request and rejects late response',()=>{
 const scope=createProductScope({origin:'https://a.test',userId:'u',tenantId:'a'});
 const old=scope.capture();
 scope.switchTo({origin:'https://a.test',userId:'u',tenantId:'b'});
 assert.equal(old.signal.aborted,true);
 assert.equal(scope.accept(old.generation),false);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/platform/product-session.test.ts
```

预期：createProductScope 未定义。

- [ ] **Step 3：实现最小行为。**

```ts
import { createScopeController } from '@weknora/domain/scope';
export function createProductScope(initial:{origin:string;userId:string|null;tenantId:string|null}) {
 const controller=createScopeController(initial);
 return {
  switchTo(next:typeof initial){controller.switchScope(next.origin,next.userId,next.tenantId);},
  capture(){const h=controller.current();return {generation:h.scope.generation,signal:h.signal};},
  accept(generation:number){return controller.isCurrent(generation);},
  logout(){controller.switchScope(initial.origin,null,null);},
 };
}
```
logout 实际 origin 应跟随当前 scope，加入切 origin 再 logout 的失败测试后修正上面最小实现；不要把初始服务器写死到最终代码。

- [ ] **Step 4：接通实际入口。**

Provider 将 refresh invalidate 与 scope advance 绑定；用户/空间从产品登录与成员查询获得，不解读 Happy 加密 token。刷新开始捕获 generation，写凭证前再检查；切账号禁止旧 refresh 覆盖新凭证。401仅单飞刷新一次，403不刷新。切空间关闭语音和远程订阅，但不取消服务端任务。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 两个并发401只刷新一次；刷新中退出/切 origin不写回旧 token。
- draft/cache key在W09按origin/user/tenant隔离；登录凭证不存普通设置。
- `pnpm exec tsx --test packages/api-client/src/auth/refresh-coordinator.test.ts apps/mobile/sources/weknora/platform/product-session.test.ts`（执行前核对现有测试名）；双平台真实登录、切空间后列表不串数据。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add apps/mobile/sources/weknora/platform/product-session.ts apps/mobile/sources/weknora/platform/product-session.test.ts apps/mobile/sources/weknora/auth/session.tsx apps/mobile/sources/app/_layout.tsx
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): bind product identity to invalidatable scope"
```

### W08：原生 OIDC 一次性交换

**依赖：** W07。

**Files：**

- Create: `internal/application/repository/mobile_auth_exchange.go`、`internal/application/repository/mobile_auth_exchange_test.go`
- Create: `internal/handler/auth_mobile_exchange.go`、`internal/handler/auth_mobile_exchange_test.go`
- Modify: `internal/handler/auth.go`、`internal/router/routes_infra.go`、`packages/api-client/src/auth/oidc.ts`、`apps/mobile/sources/weknora/auth/AuthReturnScreen.tsx`
- Create: `migrations/versioned/000123_mobile_auth_exchange.{up,down}.sql`、`migrations/sqlite/000043_mobile_auth_exchange.{up,down}.sql`

**Interfaces：**

Produces 仓储 `ConsumeMobileExchange(ctx context.Context, codeHash,stateHash,redirectURI,challenge string,now time.Time) (string,error)`，返回待签发产品身份 subject 引用；禁止储存/返回长期 token 到深链。新增 POST `/auth/mobile/exchange` body={code,state,redirect_uri,code_verifier}；SDK 新 `exchangeNative`，保持原 `exchange` 兼容直到所有调用者明确切换。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package repository
import (
 "context"
 "testing"
 "time"
 "github.com/stretchr/testify/require"
)
func TestMobileExchangeSingleUse(t *testing.T) {
 db:=openRunTestDB(t); s:=NewMobileExchangeStore(db)
 now:=time.Now().UTC()
 require.NoError(t,s.Put(context.Background(),MobileExchange{CodeHash:"h",StateHash:"s",RedirectURI:"weknora://auth-return",Challenge:"p",Subject:"u1",ExpiresAt:now.Add(time.Minute)}))
 _,err:=s.ConsumeMobileExchange(context.Background(),"h","s","weknora://auth-return","p",now); require.NoError(t,err)
 _,err=s.ConsumeMobileExchange(context.Background(),"h","s","weknora://auth-return","p",now); require.Error(t,err)
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/application/repository -run TestMobileExchange -count=1
```

预期：NewMobileExchangeStore/MobileExchange 未定义。

- [ ] **Step 3：实现最小行为。**

定义 `MobileExchange{CodeHash,StateHash,RedirectURI,Challenge,Subject string; ExpiresAt time.Time; ConsumedAt *time.Time}`，`NewMobileExchangeStore(db *gorm.DB)`、`Put(ctx,row) error`。只存一次性随机 code 的 SHA-256；CAS consume 在事务中检查过期、redirect、state、S256 challenge。建议code有效期60秒，配置上限120秒；检验并发20次只一个成功。
```sql
UPDATE mobile_auth_exchanges SET consumed_at = ?
WHERE code_hash = ? AND state_hash = ? AND redirect_uri = ?
  AND challenge = ? AND consumed_at IS NULL AND expires_at > ?;
```
subject 只在 CAS 成功事务内读取；失败返回统一无效，不泄漏哪一字段错误。

- [ ] **Step 4：接通实际入口。**

沿现有服务端 IdP callback 验证身份后签发一次性 app code；手机登录启动绑定 state/redirect/challenge，不允许浏览器请求替换原生返回地址。exchange通过现有token签发服务返回JSON，深链无 token。客户端验证state并调用 exchangeNative，清除一次性state/verifier；旧浏览器登录行为保持。IdP 与应用回跳未经实测时功能关闭。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- URL带token拒绝；重复code、过期、错误verifier、跨redirect、错误state全部失败；成功生成真实产品令牌可读取本人会话。
- PostgreSQL/SQLite原子消费；移动回调冷启动/暖启动分别测试。
- 已有OIDC web登录回归；真实IdP缺失记blocked-env，P1密码登录不被阻塞。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/application/repository/mobile_auth_exchange.go internal/application/repository/mobile_auth_exchange_test.go internal/handler/auth_mobile_exchange.go internal/handler/auth_mobile_exchange_test.go internal/handler/auth.go internal/router/routes_infra.go packages/api-client/src/auth/oidc.ts apps/mobile/sources/weknora/auth/AuthReturnScreen.tsx migrations/versioned/000123_mobile_auth_exchange.up.sql migrations/versioned/000123_mobile_auth_exchange.down.sql migrations/sqlite/000043_mobile_auth_exchange.up.sql migrations/sqlite/000043_mobile_auth_exchange.down.sql
git diff --cached --check
git diff --cached --stat
git commit -m "feat(auth): support one-time native OIDC exchange"
```

### W09：原生流解码、持久投影与 cursor 提交

**依赖：** W01、W03、W06、W07。

**Files：**

- Create: `packages/domain/src/mobile/execution-cache.ts`、`packages/domain/src/mobile/execution-cache.test.ts`
- Create: `apps/mobile/sources/weknora/platform/stream-transport.ts`、`apps/mobile/sources/weknora/platform/stream-transport.test.ts`
- Create: `apps/mobile/sources/weknora/platform/execution-storage.ts`
- Modify: `packages/domain/package.json`、`apps/mobile/package.json`、`pnpm-lock.yaml`

**Interfaces：**

Produces `commitEvent(event:ExecutionEvent,save:(event:ExecutionEvent)=>Promise<void>,setCursor:(seq:number)=>Promise<void>):Promise<void>`；`StreamTransport.open(request:{url:string;headers:Record<string,string>;signal:AbortSignal},onBytes:(bytes:Uint8Array)=>void):Promise<void>`。原生持久库选择Expo兼容SQLite，敏感payload应用层AEAD加密；密钥SecureStore，不自制加密算法。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { commitEvent } from './execution-cache.ts';
test('projection failure cannot advance cursor',async()=>{
 let cursor=0;
 const event={schema_version:1 as const,run_id:'r',attempt_id:'a',seq:1,type:'text.delta',occurred_at:'2026-09-12T00:00:00Z',payload:{text:'你'}};
 await assert.rejects(commitEvent(event,async()=>{throw new Error('disk_full');},async seq=>{cursor=seq;}),/disk_full/);
 assert.equal(cursor,0);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test packages/domain/src/mobile/execution-cache.test.ts
```

预期：commitEvent 未定义。

- [ ] **Step 3：实现最小行为。**

```ts
import type { ExecutionEvent } from '@weknora/contracts';
export async function commitEvent(event:ExecutionEvent,save:(event:ExecutionEvent)=>Promise<void>,setCursor:(seq:number)=>Promise<void>) {
 await save(event);
 await setCursor(event.seq);
}
```
生产SQLite在同一事务提交投影和cursor。唯一键(scope,run_id,seq)；每个实体稳定ID更新投影；重复事件不叠加文本。schema加密版本、迁移和磁盘满显式处理；nonce每条写入随机且不复用，AAD包含scope/run/seq。

- [ ] **Step 4：接通实际入口。**

使用支持增量bytes的原生transport（优先验证Expo fetch读流），TextDecoder stream模式处理UTF-8；SSE parser正确保留跨chunk CRLF，不直接对每chunk替换孤立CR。兼容旧payload.seq和新id，id/payload冲突拒绝。原生网络能力不成立则此项blocked-env，不能用Web fetch证明。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 中文UTF-8逐字节、CRLF在两个chunk、最后无换行、心跳、256+事件、重复/乱序事件、磁盘满。
- 杀App后恢复同一cursor；cursor_expired先拉一致快照；快照+重放恰好一次投影。
- 清理账号缓存不删除其他账号；无离线自动发送命令。
- 依赖安装只在执行工作区按锁更新；匹配Expo 55后双平台bundle及真实读流。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add packages/domain/src/mobile/execution-cache.ts packages/domain/src/mobile/execution-cache.test.ts apps/mobile/sources/weknora/platform/stream-transport.ts apps/mobile/sources/weknora/platform/stream-transport.test.ts apps/mobile/sources/weknora/platform/execution-storage.ts packages/domain/package.json apps/mobile/package.json pnpm-lock.yaml
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): persist streamed execution projections safely"
```

### W10：Happy 会话视图模型和产品导航

**依赖：** W05–W07、W09。

**Files：**

- Create: `apps/mobile/sources/weknora/conversations/view-model.ts`、`apps/mobile/sources/weknora/conversations/view-model.test.ts`
- Modify: `apps/mobile/sources/-session/SessionView.tsx`、`apps/mobile/sources/components/ChatList.tsx`、`apps/mobile/sources/app/(app)/index.tsx`
- Create: `apps/mobile/sources/weknora/conversations/ConversationScreen.tsx`

**Interfaces：**

Produces `createSendController(send:(text:string,requestID:string)=>Promise<void>)` with `submit(text,requestID):Promise<void>`, `draft():string`, `busy():boolean`；ConversationViewModel包含messages、pendingInteractions、capabilities、execution和有类型commands。Consumes W06 SDK/W09投影，原Happy数据结构只在view-model映射。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createSendController } from './view-model.ts';
test('failed send keeps the draft',async()=>{
 const vm=createSendController(async()=>{throw new Error('offline');});
 await assert.rejects(vm.submit('review my report','q1'),/offline/);
 assert.equal(vm.draft(),'review my report');
 assert.equal(vm.busy(),false);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/conversations/view-model.test.ts
```

预期：createSendController 未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export function createSendController(send:(text:string,requestID:string)=>Promise<void>) {
 let text='',pending=false;
 return {draft:()=>text,busy:()=>pending,async submit(value:string,id:string){
  if(pending) throw new Error('SEND_IN_PROGRESS');
  text=value; pending=true;
  try {await send(value,id);text='';} finally {pending=false;}
 }};
}
```
网络unknown时保存原requestID并进入W06 lookup，不生成新ID重发。React状态订阅通过外层hook；messages/工具/审批使用稳定ID，不将展示序号当产品ID。

- [ ] **Step 4：接通实际入口。**

为ChatList增加受控数据入口，旧Happy调用路径保留边界兼容；ConversationScreen注入产品VM到SessionView。首页使用产品auth，不再要求Happy useAuth成功。Agent选择消费已有产品Agent列表，无KB仍允许新会话；切Agent只影响下一轮，旧消息保留agent_id。unsupported按钮显示原因而不是调用原Happy服务器。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 输入失败保留草稿、成功清空、双击一次提交、过期审批卡重取。
- 无KB通用Agent与知识问答同一会话组件；工具/思考/正文分开渲染。
- 组件测试真实挂载ConversationScreen验证SDK调用和展示；原生renderer版本与React一致，不用纯VM测试代替UI接线。
- 按Happy交互矩阵逐屏对比键盘、长列表、文件/Diff入口与取消反馈。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add apps/mobile/sources/weknora/conversations/view-model.ts apps/mobile/sources/weknora/conversations/view-model.test.ts apps/mobile/sources/-session/SessionView.tsx apps/mobile/sources/components/ChatList.tsx apps/mobile/sources/app/(app)/index.tsx apps/mobile/sources/weknora/conversations/ConversationScreen.tsx
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): connect Happy conversations to product state"
```

### W11：工作台列表、Agent 和空间入口

**依赖：** W03、W07、W10。

**Files：**

- Create: `internal/application/repository/workbench_list.go`、`internal/application/repository/workbench_list_test.go`
- Create: `apps/mobile/sources/weknora/workbench/WorkbenchScreen.tsx`、`apps/mobile/sources/weknora/workbench/list-query.ts`、`apps/mobile/sources/weknora/workbench/list-query.test.ts`
- Modify: `internal/router/routes_workbench.go`、`packages/api-client/src/mobile/executions.ts`、`apps/mobile/sources/app/(app)/index.tsx`

**Interfaces：**

Produces `buildExecutionListQuery(filter:{status?:string;agentID?:string;cursor?:string}):string`；服务端ListOwnedExecutions(ctx,tenant,owner,filter) returning items+next_cursor。cursor服务端签名或严格解析并绑定filter；order(created_at,run_id)稳定分页，query条件始终含owner与tenant。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { buildExecutionListQuery } from './list-query.ts';
test('filter encodes agent ID without introducing another parameter',()=>{
 const q=buildExecutionListQuery({agentID:'a&tenant_id=other',status:'running'});
 const p=new URLSearchParams(q);
 assert.equal(p.get('agent_id'),'a&tenant_id=other');
 assert.equal(p.has('tenant_id'),false);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/workbench/list-query.test.ts
```

预期：buildExecutionListQuery 未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export function buildExecutionListQuery(f:{status?:string;agentID?:string;cursor?:string}) {
 const q=new URLSearchParams();
 if(f.status)q.set('status',f.status);
 if(f.agentID)q.set('agent_id',f.agentID);
 if(f.cursor)q.set('cursor',f.cursor);
 return q.toString();
}
```
Go查询按运行快照中的agent_id/产品消息归属筛选，不使用只适用于IM的SessionListQuery.AgentID捷径。固定limit上限100，默认30。

- [ ] **Step 4：接通实际入口。**

新增GET /workbench/executions，对应WorkBenchScreen显示运行中/待处理/失败/已完成，点击进入真实session/run。底部入口为工作台、Agent、资源、空间与设置。空间切换消费W07；账单入口检查独立billing权限，普通成员可查看自己任务消耗，不扩大订单可见性。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 数据库种入同空间两个owner和不同空间同名agent，列表只能命中当前授权集合。
- 并发插入不重复分页；错误cursor拒绝，不回退全量列表。
- 原生空态、加载、网络失败、刷新、大字体、读屏、横屏各记录证据。
- `go test ./internal/application/repository -run TestWorkbenchList -count=1`。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/application/repository/workbench_list.go internal/application/repository/workbench_list_test.go apps/mobile/sources/weknora/workbench/WorkbenchScreen.tsx apps/mobile/sources/weknora/workbench/list-query.ts apps/mobile/sources/weknora/workbench/list-query.test.ts internal/router/routes_workbench.go packages/api-client/src/mobile/executions.ts apps/mobile/sources/app/(app)/index.tsx
git diff --cached --check
git diff --cached --stat
git commit -m "feat(workbench): list owned executions in native workspace"
```

### W12：前后台恢复控制器与两条真实链路

**依赖：** W09–W11；OIDC场景另外依赖W08。

**Files：**

- Create: `apps/mobile/sources/weknora/executions/recovery.ts`、`apps/mobile/sources/weknora/executions/recovery.test.ts`
- Modify: `apps/mobile/sources/weknora/conversations/ConversationScreen.tsx`
- Create: `docs/evidence/mobile-workbench/W12-native-recovery.md`

**Interfaces：**

Produces `recoverRun(ports:{status:()=>Promise<{terminal:boolean}>;subscribe:()=>Promise<void>;refreshHistory:()=>Promise<void>}):Promise<void>`。AppState active触发该控制器；background仅关闭订阅，绝不调用cancel。W06查询结果映射terminal，其他unknown保持可见。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { recoverRun } from './recovery.ts';
test('completed run restores history without opening stream',async()=>{
 const calls:string[]=[];
 await recoverRun({status:async()=>{calls.push('status');return {terminal:true};},subscribe:async()=>{calls.push('stream');},refreshHistory:async()=>{calls.push('history');}});
 assert.deepEqual(calls,['status','history']);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/executions/recovery.test.ts
```

预期：recoverRun 未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export async function recoverRun(p:{status:()=>Promise<{terminal:boolean}>;subscribe:()=>Promise<void>;refreshHistory:()=>Promise<void>}) {
 const state=await p.status();
 await p.refreshHistory();
 if(!state.terminal) await p.subscribe();
}
```
生产subscribe从W09最后提交cursor开始；并发active事件单飞，scope切换取消当前恢复；HTTP404不自动新建会话。

- [ ] **Step 4：接通实际入口。**

ConversationScreen订阅AppState并在卸载时清理；恢复失败显示重试查询按钮。两条真实链：①选知识资源→引用回答→历史；②无KB custom Agent→受控工具→批准/拒绝→产物。记录服务端RunID、模型版本与双平台客户端版本，脱敏证据中不保留token。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 在生成中切后台、杀App、切网络、服务端完成后重启，历史无重复且同一Run。
- 人为丢弃start响应后恢复原请求，数据库只有一次执行/用量。
- iOS和Android分别验证；模拟器进程存活不算交互验收。
- 文档记录步骤、预期、实测、失败原因与必要截图，未运行项保持未验收。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add apps/mobile/sources/weknora/executions/recovery.ts apps/mobile/sources/weknora/executions/recovery.test.ts apps/mobile/sources/weknora/conversations/ConversationScreen.tsx docs/evidence/mobile-workbench/W12-native-recovery.md
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): recover executions across app lifecycle"
```


