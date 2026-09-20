# Craft 05 计量、生命周期与发布 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在可核对用量、有限预算、资源保留和故障回退条件下运营Craft。

**Architecture:** Craft记录实际模型与资源消耗并接入商业专项；运行和资源生命周期保持租户归属。收费与生产开放均受专项接口及真实故障门禁约束。

**Tech Stack:** Go/Gin/GORM、现有 Sandbox/对象存储、tRPC-Agent-Go/OpenCode、React/assistant-ui、TypeScript/pnpm；具体依赖沿专项锁定版本。

**Spec:** [产品方案](../specs/2026-09-10-onyx-craft-product-proposal.md)；必读 [总计划及共享契约](2026-09-10-craft-product-implementation.md)。

## Global Constraints

- “首版同一工作区串行修改。”
- “主 Run、ToolCall 和恢复调度复用现有 tRPC 恢复方案。”
- “关闭或刷新页面只影响订阅，不自动重新提交任务。”
- “历史版本引用保持原始内容，不能静默重定向到最新文件。”
- “基础权限与取消安全是每个可运行版本的前提”。
- “本方案不设定套餐、价格或充值规则，商业权威保持在现有专项方案。”
- 本文所有步骤为待执行计划。继承总计划全部门禁、错误类型、身份/版本约束，不把测试替身结果当成上线证据。

---

## 文件结构与执行约定

文件所有权在各任务 Files 中列出；接口定义只在对应责任模块维护。按任务依赖执行，修改共享入口前核对其他任务变更。Go 示例的测试文件使用被测包及 `testing`，生产代码按代码块使用的标准库导入；外部 craft 类型从 `github.com/Tencent/WeKnora/internal/craft` 导入。TypeScript 测试使用 `node:test` 和 `node:assert/strict`；UI 浏览器测试另用 Playwright。最小代码展示核心规则，后续集成动作和验收表同属必做内容，不能只实现纯函数就勾选整个任务。

### Task O01: 主子模型物理调用用量去重

**依赖与用户结果：** 依赖R05、商业归属语义；能说明每次实际调用来自哪个空间/Run。

**Files:**
- Create: `internal/craft/usage.go`
- Test: `internal/craft/usage_test.go`
- Create: `internal/application/service/craft_usage.go`
- Test: `internal/application/service/craft_usage_test.go`
- Create: `internal/application/repository/craft_usage.go`
- Create: `migrations/versioned/000097_craft_usage.up.sql`
- Create: `migrations/versioned/000097_craft_usage.down.sql`
- Create: `migrations/sqlite/000018_craft_usage.up.sql`
- Create: `migrations/sqlite/000018_craft_usage.down.sql`
- Test: `internal/application/repository/craft_usage_test.go`

**Interfaces:**
- Consumes：服务端模型调用入口实际attempt和OC message usage观测；不信任模型参数里的tenant或费用。
- Produces：`UsageFact { ID,RunID,DelegationID,CallID,AttemptID,Runtime,ModelID,Funding,Status string; TenantID uint64; Input,Output,Cached int64 }`；`UsageKey(tenant uint64,callID,attemptID string) string`；`UsageSink.Append(ctx context.Context,f UsageFact) error`，同ID同事实幂等，不同事实走显式revision/correction，不能覆盖账目。

- [ ] **Step 1：先写失败测试。**

```go
func TestUsageKeyCountsPhysicalAttempts(t *testing.T) {
    if UsageKey(1,"call","a1")==UsageKey(1,"call","a2"){t.Fatal("retry lost")}
    if UsageKey(1,"call","a1")==UsageKey(2,"call","a1"){t.Fatal("tenant collision")}
    if UsageKey(1,"call","a1")!=UsageKey(1,"call","a1"){t.Fatal("redelivery counted twice")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestUsageKey -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func UsageKey(tenant uint64,callID,attemptID string) string {
    raw,_:=json.Marshal([]string{strconv.FormatUint(tenant,10),callID,attemptID})
    hash:=sha256.Sum256(raw);return hex.EncodeToString(hash[:])
}
```

