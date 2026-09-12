# 移动工作台 06：交互补齐、运行治理与发布验收 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 闭合交互追踪、删除与费用恢复、部署隔离和双平台发布证据，形成可分阶段发布的工作台。

**Architecture:** 发布以明确能力集合和独立证据为准。先关闭新准入再升级/回退，保留旧任务的清理与结算；以持久事件和外部观察对账，而不是重发执行。

**Tech Stack:** 当前Go/Expo工具链、数据库迁移、容器编排、原生构建、故障注入与JSON验收记录。

**Spec:** [技术架构](../specs/2026-09-12-mobile-ai-saas-workbench-architecture.md) §2.1、§6、§14–17；[总计划](2026-09-12-mobile-ai-saas-workbench.md)。执行者同时读取本册与规格。

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

交互清单和发布证据放docs，通用运行安全放internal/execution与repository。部署配置只属于本工作台，不修改其他项目服务。W32依赖原Happy子计划完成尚未实现的交互，不能把禁用状态当完整保留。

### W32：高级交互能力端口与保留清单闭合

**依赖：** W10、W16、W22、W28、W31；原H24–H33按原计划分别满足。

**Files：**

- Create: `apps/mobile/sources/weknora/conversations/advanced.ts`、`apps/mobile/sources/weknora/conversations/advanced.test.ts`
- Modify: `docs/migrations/happy/interaction-matrix.json`、`apps/mobile/sources/-session/SessionView.tsx`
- Create: `docs/evidence/mobile-workbench/W32-interaction-parity.md`
- Modify: `docs/migrations/happy/source-manifest.json`（实际目标路径校正）

**Interfaces：**

