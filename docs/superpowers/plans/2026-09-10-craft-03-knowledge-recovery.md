# Craft 03 知识与可恢复协作 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 主 Agent 使用有权访问的企业资料，用户可处理交互、重连与恢复作品。

**Architecture:** 知识由主Runtime检索后形成有来源的材料；子执行核对结果进入既有恢复系统。交互使用持久Decision，恢复同时验证文件与OpenCode会话。

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

### Task C01: 知识材料包与引用溯源

**依赖与用户结果：** 依赖R03/R05/W06；用户选择知识后看到作品依据及原始来源。

**Files:**
- Create: `internal/craft/knowledge.go`
- Test: `internal/craft/knowledge_test.go`
- Create: `internal/application/service/craft_knowledge.go`
- Test: `internal/application/service/craft_knowledge_test.go`
- Create: `packages/views/src/craft/sources.tsx`

**Interfaces:**
- Consumes：已有知识检索权限/引用解析、R03 Stage。
- Produces：`Source { ID,Ref,Excerpt,Digest string; TenantID uint64 }`；`KnowledgeBundle { Sources []Source; Truncated bool }`；`BoundSources(sources []Source,maxBytes int) KnowledgeBundle`；`CraftKnowledgeService.Build(ctx context.Context,scope craft.Scope,query string,knowledgeIDs []string)(craft.KnowledgeBundle,error)`。最大内容64KiB、最多20条，不含原始凭据。

- [ ] **Step 1：先写失败测试。**

```go
func TestSourcesKeepRefsWhenBounded(t *testing.T) {
    b:=BoundSources([]Source{{ID:"c1",Ref:"resource://a",Excerpt:"123"},{ID:"c2",Ref:"resource://b",Excerpt:"456"}},3)
    if len(b.Sources)!=1 || b.Sources[0].Ref!="resource://a" || !b.Truncated {t.Fatalf("%+v",b)}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestSourcesKeep -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type Source struct { ID,Ref,Excerpt,Digest string; TenantID uint64 }
type KnowledgeBundle struct { Sources []Source; Truncated bool }
func BoundSources(sources []Source,maxBytes int) KnowledgeBundle {
    b:=KnowledgeBundle{Sources:[]Source{}}
    used:=0
    for _,s:=range sources {
        if len(b.Sources)>=20 || used+len(s.Excerpt)>maxBytes {b.Truncated=true;continue}
        b.Sources=append(b.Sources,s);used+=len(s.Excerpt)
    }
    return b
}
```

- [ ] **Step 4：** Build先通过现有用户/空间/共享知识ACL逐库检索；不能简单要求Source.TenantID等于当前空间，因为合法共享库可能在其他空间。检索结果按现有引用服务解析可访问resource，把受控摘录/digest/citation ID写材料清单。权限检查失败整个请求返回明确错误，不偷偷扩展到全部库。

- [ ] **Step 5：** 主Agent保留引用与检索目的，子Agent仅收到材料文件/清单，不获得数据库、向量库和全量知识凭据。将资料文本明确标为数据，不让其指令覆盖系统操作权限。成果引用使用稳定citation ID，与版本manifest关联；主交付区分资料事实、推导结论、Check。

- [ ] **Step 6：** 服务测试同用户授权库/未授权库/共享库/已撤销共享/材料过大/检索空结果；调用真实ACL入口，断言未授权资源不进入writer。sources视图点击引用沿现有资源权限，不把过期URL长期保存。

- [ ] **Step 7：** 浏览器选择两个来源生成报告，核对标题引用与source ID一致；撤销一个来源后再次修改或下载受限来源触发403，不能恢复旧权限快照继续检索。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestSourcesKeep -count=1
```

预期 PASS。还必须逐项确认：

- 检索和材料打包均保留来源，不将子模型臆造的引用标为真实。
- `go test ./internal/application/service -run TestCraftKnowledge -count=1`，实际知识场景验收独立记录。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/knowledge.go internal/craft/knowledge_test.go internal/application/service/craft_knowledge.go internal/application/service/craft_knowledge_test.go packages/views/src/craft/sources.tsx
git diff --cached --check
git commit -m "feat(craft): c01 知识材料包与引用溯源"
```

### Task C02: 完整问题、权限与可靠决定投递

**依赖与用户结果：** 依赖R06、G3；刷新或重启后用户只需处理一次决定。