- [ ] **Step 4：** 原始调用身份由模型网关在转发前生成，绑定tenant/run/delegation/模型/资金来源；主Runtime和OC物理attempt分别记录。OC事件只用于核对，不再作为第二个计费来源；工具结果转交主Agent不重复加OC tokens，但主Agent随后真正调用模型另记一笔。

- [ ] **Step 5：** 按Files新增usage事实表/outbox，unique tenant/call/attempt/revision；保存原始UsageFact、observed_at、revision、delivery状态/重试次数，down先删outbox后删事实。迁移编号碰撞按总计划调整，不修改已发布文件。记录reported/unknown/corrected，流断未获取usage保持unknown，不填0；缓存token与input/output口径按固定provider契约映射。

- [ ] **Step 6：** UsageSink与实际调用日志在持久化边界关联；投递官方OpenMeter的具体CloudEvent字段/meter由G4专项映射，使用稳定ID重投。平台模型可计模型Credits，BYOK模型只记录观察数据不收平台模型Credits，sandbox/storage仍可单独计量。

- [ ] **Step 7：** 服务测试2主调用+3子调用应有5个物理事实；重复OC汇总事件仍5；一次真实模型重试变6；跨tenant相同callID不混合。晚到usage补报而非新收费，断流unknown与取消后迟到事实有对账记录。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestUsageKey -count=1
```

预期 PASS。还必须逐项确认：

- 运行usage service/repository测试；最终总量可追到物理attempt原始观测。
- 未知消耗单列，不能凭最终一次模型响应估算整轮并宣称精确。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/usage.go internal/craft/usage_test.go internal/application/service/craft_usage.go internal/application/service/craft_usage_test.go internal/application/repository/craft_usage.go migrations/versioned/000097_craft_usage.up.sql migrations/versioned/000097_craft_usage.down.sql migrations/sqlite/000018_craft_usage.up.sql migrations/sqlite/000018_craft_usage.down.sql internal/application/repository/craft_usage_test.go
git diff --cached --check
git commit -m "feat(craft): o01 主子模型物理调用用量去重"
```

### Task O02: 受控模型入口与预算准入

**依赖与用户结果：** 依赖O01、G4正式接口；新收费操作先准入，已受理任务在有限预算内收尾。

**Files:**
- Create: `internal/craft/budget.go`
- Test: `internal/craft/budget_test.go`
- Create: `internal/application/service/craft_budget.go`
- Test: `internal/application/service/craft_budget_test.go`
- Create: `internal/handler/craft_model_gateway.go`
- Test: `internal/handler/craft_model_gateway_test.go`

**Interfaces:**
- Consumes：商业专项确认的准入/预占/增补/结算；以下是Craft内部端口，不声称是OpenMeter API。
- Produces：`BudgetGrant { ID string; Deadline time.Time; MaxCalls,UsedCalls int; Allowed bool }`；`BudgetAllows(g BudgetGrant,now time.Time) bool`；`BudgetPort.Admit(ctx context.Context,scope Scope,runID string)(BudgetGrant,error)`、`AuthorizeCall(ctx context.Context,grantID,callID string) error`、`Reconcile(ctx context.Context,grantID string) error`。资金金额与Credits换算由商业实现掌握，不在Craft模型中增加余额。

- [ ] **Step 1：先写失败测试。**

```go
func TestBudgetCannotRunAfterDeadline(t *testing.T) {
    now:=time.Unix(100,0)
    g:=BudgetGrant{ID:"g",Deadline:now,MaxCalls:3,UsedCalls:0,Allowed:true}
    if BudgetAllows(g,now){t.Fatal("expired")}
    g.Deadline=now.Add(time.Minute);g.UsedCalls=3
    if BudgetAllows(g,now){t.Fatal("call cap")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestBudgetCannot -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type BudgetGrant struct { ID string; Deadline time.Time; MaxCalls,UsedCalls int; Allowed bool }
func BudgetAllows(g BudgetGrant,now time.Time) bool {
    return g.Allowed && g.ID!="" && now.Before(g.Deadline) && g.UsedCalls<g.MaxCalls
}
```

- [ ] **Step 4：** 先核对G4已批准接口与官方部署版本，写映射表和契约测试；缺少严格预算能力时该任务保持blocked_external、收费flag保持false。可以用测试BudgetPort完成Craft边界单测，但不能将fake AllowAll用于真实收费环境。支付/订阅/充值设计不在本任务决策。

