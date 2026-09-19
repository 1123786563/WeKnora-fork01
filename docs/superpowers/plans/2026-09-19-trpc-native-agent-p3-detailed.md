# tRPC Native Agent P3 — 原生 Runner、模型、上下文与恢复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立保留业务能力且可恢复的原生执行链。

**Architecture:** Provider 直接采用 SDK Model，Runner/原生 Agent 负责推理和组合。Session/Memory 与业务治理通过已冻结契约接入；不复制旧循环。

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

1. 静默 channel 关闭：在下面任务的 RED 场景和集成验收中分别验证。
2. 多模态/工具分片丢失：在下面任务的 RED 场景和集成验收中分别验证。
3. 检查点版本变化：在下面任务的 RED 场景和集成验收中分别验证。
4. 压缩破坏 tool pair：在下面任务的 RED 场景和集成验收中分别验证。
5. Memory 禁用后仍写：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P3.1 原生模型工厂与 Provider 参数保真 | P1.5, P2.5 | `internal/agent/native/model.go`<br>`internal/agent/native/model_test.go`<br>`internal/agent/native/provider_matrix_test.go` |
| P3.2 Runner 生命周期及合法终态 | P3.1, P2.6 | `internal/agent/native/runner.go`<br>`internal/agent/native/runner_test.go` |
| P3.3 GraphAgent 恢复、版本 envelope 与六间隙 | P3.2 | `internal/agent/native/graph.go`<br>`internal/agent/native/checkpoint.go`<br>`internal/agent/native/recovery_test.go` |
| P3.4 上下文压缩、Memory 与会话能力装配 | P3.3, P1.4 | `internal/agent/native/context.go`<br>`internal/agent/native/context_test.go`<br>`internal/application/service/native_capabilities.go`<br>`internal/application/service/native_capabilities_test.go` |

### Task P3.1: 原生模型工厂与 Provider 参数保真

**Depends on:** P1.5, P2.5。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/model.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/model_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/provider_matrix_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现 nativecontract.ModelResolver；消费现有模型配置和凭据解析，返回 SDK model.Model + ConfigBinding + FundingBinding；不经过 internal/models/chat 的双向消息桥。

- [ ] **Step 1：先写失败测试。** 覆盖 features.tsv 每个 Provider/特殊参数；图片、reasoning、tool-call 分片、代理/timeout、取消；未经授权的 credential ref 不解析。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native -run 'NativeModel|ProviderMapping' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
var _ nativecontract.ModelResolver = (*ModelResolver)(nil)
// provider_matrix_test.go 每个现有 provider 一个具名子测试；
// mock HTTP 服务捕获实际请求 JSON，断言工具 ID、图片、reasoning 字段、超时与认证头。
// 真实 provider 用例另组，不能由 mock 子测试的 PASS 代替。
```

先按旧配置类型逐字段映射到选定 SDK provider；没有原生等价字段只添加该字段扩展并登记删除条件。每次调用解析当前凭据引用，不能把密钥写进 checkpoint。明确 SDK/provider 自带重试，防止与业务重试相乘；重试次数、总 deadline、并发上限来自现有配置。新增 attempt 才能重发模型请求。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。mock 请求快照不含密钥；真实 provider 各项保存请求身份、脱敏结果、usage 与费用；缺失项阻止产品门槛通过。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/model.go internal/agent/native/model_test.go internal/agent/native/provider_matrix_test.go
git commit -m "feat: implement p3.1 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P3.2: Runner 生命周期及合法终态

**Depends on:** P3.1, P2.6。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/runner.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/runner_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 新增 Execute(ctx context.Context, admission nativecontract.Admission, fence nativecontract.Fence) error；runner 依赖 Session/Memory、ToolBoundary、UsageLedger、CommitCoordinator；终态经可靠提交。

- [ ] **Step 1：先写失败测试。** channel 静默关闭、nil event、error event、取消、有效 finish、Session append 失败；除完整 finish+barrier 外不得 succeeded。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native -run 'NativeRunner|ChannelClose' -count=20
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func CanComplete(hasFinal bool, pendingTools int, barrierApplied bool) bool {
    return hasFinal && pendingTools == 0 && barrierApplied
}
func TestChannelCloseCannotComplete(t *testing.T) {
    require.False(t, CanComplete(false, 0, true))
    require.False(t, CanComplete(true, 1, true))
    require.False(t, CanComplete(true, 0, false))
    require.True(t, CanComplete(true, 0, true))
}
```

