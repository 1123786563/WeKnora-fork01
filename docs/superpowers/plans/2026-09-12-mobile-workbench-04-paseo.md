# 移动工作台 04：Paseo 与远程执行治理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过受控Paseo节点运行一个Coding Agent，并具备身份隔离、启动对账、事件恢复、取消与可信用量边界。

**Architecture:** Go持有产品Run和命令权威；薄TypeScript Bridge包装固定SDK；个人节点再增加出站连接器。执行位置、driver与engine_type分离，任何未知操作不通过任意RPC透传。

**Tech Stack:** 现有Go/GORM、Node/TypeScript（W17固定上游支持版本）、Paseo公开SDK、TLS/mTLS、节点命令日志。

**Spec:** [技术架构](../specs/2026-09-12-mobile-ai-saas-workbench-architecture.md) §3.2、§5、§7–11、§14；[总计划](2026-09-12-mobile-ai-saas-workbench.md)。执行者同时读取本册与规格。

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

`services/paseo-adapter`只做协议和节点连接；`internal/execution`定义控制契约与安全策略；运行绑定/命令仓储在现有repository中。W17–W22先打通一个托管节点，W23是独立自带节点增量，不能用其未完成阻塞平台移动首版。

### W17：固定上游与能力探针

**依赖：** W01；执行前选定受控测试节点和单个provider。

**Files：**

- Create: `services/paseo-adapter/package.json`、`services/paseo-adapter/tsconfig.json`、`services/paseo-adapter/src/capabilities.ts`、`services/paseo-adapter/src/capabilities.test.ts`
- Create: `docs/migrations/paseo/compatibility.json`、`docs/evidence/mobile-workbench/W17-paseo-probe.md`
- Modify: `pnpm-workspace.yaml`、`pnpm-lock.yaml`

**Interfaces：**

