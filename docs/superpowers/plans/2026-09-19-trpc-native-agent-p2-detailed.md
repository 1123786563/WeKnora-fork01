# tRPC Native Agent P2 — 运行治理、审批、预算与副作用 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使原生执行每次模型/工具/子任务调用遵循原有业务治理。

**Architecture:** 治理接口不承担推理循环。事务日志记录准入、attempt、决策与外部结果，恢复按工具能力选择查询、幂等重试或等待用户。

**Tech Stack:** Go 1.26.0；当前根 SDK 候选 v1.11.0（尚非获批完整产品组合）；SQLite/PostgreSQL 与 P1.0 确认的其他活跃方言；现有 React/TypeScript、Flutter/Expo 和 Go 客户端。

**Spec:** [已确认规格](../specs/2026-09-19-trpc-native-agent-migration-design.md)、[原总计划](2026-09-19-trpc-native-agent-migration.md)、[P0](2026-09-19-trpc-native-agent-p0.md)、[完整业务接口草案](trpc-native/interfaces.md)。

## Global Constraints

- “首版保留现有全部业务功能；验收后通过维护窗口一次切换。”
- “旧会话不恢复执行，也不隐式导入新上下文。”
- “归档不等于删除；未批准任何历史数据、附件或审计记录的清除。”
- “适配器只处理业务边界或原生组件缺失能力，每个适配器记录原因、覆盖测试和可删除条件。”
- “缺少凭据、服务或设备的项目标记 `blocked-env`，不得计为通过；必需项存在阻塞或失败时不得上线。”
- 不拆分独立 Agent 服务；不双执行真实副作用。身份与权限来自服务端；秘密仅保存引用。
- P0 NO-GO 仍有效。此文件是受前置门槛约束的详细实施方案，不是已选定数据库/SDK 组合的声明；门槛未关闭时只执行证据、契约和隔离探针任务。
- 每项代码任务按 RED → GREEN → 回归 → 需求/质量审查 → 修复复核 → 限定文件提交；不得将计划内代码示例当作已执行证据。
- 并行执行最多 4 Track；一 Track 一 branch/worktree，一 worktree 最多一 implementer；同一文件不得由两个 agent 修改。
- 详细依赖、共享文件队列、完整测试、证据格式见 [总索引与 DAG](2026-09-19-trpc-native-agent-p1-p9-detailed.md)。

## Review Focus

1. 审批后撤权：在下面任务的 RED 场景和集成验收中分别验证。
2. 旧 worker 提交：在下面任务的 RED 场景和集成验收中分别验证。
3. 未知非幂等结果：在下面任务的 RED 场景和集成验收中分别验证。
4. 父子预算并发：在下面任务的 RED 场景和集成验收中分别验证。
5. 断线误当取消：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P2.1 准入、幂等请求与排空开关 | P1.1 | `internal/application/service/native_admission.go`<br>`internal/application/service/native_admission_test.go` |
| P2.2 租约、epoch fencing 与恢复资格 | P2.1, P1.2 | `internal/application/repository/native_lease.go`<br>`internal/application/repository/native_lease_test.go`<br>`internal/application/service/native_recovery.go`<br>`internal/application/service/native_recovery_test.go` |
| P2.3 工具计划、attempt 与外部副作用恢复 | P2.2, P1.5 | `internal/agent/native/tool.go`<br>`internal/agent/native/tool_test.go`<br>`internal/application/repository/native_tool_journal.go`<br>`internal/application/repository/native_tool_journal_test.go` |
| P2.4 审批、OAuth 等待与一次性决策 | P2.3 | `internal/application/service/native_pending.go`<br>`internal/application/service/native_pending_test.go`<br>`internal/application/repository/native_pending.go`<br>`internal/application/repository/native_pending_test.go` |
| P2.5 父子预算、用量去重与失败结算 | P2.2, P1.5 | `internal/application/service/native_usage.go`<br>`internal/application/service/native_usage_test.go`<br>`internal/application/repository/native_usage.go`<br>`internal/application/repository/native_usage_test.go` |
| P2.6 取消、追加输入与子任务控制 | P2.4, P2.5 | `internal/application/service/native_control.go`<br>`internal/application/service/native_control_test.go` |

### Task P2.1: 准入、幂等请求与排空开关

**Depends on:** P1.1。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/application/service/native_admission.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_admission_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现 nativecontract.RunControl 的 Admit/Get，消费 AdmissionControlSource。新增 ValidateAdmissionControls(c AdmissionControls) error；生产接线仍受 P0 gate 限制。

