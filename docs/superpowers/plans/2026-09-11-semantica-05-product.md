# Semantica 产品集成 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把索引与推理能力接入Go API、React客户端和后端迁移流程。

**Architecture:** 用户经Go使用能力；客户端只反映授权结果，后端切换有明确管理动作。

**Tech Stack:** Go/Gin/GORM、Python/gRPC/Semantica、PostgreSQL/候选隔离图与向量存储、React/TypeScript；按涉及范围使用。

**Spec:** [架构规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)；[总计划与完整类型表](2026-09-11-semantica-implementation.md)。

## Global Constraints

完整继承总计划 Global Constraints，必须先读；本计划不扩大语义服务所有权、授权范围或首版能力。所有代码和测试均为后续实施输入，未执行。

[ADR-0002](../../adr/0002-semantica-independent-service.md) 与 [2026-09-20 rebaseline](2026-09-20-semantica-rebaseline.md) 优先。全部 24 项仍为 pending；不得因计划存在而默认启用、自动迁移或把未验收 KB 切换为 Semantica。

---


## W01：用户API与共享客户端契约

**依赖：** Q04,I05。

**文件与职责：**

- `internal/handler/semantic.go`：状态/重试/检索/推理API
- `internal/handler/semantic_test.go`：权限与错误合同
- `internal/router/routes_knowledge.go`：业务路由
- `packages/contracts/src/semantic.ts`：TS显式DTO及校验
- `packages/contracts/test/semantic.test.ts`：64位ID与状态测试
- `packages/api-client/src/semantic.ts`：客户端调用
- `packages/api-client/src/semantic.test.ts`：错误/取消映射
- `packages/contracts/package.json`：新增子路径export
- `packages/api-client/package.json`：新增子路径export

**接口：** 建议路由 GET /api/v1/knowledge-bases/:id/semantic/status、POST .../semantic/search、POST .../semantic/reason、POST .../semantic/preview、POST .../semantic/enable；POST /api/v1/knowledge/:id/semantic/retry。客户端 `previewSemanticPilot(kbID, signal?)` 和 `enableSemanticPilot(kbID, settings, signal?)` 只发送业务请求与 DTO，不传授权角色；`captureRequest(action: () => Promise<unknown>): Promise<{body?: string}>` 是 api-client 测试 helper。ID/revision为十进制字符串，后端禁止body覆盖path scope。`newSemanticHandlerFixture(t) *semanticHandlerFixture` 必须使用认证 context 和实际 KB grant；其 `GrantKBRole(subjectID, kbID, role string)` 与 `RequestAs(subjectID, method, path string, body any)` 用于 handler 合同测试。preview/enable 需要 KB 管理权限，默认关闭。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
test("semantic revision preserves uint64 precision", () => {
  const value = parseSemanticStatus({
    document_id: "d1", revision: "18446744073709551615",
    semantic_status: "stale", active_generation: "g1"
  });
  assert.equal(value.revision, "18446744073709551615");
});
test("pilot client omits authorization role from its request", async () => {
  const request = await captureRequest(() => previewSemanticPilot("kb-1"));
  assert.deepEqual(JSON.parse(request.body || "{}"), {});
  assert.equal("role" in JSON.parse(request.body || "{}"), false);
});

func TestSemanticPilotRequiresKBManager(t *testing.T) {
    f := newSemanticHandlerFixture(t)
    f.GrantKBRole("reader-1", "kb-1", "reader")
    denied := f.RequestAs("reader-1", http.MethodPost, "/api/v1/knowledge-bases/kb-1/semantic/preview", map[string]any{"role": "manager"})
    if denied.Code != http.StatusForbidden { t.Fatalf("got %d", denied.Code) }
    f.GrantKBRole("manager-1", "kb-1", "manager")
    allowed := f.RequestAs("manager-1", http.MethodPost, "/api/v1/knowledge-bases/kb-1/semantic/preview", map[string]any{"role": "reader"})
    if allowed.Code != http.StatusOK { t.Fatalf("got %d", allowed.Code) }
}
```

- [ ] **2. 确认 RED**。执行 `pnpm exec tsx --test packages/contracts/test/semantic.test.ts packages/api-client/src/semantic.test.ts` 与 `go test ./internal/handler -run TestSemanticPilot -count=1`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 使用现有session/API key权限路由，不另建前端直连Python；读状态需资源可读，重试/preview/enable/配置变更需 KB 管理权限。默认 disabled；preview 返回存量规模和预算设置，enable 才提交自动 backfill。管理端预算设置必须经授权、额度校验和持久化，随后仅由 Go 为每次模型调用派生预算上限，客户端不得把预算上限传入模型执行 wire 请求。**

- [ ] **4. 定义带 requested_mode、actual_mode、status、generation、stale/version、证据、推理类型和限制的DTO；未知枚举视为兼容未知状态，不能默认ready；内部错误不泄露服务地址/凭据**

- [ ] **5. 客户端支持AbortSignal，retry通过服务端幂等提交；所有scope由服务端根据path与身份决定；补充API文档及路由合同测试**

关键实现约束：

```
export type SemanticStatus =
  | "disabled" | "queued" | "indexing" | "ready" | "stale" | "failed" | "deleting";
