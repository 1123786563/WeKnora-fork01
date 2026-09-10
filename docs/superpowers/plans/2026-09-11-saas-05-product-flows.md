# SaaS 05 用户端与管理端 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让购买、退款、安装、同步、审批和预算状态在产品中可操作且可核对。

**Architecture:** 共享contracts与API client保持跨端复用，Web页面只消费后端权威状态；用scope generation隔离切空间时的迟到响应。

**Tech Stack:** 当前仓库 Go 1.26、Gin、GORM、PostgreSQL／SQLite 版本化迁移；React 19、TypeScript 6、pnpm 10.28.2；官方 OpenMeter 固定版本；优先复用现有依赖。

**Spec:** [完整规格](../specs/2026-09-10-saas-billing-connectors-design.md)；[接口验证清单](../specs/2026-09-10-saas-billing-connectors-interface-verification.md)。执行者必须同时阅读本计划引用的规格章节与上下游契约。

## Global Constraints

- 每个空间始终独立购买、付款并承担费用；共享组织不改变归属，不共享余额或套餐。
- 费用预算批准和外部写入批准是独立条件，两者都满足才能执行。
- 首期不包含：跨空间合并付款或余额池、自动扣款、独立席位与存储加购包、嵌入式第三方 Web 应用、公开开发者上架与交易分成。
- 固定候选基线：`v1.0.0-beta.232`，提交 `887e0cac903ccd06e74d61ed23c651651d10c7a9`。
- 未在验证清单取得真实通过证据的外部能力，不得标记为生产可用；不使用旧本地 OpenMeter fork 接口。
- 保留已有 React、Craft、移动端和其他任务的工作；仅提交本任务明确拥有的文件，不使用 `git add .`。
- 下列内部接口是计划新增契约，不声称当前已存在；外部请求字段以 P00 的已验证契约为准。
- 时区、精度与分段报价采用完整规格的推荐默认值；外部模型无法无损表达时在 P00 停止依赖任务并修正规格，不能自行换商业语义。

---

## 依赖与执行边界

W01依赖商业HTTP契约；W04依赖A01/A02/A07；W05依赖A03/U04；只有完成真实后端接线才可做最终浏览器验收。

本文件仅编写计划，不表示已实现或已测试。执行时先使用 using-git-worktrees 建立隔离工作区；所有命令从仓库根执行。每一步只做所列一个动作；较长代码修改按列出的文件／函数逐项执行，完成该任务的 RED→GREEN 与独立 review 后再进入下一任务。

## 文件结构与职责

packages/contracts/api-client：明确wire契约；apps/web/src/commercial与appconnector：按业务流分文件；不复制凭据或业务规则到UI。当前Node测试仅覆盖状态/契约，真实浏览器流程由O03验收。

### Task W01: 共享商业契约与API client

**Files:**
- Create: `packages/contracts/src/commercial.ts`、`packages/contracts/test/commercial.test.ts`
- Create: `packages/api-client/src/commercial.ts`、`packages/api-client/src/commercial.test.ts`
- Modify: `packages/contracts/src/index.ts`、`packages/api-client/src/client.ts`、`packages/api-client/src/index.ts`、`package.json`

**Interfaces:**
- Consumes: F05/C01/C05路由；现有ClientRequest与HttpTransport；ScopeController。
- Produces: TypeScript `OrderView`、`CommercialSummary`、`parseOrderView(unknown): OrderView`；`createCommercialApi(request: (input: ClientRequest)=>Promise<unknown>)` 返回 `getOrder(id:string,signal?:AbortSignal):Promise<OrderView>`、`summary(signal?:AbortSignal):Promise<CommercialSummary>`、`quote(input:QuoteInput):Promise<QuoteView>`、`createOrder(input:CreateOrderInput):Promise<OrderView>`、`requestRefund(input:RefundInput):Promise<{id:string;state:string}>`。所有输入与结果类型在 commercial.ts 定义。