- [ ] **Step 1：先写失败测试。** 相同 RequestID/hash 返回原 Run；不同 hash 冲突；admission 关闭/依赖未准备/未批准 native 时零模型调用；WorkerDrain 只阻止新 claim，不丢失旧结果。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/application/service -run 'NativeAdmission|BudgetExhaustionForNotification' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func ValidateAdmissionControls(c nativecontract.AdmissionControls) error {
    if !c.NativeExecutionApproved || !c.DependenciesReady {
        return nativecontract.Failure{Code: nativecontract.ErrExecutionGate}
    }
    if !c.AdmissionEnabled { return nativecontract.Failure{Code: nativecontract.ErrAdmissionClosed} }
    return nil
}
func TestClosedAdmission(t *testing.T) {
    err := ValidateAdmissionControls(nativecontract.AdmissionControls{})
    require.Error(t, err)
}
```

准入依次重查 scope、配置版本、资源权限、预算；事务写 request 幂等键与 Run。幂等键至少 tenant/owner/request；输入 hash 包含会影响执行的模型/Agent/附件/输入，不含临时 token。冻结开关读取失败时拒绝新任务；查询/取消与新建准入分开。数据库仓储写操作依赖 P1.2 集成后测试，P1.1 后只能以契约 fake 开发治理逻辑。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。两并发请求只产生一 Run、一预算预留；预算耗尽通知与终态均可靠持久化。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/application/service/native_admission.go internal/application/service/native_admission_test.go
git commit -m "feat: implement p2.1 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P2.2: 租约、epoch fencing 与恢复资格

**Depends on:** P2.1, P1.2。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/application/repository/native_lease.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_lease_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_recovery.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_recovery_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 新增 Claim(ctx context.Context, run nativecontract.RunIdentity, owner string, now time.Time, ttl time.Duration) (nativecontract.Fence,error)、Renew(ctx,Fence,time.Time,time.Duration) (Fence,error)。

- [ ] **Step 1：先写失败测试。** 两个 worker 同时 claim 只有一个成功；过期后接管 epoch+1；旧 worker 写结果/事件/终态/usage 全拒绝；恢复时配置不兼容拒绝执行，撤权后禁止外部调用。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/application/repository ./internal/application/service -run 'NativeLease|NativeRecovery|StaleWorker' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
UPDATE native_runs
SET lease_owner = :owner, epoch = epoch + 1, lease_until = :until
WHERE tenant_id = :tenant AND run_id = :run
  AND lease_until <= :now AND status NOT IN ('succeeded','failed','cancelled');
-- affected_rows 必须等于 1；随后同事务读取新 epoch。
```

用数据库时钟或明确一致的数据库时间采样比较租约，不用单进程 mutex。fence 传入每个状态提交、attempt/decision/usage/intent 操作，外部系统不支持 fence 时依赖 P2.3 的幂等/查询。恢复读取当前授权与配置快照版本，不能重建永久可信旧 context。续租失败停止 dispatch，已发生动作进入结果核对。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。真实 PostgreSQL 并发连接与 SQLite 双连接各验证一次；过期 writer 成功提交数为零。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/application/repository/native_lease.go internal/application/repository/native_lease_test.go internal/application/service/native_recovery.go internal/application/service/native_recovery_test.go
git commit -m "feat: implement p2.2 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P2.3: 工具计划、attempt 与外部副作用恢复

**Depends on:** P2.2, P1.5。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/tool.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tool_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_tool_journal.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_tool_journal_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现 nativecontract.ToolBoundary 与 AttemptJournal。新增 RecoveryAction(policy RecoveryPolicy, confirmed bool) string，返回 reuse/query/retry/hold；CallableTool 与 StreamableTool 均需包装。

- [ ] **Step 1：先写失败测试。** 外部成功后结果写失败；同 CallID 参数不同；审批后撤权；不支持查询且非幂等动作恢复进入 waiting_user，执行次数维持 1。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native ./internal/application/repository -run 'NativeTool|NativeAttempt|UnknownNonIdempotent' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func RecoveryAction(p nativecontract.RecoveryPolicy, confirmed bool) string {
    if confirmed { return "reuse" }
    switch p {
    case nativecontract.RecoveryQuery: return "query"
    case nativecontract.RecoveryIdempotent, nativecontract.RecoveryReadOnly: return "retry"
    default: return "hold"
    }
}
func TestUnknownNonIdempotentToolHolds(t *testing.T) {
    require.Equal(t, "hold", RecoveryAction(nativecontract.RecoveryHold, false))
}
```

持久化 plan+ArgsHash+schema/config version→保存决定→当前权限/预算/fence复验→持久化 started attempt→调用→保存 outcome→CommitIntent barrier。幂等 retry 必须核对 provider key 仍有效；过期则 hold。流式输出只是进度，不能视为外部完成证明。结果查询失败保留 unknown；用户确认解决只改变核对状态，不能伪造工具成功。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。工具 fake 计数之外，还需 P7 真实外部状态核对；任何 MCP/Skills/child 直连绕过会使任务失败。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/tool.go internal/agent/native/tool_test.go internal/application/repository/native_tool_journal.go internal/application/repository/native_tool_journal_test.go
git commit -m "feat: implement p2.3 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P2.4: 审批、OAuth 等待与一次性决策

**Depends on:** P2.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/application/service/native_pending.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_pending_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_pending.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_pending_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现 nativecontract.PendingDecisionService；完全保留 interfaces.md §3A 的 PendingKey、PendingDecisionDetail、ResolvePendingRequest 与 OAuth 类型，不另发明简化 approval boolean。

- [ ] **Step 1：先写失败测试。** 重复决策同 payload 幂等、冲突 payload 拒绝；旧 revision、跨空间、过期 OAuth state 拒绝；进程重启仍能列出原因、工具参数摘要、允许动作。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/application/service ./internal/application/repository -run 'NativePending|NativeOAuth' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
UPDATE native_pending_decisions
SET status = 'resolved', revision = revision + 1, decision_hash = :hash
WHERE tenant_id = :tenant AND run_id = :run AND pending_id = :pending
  AND revision = :revision AND status = 'pending';
-- 零行更新时读取现值：同 decision identity/hash 幂等，否则 conflict。
```