export interface SemanticDocumentStatus {
  document_id: string;
  revision: string;
  semantic_status: SemanticStatus;
  active_generation?: string;
}
// parseSemanticStatus在contracts/src/semantic.ts实现，拒绝浮点revision。
```

- [ ] **6. 确认 GREEN 与验收**。重跑 `pnpm exec tsx --test packages/contracts/test/semantic.test.ts packages/api-client/src/semantic.test.ts`，预期退出码 0；另完成：运行 go test ./internal/handler -run TestSemantic -count=1；共享typecheck覆盖新入口；API key和session都不能越权查看或重试。

- [ ] **7. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 W01 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): w01 用户API与共享客户端契约`。

## W02：React索引状态与推理证据流程

**依赖：** W01。

**文件与职责：**

- `packages/domain/src/semantic.ts`：状态展示纯函数
- `packages/domain/src/semantic.test.ts`：可用性与失败状态
- `apps/web/src/semantic/SemanticPanel.tsx`：状态与操作组件
- `apps/web/src/semantic/EvidencePanel.tsx`：版本引用与推理类别
- `apps/web/src/semantic/view-model.ts`：交互状态
- `apps/web/src/semantic/view-model.test.ts`：取消/陈旧结果状态
- `apps/web/src/App.tsx`：挂接现有KB/文档导航
- `apps/web/src/styles.css`：复用现有tokens的必要样式
- `apps/web/tests/semantic-flow.spec.ts`：真实浏览器用户流程
- `apps/web/playwright.semantic.config.ts`：语义测试配置与真实服务baseURL
- `apps/web/package.json`：新增Playwright测试开发依赖与脚本，固定实际安装版本
- `pnpm-lock.yaml`：仅由包管理器更新测试依赖锁
- `package.json`：把新测试与typecheck入口加入现有脚本

**接口：** deriveSemanticView(status:SemanticDocumentStatus)->{label:string,canRetry:boolean,canReadText:boolean}；SemanticPanel接收KB/document身份及API client，EvidencePanel接收query结果；主App通过现有导航接入，不重写布局。试点 KB 管理页展示 preview 的存量规模和预算设置，再提供 enable；未验收 KB 继续显示 native。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
test("semantic failure keeps parsed text readable", () => {
  const view = deriveSemanticView({document_id:"d1",revision:"1",semantic_status:"failed"});
  assert.equal(view.canReadText, true);
  assert.equal(view.canRetry, true);
});
test("deleting document cannot retry semantic indexing", () => {
  assert.equal(deriveSemanticView({document_id:"d1",revision:"2",semantic_status:"deleting"}).canRetry, false);
});
```

- [ ] **2. 确认 RED**。执行 `pnpm exec tsx --test packages/domain/src/semantic.test.ts apps/web/src/semantic/view-model.test.ts`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 新增索引状态与单独重试交互，区分解析完成/语义失败/旧索引；KB 管理员先看到 preview 的存量规模和预算设置才可 enable，启用后显示 backfill 进度；权限决定操作显示且服务器再次校验，避免前端状态充当授权**

- [ ] **4. 展示 requested/actual mode 及普通检索、GraphRAG、规则推导、模型推断的实际类型；证据点开准确document/revision/chunk，历史原文不可取时明确说明，不跳成当前版本。Reason 未完成显示重试入口，普通检索必须显示未使用语义图谱。**

- [ ] **5. 取消请求时Abort并丢弃迟到响应；query_id/generation绑定当前视图，权限错误清空结果。不得在模型推断旁显示“已证明”**

- [ ] **6. 将新domain文件和API入口纳入typecheck脚本；复用现有样式，不引入新的组件框架。浏览器测试从已有会话fixture登录并走真实Go接口**

关键实现约束：

```
const canRetry = status.semantic_status === "failed" || status.semantic_status === "stale";
if (response.query_id !== activeQueryId || signal.aborted) return;
// 接收成功响应后才渲染知识内容；progress只能更新不含证据的进度状态。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `pnpm exec tsx --test packages/domain/src/semantic.test.ts apps/web/src/semantic/view-model.test.ts`，预期退出码 0；另完成：pnpm typecheck:shared、pnpm typecheck:web通过；执行 `pnpm --filter @weknora/web exec playwright test --config playwright.semantic.config.ts`，覆盖上传→索引→问答→引用→重试→取消→撤权；保存截图与无障碍键盘操作证据。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 W02 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): w02 React索引状态与推理证据流程`。