**Files:**
- Create: `internal/craft/decision.go`
- Test: `internal/craft/decision_test.go`
- Create: `internal/application/service/craft_decision_delivery.go`
- Test: `internal/application/service/craft_decision_delivery_test.go`
- Create: `packages/views/src/craft/interaction.tsx`
- Create: `apps/web/e2e/craft-interaction.spec.ts`

**Interfaces:**
- Consumes：R06 Interaction/DecisionAllowed，tRPC Decision的pending_id/decision_id/expected_revision和持久outbox。
- Produces：`Answer { QuestionID string; Choices []string; Text string }`；`PendingDecision { Interaction; Options map[string][]string; Multiple map[string]bool; Answers []Answer }`；`DecisionMatch(expected,current int64,expectedHash,currentHash string) bool`；`CraftDecisionDelivery.Deliver(ctx context.Context,key runtime.RunKey,pendingID string) error`。远端问题answer与permission once/reject分别映射锁定协议。

- [ ] **Step 1：先写失败测试。**

```go
func TestDecisionRevisionAndScopeHash(t *testing.T) {
    if DecisionMatch(1,2,"a","a") || DecisionMatch(1,1,"a","b") {t.Fatal("stale decision accepted")}
    if !DecisionMatch(2,2,"a","a") {t.Fatal("same revision rejected")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestDecisionRevision -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func DecisionMatch(expected,current int64,expectedHash,currentHash string) bool {
    return expected==current && expectedHash!="" && expectedHash==currentHash
}
```

- [ ] **Step 4：** 扩展question多题/单选/多选/自定义答案，服务端验证question ID、合法choice、单选最多1、文本最大8KiB；permission UI逐项显示命令/路径/作用域，首期只提供本次同意和拒绝，不默认开放session永久批准。

- [ ] **Step 5：** 持久决定与待发送记录在同事务提交；Delivery按Run fence取待发送项，重新检查权限/参数摘要，再向绑定OC request ID投递。成功后标ack；网络超时先查询pending状态并核对结果，不能仅因请求消失就判已接受。协议不能确认时waiting_user/delivery_unknown，提示已记录决定但送达未确认。

- [ ] **Step 6：** 相同decisionID+payload返回原结果，异payload或竞争revision409；已取消/终态请求410；问题回答不能作为权限批准。waiting不持有阻塞goroutine或worker lease，主恢复worker按已有机制重新调度。

- [ ] **Step 7：** 集成故障测试在decision保存后kill服务再启动，确认同一pending展示原答案；在OC已接受但ack前kill，不能产生重复新决定。浏览器两标签同时批准只有一个有效，另一个重载已处理状态。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestDecisionRevision -count=1
```

预期 PASS。还必须逐项确认：

- 运行service决定投递测试和craft-interaction浏览器；报告未知投递的实际UI。
- 模型和技能不能自发创建用户批准记录；日志保留操作人/范围而不含密钥。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/decision.go internal/craft/decision_test.go internal/application/service/craft_decision_delivery.go internal/application/service/craft_decision_delivery_test.go packages/views/src/craft/interaction.tsx apps/web/e2e/craft-interaction.spec.ts
git diff --cached --check
git commit -m "feat(craft): c02 完整问题、权限与可靠决定投递"
```

### Task C03: 快照重连与事件缺口修复

**依赖与用户结果：** 依赖W04和G3 RunEvent回放；刷新、休眠和切空间均能回到服务端事实。

**Files:**
- Create: `packages/domain/src/craft/reconnect.ts`
- Test: `packages/domain/src/craft/reconnect.test.ts`
- Modify: `packages/core/src/craft/controller.ts`
- Test: `packages/core/src/craft/reconnect.test.ts`
- Create: `apps/web/e2e/craft-reconnect.spec.ts`

**Interfaces:**
- Consumes：W04 `CraftState/CraftEvent`、既有Last-Event-ID事件协议及snapshot last_seq。
- Produces：`replayAction(last:number,incoming:number):'drop'|'apply'|'reload'`；controller `reconnect():Promise<void>`；`load`在刷新时复用同session和run，不创建run。

- [ ] **Step 1：先写失败测试。**