**行为与边界：** HTTP测试用已有HttpTransport fake断言method/path/body，金额始终字符串；跨scope请求结果丢弃由现有scope generation判断。纯契约测试不等于UI验收。

**验收映射：** 规格12；AC-01/05；OPS-04

- [ ] **Step 1: 在 `packages/contracts/test/commercial.test.ts` 写入以下失败断言。**

```typescript
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseOrderView } from '../src/commercial.ts';
test('paid is distinct from fulfilled', () => {
  const value = parseOrderView({id:'o1',payment:'paid',fulfillment:'pending',amount_fen:'100',currency:'CNY'});
  assert.equal(value.fulfillment, 'pending');
  assert.throws(() => parseOrderView({...value, amount_fen:100}));
});
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
pnpm exec tsx --test packages/contracts/test/commercial.test.ts packages/api-client/src/commercial.test.ts
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```typescript
export interface OrderView {
 id: string; payment: 'pending'|'paid'|'closed';
 fulfillment: 'pending'|'processing'|'fulfilled'|'attention';
 amount_fen: string; currency: 'CNY';
}
export interface CommercialSummary {
 plan_name:string; paid_until:string|null; available:string;
 held:string; refund_locked:string; as_of:string; stale:boolean;
}
export interface QuoteView { id:string;amount_fen:string;credit_delta:string;expires_at:string; }
export interface QuoteInput { plan_key:string; plan_version:number; subscription_version:number; }
export interface CreateOrderInput { quote_id:string; provider:'wechat'|'alipay'; idempotency_key:string; }
export interface RefundInput { order_id:string; amount_fen:string; reason:string; idempotency_key:string; }
export function parseOrderView(value:unknown):OrderView {
 if(typeof value!=='object'||value===null) throw new Error('invalid order');
 const v=value as Record<string,unknown>;
 if(typeof v.id!=='string'||typeof v.amount_fen!=='string'||!/^\d+$/.test(v.amount_fen)||
    v.currency!=='CNY'||!['pending','paid','closed'].includes(String(v.payment))||
    !['pending','processing','fulfilled','attention'].includes(String(v.fulfillment))) throw new Error('invalid order');
 return v as unknown as OrderView;
}
```

client映射 GET `/api/v1/commercial/orders/:id`、`/summary`、POST `/quotes`、`/orders`、`/refunds`；服务端响应统一现有 `{success:true,data}` envelope，先取data再解析；summary也逐字段校验。写输入不接受tenant_id，由transport当前scope处理。索引导出与根typecheck/test命令纳入新文件。失败明确错误码，不自动重试付款创建的新key。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
pnpm exec tsx --test packages/contracts/test/commercial.test.ts packages/api-client/src/commercial.test.ts
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add packages/contracts/src/commercial.ts packages/contracts/test/commercial.test.ts packages/api-client/src/commercial.ts packages/api-client/src/commercial.test.ts packages/contracts/src/index.ts packages/api-client/src/client.ts packages/api-client/src/index.ts package.json docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: expose typed space commerce client"
```

### Task W02: 空间购买、充值与订单结果页面

**Files:**
- Create: `apps/web/src/commercial/order-state.ts`、`apps/web/src/commercial/order-state.test.ts`
- Create: `apps/web/src/commercial/BillingPage.tsx`、`apps/web/src/commercial/CheckoutPage.tsx`
- Modify: `apps/web/src/App.tsx`

**Interfaces:**
- Consumes: W01OrderView与commercial client；现有ScopeController和packages/ui。
- Produces: `orderMessage(OrderView): string`；`BillingPage` props `{client:WeKnoraClient;scopeController:ReturnType<typeof createScopeController>}`；CheckoutPage相同props加`orderId:string`。

**行为与边界：** 分别验证初购、充值、升级、续费、关闭页面重开、paid未到账、错误重试不重复购买；布局沿用现有设计系统，不重写App导航。

