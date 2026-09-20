# tRPC Native Agent P4 — 工具、MCP、Skills、知识与委派 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 迁移现有全部工具族并保持统一治理边界。

**Architecture:** 每个工具族在独立文件中提供原生工具协议实现，复用既有业务服务。动态发现、Skills 与子 Agent 无例外经过 ToolBoundary。

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

1. 发现后撤权：在下面任务的 RED 场景和集成验收中分别验证。
2. 工具名称/schema 冲突：在下面任务的 RED 场景和集成验收中分别验证。
3. Skills 本地 fallback：在下面任务的 RED 场景和集成验收中分别验证。
4. 远程成功但断线：在下面任务的 RED 场景和集成验收中分别验证。
5. 子任务扩大权限/预算：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P4.1 知识检索、引用与文档工具 | P3.4, P2.3 | `internal/agent/native/tools/knowledge.go`<br>`internal/agent/native/tools/knowledge_test.go` |
| P4.2 MCP 动态发现、OAuth 与名称映射 | P3.4, P2.4 | `internal/agent/native/tools/mcp.go`<br>`internal/agent/native/tools/mcp_test.go` |
| P4.3 Skills 安装绑定与 Sandbox 执行 | P3.4, P2.3 | `internal/agent/native/tools/skills.go`<br>`internal/agent/native/tools/skills_test.go`<br>`internal/agent/native/tools/sandbox.go`<br>`internal/agent/native/tools/sandbox_test.go` |
| P4.4 Connector 与专业 Agent/远程执行器 | P3.4, P2.4 | `internal/agent/native/tools/connector.go`<br>`internal/agent/native/tools/connector_test.go`<br>`internal/agent/native/tools/specialist.go`<br>`internal/agent/native/tools/specialist_test.go` |
| P4.5 原生多 Agent 委派 | P3.4, P2.6 | `internal/agent/native/delegate.go`<br>`internal/agent/native/delegate_test.go` |
| P4.6 工具注册集成与全功能覆盖核对 | P4.1, P4.2, P4.3, P4.4, P4.5 | `internal/agent/native/tools/registry.go`<br>`internal/agent/native/tools/registry_test.go`<br>`internal/application/service/native_tool_registry.go`<br>`internal/container/agent_runtime.go` |

### Task P4.1: 知识检索、引用与文档工具

**Depends on:** P3.4, P2.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/tools/knowledge.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/knowledge_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 输出 NewKnowledgeTools 的 SDK tool.Tool 列表，全部经 ToolBoundary.Wrap；业务服务继续负责知识权限、检索/GraphRAG/知识推理与文档读取。具体现有构造参数在 P0 features.tsv 对应 entry 映射，不改变服务接口。

- [ ] **Step 1：先写失败测试。** 两个空间同文档 ID；共享资源只读；引用来源撤权；空检索、截断、分页与不存在文档。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native/tools -run NativeKnowledge -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
// knowledge_test.go：使用 httptest.Server 捕获下游请求。
// 请求 A 空间工具时下游只允许 A 的 resource IDs；拒绝场景下调用计数必须为 0。
// 允许场景输出中的 citation.document_id 必须来自授权检索结果集合。
```

逐项将知识族旧 schema/description/限制转为 SDK schema；引用 ID 绑定资源与来源位置，不从模型文本自行信任 ID。检索失败返回明确工具错误；结果截断仍保留可解析引用。文档下载复用当前权限入口。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。真实知识资源执行检索、来源跳转、撤权；与 P0 功能行逐项对应。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/tools/knowledge.go internal/agent/native/tools/knowledge_test.go
git commit -m "feat: implement p4.1 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P4.2: MCP 动态发现、OAuth 与名称映射

**Depends on:** P3.4, P2.4。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/tools/mcp.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/mcp_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 消费 ToolBoundary、PendingDecisionService；工具身份含 ServiceID/Name/SchemaHash/ConfigVersion，发现结果不能绕过 Wrap。

- [ ] **Step 1：先写失败测试。** 同名工具来自两服务；发现后撤权；schema 更新使旧批准失效；OAuth 重启/重复回调；streamable 与 callable 均计数。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native/tools -run 'NativeMCP|MCPName' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func MCPName(serviceID, toolName string) string {
    enc := base64.RawURLEncoding.EncodeToString
    return "mcp_" + enc([]byte(serviceID)) + "." + enc([]byte(toolName))
}
func TestMCPNameCollision(t *testing.T) {
    require.NotEqual(t, MCPName("a/b", "c"), MCPName("a", "b/c"))
}
```

使用经过 P0 验证可拦截的 native MCP 组件；名称暴露长度/字符受 provider 限制时使用稳定 hash 映射并保存反向身份表，不能静默截断碰撞。每次 discovery/call 重查连接归属；OAuth 等待可靠保存，凭据由既有服务处理。MCP errors、输出上限、取消与远端结果查询映射统一失败语义。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。用授权 MCP 测试服务验证发现、调用、OAuth、撤权和取消，不只 fake。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/tools/mcp.go internal/agent/native/tools/mcp_test.go
git commit -m "feat: implement p4.2 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P4.3: Skills 安装绑定与 Sandbox 执行

**Depends on:** P3.4, P2.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/tools/skills.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/skills_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/sandbox.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/sandbox_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 消费 SDK skill.Repository/codeexecutor 已核验接口；执行只使用既有受控 Sandbox。SkillSetHash、InstallationID、WorkspaceRef 来自 ConfigBinding。 新增 ValidateSkillExecutor(kind string) error，装配缺失/本地执行器时直接拒绝。