- [ ] **Step 5：** 网关执行凭据绑定tenant/run/delegation/grant、允许模型和deadline，短期可撤销；恢复重新签发，不在checkpoint保存密钥。OC模型baseURL指向受控gateway，gateway拒绝客户端自定上游地址/model越界；拒绝通过环境/工作区读取平台长效key。

- [ ] **Step 6：** AuthorizeCall在真实转发前原子预留调用和商业预算，同callID重试不重复准入；不同physical attempt有新预算消耗。余额不足阻止新调用，取消后禁止追加，已准入请求在允许截止内收尾；结果不明维持待核对占用，不提前全额释放。

- [ ] **Step 7：** 并发测试两个最后额度请求仅一通过；过期/取消/跨tenant token拒绝；新增调用导致商业增补不足时主Agent清楚显示预算停止。读取/下载/清理仍沿商业超限语义开放，不能一刀切禁用空间。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestBudgetCannot -count=1
```

预期 PASS。还必须逐项确认：

- BudgetAllows只是快速检查，严格余额/并发控制必须由商业接口事务或等效保证实现。
- G4契约集成与实际账务对账未过前，收费上线禁止。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/budget.go internal/craft/budget_test.go internal/application/service/craft_budget.go internal/application/service/craft_budget_test.go internal/handler/craft_model_gateway.go internal/handler/craft_model_gateway_test.go
git diff --cached --check
git commit -m "feat(craft): o02 受控模型入口与预算准入"
```

### Task O03: Sandbox 驻留、快照和资源回收

**依赖与用户结果：** 依赖C04/C05与O02；作品可再打开，活跃资源不会被清理器删除。

**Files:**
- Create: `internal/craft/lifecycle.go`
- Test: `internal/craft/lifecycle_test.go`
- Create: `internal/application/service/craft_lifecycle.go`
- Test: `internal/application/service/craft_lifecycle_test.go`
- Modify: `internal/container/cleanup.go`
- Create: `docs/operations/craft-lifecycle.md`

**Interfaces:**
- Consumes：Sandbox绑定与生命周期锁、Run/Task状态、C05快照、VersionStore引用。
- Produces：`ResourceState { Active,Unknown,DecisionPending,Referenced bool; EligibleAt time.Time }`；`CanDeleteResource(ResourceState,time.Time) bool`；`CraftLifecycle.Sweep(ctx context.Context,limit int) error`，批次默认100。资源清理按实际运行关系查询，不信任调用方布尔作为唯一事实。

- [ ] **Step 1：先写失败测试。**

```go
func TestLifecycleKeepsUnknownAndReferencedResources(t *testing.T) {
    now:=time.Unix(100,0)
    for _,s:=range []ResourceState{{Unknown:true},{Active:true},{DecisionPending:true},{Referenced:true}} {
        if CanDeleteResource(s,now){t.Fatal("live resource removed")}
    }
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestLifecycleKeeps -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type ResourceState struct { Active,Unknown,DecisionPending,Referenced bool; EligibleAt time.Time }
func CanDeleteResource(s ResourceState,now time.Time) bool {
    return !s.Active && !s.Unknown && !s.DecisionPending && !s.Referenced && !now.Before(s.EligibleAt)
}
```

- [ ] **Step 4：** 把“沙箱休眠”和“删除产物/快照”分开策略：空闲30min且已验证完整快照可休眠，进程状态无法恢复需明确重启；保留历史版本沿现有session政策，不能因sandbox TTL删对象。orphan对象先记录候选24h再查引用删除；这些是可配置初始建议值。

- [ ] **Step 5：** Sweep先持生命周期锁再查活跃Run/unknown/decision和版本引用，CAS标deleting，阻止新dispatch/恢复，然后调用provider删除；失败保留重试记录。清理后清除绑定必须generation匹配，避免删除新实例。

- [ ] **Step 6：** 恢复/保存快照和回收竞争用两个worker集成测试；孤儿上传先记录candidate而未到期不能删；会话删除必须先tombstone阻止接管，再处理Run取消与资源引用，未知远端任务保留风险记录。