**验收映射：** B15/B16/B17；AC-05/07/09；OPS-04

- [ ] **Step 1: 在 `apps/web/src/commercial/order-state.test.ts` 写入以下失败断言。**

```typescript
import test from 'node:test';
import assert from 'node:assert/strict';
import { orderMessage } from './order-state.ts';
test('payment success never implies credits delivered',()=>{
 assert.equal(orderMessage({id:'1',payment:'paid',fulfillment:'pending',amount_fen:'100',currency:'CNY'}),'已付款，权益处理中');
});
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:web && pnpm build:web
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```typescript
import type { OrderView } from '@weknora/contracts';
export function orderMessage(order:OrderView):string {
 if(order.fulfillment==='fulfilled') return '权益已生效';
 if(order.payment==='paid') return '已付款，权益处理中';
 if(order.payment==='closed') return '订单已关闭';
 return '等待付款';
}
```

BillingPage展示当前空间名、套餐／到期、额度批次／有效期、占用与退款锁定、as_of和stale；CheckoutPage先报价再创建订单，显示人民币实付、月周期、升级差价和充值12个月到期说明。付款成功后轮询原订单ID，不重建订单；页面关闭不取消后台履约。

核心渲染绑定真实状态：

```tsx
<section aria-live="polite">
 <h2>{spaceName} 的订单</h2>
 <p>{orderMessage(order)}</p>
 <button type="button" onClick={refreshOrder}>刷新订单状态</button>
</section>
```

`spaceName`来自当前空间，`order`为已解析OrderView，`refreshOrder:()=>void`只查询当前orderId；卸载/切空间abort并使用scopeController.isCurrent丢弃迟到结果。按钮权限从后端capability投影读取，隐藏按钮不能代替服务端校验。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:web && pnpm build:web
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add apps/web/src/commercial/order-state.ts apps/web/src/commercial/order-state.test.ts apps/web/src/commercial/BillingPage.tsx apps/web/src/commercial/CheckoutPage.tsx apps/web/src/App.tsx docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: show scoped checkout and fulfillment states"
```

### Task W03: 账单授权、退款申请和平台审核页面

**Files:**
- Create: `apps/web/src/commercial/refund-state.ts`、`apps/web/src/commercial/refund-state.test.ts`
- Create: `apps/web/src/commercial/RefundPage.tsx`、`apps/web/src/commercial/AdminCommercialPage.tsx`
- Modify: `packages/contracts/src/commercial.ts`、`packages/api-client/src/commercial.ts`、`apps/web/src/App.tsx`

**Interfaces:**
- Consumes: C05退款状态；F02商业授权；W01 client。
- Produces: `refundMessage(state:string):string`；`RefundView{id:string;state:string;amount_fen:string;locked_credits:string}`；client `getRefund(id:string)`、`reviewRefund(id:string,decision:"approve"|"reject",expectedVersion:number)`。

**行为与边界：** 浏览器覆盖Owner委派账单管理员、未授权Admin、空间申请与平台审批、退款unknown、权益撤回重试；台账保存各角色看到的结果。

**验收映射：** B18/B19/B20/B21；AC-02/10；OPS-01/04

- [ ] **Step 1: 在 `apps/web/src/commercial/refund-state.test.ts` 写入以下失败断言。**

