### Task 4: W04: 应用目录、连接与同步页面及API

> **协调者补充（deviation note）**：(1) apps/web 新文件需 `git add -f`（.gitignore `web/` 误伤，W02 已实证）。(2) Go 侧模式：路由仿 internal/router/routes_commercial.go（RegisterXxxRoutes(v1, params.XxxHandler)，router.go RouterParams 加字段 + 注册调用，container.go must(container.Provide(handler.NewXxxHandler))，见 router.go:91/303 与 container.go:455 先例）；handler 仿 internal/handler/commercial.go（tenant 一律取认证上下文，能力/授权分层拒绝）。A01 模型在 internal/appconnector/access.go 与 model.go（Connection/Installation 类型与 CanUseConnection/CanInstallInstallation），A02 在 oauth.go，A07 绑定在 sync.go（SyncBinding/BindingState/SyncPausedError/IsValidPauseReason），仓库层在 internal/application/repository/appconnector/install.go。(3) packages 侧为加性导出并把新文件加入根 typecheck:shared 清单（package.json）。

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

- [ ] **Step 4: 加入本任务"行为与边界"列明的反例，并执行同一定向命令。**

```sh
pnpm --filter @weknora/web test && pnpm typecheck:shared && pnpm typecheck:web
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
go build ./internal/handler ./internal/router ./internal/container && go vet 同三包
pnpm run test:shared
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add packages/contracts/src/appconnector.ts packages/api-client/src/appconnector.ts
git add -f apps/web/src/appconnector/connection-state.ts apps/web/src/appconnector/connection-state.test.ts apps/web/src/appconnector/AppsPage.tsx
git add packages/contracts/src/index.ts packages/api-client/src/client.ts packages/api-client/src/index.ts package.json internal/handler/app_connector.go internal/router/routes_app_connectors.go internal/router/router.go internal/container/container.go apps/web/src/App.tsx docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: expose space apps connections and sync status"
```