- [ ] **Step 7：** 统计sandbox实际start/stop/驻留时间与存储bytes-day，事件重送去重；配额超限只限制新增资源，既有授权下载/清理仍可用。provider TTL无法延长时提前显示workspace风险，不能假设容器永久存活。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestLifecycleKeeps -count=1
```

预期 PASS。还必须逐项确认：

- 运行lifecycle测试包含worker竞争/删除失败重试/新generation保护。
- 文档给出休眠、重建、删除各自数据保证；不承诺进程内存快照。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/lifecycle.go internal/craft/lifecycle_test.go internal/application/service/craft_lifecycle.go internal/application/service/craft_lifecycle_test.go internal/container/cleanup.go docs/operations/craft-lifecycle.md
git diff --cached --check
git commit -m "feat(craft): o03 Sandbox 驻留、快照和资源回收"
```

### Task O04: 用量展示与运行诊断

**依赖与用户结果：** 依赖O01/O03；空间管理员能核对消耗，用户能理解失败与恢复状态。

**Files:**
- Create: `packages/domain/src/craft/usage.ts`
- Test: `packages/domain/src/craft/usage.test.ts`
- Create: `packages/views/src/craft/usage.tsx`
- Create: `internal/handler/session/craft_usage.go`
- Test: `internal/handler/session/craft_usage_test.go`
- Create: `docs/operations/craft-observability.md`

**Interfaces:**
- Consumes：O01 UsageFact与商业专项展示结果、已有角色/空间scope。
- Produces：`UsageView {knownCalls:number;unknownCalls:number;inputTokens:number;outputTokens:number;funding:'platform'|'byok'|'mixed'}`；`usageLabel(v:UsageView):string`；GET `/api/v1/sessions/:session_id/craft/usage`返回上述字段和明确 `as_of`，金额仅由商业视图提供。

- [ ] **Step 1：先写失败测试。**

```typescript
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {usageLabel} from './usage.ts';
test('unknown consumption stays visible',()=>{
  assert.equal(usageLabel({knownCalls:3,unknownCalls:1,inputTokens:10,outputTokens:20,funding:'byok'}),'已记录 3 次调用，1 次用量待核对');
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test packages/domain/src/craft/usage.test.ts
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```typescript
export interface UsageView {knownCalls:number;unknownCalls:number;inputTokens:number;outputTokens:number;funding:'platform'|'byok'|'mixed'}
export function usageLabel(v:UsageView):string {
  return `已记录 ${v.knownCalls} 次调用，${v.unknownCalls} 次用量待核对`;
}
```

- [ ] **Step 4：** 作品执行详情显示主/子调用、已知/未知用量、沙箱驻留、Check、失败原因；BYOK清楚标明模型由空间自行承担，不能将所有消耗显示免费。空间管理员可看聚合成本，普通成员按现有权限仅看自己可访问作品。

- [ ] **Step 5：** 新增低基数指标：craft_delegations_total{status}、craft_reconcile_total{outcome}、craft_pending_decisions、craft_preview_failures_total、craft_usage_unknown_total、craft_workspace_restore_seconds。run/tenant ID放受控日志和trace属性，不作为无限基数metrics label。

- [ ] **Step 6：** 日志串起tenant/session/run/tool_call/delegation/promptID，但token、材料正文和完整模型日志不进默认日志。排障手册从用户session定位主Run、子关联、原始用量、检查事实，给出“正在停止”“不明结果”“预览失败”三个路径。

- [ ] **Step 7：** handler测试跨空间、viewer、管理员，输出不含供应商密钥；前端测试unknown非0文案，晚到修正可刷新as_of。真实报告统计P50/P95首预览时间及主/子tokens，只报告样本量和实测，不提前写成本承诺。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
pnpm exec tsx --test packages/domain/src/craft/usage.test.ts
```

预期 PASS。还必须逐项确认：