```typescript
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {replayAction} from './reconnect.ts';
test('event gaps require authoritative reload',()=>{
  assert.equal(replayAction(8,8),'drop');
  assert.equal(replayAction(8,10),'reload');
  assert.equal(replayAction(8,9),'apply');
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test packages/domain/src/craft/reconnect.test.ts
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```typescript
export function replayAction(last:number,incoming:number):'drop'|'apply'|'reload' {
  if(incoming<=last)return 'drop';
  return incoming===last+1?'apply':'reload';
}
```

- [ ] **Step 4：** controller使用完整主Run事件序列计数，包括不是Craft kind的事件；不能只数子事件导致假缺口。先获得快照序号N，再订阅after N；订阅缓冲与快照替换按generation原子应用，过期游标410和真实缺口重新读快照。

- [ ] **Step 5：** 重连退避1/2/4/8/15秒且jitter，上限15秒；dispose切空间立即终止定时器与请求。fetch断开只停止订阅，不调用cancel，不重新发送用户prompt。主模型attempt切换按tRPC投影替换旧未完成文本，不把新attempt附在旧文本末尾。

- [ ] **Step 6：** controller测试用fakeSSE按seq 1/2/2/4触发一次reload，snapshot seq4后接5不重复；scope从1变2后旧fetch resolve不能写入。浏览器在生成、waiting、finished三时点刷新，POST次数不变、消息不重复、preview仍是选定版本。

- [ ] **Step 7：运行 GREEN 与验收。**

```bash
pnpm exec tsx --test packages/domain/src/craft/reconnect.test.ts
```

预期 PASS。还必须逐项确认：

- 运行共享controller测试与浏览器offline/online场景。
- 重连期间UI显示同步中；不能把没有事件显示成执行完成。

- [ ] **Step 8：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add packages/domain/src/craft/reconnect.ts packages/domain/src/craft/reconnect.test.ts packages/core/src/craft/controller.ts packages/core/src/craft/reconnect.test.ts apps/web/e2e/craft-reconnect.spec.ts
git diff --cached --check
git commit -m "feat(craft): c03 快照重连与事件缺口修复"
```

### Task C04: 进程故障后核对子执行

**依赖与用户结果：** 依赖R04、G3；接管不会重复修改文件，不能确认时明确等待。

**Files:**
- Create: `internal/craft/recovery.go`
- Test: `internal/craft/recovery_test.go`
- Create: `internal/application/service/craft_recovery.go`
- Test: `internal/application/service/craft_recovery_test.go`
- Create: `internal/application/service/craft_recovery_process_test.go`

**Interfaces:**
- Consumes：Store任务/结果、Executor.Observe、tRPC恢复worker与真实Fence。
- Produces：`RecoveryFacts { ResultStored,ExactCompleted,RemoteRunning,VersionCompatible,WorkspaceAvailable bool }`；`RecoveryRoute(RecoveryFacts) string`；`CraftRecovery.Reconcile(ctx context.Context,fence runtime.Fence,taskID string)(craft.Result,error)`。route取reuse/collect/observe/wait，不提供自动重发未知写入分支。

- [ ] **Step 1：先写失败测试。**

```go
func TestUnknownExecutionNeverResubmits(t *testing.T) {
    if RecoveryRoute(RecoveryFacts{VersionCompatible:true,WorkspaceAvailable:true})!="wait" {t.Fatal("unsafe retry")}
    if RecoveryRoute(RecoveryFacts{ResultStored:true})!="reuse" {t.Fatal("lost persisted result")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestUnknownExecution -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type RecoveryFacts struct { ResultStored,ExactCompleted,RemoteRunning,VersionCompatible,WorkspaceAvailable bool }
func RecoveryRoute(f RecoveryFacts) string {
    if f.ResultStored{return "reuse"}
    if !f.VersionCompatible || !f.WorkspaceAvailable{return "wait"}
    if f.ExactCompleted{return "collect"}
    if f.RemoteRunning{return "observe"}
    return "wait"
}
```

- [ ] **Step 4：** 恢复入口先用当前认证/配置解析器重建权限，再查已保存ToolCall结果；有结果仅apply_result。无结果校验sandbox generation、OC session、runtime digest、promptID，ExactCompleted只由R04完整快照谓词给出。复用记录也必须重新授权其资源访问。

- [ ] **Step 5：** Observe仍running继续观察并受绝对deadline约束；session不存在、镜像不兼容、文件缺失、不明POST均转已有waiting reason。禁止以空session续跑，禁止用户普通聊天消息解除不明等待；人工retry通过已有Decision新attempt并保留副作用风险说明。

- [ ] **Step 6：** process_test借用tRPC真实数据库/子进程harness，新增barrier：提交前记录后、OC接受后响应前、结果保存后checkpoint前、lease丢失后。父进程杀Go进程，外部OC保持存活；记录promptPOST计数，已受理任务恢复后计数仍1。

