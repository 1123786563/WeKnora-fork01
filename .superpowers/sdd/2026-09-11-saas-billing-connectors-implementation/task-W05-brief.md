### Task 5: W05: 任务预算与具体写操作审批交互

> **协调者补充（deviation note）**：(1) apps/web 新文件需 `git add -f`。(2) A03 Action 状态词表固定为 awaiting_approval/authorized/queued/dispatched/succeeded/failed/unknown（internal/appconnector/action.go:30-36），actionMessage 须全覆盖（unknown/awaiting_approval 按 Step 3 原文，其余默认'操作处理中'，succeeded'操作已完成'）。(3) U04 预算服务在 internal/application/service/commercial/budget.go（BudgetService.Extend(ctx, tenant, runID, key, extra Credits)），追加端点挂在 routes_commercial.go 的 commercial 组（能力门复用 RequireExplicitCommercialCapability）。（4）W04 已建 internal/handler/app_connector.go（app 前缀视图类型）与 routes_app_connectors.go——本任务在其上加性扩展，不改 W04 既有端点行为。(5) packages 契约/client 为加性扩展（ActionView + parser + prepare/approve/execute/get 方法）。

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

ActionApproval渲染server snapshot的完整目标和内容，Approve提交id+digest+expectedVersion；编辑内容调用Prepare产生新digest。unknown只提供查询/联系核对入口，不显示"再发一次"。TaskBudget分别展示批准上限、已用、占用和可追加原因，追加按钮与外部批准互不合并。

事件重连以server snapshot恢复，不依赖组件内pending；现有Craft/mobile消费者通过共享contract复用，不能在本任务覆盖它们的导航。在现有任务详情的接入位置挂载这两个组件，并在 routes_app_connectors/routes_commercial 注册本任务端点；不只导出未使用组件。实际组件至少包含可访问label、焦点返回和aria-live状态提示。

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:shared && pnpm typecheck:web && pnpm build:web
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
go build ./internal/handler ./internal/router && go vet 同两包
pnpm run test:shared
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add apps/web/src/appconnector/action-state.ts apps/web/src/appconnector/action-state.test.ts
git add -f apps/web/src/appconnector/ActionApproval.tsx apps/web/src/commercial/TaskBudget.tsx
git add packages/contracts/src/appconnector.ts packages/api-client/src/appconnector.ts internal/handler/app_connector.go internal/router/routes_app_connectors.go internal/router/routes_commercial.go apps/web/src/App.tsx docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: distinguish budget approval from external action approval"
```