数据库保存 waiting 详情及授权服务身份；OAuth state 绑定 principal/空间/service/redirect allowlist/有效期，凭据由既有安全存储保存。审批只针对确定 plan hash；修改工具参数使旧批准失效。解决待决策后进入调度队列，由 worker 重新授权而非 HTTP handler 直接执行。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。测试同时读列表/详情/重复 resolve；重启与撤权用例不得仅测 UI。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/application/service/native_pending.go internal/application/service/native_pending_test.go internal/application/repository/native_pending.go internal/application/repository/native_pending_test.go
git commit -m "feat: implement p2.4 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P2.5: 父子预算、用量去重与失败结算

**Depends on:** P2.2, P1.5。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/application/service/native_usage.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_usage_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_usage.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_usage_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现 nativecontract.UsageLedger；ObservationID、AttemptID、ProviderRequestID、Revision 依 §4；共享 FundingBinding.BudgetRootRunID，平台模型/BYOK 使用现有商业归属。

- [ ] **Step 1：先写失败测试。** 重复回调和日志回放只记一次；两个子任务竞争剩余预算不得超额；取消/失败已发生用量仍计入；unknown 不得当零；大整数经过 wire 无精度损失。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/application/service ./internal/application/repository -run 'NativeUsage|NativeBudget|BudgetExhaustionForNotification' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
CREATE UNIQUE INDEX native_usage_observation
ON native_usage(tenant_id, run_id, attempt_id, observation_id, revision);
-- 相同 identity 不同 payload 必须 conflict，不能忽略。
-- 补充观察 revision > previous 时记录差量/修正，禁止把累计量多次相加。
```

预算预留采用现有商业事务/API 幂等身份；所有子调用以预算根申请额度。调用完成按已发生 usage 结算；不确定用量进入 reconciliation，不释放成免费调用。业务终态和预算耗尽通知走可靠事件写入；SDK callback 不是账本。明确 token 累计/增量口径并逐 provider 校验。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。真实商业测试账户核对预留/结算/释放与 BYOK 分支；不可用标记 blocked-env。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/application/service/native_usage.go internal/application/service/native_usage_test.go internal/application/repository/native_usage.go internal/application/repository/native_usage_test.go
git commit -m "feat: implement p2.5 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P2.6: 取消、追加输入与子任务控制

**Depends on:** P2.4, P2.5。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/application/service/native_control.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_control_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 完成 RunControl.Cancel/Steer；SteerInput.InputID 幂等，ExpectedRevision CAS；DelegateService 留给 P4.5 实现。 命令分类函数 CancelRequested(explicitCommand,streamDisconnected bool) bool 不将断线转换为业务取消。

- [ ] **Step 1：先写失败测试。** 断开 SSE 不取消；重复取消无重复外部命令；父取消传播；远端无法确认停止保留 cancelling/待核对；旧 revision steer 拒绝。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/application/service -run 'NativeControl|NativeCancel|NativeSteer' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func CancelRequested(explicitCommand bool, streamDisconnected bool) bool {
    return explicitCommand
}
func TestDisconnectDoesNotCancel(t *testing.T) {
    require.False(t, CancelRequested(false, true))
    require.True(t, CancelRequested(true, false))
    require.True(t, CancelRequested(true, true))
}
```

Run 持久化 cancel intent，worker 在安全边界检查并通知全部 child/sandbox；外部结果未知时不能直接 cancelled。steer 存储到有序输入日志，只在框架接受输入的安全点消费一次，禁用运行中任意修改 graph State。resume/continue/fork 只能针对新命名空间；旧归档明确只读。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。上下文示例只是边界校准；必须测试真正 handler 断线后 worker 继续、明确取消后 child 收到命令。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/application/service/native_control.go internal/application/service/native_control_test.go
git commit -m "feat: implement p2.6 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。