- 运行handler访问矩阵与domain测试；用一个真实Run从界面查到可核对事实。
- 凭据和知识正文不出现在诊断导出中。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add packages/domain/src/craft/usage.ts packages/domain/src/craft/usage.test.ts packages/views/src/craft/usage.tsx internal/handler/session/craft_usage.go internal/handler/session/craft_usage_test.go docs/operations/craft-observability.md
git diff --cached --check
git commit -m "feat(craft): o04 用量展示与运行诊断"
```

### Task O05: 发布能力门禁、故障演练与回退

**依赖与用户结果：** 依赖W06/C全阶段；各类型按D对应完成情况，收费另依赖O02。

**Files:**
- Create: `internal/craft/release.go`
- Test: `internal/craft/release_test.go`
- Create: `scripts/check-craft-release.py`
- Create: `docs/testing/craft/release-evidence.json`
- Create: `docs/operations/craft-release.md`

**Interfaces:**
- Consumes：27任务实际证据、G1–G4外部专项SHA与provider/DB矩阵。
- Produces：`ReleaseFacts { Web,Permissions,Cancel,Reconnect,Recovery,Versioning,Isolation,Quota,Usage,Billing bool }`；`CanRelease(f ReleaseFacts,paid bool) bool`；evidence JSON字段 `head,dependencies,commands,scenarios,capabilities,limitations`，每scenario记录passed/failed/not_run与证据路径。

- [ ] **Step 1：先写失败测试。**

```go
func TestPaidReleaseRequiresCommercialEvidence(t *testing.T) {
    f:=ReleaseFacts{Web:true,Permissions:true,Cancel:true,Reconnect:true,Recovery:true,Versioning:true,Isolation:true,Quota:true,Usage:true}
    if CanRelease(f,true){t.Fatal("paid without billing")}
    if !CanRelease(f,false){t.Fatal("controlled release rejected")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestPaidRelease -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type ReleaseFacts struct { Web,Permissions,Cancel,Reconnect,Recovery,Versioning,Isolation,Quota,Usage,Billing bool }
func CanRelease(f ReleaseFacts,paid bool) bool {
    return f.Web && f.Permissions && f.Cancel && f.Reconnect && f.Recovery && f.Versioning && f.Isolation && f.Quota && f.Usage && (!paid || f.Billing)
}
```

- [ ] **Step 4：** check-craft-release.py读取evidence JSON，要求head等于待发布commit、依赖SHA齐全、必需场景status=passed且证据文件存在；不能只有手填布尔。默认能力只发布经过测试的PostgreSQL+Docker组合；SQLite/Cube/E2B各自单独capability条目，没有证据禁用或保守等待。

- [ ] **Step 5：** 演练主服务SIGKILL、OC崩溃、sandbox删除、存储失败、两worker抢占、浏览器断网、decision竞争、usage迟到、预算不足、预览越权；记录主Run最终状态、prompt次数、旧版SHA、用户看到的下一步。至少验证一次从持久快照恢复，不能用正常退出替代崩溃。

- [ ] **Step 6：** 发布开关顺序：默认全关→内部tenant网页→限定tenant协作恢复→逐kind开启→商业门禁通过后收费开放。回退先停止新准入、保留运行worker与旧镜像完成/等待现有Run；不能直接删表、删除旧OC镜像或把active会话换builtin。版本不兼容暂停并给出恢复选项。

- [ ] **Step 7：** 最终回归旧builtin聊天、知识问答、MCP审批、旧产物下载与终端权限；运行现有后端相关包、共享TS typecheck、web build和Craft E2E。命令及结果逐条写evidence，不要求用无关全仓故障掩盖本功能结果；失败边界如实列出。

- [ ] **Step 8：** 发布手册包含配置、预览域名/TLS、持久卷、备份与恢复、模型凭据轮换、保留策略、数据库升级顺序、停止新准入与回退命令。部署实施是后续明确动作，本任务先产出可审阅发布包，不因计划执行自动向生产发布。

- [ ] **Step 9：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestPaidRelease -count=1
```

预期 PASS。还必须逐项确认：

- `python3 scripts/check-craft-release.py docs/testing/craft/release-evidence.json` 必须验证真实证据，未运行项不通过。
- 功能完成、已验收、已发布分别记录；不可把文档检查称为生产通过。

- [ ] **Step 10：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/release.go internal/craft/release_test.go scripts/check-craft-release.py docs/testing/craft/release-evidence.json docs/operations/craft-release.md
git diff --cached --check
git commit -m "feat(craft): o05 发布能力门禁、故障演练与回退"
```