采用 P0 核验的 Runner/LLMAgent 组合，消费 events 并映射业务结果；channel 关闭但未合法完成返回 ErrIncomplete。所有工具只从受控 registry 注入。资源 Close 在所有错误路径执行，context 来自 worker。不得把旧 ReAct 循环嵌进 graph。产品构造/dispatch 前必须由独立 gate-resolution 审查确认 P0 原条件已满足；否则只继续 isolated nativeprobe。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。真实 SDK 完整对话、工具往返及 append 失败零后续 dispatch；状态函数通过不是 Runner 验收。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/runner.go internal/agent/native/runner_test.go
git commit -m "feat: implement p3.2 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P3.3: GraphAgent 恢复、版本 envelope 与六间隙

**Depends on:** P3.2。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/graph.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/checkpoint.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/recovery_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 消费 SDK graph.CheckpointSaver、nativecontract.CheckpointWrite/CommitCoordinator；新增 CompatibleCheckpoint(savedSDK,savedGraph,currentSDK,currentGraph string) bool。

- [ ] **Step 1：先写失败测试。** 六间隙分别中断；checkpoint 先于 Session、错误 graph/schema、重复 tool result、过期 fence；普通会话也可恢复。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native -run 'NativeRecovery|Checkpoint|RejectsOldGraph' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func CompatibleCheckpoint(savedSDK, savedGraph, currentSDK, currentGraph string) bool {
    return savedSDK == currentSDK && savedGraph == currentGraph
}
func TestRejectsOldGraph(t *testing.T) {
    require.False(t, CompatibleCheckpoint("v1.11.0", "g1", "v1.11.0", "g2"))
}
```

选择原生 recoverable Agent 组合，框架推进推理，业务层仅协调持久化。checkpoint envelope 再加 schema_version 与 namespace/lineage 校验；不兼容时显式拒绝并保留待处置记录。结果调用 ID 必须查 journal 确认；pending writes、interrupt、resume 按固定 SDK 源码实现。逐工具耐久边界无法建立就维持 NO-GO，禁止退化为每轮只保存一次完整结果。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。P7 进程级矩阵进一步验收；本任务确定性错误注入必须先覆盖所有六间隙。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/graph.go internal/agent/native/checkpoint.go internal/agent/native/recovery_test.go
git commit -m "feat: implement p3.3 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P3.4: 上下文压缩、Memory 与会话能力装配

**Depends on:** P3.3, P1.4。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/context.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/context_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_capabilities.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_capabilities_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 消费 SDK Session summary + MemoryGovernance，构造 native Agent 配置；保留现有 Agent prompt/persona、语言、setup/finish、附件/知识选择与上下文限制。 新增 ValidateToolPairs(calls,results []string) error，在 SDK 压缩实际输出提取 ID 后检查孤立结果。

- [ ] **Step 1：先写失败测试。** 压缩不能留下孤立工具结果；摘要不能越过 through_event_id；禁用/删除 Memory 后后台作业不能复活；新会话不得含归档消息。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native ./internal/application/service -run 'NativeContext|NativeCapabilities|Summary' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func ValidateToolPairs(calls, results []string) error {
    known := make(map[string]bool)
    for _, id := range calls { known[id] = true }
    for _, id := range results {
        if !known[id] { return fmt.Errorf("orphan tool result: %s", id) }
    }
    return nil
}
func TestSummaryRejectsOrphanResult(t *testing.T) {
    require.Error(t, ValidateToolPairs([]string{"a"}, []string{"b"}))
    require.NoError(t, ValidateToolPairs([]string{"a"}, []string{"a"}))
}
```

设置 token/context 上限并绑定 model tokenizer/计数策略；摘要绑定稳定历史边界，保留完整未闭合 tool call/result 对。Memory 读取与后台抽取走 P1.4 facade；setup/finish 重放需幂等，不对旧历史自动补执行。将已存在的 persona、专业 Agent 系统 prompt、工具权限、资源选择映射到 native Agent options，不修改产品能力。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。旧功能清单每条有新装配入口；重启/压缩/新 attempt 不改变工具关联或授权范围。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/context.go internal/agent/native/context_test.go internal/application/service/native_capabilities.go internal/application/service/native_capabilities_test.go
git commit -m "feat: implement p3.4 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。