Produces `PaseoCapabilities{create:boolean;observe:boolean;events:boolean;cancel:boolean;approval:boolean;steer:boolean;exportArtifact:boolean;lookupByRequest:boolean}`；`requireCore(c:PaseoCapabilities):void`。compatibility.json记录commit、SDK版本、Node版本、provider、实际方法映射、重放/启动查询/权限限制实测结果；不以空字段冒充支持。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { requireCore } from './capabilities.ts';
test('daemon without observable execution is not an admitted target',()=>{
 const c={create:true,observe:false,events:true,cancel:true,approval:false,steer:false,exportArtifact:false,lookupByRequest:false};
 assert.throws(()=>requireCore(c),/PASEO_CORE_UNAVAILABLE/);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test services/paseo-adapter/src/capabilities.test.ts
```

预期：capabilities模块未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export interface PaseoCapabilities {create:boolean;observe:boolean;events:boolean;cancel:boolean;approval:boolean;steer:boolean;exportArtifact:boolean;lookupByRequest:boolean}
export function requireCore(c:PaseoCapabilities){
 if(!c.create||!c.observe||!c.events||!c.cancel)throw new Error('PASEO_CORE_UNAVAILABLE');
}
```
用只读Git查询确定候选SHA，隔离参考checkout读取package.json和公开SDK导出；记录实际方法与参数。工作区新增`services/paseo-adapter`，不复制上游app/CLI。package设置private/type=module、test=node:test/tsx，依赖精确版本。

- [ ] **Step 4：接通实际入口。**

探针在受控测试目录发一个无外部写入任务：create→查询→事件→完成；另起可停止任务验证cancel。通过代理丢弃回执，验证是否能按request关联查询；不能则标lookupByRequest=false并由W20停在unknown。approval/steer/导出未支持就声明不可用，不造API方法。固定版本许可证与notice入来源清单。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- `requireCore`正负测试、真实SDK编译和受控daemon契约测试分开记录。
- SDK/daemon不兼容、无provider、无鉴权、观察不可用均拒绝目标准入。
- 不执行上游安装/发布脚本；执行阶段安装明确依赖并审阅锁文件。
- 未获可用受控节点或模型凭据则blocked-env，W18模型可继续，但不得将远程链路标可用。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add services/paseo-adapter/package.json services/paseo-adapter/tsconfig.json services/paseo-adapter/src/capabilities.ts services/paseo-adapter/src/capabilities.test.ts docs/migrations/paseo/compatibility.json docs/evidence/mobile-workbench/W17-paseo-probe.md pnpm-workspace.yaml pnpm-lock.yaml
git diff --cached --check
git diff --cached --stat
git commit -m "feat(paseo): pin SDK and validate core capabilities"
```

### W18：执行目标、工作目录与当前授权

**依赖：** W02、W17。

**Files：**

- Create: `internal/execution/target.go`、`internal/execution/target_test.go`
- Create: `internal/application/repository/execution_target.go`、`internal/application/repository/execution_target_test.go`
- Create: `internal/handler/execution_target.go`
- Modify: `internal/router/routes_workbench.go`
- Create: `migrations/versioned/000126_execution_targets.{up,down}.sql`、`migrations/sqlite/000046_execution_targets.{up,down}.sql`

**Interfaces：**

Produces `Target{ID string;TenantID uint64;OwnerID,Kind,State string;CredentialVersion int64}`；`AuthorizeTarget(target Target,tenant uint64,actor string) error`；`Workspace{ID,TargetID,RootRef string;TenantID uint64}`。仓储GetOwnedTarget(ctx,tenant,actor,targetID)返回Target；root_ref由节点注册的根映射解析，客户端不可自报绝对路径。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package execution
import "testing"
func TestTargetCannotCrossSpace(t *testing.T) {
 target:=Target{ID:"n",TenantID:1,OwnerID:"u",Kind:"managed_node",State:"active",CredentialVersion:1}
 if AuthorizeTarget(target,2,"u")==nil {t.Fatal("cross-space target authorized")}
 if err:=AuthorizeTarget(target,1,"u");err!=nil {t.Fatal(err)}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/execution -run TestTarget -count=1
```

预期：Target/AuthorizeTarget 未定义。

- [ ] **Step 3：实现最小行为。**

```go
func AuthorizeTarget(t Target,tenant uint64,actor string) error {
 if t.TenantID!=tenant||t.OwnerID!=actor||t.State!="active" {return errors.New("target_forbidden")}
 return nil
}
```
首版个人owner模式，空间共享目标需独立ACL而非Admin默认访问。表中唯一(runtime_id,external_target_id)，target绑定不可跨空间更新；迁移/换绑通过撤销新注册。

- [ ] **Step 4：接通实际入口。**

GET execution-targets只返回授权目标；注册和撤销路由分别验证用户与节点身份。workspace查询同时匹配tenant和target；托管root可使用管理员配置，读取前canonicalize并检查实际打开文件未逃逸（符号链接/TOCTOU在W26节点导出落实）。手机选择目标与工作目录只保存opaque ID。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 同owner不同空间、同空间不同owner、revoked、credential_version旧值拒绝；返回无内部地址或密钥。
- SQLite/PostgreSQL唯一冲突、并发撤销与准入原子边界。
- 服务器不访问客户端提供的任意URL；metadata/private网段地址不能通过用户参数成为探测目标。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/execution/target.go internal/execution/target_test.go internal/application/repository/execution_target.go internal/application/repository/execution_target_test.go internal/handler/execution_target.go internal/router/routes_workbench.go migrations/versioned/000126_execution_targets.up.sql migrations/versioned/000126_execution_targets.down.sql migrations/sqlite/000046_execution_targets.up.sql migrations/sqlite/000046_execution_targets.down.sql
git diff --cached --check
git diff --cached --stat
git commit -m "feat(execution): scope targets and workspaces to product owners"
```

### W19：薄 SDK Bridge 与固定命令协议

**依赖：** W17、W18。

**Files：**

- Create: `services/paseo-adapter/src/protocol.ts`、`services/paseo-adapter/src/bridge.ts`、`services/paseo-adapter/src/bridge.test.ts`、`services/paseo-adapter/src/sdk-port.ts`
- Create: `internal/execution/bridge.go`、`internal/execution/bridge_test.go`

**Interfaces：**

Produces TS `StartCommand{commandID,runID,attemptID,targetID,workspaceRef,prompt,provider string;epoch:number;expiresAt:number}`；`PaseoPort{create(input:{cwd:string;prompt:string;provider:string}):Promise<{id:string}>;observe(id:string):Promise<{state:string}>;cancel(id:string):Promise<void>}`。Bridge通过受控workspace resolver获得cwd；其他操作W21/W22扩展。Go请求同字段的JSON协议，带version=1。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { startViaBridge } from './bridge.ts';
test('expired command never reaches SDK',async()=>{
 let calls=0;
 const port={create:async()=>{calls++;return {id:'a'};},observe:async()=>({state:'running'}),cancel:async()=>{}};
 await assert.rejects(startViaBridge({commandID:'c',runID:'r',attemptID:'a',targetID:'n',workspaceRef:'w',prompt:'hi',provider:'p',epoch:1,expiresAt:10},port,async()=>'/safe',20),/COMMAND_EXPIRED/);
 assert.equal(calls,0);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test services/paseo-adapter/src/bridge.test.ts
```

预期：startViaBridge未定义。

- [ ] **Step 3：实现最小行为。**

```ts
import type { StartCommand, PaseoPort } from './protocol.ts';
export async function startViaBridge(c:StartCommand,p:PaseoPort,resolve:(ref:string)=>Promise<string>,now:number){
 if(c.expiresAt<=now)throw new Error('COMMAND_EXPIRED');
 const cwd=await resolve(c.workspaceRef);
 return p.create({cwd,prompt:c.prompt,provider:c.provider});
}
```
生产入口先验证mTLS服务身份、签名、epoch、授权版本和command hash，再调用该接缝。请求大小上限、超时、允许操作固定；不提供任意method/path/headers代理。

- [ ] **Step 4：接通实际入口。**

sdk-port.ts只引用W17确认的公开SDK方法。已知create接`client.agents.create({config:{provider},cwd,prompt})`；observe/cancel的实际调用用W17固定映射实现，缺能力启动即报错。Go BridgeClient配置固定私网地址和服务身份；没有配置时返回unavailable，不能nil调用。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 过期签名、错误服务身份、未知version/operation、workspace不属于target、旧epoch均SDK调用0次；有效请求恰好1次。
- JSON双语言fixture往返；bridge单元通过后做真实SDK create/observe/cancel。
- 响应不得暴露cwd之外的宿主信息、凭据、provider原始token；错误码分类而非把异常全文展示。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add services/paseo-adapter/src/protocol.ts services/paseo-adapter/src/bridge.ts services/paseo-adapter/src/bridge.test.ts services/paseo-adapter/src/sdk-port.ts internal/execution/bridge.go internal/execution/bridge_test.go
git diff --cached --check
git diff --cached --stat
git commit -m "feat(paseo): bridge typed execution commands to the pinned SDK"
```

### W20：分发命令日志与不确定启动恢复

**依赖：** W04、W19。

**Files：**

- Create: `internal/application/repository/execution_dispatch.go`、`internal/application/repository/execution_dispatch_test.go`
- Create: `internal/application/service/workbench/remote_dispatch.go`
- Create: `services/paseo-adapter/src/command-log.ts`、`services/paseo-adapter/src/command-log.test.ts`
- Create: `migrations/versioned/000127_execution_dispatch.{up,down}.sql`、`migrations/sqlite/000047_execution_dispatch.{up,down}.sql`

**Interfaces：**

Produces `DispatchRecord{CommandID,RunID,AttemptID,PayloadHash,State,ExternalID string;TenantID uint64;Epoch int64}`；Go store ClaimDispatch/SaveReceipt/ReconcileUnknown。TS `dispatchOnce(log:CommandLog,id:string,hash:string,start:()=>Promise<string>):Promise<string>`；CommandLog.begin(id,hash):Promise<'new'|'unknown'|{externalID:string}>、complete(id,externalID):Promise<void>。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { dispatchOnce } from './command-log.ts';
test('unknown previous dispatch is not started again',async()=>{
 let calls=0;
 const log={begin:async()=> 'unknown' as const,complete:async()=>{}};
 await assert.rejects(dispatchOnce(log,'c','h',async()=>{calls++;return 'external';}),/DISPATCH_UNKNOWN/);
 assert.equal(calls,0);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test services/paseo-adapter/src/command-log.test.ts
```

预期：dispatchOnce 未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export interface CommandLog {begin(id:string,hash:string):Promise<'new'|'unknown'|{externalID:string}>;complete(id:string,externalID:string):Promise<void>}
export async function dispatchOnce(log:CommandLog,id:string,hash:string,start:()=>Promise<string>){
 const old=await log.begin(id,hash);
 if(old==='unknown')throw new Error('DISPATCH_UNKNOWN');
 if(old!=='new')return old.externalID;
 const externalID=await start();
 await log.complete(id,externalID);
 return externalID;
}
```
begin是持久事务，不是Map。先落started intent并fsync/commit再SDK调用；相同ID不同hash冲突。start抛错不能把intent删除；complete失败保留unknown。Go claim和预算授权在分发前有效。

- [ ] **Step 4：接通实际入口。**

remote worker使用W02 driver专用Scan/Claim，运行控制Epoch更新与命令日志关联。Bridge丢失响应后Go先查receipt和externalID；W17支持lookupByRequest才自动核对，否则等待用户核验。确认未启动才允许原逻辑命令再次派发，不能用新key绕过去重。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 逐个注入：claim后、SDK启动后、receipt落盘前、Go保存前；观察实际进程数始终≤1，未知不生成第二个。
- 多Bridge实例只一个有效控制者；旧Fence不允许发新命令；重启恢复日志仍有效。
- 20并发同命令、一条不同hash、不同租户同request均测数据库唯一性。
- 真实节点演练，不以mock证明exactly-once。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/application/repository/execution_dispatch.go internal/application/repository/execution_dispatch_test.go internal/application/service/workbench/remote_dispatch.go services/paseo-adapter/src/command-log.ts services/paseo-adapter/src/command-log.test.ts migrations/versioned/000127_execution_dispatch.up.sql migrations/versioned/000127_execution_dispatch.down.sql migrations/sqlite/000047_execution_dispatch.up.sql migrations/sqlite/000047_execution_dispatch.down.sql
git diff --cached --check
git diff --cached --stat
git commit -m "feat(paseo): persist dispatch uncertainty without duplicate starts"
```

### W21：远程事件去重和产品快照

**依赖：** W03、W20。

**Files：**

- Create: `internal/application/repository/execution_observation.go`、`internal/application/repository/execution_observation_test.go`
- Create: `services/paseo-adapter/src/events.ts`、`services/paseo-adapter/src/events.test.ts`
- Modify: `internal/handler/session/workbench_read.go`
- Create: `migrations/versioned/000128_execution_observations.{up,down}.sql`、`migrations/sqlite/000048_execution_observations.{up,down}.sql`

**Interfaces：**

Produces `SourceEvent{bindingID,generation,eventID,attemptID,type:string;payload:Record<string,unknown>}`；`sourceKey(e:SourceEvent):string`。Go IngestSourceEvent(ctx,binding,source)读取绑定解析tenant/run，事务插入来源唯一键并分配产品seq；产品事件W01格式。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { sourceKey } from './events.ts';
test('daemon restart does not collide with old source sequence',()=>{
 const a={bindingID:'b',generation:'g1',eventID:'1',attemptID:'a',type:'text',payload:{text:'same'}};
 assert.notEqual(sourceKey(a),sourceKey({...a,generation:'g2'}));
 assert.equal(sourceKey(a),sourceKey({...a,payload:{text:'different'}}));
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test services/paseo-adapter/src/events.test.ts
```

预期：sourceKey未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export interface SourceEvent {bindingID:string;generation:string;eventID:string;attemptID:string;type:string;payload:Record<string,unknown>}
export function sourceKey(e:SourceEvent){return JSON.stringify([e.bindingID,e.generation,e.eventID]);}
```
同source key不同payload hash必须报冲突，不能静默接受。daemon重启generation从可信连接会话分配，不由模型提供；文本相同也可为合法新事件。

- [ ] **Step 4：接通实际入口。**

将上游timeline快照/增量明确转换为产品实体事件；只有确定是delta才能拼接。来源cursor推进与本地ingest commit绑定。取消后进程退出和用量作为observation保存，不改Run终态；snapshot汇总execution_status与settlement_status。seq超safe integer拒绝，W01约束一致。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 重复事件不重复追加文本；同文本不同event保留；重置seq、不同attempt、来源冲突、离线回补。
- 终态前尾部事件大于一页时全部导入；源历史缺失标incomplete并取已确认快照。
- Go并发ingest唯一键与产品seq原子性，恢复后手机历史与节点结果一致。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/application/repository/execution_observation.go internal/application/repository/execution_observation_test.go services/paseo-adapter/src/events.ts services/paseo-adapter/src/events.test.ts internal/handler/session/workbench_read.go migrations/versioned/000128_execution_observations.up.sql migrations/versioned/000128_execution_observations.down.sql migrations/sqlite/000048_execution_observations.up.sql migrations/sqlite/000048_execution_observations.down.sql
git diff --cached --check
git diff --cached --stat
git commit -m "feat(paseo): project deduplicated source events into product runs"
```

### W22：远程取消、审批和工作目录锁

**依赖：** W05、W18、W20、W21。

**Files：**

- Create: `internal/execution/control.go`、`internal/execution/control_test.go`
- Create: `services/paseo-adapter/src/control.ts`、`services/paseo-adapter/src/control.test.ts`
- Modify: `internal/application/service/workbench/interaction.go`、`internal/application/service/agent_run_lifecycle.go`

**Interfaces：**

Produces `ExecutionObservation{ProcessState string;Fresh bool}`；`MayReleaseWorkspace(runStatus string,o ExecutionObservation) bool`。Bridge command联合cancel/steer/submitInteraction；远程工具批准绑定external_pending_id、参数hash、credential_version、expected_revision，经W05授权后发送。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package execution
import "testing"
func TestCanceledRunKeepsWorkspaceWhileStopUnknown(t *testing.T) {
 if MayReleaseWorkspace("canceled",ExecutionObservation{ProcessState:"unknown",Fresh:true}) {t.Fatal("released unconfirmed workspace")}
 if !MayReleaseWorkspace("canceled",ExecutionObservation{ProcessState:"exited",Fresh:true}) {t.Fatal("confirmed exit remains locked")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/execution -run TestCanceledRun -count=1
```

预期：MayReleaseWorkspace未定义。

- [ ] **Step 3：实现最小行为。**

```go
func MayReleaseWorkspace(status string,o ExecutionObservation) bool {
 terminal:=status=="canceled"||status=="failed"||status=="succeeded"
 return terminal&&o.Fresh&&(o.ProcessState=="exited"||o.ProcessState=="destroyed")
}
```
取消先撤产品执行授权，再发送有幂等ID的stop；SDK成功响应后仍Observe确认停止。等待超时保持stop_unconfirmed；原session活动槽可释放，但独立workspace锁不得释放。

- [ ] **Step 4：接通实际入口。**

Paseo能力不支持审批/追加就返回unavailable；不得以bash模拟。远程已处理交互返回409并重取，不默认重复批准。节点授权失效后禁止新控制与收费调用，允许受限stop/observe清理。手机展示已取消但停止待确认，提供查询而非再次启动按钮。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 取消时断网、SDK返回成功但进程仍在、target撤销、旧epoch命令、双端相反审批。
- 工作目录未解锁时新Run拒绝，其他独立目录不被误锁。
- 工具批准确认参数摘要，改变参数后旧批准无效；Go/TS控制协议对照。
- 在线受控节点10秒内停止确认或明确unknown，真实证据。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/execution/control.go internal/execution/control_test.go services/paseo-adapter/src/control.ts services/paseo-adapter/src/control.test.ts internal/application/service/workbench/interaction.go internal/application/service/agent_run_lifecycle.go
git diff --cached --check
git diff --cached --stat
git commit -m "feat(paseo): distinguish product cancellation from process termination"
```

### W23：个人节点出站注册与撤销

**依赖：** W18–W22；托管切片验收通过。

**Files：**

- Create: `services/paseo-adapter/src/node-connector.ts`、`services/paseo-adapter/src/node-connector.test.ts`
- Create: `internal/execution/registration.go`、`internal/execution/registration_test.go`
- Create: `internal/handler/execution_registration.go`
- Create: `migrations/versioned/000129_execution_registrations.{up,down}.sql`、`migrations/sqlite/000049_execution_registrations.{up,down}.sql`

**Interfaces：**

Produces `NodeGrant{targetID:string;epoch:number;expiresAt:number;operations:string[]}`；`validateNodeGrant(g:NodeGrant,expected:{targetID:string;epoch:number},operation:string,now:number):void`。注册挑战server nonce+公开密钥指纹，节点证明私钥持有，用户确认后短期节点证书；长期产品Bearer不落节点。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { validateNodeGrant } from './node-connector.ts';
test('old epoch and expired grants cannot operate a node',()=>{
 const g={targetID:'n',epoch:1,expiresAt:100,operations:['start']};
 assert.throws(()=>validateNodeGrant(g,{targetID:'n',epoch:2},'start',50),/NODE_GRANT/);
 assert.throws(()=>validateNodeGrant(g,{targetID:'n',epoch:1},'start',101),/NODE_GRANT/);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test services/paseo-adapter/src/node-connector.test.ts
```

预期：validateNodeGrant未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export interface NodeGrant {targetID:string;epoch:number;expiresAt:number;operations:string[]}
export function validateNodeGrant(g:NodeGrant,e:{targetID:string;epoch:number},op:string,now:number){
 if(g.targetID!==e.targetID||g.epoch!==e.epoch||g.expiresAt<=now||!g.operations.includes(op))throw new Error('NODE_GRANT');
}
```
签名验证必须在此函数前由标准TLS/签名库完成；不能拿自报JSON当签名令牌。连接器仅主动连已配置平台，daemon只绑定本机受控端口。注册挑战一次性消费，撤销提升credential_version并禁新命令。

- [ ] **Step 4：接通实际入口。**

实现断线退避、心跳、命令ack和节点本地日志；凭据保存系统安全存储或最小权限文件，轮换不复用注册挑战。配对二维码不含长期密钥；更换空间必须重新确认。首次目标仅个人owner，默认不能共享给其他成员。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 重放挑战、换公钥、跨target、过期证书、旧epoch、平台失联、节点时钟漂移。
- 在线撤销停止新命令；离线自主进程明确无法保证立刻停止，UI说明与能力flag一致。
- 保留托管模式不依赖连接器；需要公网部署/节点安装授权时执行者按既有授权范围操作，缺条件blocked-env。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add services/paseo-adapter/src/node-connector.ts services/paseo-adapter/src/node-connector.test.ts internal/execution/registration.go internal/execution/registration_test.go internal/handler/execution_registration.go migrations/versioned/000129_execution_registrations.up.sql migrations/versioned/000129_execution_registrations.down.sql migrations/sqlite/000049_execution_registrations.up.sql migrations/sqlite/000049_execution_registrations.down.sql
git diff --cached --check
git diff --cached --stat
git commit -m "feat(paseo): enroll revocable outbound personal nodes"
```

### W24：受控工具、可信用量与预算树接线

**依赖：** W04、W20–W22；open-connector实际调用另依赖其计划的受控dispatcher验收。

**Files：**

- Create: `internal/execution/usage_policy.go`、`internal/execution/usage_policy_test.go`
- Create: `internal/application/service/workbench/remote_usage.go`、`internal/application/service/workbench/remote_usage_test.go`
- Modify: `internal/application/service/commercial/execution.go`、`internal/container/agent_runtime.go`
- Create: `docs/evidence/mobile-workbench/W24-billing.md`

**Interfaces：**

Consumes现有 `ExecutionGate.Begin(ctx,commercial.BudgetRequest)` / `Finish(ctx,reservationID,commercial.UsageFact)`；Produces `AllowModelSettlement(source,funding string) bool`，trusted source为platform_gateway，BYOK不重复模型扣费。远程事件记录与结算输入分开，Bridge不能给自己声明trusted。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package execution
import "testing"
func TestRemoteUsageCannotSelfAuthorizeBilling(t *testing.T) {
 if AllowModelSettlement("personal_node","platform") {t.Fatal("untrusted report charged")}
 if AllowModelSettlement("platform_gateway","byok") {t.Fatal("BYOK double charged")}
 if !AllowModelSettlement("platform_gateway","platform") {t.Fatal("trusted usage rejected")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/execution -run TestRemoteUsage -count=1
```

预期：AllowModelSettlement未定义。

- [ ] **Step 3：实现最小行为。**

```go
func AllowModelSettlement(source,funding string) bool {
 return source=="platform_gateway"&&funding=="platform"
}
```
上述是外部接入策略枚举，adapter映射到现有commercial的真实funding枚举后才构造UsageFact；source来自鉴权服务身份和存储绑定，不接受body自报。按call/attempt/revision唯一消费、未知与display_only不结算，取消后的可信迟到用量仍可进入原预占。

- [ ] **Step 4：接通实际入口。**

平台托管Coding Agent的模型与外部业务操作走平台网关/ActionService；shell出口和凭据隔离由W35部署验证。复用同一Action ID，不嵌套第二次审批/预占；个人节点本机自由操作不宣称可治理。子任务预算绑定同root，禁止子任务再复制余额，追加/模型切换重新准入。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 平台模型、BYOK、个人节点虚报、重复用量、取消后用量、结果未知、超预算、子树并发分别记录数据库和OpenMeter证据。
- budget.Begin与Finish实际调用次数、reservation_ref不重复；网关失败不自动回退付费provider。
- 微信/支付宝/退款不在本任务重建；外部商业环境不可用只阻止收费上线。
- `go test ./internal/execution ./internal/application/service/workbench ./internal/application/service/commercial -run 'TestRemoteUsage|TestRemoteSettlement|TestExecution' -count=1`。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add internal/execution/usage_policy.go internal/execution/usage_policy_test.go internal/application/service/workbench/remote_usage.go internal/application/service/workbench/remote_usage_test.go internal/application/service/commercial/execution.go internal/container/agent_runtime.go docs/evidence/mobile-workbench/W24-billing.md
git diff --cached --check
git diff --cached --stat
git commit -m "feat(billing): account only trusted remote execution usage"
```