- [ ] **Step 7：** 测试接管者新epoch写回成功、旧epoch的late event/result拒绝；租约失效不能仅因抢占成功而重发。实际权限撤销、OC digest变更与等待重启各一例。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestUnknownExecution -count=1
```

预期 PASS。还必须逐项确认：

- 运行 `go test ./internal/application/service -run TestCraftRecovery -count=1` 以及显式开启的真实进程测试。
- SIGKILL与持久数据库证据必须齐备；mock recovery route不等于恢复验收。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/recovery.go internal/craft/recovery_test.go internal/application/service/craft_recovery.go internal/application/service/craft_recovery_test.go internal/application/service/craft_recovery_process_test.go
git diff --cached --check
git commit -m "feat(craft): c04 进程故障后核对子执行"
```

### Task C05: 文件和 OpenCode 会话快照恢复

**依赖与用户结果：** 依赖C04/W01；历史下载保持不变，可安全从旧版继续编辑。

**Files:**
- Create: `internal/craft/snapshot.go`
- Test: `internal/craft/snapshot_test.go`
- Create: `internal/application/service/craft_snapshot.go`
- Create: `internal/application/repository/craft_snapshot.go`
- Create: `migrations/versioned/000096_craft_snapshots.up.sql`
- Create: `migrations/versioned/000096_craft_snapshots.down.sql`
- Create: `migrations/sqlite/000017_craft_snapshots.up.sql`
- Create: `migrations/sqlite/000017_craft_snapshots.down.sql`
- Test: `internal/application/service/craft_snapshot_test.go`
- Modify: `internal/handler/session/craft.go`
- Modify: `packages/views/src/craft/workbench.tsx`

**Interfaces:**
- Consumes：Workspace、VersionStore、Sandbox生命周期锁。
- Produces：`Snapshot { WorkspaceID,VersionID,FilesDigest,SessionDigest,RuntimeDigest string; Quiescent bool }`；`CanRestore(s Snapshot,active bool,runtimeDigest string) bool`；`CraftSnapshotService.Capture(ctx context.Context,scope craft.Scope,versionID string)(craft.Snapshot,error)`、`Restore(ctx context.Context,scope craft.Scope,snapshotID string,revision int64)(craft.Workspace,error)`。snapshotID在存储实体中为不可变ID，VersionID关联manifest。snapshot表保存tenant/workspace/version、文件与OC数据对象ref及digest、runtime digest、quiescent、创建时间和manifest版本；所有对象引用与权限沿W01。

- [ ] **Step 1：先写失败测试。**

```go
func TestRestoreNeedsSessionAndQuiescence(t *testing.T) {
    s:=Snapshot{WorkspaceID:"w",VersionID:"v",FilesDigest:"f",RuntimeDigest:"d",Quiescent:true}
    if CanRestore(s,false,"d") {t.Fatal("files alone accepted")}
    s.SessionDigest="oc"
    if !CanRestore(s,false,"d") || CanRestore(s,true,"d") {t.Fatal("active restore")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestRestoreNeeds -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type Snapshot struct { WorkspaceID,VersionID,FilesDigest,SessionDigest,RuntimeDigest string; Quiescent bool }
func CanRestore(s Snapshot,active bool,runtimeDigest string) bool {
    return !active && s.Quiescent && s.WorkspaceID!="" && s.VersionID!="" &&
        s.FilesDigest!="" && s.SessionDigest!="" && s.RuntimeDigest==runtimeDigest
}
```

- [ ] **Step 4：** Capture仅在OC确定idle、无pending/未知任务并持有工作区生命周期锁时进行；导出工作文件与OC持久data（SQLite使用一致备份或停进程后复制，不复制半写WAL），清单记runtime/schema/技能摘要。过滤短期token/密钥，加密受控存储并逐对象校验checksum。

- [ ] **Step 5：** Restore拒绝active/waiting/unknown Run与revision变化；先在新generation恢复到临时sandbox，验证文件摘要、OC session存在及消息链。原绑定保留到新实例健康且CAS替换成功，失败清理新实例并保持旧绑定，不能半恢复。旧runtime不可用返回明确unsupported。

- [ ] **Step 6：** 恢复后新Run继续原版本对应的子session历史；绑定从历史快照生成新generation，审计记录parent version，新产物生成新版本ID，旧版本不变。基础仅下载旧版一直可用；没有完整snapshot的旧版隐藏/禁用继续编辑并给出原因。