```typescript
import test from 'node:test';
import assert from 'node:assert/strict';
import { refundMessage } from './refund-state.ts';
test('remote success with pending revocation is not retried payment',()=>{
 assert.equal(refundMessage('revocation_pending'),'退款已完成，权益调整处理中');
 assert.equal(refundMessage('refund_unknown'),'退款结果核对中');
});
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:web
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```typescript
export function refundMessage(state:string):string {
 if(state==='completed') return '退款与权益调整已完成';
 if(state==='revocation_pending') return '退款已完成，权益调整处理中';
 if(state==='refund_unknown') return '退款结果核对中';
 if(state==='rejected') return '退款申请未通过';
 return '退款处理中';
}
```

RefundPage展示原订单、申请金额、锁定与核对状态，不提供 unknown 时再次退款的新键。AdminCommercialPage分开目录草稿/发布、授权审计、退款审核、paid待履约、结算差异列表；目录发布需预览不可变版本，退款批准展示当前核算的可退本金/额度和expectedVersion。平台页面只对真实运营能力显示，空间Owner也不能自行审批。

所有恢复按钮传原operation ID，不新建业务操作；审批前刷新可退状态，版本冲突提示重新核对。每个字段在contracts显式parse，禁止用any透传运行结果。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:web
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add apps/web/src/commercial/refund-state.ts apps/web/src/commercial/refund-state.test.ts apps/web/src/commercial/RefundPage.tsx apps/web/src/commercial/AdminCommercialPage.tsx packages/contracts/src/commercial.ts packages/api-client/src/commercial.ts apps/web/src/App.tsx docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: separate refund requests from platform review"
```

### Task W04: 应用目录、连接与同步页面及API

**Files:**
- Create: `packages/contracts/src/appconnector.ts`、`packages/api-client/src/appconnector.ts`
- Create: `apps/web/src/appconnector/connection-state.ts`、`apps/web/src/appconnector/connection-state.test.ts`、`apps/web/src/appconnector/AppsPage.tsx`
- Create: `internal/handler/app_connector.go`、`internal/router/routes_app_connectors.go`
- Modify: `internal/router/router.go`、`internal/container/container.go`、`apps/web/src/App.tsx`
- Modify: `packages/contracts/src/index.ts`、`packages/api-client/src/client.ts`、`packages/api-client/src/index.ts`、`package.json`

**Interfaces:**
- Consumes: A01/A02/A07；现有DataSource API与scopeController。
- Produces: `ConnectionView{id:string;kind:"personal"|"space";state:string;owner_id:string|null}`；`connectionLabel(ConnectionView):string`；GET/POST `/api/v1/apps/installations`、POST `/:id/upgrade`、`/:id/disable`、GET/POST `/api/v1/apps/connections`、POST `/connections/:id/revoke`；创建连接走A02授权，不返回credential。

**行为与边界：** 同安装多连接、空间切换迟到响应、授权回调恢复、撤销、同步暂停恢复、版本scope新增；输出中搜索无access_token/refresh_token/密钥。

**验收映射：** B08/B09/B24；AC-15/16；OPS-04

- [ ] **Step 1: 在 `apps/web/src/appconnector/connection-state.test.ts` 写入以下失败断言。**

```typescript
import test from 'node:test';
import assert from 'node:assert/strict';
import { connectionLabel } from './connection-state.ts';
test('installation does not imply shared credentials',()=>{
 assert.equal(connectionLabel({id:'c',kind:'personal',state:'active',owner_id:'u'}),'个人连接');
});
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:shared && pnpm typecheck:web
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```typescript
export interface ConnectionView {
 id:string;kind:'personal'|'space';state:string;owner_id:string|null;
}
export function connectionLabel(c:ConnectionView):string {
 if(c.state==='revoked') return '连接已撤销';
 return c.kind==='personal'?'个人连接':'空间连接';
}
```

UI依次展示安装版本与费用/权限说明、个人/空间连接、资源范围、目标知识库、同步进度与暂停原因。后端只返回connection投影，credential_ref也不需暴露。W04 同步将 appconnector 的 type/parser/client 导出到各 package index，并把新文件加入根 typecheck 命令，不能仅写未被引用的孤立模块。新增路由复用现有tenant上下文和API-key约束，所有ID按tenant查找，写请求使用expectedVersion。安装升级scope变化必须进入重新授权提示；普通成员提供申请入口而非假安装成功。

sync API继续现有datasource handler，响应增加binding与暂停原因的显式字段；对旧数据源显示迁移/重新授权状态，不悄悄替换凭据归属。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:shared && pnpm typecheck:web
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add packages/contracts/src/appconnector.ts packages/api-client/src/appconnector.ts apps/web/src/appconnector/connection-state.ts apps/web/src/appconnector/connection-state.test.ts apps/web/src/appconnector/AppsPage.tsx packages/contracts/src/index.ts packages/api-client/src/client.ts packages/api-client/src/index.ts package.json internal/handler/app_connector.go internal/router/routes_app_connectors.go internal/router/router.go internal/container/container.go apps/web/src/App.tsx docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: expose space apps connections and sync status"
```