- [ ] **Step 1：先写失败测试。** 未安装/版本改变拒绝；路径穿越；本地 executor fallback；长任务断线；取消未确认；跨空间文件和附件访问。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native/tools -run 'NativeSkill|NativeSandbox' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func ValidateSkillExecutor(kind string) error {
    if kind != "sandbox" {
        return nativecontract.Failure{Code: nativecontract.ErrForbidden}
    }
    return nil
}
func TestSkillExecutionHasNoLocalFallback(t *testing.T) {
    require.Error(t, ValidateSkillExecutor("local"))
    require.Error(t, ValidateSkillExecutor(""))
    require.NoError(t, ValidateSkillExecutor("sandbox"))
}
```

安装与授权仍归 WeKnora；固定安装版本/hash，加载只读 skill 内容。明确设置 Sandbox executor，缺失时 fail closed。工作区路径先规范化再校验 realpath/符号链接边界；长任务保存远端 job ID，经工具 journal 进行查询/取消。文件产物引用不得序列化主机秘密路径或凭据。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。授权 Sandbox 实测文件隔离、进程任务查询和断线恢复；源码检查无 fallback 仅是静态层。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/tools/skills.go internal/agent/native/tools/skills_test.go internal/agent/native/tools/sandbox.go internal/agent/native/tools/sandbox_test.go
git commit -m "feat: implement p4.3 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P4.4: Connector 与专业 Agent/远程执行器

**Depends on:** P3.4, P2.4。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/tools/connector.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/connector_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/specialist.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/specialist_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 受控工具消费既有 Connector/Craft/远程服务；保持 ADR-0001 共享运行时与租户连接控制。外部 job ID 对应 ToolOutcome.ProviderRequestID 或已有外部结果字段，不混用 child native RunID。

- [ ] **Step 1：先写失败测试。** 连接跨空间；只读 grant 尝试写；远程操作成功后网络失败；取消超时；Craft interaction/产物/用量回传。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native/tools -run 'NativeConnector|NativeSpecialist' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
-- 外部结果引用必须始终与 tenant/run/call 一起查询。
SELECT payload FROM native_tool_results
WHERE tenant_id = :tenant AND run_id = :run AND call_id = :call;
-- 不能仅按 provider_request_id 全局读取。
```

将现有专业能力清单逐个映射，保留外部引擎内部实现。连接 token 限定获准连接与动作；开始、交互、poll、cancel 均过治理边界。结果未知用查询或 hold，不以连接超时自动重建远程任务；费用记录纳入预算根。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。真实授权测试连接及专业 Agent 各执行一条允许、拒绝、取消和未知结果核对链。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/tools/connector.go internal/agent/native/tools/connector_test.go internal/agent/native/tools/specialist.go internal/agent/native/tools/specialist_test.go
git commit -m "feat: implement p4.4 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P4.5: 原生多 Agent 委派

**Depends on:** P3.4, P2.6。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/delegate.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/delegate_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现 nativecontract.DelegateService，使用已验证 native Agent 组合；子 Admission 保留 ParentRunID/BudgetRootRunID、权限交集、attempt 和结果关联。

- [ ] **Step 1：先写失败测试。** 子任务不能增加权限；父预算耗尽阻止子模型和工具；父取消扇出；同 ToolCallID 重放仅一 child；child 完成重复通知仅一结果。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native -run 'NativeDelegate|ChildBudget|ChildCancel' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func IntersectGrants(parent, current []nativecontract.ResourceGrant) []nativecontract.ResourceGrant {
    allowed := make(map[nativecontract.ResourceGrant]bool)
    for _, g := range current { allowed[g] = true }
    out := make([]nativecontract.ResourceGrant, 0)
    seen := make(map[nativecontract.ResourceGrant]bool)
    for _, g := range parent {
        if allowed[g] && !seen[g] { out = append(out, g); seen[g] = true }
    }
    return out
}
```

委派幂等键为 parent run + tool call；创建 child 与关联写同事务。权限交集仅处理已规范化精确 grant；wildcard/资源继承交由现有权限服务展开再判断。框架编排执行，WeKnora 管预算、取消、恢复、审计。并行 child 结果独立提交并由框架按稳定身份收集。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。使用真实 native 父子组合验收；不以另写业务循环替代原生编排。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/delegate.go internal/agent/native/delegate_test.go
git commit -m "feat: implement p4.5 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P4.6: 工具注册集成与全功能覆盖核对

**Depends on:** P4.1, P4.2, P4.3, P4.4, P4.5。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/tools/registry.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/tools/registry_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_tool_registry.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/container/agent_runtime.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 输出唯一受控 registry；禁止工具族任务各自改容器。消费所有工具族构造器与 P3 native capability builder。

- [ ] **Step 1：先写失败测试。** features.tsv 工具类别无漏项；所有可调用工具都经统一 gate；授权拒绝时每族外部计数均为零。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native/... ./internal/container -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
GOWORK=off go test -race ./internal/agent/native/... ./internal/application/service -count=1
rg -n 'NewRunner|WithTools|WithToolSets|codeexecutor' internal/agent/native internal/container
```

串行连接依赖，注册冲突直接失败；native execution 未批准时不得构造 Runner。按 P0 stable feature ID 逐行填新入口、测试名、证据路径。registry 暴露只读 metadata 给 UI，不暴露 service 对象。保留适配器登记理由、覆盖测试、删除条件。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。逐族测试通过且无缺失 feature；静态扫描与调用计数结合，不能仅搜字符串验收。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/tools/registry.go internal/agent/native/tools/registry_test.go internal/application/service/native_tool_registry.go internal/container/agent_runtime.go
git commit -m "feat: implement p4.6 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。