- [ ] **Step 7：** 集成测试恢复v1后再编辑得到v3，v2仍可下载；删除沙箱但存储完整则恢复；文件完整/session摘要坏则拒绝；恢复途中进程死亡保留旧绑定；无隔离数据恢复能力的provider标unsupported。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestRestoreNeeds -count=1
```

预期 PASS。还必须逐项确认：

- 同时恢复文件与OC数据，不能将“新session+旧文件”标作完整恢复。
- handler `/restore` 幂等request_id、revision、owner授权和活动锁均有竞争测试。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/snapshot.go internal/craft/snapshot_test.go internal/application/service/craft_snapshot.go internal/application/repository/craft_snapshot.go migrations/versioned/000096_craft_snapshots.up.sql migrations/versioned/000096_craft_snapshots.down.sql migrations/sqlite/000017_craft_snapshots.up.sql migrations/sqlite/000017_craft_snapshots.down.sql internal/application/service/craft_snapshot_test.go internal/handler/session/craft.go packages/views/src/craft/workbench.tsx
git diff --cached --check
git commit -m "feat(craft): c05 文件和 OpenCode 会话快照恢复"
```

### Task C06: 版本化 Skills 与受控追加检索

**依赖与用户结果：** 依赖C01–C05；构建方法可复现，需要执行中补充资料时仍走原权限入口。

**Files:**
- Create: `internal/craft/skill.go`
- Test: `internal/craft/skill_test.go`
- Create: `internal/application/service/craft_knowledge_tool.go`
- Test: `internal/application/service/craft_knowledge_tool_test.go`
- Create: `skills/craft-web-report/SKILL.md`
- Create: `skills/craft-web-report/manifest.json`

**Interfaces:**
- Consumes：C01 Build及现有MCP/Skills授权层，不直接给OC知识库连接串。
- Produces：`SkillPin { Name,Digest string }`；`SameSkillPin(expected,actual SkillPin) bool`；`CraftKnowledgeTool.Query(ctx context.Context,scope craft.Scope,taskID,query string)(craft.KnowledgeBundle,error)`；默认flag `craft.execution_knowledge=false`。query范围来自已存Task的知识授权快照并与当前ACL求交。

- [ ] **Step 1：先写失败测试。**

```go
func TestSkillPinCannotSilentlyUpgrade(t *testing.T) {
    if SameSkillPin(SkillPin{"web","a"},SkillPin{"web","b"}) {t.Fatal("silent upgrade")}
    if !SameSkillPin(SkillPin{"web","a"},SkillPin{"web","a"}) {t.Fatal("same pin rejected")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestSkillPin -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type SkillPin struct { Name,Digest string }
func SameSkillPin(expected,actual SkillPin) bool {
    return expected.Name!="" && expected.Digest!="" && expected==actual
}
```

- [ ] **Step 4：** 网页Skill正文要求：读取inputs清单与引用；生成output/index.html和相对静态资源；数据解析/计算独立验证；不访问任意外网/执行部署；将实际命令与退出码写checks.json。manifest记录name/version/digest、产物类型、工具要求，不含授权决定。

- [ ] **Step 5：** 可选追加检索以已有MCP或内部受控工具封装；短期执行凭据绑定tenant/user/run/task和允许知识范围，最多8次、每次64KiB、到期不长于Run deadline；后端Query再次授权，跨run token拒绝，取消/终态立即吊销。工具只返回C01材料结构，不返回数据库凭据。

- [ ] **Step 6：** 恢复检验Skill digest，旧内容保留到相关Run/快照过期；缺失明确等待，不悄悄换新。测试技能文件声称需要管理员权限时仍被现有权限层拒绝；模型产生的新知识库ID不能扩大检索范围。

- [ ] **Step 7：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestSkillPin -count=1
```

预期 PASS。还必须逐项确认：

- 追加检索默认关闭，打开前通过权限收窄/过期/跨run/调用预算测试。
- 网页Skill生成符合W02静态预览约束，不依赖公网CDN或开发服务。

- [ ] **Step 8：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/skill.go internal/craft/skill_test.go internal/application/service/craft_knowledge_tool.go internal/application/service/craft_knowledge_tool_test.go skills/craft-web-report/SKILL.md skills/craft-web-report/manifest.json
git diff --cached --check
git commit -m "feat(craft): c06 版本化 Skills 与受控追加检索"
```
