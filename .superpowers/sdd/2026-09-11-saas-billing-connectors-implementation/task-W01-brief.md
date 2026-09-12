### Task 1: W01: 共享商业契约与API client

> **协调者补充（deviation note）**：无迁移编号问题。仓库约定：测试用 node:test + node:assert/strict，以 `pnpm exec tsx --test` 运行；根 `package.json` 的 `test:shared` glob 已覆盖 `packages/contracts/test/*.test.ts` 与 `packages/api-client/src/*.test.ts`，`typecheck:shared` 为显式文件清单——需把 `packages/api-client/src/commercial.ts`（及 contracts/src/index.ts 已在清单）纳入。`packages/contracts/src/index.ts` 与 `packages/api-client/src/index.ts` 的导出为加性修改。

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

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

```sh
pnpm exec tsx --test packages/contracts/test/commercial.test.ts packages/api-client/src/commercial.test.ts
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
pnpm run typecheck:shared
pnpm run test:shared
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add packages/contracts/src/commercial.ts packages/contracts/test/commercial.test.ts packages/api-client/src/commercial.ts packages/api-client/src/commercial.test.ts packages/contracts/src/index.ts packages/api-client/src/client.ts packages/api-client/src/index.ts package.json docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: expose typed space commerce client"
```