### Task W05: 任务预算与具体写操作审批交互

**Files:**
- Create: `apps/web/src/appconnector/action-state.ts`、`apps/web/src/appconnector/action-state.test.ts`
- Create: `apps/web/src/appconnector/ActionApproval.tsx`、`apps/web/src/commercial/TaskBudget.tsx`
- Modify: `packages/contracts/src/appconnector.ts`、`packages/api-client/src/appconnector.ts`、`internal/handler/app_connector.go`、`internal/router/routes_app_connectors.go`、`internal/router/routes_commercial.go`、`apps/web/src/App.tsx`

**Interfaces:**
- Consumes: A03ActionService；U04BudgetService；W04API模块；当前会话/任务UI接入点。
- Produces: `ActionView{id:string;state:string;digest:string;target:string;content:string;connection_name:string}`；`actionMessage(state:string):string`；POST `/api/v1/apps/actions/prepare`、`/:id/approve`、`/:id/execute`、GET `/:id`；POST `/api/v1/commercial/tasks/:id/budget/extend`。

**行为与边界：** 审批目标/内容变更、断网重连、任务超预算、追加但写审批未满足、取消后已发送结果仍展示；不得自动勾选发送授权。

**验收映射：** B12/B13/B22；AC-11/17/18；OPS-04

- [ ] **Step 1: 在 `apps/web/src/appconnector/action-state.test.ts` 写入以下失败断言。**

```typescript
import test from 'node:test';
import assert from 'node:assert/strict';
import { actionMessage } from './action-state.ts';
test('unknown write is shown as reconciliation',()=>{
 assert.equal(actionMessage('unknown'),'正在核对外部执行结果');
 assert.equal(actionMessage('awaiting_approval'),'等待操作批准');
});
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:shared && pnpm typecheck:web && pnpm build:web
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```typescript
export function actionMessage(state:string):string {
 if(state==='unknown') return '正在核对外部执行结果';
 if(state==='awaiting_approval') return '等待操作批准';
 if(state==='succeeded') return '操作已完成';
 return '操作处理中';
}
```

ActionApproval渲染server snapshot的完整目标和内容，Approve提交id+digest+expectedVersion；编辑内容调用Prepare产生新digest。unknown只提供查询/联系核对入口，不显示“再发一次”。TaskBudget分别展示批准上限、已用、占用和可追加原因，追加按钮与外部批准互不合并。

事件重连以server snapshot恢复，不依赖组件内pending；现有Craft/mobile消费者通过共享contract复用，不能在本任务覆盖它们的导航。在现有任务详情的接入位置挂载这两个组件，并在 routes_app_connectors/routes_commercial 注册本任务端点；不只导出未使用组件。实际组件至少包含可访问label、焦点返回和aria-live状态提示。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:shared && pnpm typecheck:web && pnpm build:web
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add apps/web/src/appconnector/action-state.ts apps/web/src/appconnector/action-state.test.ts apps/web/src/appconnector/ActionApproval.tsx apps/web/src/commercial/TaskBudget.tsx packages/contracts/src/appconnector.ts packages/api-client/src/appconnector.ts internal/handler/app_connector.go internal/router/routes_app_connectors.go internal/router/routes_commercial.go apps/web/src/App.tsx docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: distinguish budget approval from external action approval"
```

