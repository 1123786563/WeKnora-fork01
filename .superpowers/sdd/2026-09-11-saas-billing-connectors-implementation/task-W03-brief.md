### Task 3: W03: 账单授权、退款申请和平台审核页面

> **协调者补充（deviation note）**：(1) 沿用 W02 的仓库约定——apps/web 新文件需 `git add -f`（根 .gitignore `web/` 无前导斜杠规则误伤 apps/web，W02 审查已实证）；页面沿用 KnowledgeBasesPage/W02 页面模式（scopeController.isCurrent 守卫、@weknora/ui、wk-* 类）。(2) C05 Go 域层退款状态词表见 internal/commercial/refund.go（含 revocation_pending；服务端投影状态以该文件为准），前端契约状态字符串须与其对齐，refundMessage 覆盖 completed/revocation_pending/refund_unknown/rejected/其余默认。(3) 对 packages/contracts/src/commercial.ts 与 packages/api-client/src/commercial.ts 的修改为加性（新增 RefundView/parse/getRefund/reviewRefund，不动 W01 既有导出）。

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

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:web
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
pnpm run typecheck:shared && pnpm run test:shared
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add -f apps/web/src/commercial/refund-state.ts apps/web/src/commercial/refund-state.test.ts apps/web/src/commercial/RefundPage.tsx apps/web/src/commercial/AdminCommercialPage.tsx
git add packages/contracts/src/commercial.ts packages/api-client/src/commercial.ts apps/web/src/App.tsx docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: separate refund requests from platform review"
```
