### Task 2: W02: 空间购买、充值与订单结果页面

> **协调者补充（deviation note）**：仓库约定——apps/web 测试为 `node --import tsx --test src/**/*.test.ts`（根命令 `pnpm --filter @weknora/web test`，另有 `pnpm typecheck:web`/`pnpm build:web`）；页面组件沿用 App.tsx 中 KnowledgeBasesPage 模式（props {client, scopeController}、scopeController.current()、isCurrent 守卫、@weknora/ui 的 Button/Card/Status、wk-* 样式类）；W01 的 client.commercial 已提供 getOrder/summary/quote/createOrder/requestRefund；不要重写 main.tsx 导航（App.tsx 加导出即可，现有页面不动）。

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

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

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