## W03：后端影子构建、切换与回滚

**依赖：** W01,I04,Q04。

**文件与职责：**

- `internal/application/service/semantic_backend.go`：desired/active状态机
- `internal/application/service/semantic_backend_test.go`：切换/CAS/回滚追赶
- `internal/handler/semantic.go`：管理后端配置接口
- `internal/application/repository/semantic_outbox.go`：后端检查点与持久切换
- `docs/superpowers/plans/semantica/backend-runbook.md`：操作与回滚手册

**接口：** BackendService.Preview(ctx,scope) (PilotPreview,error)、SetDesired(ctx,scope,backend)error、Promote(ctx,scope,expectedActiveGeneration string)error、Rollback(ctx,scope)error；PilotPreview 含 backfill_document_count 与 budget_setting。持久状态desired_backend/active_backend/active_generation/native_checkpoint/semantic_checkpoint；内部接口在本任务定义，不直接暴露数据库指针。

- [ ] **1. 编写失败测试**：在所列测试文件加入以下核心断言；夹具按总计划与当前任务定义建立。

```
func TestSemanticRollbackRequiresNativeCatchup(t *testing.T) {
    f := newSemanticBackendFixture(t)
    f.SetNativeCheckpoint(8)
    f.SetLatestDocumentRevision(9)
    if err := f.Rollback(); !errors.Is(err, ErrNativeCatchupRequired) {
        t.Fatalf("rollback accepted stale native index: %v", err)
    }
}
```

- [ ] **2. 确认 RED**。执行 `go test ./internal/application/service -run TestSemanticBackend -count=1`。预期目标断言失败；修复测试环境问题后再次确认，不把依赖缺失算业务 RED。

- [ ] **3. 默认保持 native；KB 管理员的 preview 展示存量规模和预算设置后，SetDesired 才触发自动 backfill/影子构建，active 仍保持 native。影子结果不能混入正式查询，后台更新使用同一outbox输入同步checkpoint。**

- [ ] **4. Promote 检查完整manifest、选定拓扑能力证据、验收policy与当前源版本；CAS切换active backend/generation，旧请求沿固定read lease完成且仍校验最新deny。未验收 KB 不得 promotion，其他 KB 保持 native。**

- [ ] **5. Rollback先补齐native在切换期间的文档更新/删除，检查源revision清单一致后切换；native不可用则保持语义后端或明确禁用图能力，不能静默回旧图**

- [ ] **6. 产品设置保存desired状态和失败原因；正式切换与回滚只由管理API显式动作触发，部署镜像不自动切换KB**

关键实现约束：

```
if nativeManifestDigest != currentSourceManifestDigest {
    return ErrNativeCatchupRequired
}
if err := applyCurrentDeletionBarriers(ctx, scope); err != nil { return err }
return repo.CompareAndSwapBackend(ctx, scope, expectedBackend, "native")
// applyCurrentDeletionBarriers在semantic_backend.go编排I04/业务deny；不删除墓碑。
```

- [ ] **7. 确认 GREEN 与验收**。重跑 `go test ./internal/application/service -run TestSemanticBackend -count=1`，预期退出码 0；另完成：验证影子不污染答案、并发更新导致promotion拒绝、native追赶后回滚成功、原有删除不复活；运行手册列准确API/前置检查/失败处理。

- [ ] **8. 留证与提交**。更新 `docs/superpowers/plans/semantica/progress.md` 的 W03 行，附准确命令、退出码、环境和产物位置；只暂存上述任务文件中的本任务变更，提交 `feat(semantic): w03 后端影子构建、切换与回滚`。