Produces `AdvancedOperation='goal'|'fork'|'side_chat'|'archive'|'rewind'|'duplicate'|'terminal'`；`invokeAdvanced(op,capabilities,call):Promise<void>`。call只接固定operation和服务端产生的目标引用；端口映射到已验证的产品/远程操作。每个矩阵项新增productEndpoint、sourceCommit、implementationTask、nativeEvidence、status，保留原id。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { invokeAdvanced } from './advanced.ts';
test('unsupported advanced operation cannot fall back to a shell command',async()=>{
 let calls=0;
 await assert.rejects(invokeAdvanced('fork',{fork:false},async()=>{calls++;}),/CAPABILITY_UNAVAILABLE/);
 assert.equal(calls,0);
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test apps/mobile/sources/weknora/conversations/advanced.test.ts
```

预期：invokeAdvanced未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export type AdvancedOperation='goal'|'fork'|'side_chat'|'archive'|'rewind'|'duplicate'|'terminal';
export async function invokeAdvanced(op:AdvancedOperation,c:Partial<Record<AdvancedOperation,boolean>>,call:(op:AdvancedOperation)=>Promise<void>){
 if(c[op]!==true)throw new Error('CAPABILITY_UNAVAILABLE');
 await call(op);
}
```
各功能后端实现沿原H26高级交互和H27–H33管理任务，不假定本仓库已有fork/goal端点。先读取原子计划的接口与代码步骤；发生Paseo能力差异时把确切driver映射写到本任务矩阵后再实现，不用提示词模拟已成功的fork/rewind。

- [ ] **Step 4：接通实际入口。**

终端复用已有sandbox terminal-ticket入口并另做远程目标授权；token一次性、短期，不能通用shell RPC透传。fork/side chat复制的是获准历史或引用，不复制未决审批/预算余额/执行租约；预算树关系由服务端准入决定。归档不等于取消；rewind不撤销已发生外部副作用。模型/effort选项来自能力，不保留未经授权的Happy默认模型。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 对goal、fork、side chat、archive、rewind、duplicate、terminal分别记录操作前后数据和API；不支持项仍open，不能凭不可用提示完成矩阵。
- 原H27–H33包含设置/模型/连接/知识管理等交互，逐项确认原生验收；若未完成，只允许核心能力发布，不声称完整保留。
- 审批、通知、语音、主题、大字体、平板、横屏、分享、文件/Diff每项有iOS/Android证据。
- 来源manifest指向实际apps/mobile路径，修正历史happy-mobile写法且不改来源hash含义。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'apps/mobile/sources/weknora/conversations/advanced.ts' 'apps/mobile/sources/weknora/conversations/advanced.test.ts' 'docs/migrations/happy/interaction-matrix.json' 'apps/mobile/sources/-session/SessionView.tsx' 'docs/evidence/mobile-workbench/W32-interaction-parity.md' 'docs/migrations/happy/source-manifest.json'
git diff --cached --check
git diff --cached --stat
git commit -m "feat(mobile): gate advanced interactions with verified product mappings"
```

### W33：删除墓碑、远程停止和迟到用量

**依赖：** W02、W04；远程停止与迟到费用部分另需W21、W22、W24，文件部分另需W26。

**Files：**

- Create: `internal/execution/cleanup.go`、`internal/execution/cleanup_test.go`
- Create: `internal/application/repository/execution_cleanup.go`、`internal/application/repository/execution_cleanup_test.go`
- Modify: `internal/application/repository/agent_run_lifecycle.go`、`internal/application/service/agent_run_lifecycle.go`
- Create: `migrations/versioned/000132_execution_cleanup.{up,down}.sql`、`migrations/sqlite/000052_execution_cleanup.{up,down}.sql`

**Interfaces：**

Produces `CleanupFacts{Stopped,Settled,RetentionElapsed bool}`；`CanPurge(f CleanupFacts)bool`；仓储TombstoneSession(ctx,tenant,owner,sessionID) error与ClaimCleanup/CompleteCleanup，按原所有权与删除revision隔离。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package execution
import "testing"
func TestCleanupRequiresStopAndSettlement(t *testing.T) {
 if CanPurge(CleanupFacts{Stopped:false,Settled:true,RetentionElapsed:true}){t.Fatal("orphaned live process")}
 if CanPurge(CleanupFacts{Stopped:true,Settled:false,RetentionElapsed:true}){t.Fatal("lost unsettled usage")}
 if !CanPurge(CleanupFacts{Stopped:true,Settled:true,RetentionElapsed:true}){t.Fatal("safe cleanup blocked")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/execution -run TestCleanup -count=1
```

预期：CleanupFacts/CanPurge未定义。

- [ ] **Step 3：实现最小行为。**

```go
func CanPurge(f CleanupFacts)bool{return f.Stopped&&f.Settled&&f.RetentionElapsed}
```
会话删除先持久墓碑并撤访问与新命令；保留最小run/binding/reservation引用。只有外部停止确认、结算核对完成、保留期满足才删可清理事件/文件。商业审计按其自身策略保留，不级联删除。

- [ ] **Step 4：接通实际入口。**

替换现有DeleteSessionRuns立即删除依赖记录的远程路径；旧平台路径也回归不会因新墓碑复活。清理worker可在driver关闭后继续执行。迟到外部回执只更新观察和账务，不恢复被删除内容的可访问性。blob无引用后按保留策略删除，跨会话共享引用不得误删。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 删除时远程offline、取消响应丢失、用量迟到、文件上传完成迟到、回调重放；均保留必要控制记录且无法再读会话。
- 同space其他owner不能删除；恢复备份后墓碑不被旧事件覆盖。
- sqlite/postgres事务测试与真实节点删会话演练；未知保持cleanup_pending并告警，不无限静默等待。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'internal/execution/cleanup.go' 'internal/execution/cleanup_test.go' 'internal/application/repository/execution_cleanup.go' 'internal/application/repository/execution_cleanup_test.go' 'internal/application/repository/agent_run_lifecycle.go' 'internal/application/service/agent_run_lifecycle.go' 'migrations/versioned/000132_execution_cleanup.up.sql' 'migrations/versioned/000132_execution_cleanup.down.sql' 'migrations/sqlite/000052_execution_cleanup.up.sql' 'migrations/sqlite/000052_execution_cleanup.down.sql'
git diff --cached --check
git diff --cached --stat
git commit -m "feat(runtime): defer destructive cleanup until execution reconciliation"
```

### W34：安全部署、能力开关与可观测性

**依赖：** W03、W07、W13–W15、W33；启用Paseo需W19–W24，启用语音需W30。

**Files：**

- Create: `deploy/mobile-workbench/compose.yaml`、`deploy/mobile-workbench/README.md`
- Create: `internal/execution/deployment_policy.go`、`internal/execution/deployment_policy_test.go`
- Modify: `internal/config/config.go`、`internal/container/agent_runtime.go`
- Create: `docs/evidence/mobile-workbench/W34-deployment.md`

**Interfaces：**

Produces `DeploymentPolicy{Privileged,DockerSocket,SharedTenantHome,EnforcedEgress bool}`；`ValidateManagedPolicy(p DeploymentPolicy)error`。配置分read_enabled/platform_admission/paseo_admission/voice_admission/notifications_enabled与worker_drain，不用一个开关同时砍掉查询/清理。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package execution
import "testing"
func TestManagedNodeRejectsSharedPrivileges(t *testing.T) {
 for _,p:=range []DeploymentPolicy{{Privileged:true},{DockerSocket:true},{SharedTenantHome:true},{EnforcedEgress:false}} {
  if ValidateManagedPolicy(p)==nil{t.Fatal("unsafe managed deployment")}
 }
 if err:=ValidateManagedPolicy(DeploymentPolicy{EnforcedEgress:true});err!=nil{t.Fatal(err)}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/execution -run TestManagedNode -count=1
```

预期：DeploymentPolicy/ValidateManagedPolicy未定义。

- [ ] **Step 3：实现最小行为。**

```go
func ValidateManagedPolicy(p DeploymentPolicy)error{
 if p.Privileged||p.DockerSocket||p.SharedTenantHome||!p.EnforcedEgress{return errors.New("managed_policy_rejected")};return nil
}
```
实际部署采用非root、只读根文件系统、独立工作卷、CPU/内存/PID限额，秘密通过安全挂载。compose只是本地受控验证：egress网关或主机网络策略必须实际限制出口，不能用配置bool替代隔离证据。

- [ ] **Step 4：接通实际入口。**

API/Bridge固定私网地址与mTLS；不发布daemon管理口、数据库或open-connector控制台。日志/trace仅关联ID，无token/prompt/file正文。指标包含dispatch_unknown、stop_unconfirmed、observer_age、settlement_backlog、notification_backlog；告警关联操作手册。关闭新准入后允许现有任务收尾与清理。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 实际容器尝试读取其他tenant卷、访问Docker socket、未经网关联网、读取长期secret，均失败；合法模型/Action调用成功。
- 两Bridge实例租约竞争、滚动升级drain、版本不兼容拒绝。
- 备份密钥、对象存储访问与凭据轮换单独演练；不在日志输出环境变量或DSN。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'deploy/mobile-workbench/compose.yaml' 'deploy/mobile-workbench/README.md' 'internal/execution/deployment_policy.go' 'internal/execution/deployment_policy_test.go' 'internal/config/config.go' 'internal/container/agent_runtime.go' 'docs/evidence/mobile-workbench/W34-deployment.md'
git diff --cached --check
git diff --cached --stat
git commit -m "feat(deploy): isolate managed workbench execution and admission"
```

### W35：事件保留、备份恢复与崩溃演练

**依赖：** W03、W33、W34；远程故障场景需W20–W24。

**Files：**

- Create: `internal/execution/restore_policy.go`、`internal/execution/restore_policy_test.go`
- Create: `scripts/mobile-workbench/fault-scenarios.mjs`
- Create: `docs/evidence/mobile-workbench/W35-recovery.md`
- Modify: `internal/application/repository/agent_run_events.go`、`deploy/mobile-workbench/README.md`

**Interfaces：**

Produces `MayDispatchAfterRestore(reconciled bool,unknown int)bool`。故障脚本仅作用于显式allowlisted非生产测试部署，场景start_ack_lost/bridge_restart/daemon_restart/db_unavailable/restore_snapshot；每个场景输出JSON containing baseline_sha,run_id,actual_process_count,usage_count,result。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```go
package execution
import "testing"
func TestRestoreCannotReplayUnreconciledCommands(t *testing.T) {
 if MayDispatchAfterRestore(false,0)||MayDispatchAfterRestore(true,1){t.Fatal("unsafe redispatch")}
 if !MayDispatchAfterRestore(true,0){t.Fatal("reconciled restore blocked")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/execution -run TestRestore -count=1
```

预期：MayDispatchAfterRestore未定义。

- [ ] **Step 3：实现最小行为。**

```go
func MayDispatchAfterRestore(reconciled bool,unknown int)bool{return reconciled&&unknown==0}
```
备份恢复默认关闭准入，逐binding观察仍活跃的外部进程、核对用量与命令回执、重建投影，才开放新的分发。不能从旧备份的queued状态推断从未执行。保留期建议30天可配置；先生成一致快照再裁剪，活跃审批和商业记录按各自保留策略。

- [ ] **Step 4：接通实际入口。**

脚本先校验WB_TEST_DEPLOYMENT标识、origin允许列表与版本，再通过测试部署控制接口/容器操作注入故障；没有显式目标立即非零退出。对照数据库Run数量、外部进程数量和商业用量记录，不只检查HTTP200。快照恢复后node credential重新核验。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 每场景重复三次，实际重复启动0、跨租户泄漏0、未确认费用丢失0；有unknown则保持阻塞不自动判失败再启动。
- object metadata恢复与对象摘要对应；删除墓碑保留；cursor_expired恢复无丢内容。
- 控制面恢复时间与数据损失实际测量，未设业务SLA不承诺固定RTO/RPO。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'internal/execution/restore_policy.go' 'internal/execution/restore_policy_test.go' 'scripts/mobile-workbench/fault-scenarios.mjs' 'docs/evidence/mobile-workbench/W35-recovery.md' 'internal/application/repository/agent_run_events.go' 'deploy/mobile-workbench/README.md'
git diff --cached --check
git diff --cached --stat
git commit -m "test(workbench): verify crash recovery and restore barriers"
```

### W36：原生升级、兼容窗口与性能验收

**依赖：** W12、W16、W34；产物需W27，语音需W31，高级能力需W32。

**Files：**

- Create: `packages/domain/src/mobile/compatibility.ts`、`packages/domain/src/mobile/compatibility.test.ts`
- Create: `docs/evidence/mobile-workbench/W36-native-release.md`、`docs/evidence/mobile-workbench/W36-performance.md`
- Modify: `apps/mobile/package.json`、`deploy/mobile-workbench/README.md`
- Modify: `pnpm-lock.yaml`（仅依赖确实变化时同步）

**Interfaces：**

Produces `protocolMode(client:number,minimum:number,maximum:number):'full'|'upgrade_required'|'server_upgrade_required'`。服务端capabilities携带兼容上下界；未知schema不允许发控制命令，保留安全登录/升级说明。包和原生runtime版本在发布清单固定。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { protocolMode } from './compatibility.ts';
test('unsupported clients cannot issue control commands',()=>{
 assert.equal(protocolMode(1,2,3),'upgrade_required');
 assert.equal(protocolMode(4,2,3),'server_upgrade_required');
 assert.equal(protocolMode(2,2,3),'full');
});
```

- [ ] **Step 2：运行 RED。**

```bash
pnpm exec tsx --test packages/domain/src/mobile/compatibility.test.ts
```

预期：protocolMode未定义。

- [ ] **Step 3：实现最小行为。**

```ts
export function protocolMode(client:number,min:number,max:number):'full'|'upgrade_required'|'server_upgrade_required'{
 if(![client,min,max].every(Number.isSafeInteger)||min<1||max<min)throw new Error('INVALID_PROTOCOL_RANGE');
 return client<min?'upgrade_required':client>max?'server_upgrade_required':'full';
}
```
数据库迁移用expand/contract，旧App字段兼容窗口至少覆盖本次发布仍支持的上一协议版本；破坏性清理在用户升级和兼容数据验证后才做。原生依赖变更不能靠JS热更新替代原生发布。

- [ ] **Step 4：接通实际入口。**

运行当前锁文件安装、共享回归、mobile typecheck、双平台Expo export、原生Debug/Release构建、安装启动与逐屏交互；每层分别记录。测试发布版真实包，不只Metro。性能场景冻结：100个会话/每会话1000事件/10并发Run，本地或指定测试网络条件单独记载；不能把这些数字当生产容量承诺。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 目标：正常网络列表p95≤500ms、命令受理p95≤1s（不含模型）、恢复5s内获取权威状态；未达标留测量与修复，不改指标伪通过。
- 老App+新API、新App+最低API、回退禁新准入后旧Run仍可查询/清理。
- iOS/Android大字体、键盘、横屏、平板、文件分享、推送与音频；双平台签名发行配置另列。
- 原生支付发行策略单独审查目标商店/地区；未完成不开放购买入口，不把Web支付当商店验收。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'packages/domain/src/mobile/compatibility.ts' 'packages/domain/src/mobile/compatibility.test.ts' 'docs/evidence/mobile-workbench/W36-native-release.md' 'docs/evidence/mobile-workbench/W36-performance.md' 'apps/mobile/package.json' 'deploy/mobile-workbench/README.md' 'pnpm-lock.yaml'
git diff --cached --check
git diff --cached --stat
git commit -m "test(mobile): verify native release compatibility and performance"
```

### W37：完整验收门禁与分阶段交付报告

**依赖：** 对应发布能力的全部W任务；完整Happy保留另外要求原H矩阵全部验收。

**Files：**

- Create: `scripts/mobile-workbench/check-acceptance.mjs`、`scripts/mobile-workbench/check-acceptance.test.mjs`
- Create: `docs/evidence/mobile-workbench/acceptance.json`、`docs/evidence/mobile-workbench/release-report.md`
- Modify: `docs/superpowers/plans/mobile-workbench-progress.md`（实现时建立的唯一W台账）

**Interfaces：**

Produces `missingEvidence(rows:Array<{kind:string;status:string}>,required:string[]):string[]`。证据记录追加baseline_sha、command、exit_code、artifact_path/hash、observed_at、review_ref；kind为unit/database/backend_model/ios_native/android_native/remote/billing/security/recovery。

- [ ] **Step 1：写失败测试。** 以下代码放入本任务 Test 文件；一个测试失败必须定位到本任务行为，不接受环境故障冒充 RED。

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import { missingEvidence } from './check-acceptance.mjs';
test('mock and skipped runs cannot satisfy native acceptance',()=>{
 const rows=[{kind:'unit',status:'pass'},{kind:'ios_native',status:'skipped'}];
 assert.deepEqual(missingEvidence(rows,['unit','ios_native','android_native']),['ios_native','android_native']);
});
```

- [ ] **Step 2：运行 RED。**

```bash
node --test scripts/mobile-workbench/check-acceptance.test.mjs
```

预期：check-acceptance模块或missingEvidence未定义。

- [ ] **Step 3：实现最小行为。**

```js
export function missingEvidence(rows,required){
 return required.filter(kind=>!rows.some(row=>row.kind===kind&&row.status==='pass'));
}
```
CLI在上述分类之外校验文件存在/摘要、命令非空、退出码0、baseline对应候选提交、报告无skip和审查引用。不能仅凭手填status=pass；独立reviewer阅读原始证据与实际请求/数据库结果。

- [ ] **Step 4：接通实际入口。**

core发布要求W01–W07、W09–W16及其数据库/双平台/模型证据；OIDC宣称需W08，remote需W17–W24，resources需W25–W28，voice需W29–W31。所有发布还需要适用的W33–W36治理门槛；某能力未启用时显式声明not_in_release，不把它标验收通过。完整Happy交互要W32和原H清单全部闭合。

- [ ] **Step 5：GREEN、回归与独立审查。** 重跑 Step 2，预期全部 PASS、无意外 skip；再完成以下可判定验收，记录命令、退出码和证据。

- 验收缺项、过期SHA、缺文件、非零exit、SKIP、只有实现者报告均非零退出。
- 两条平台Agent链、跨端审批、重连、Paseo故障、取消后结算、预览安全分别可追踪至真实证据。
- 最终全分支规格/质量审查通过，报告已交付/未交付/blocked-env/已知风险与回退命令。
- 完成计划编写不创建此acceptance的pass记录；本任务仅在执行时收集真实结果。

规格审查核对本任务接口与架构覆盖；质量审查核对权限、竞态、持久化和错误路径。修复发现后重跑受影响检查，审查通过前不进入依赖任务。

- [ ] **Step 6：范围提交。** 在隔离实现分支执行，显式列出 Step 1–4 产生的文件；审核暂存 diff 后提交。更新本计划台账，不覆盖其他计划状态。

```bash
git add 'scripts/mobile-workbench/check-acceptance.mjs' 'scripts/mobile-workbench/check-acceptance.test.mjs' 'docs/evidence/mobile-workbench/acceptance.json' 'docs/evidence/mobile-workbench/release-report.md' 'docs/superpowers/plans/mobile-workbench-progress.md'
git diff --cached --check
git diff --cached --stat
git commit -m "docs(workbench): record evidence-gated release acceptance"
```
