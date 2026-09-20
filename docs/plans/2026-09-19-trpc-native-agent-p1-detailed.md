# tRPC Native Agent P1 — 数据权威、存储与可靠投影 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立范围隔离的新 Session/Memory 存储和六间隙提交屏障。

**Architecture:** 在既有业务数据库与选定 SDK Service 之间提供最小受控适配。业务日志、Session、Memory、checkpoint 和事件分别保持唯一权威。

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

1. 空间/owner/subject 冲突：在下面任务的 RED 场景和集成验收中分别验证。
2. 稳定事件改 payload：在下面任务的 RED 场景和集成验收中分别验证。
3. 删除后异步 Memory 复活：在下面任务的 RED 场景和集成验收中分别验证。
4. Session append 失败仍推进：在下面任务的 RED 场景和集成验收中分别验证。
5. 过期 fence 写入：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P1.0 关闭存储选型门槛，冻结业务契约 | P0 | `docs/superpowers/plans/trpc-native/storage-decision.md`<br>`docs/superpowers/plans/trpc-native/interfaces.md`<br>`docs/superpowers/plans/trpc-native/p0-decision.md` |
| P1.1 身份、错误与跨 Track 接口冻结 | P1.0 | `internal/agent/nativecontract/contracts.go`<br>`internal/agent/nativecontract/scope.go`<br>`internal/agent/nativecontract/scope_test.go` |
| P1.2 新命名空间、表约束与方言迁移 | P1.1 | `internal/application/repository/native_schema.go`<br>`internal/application/repository/native_schema_test.go`<br>`docs/superpowers/plans/trpc-native/schema-manifest.md` |
| P1.3 Session Service 持久化、幂等与授权 facade | P1.2 | `internal/agent/native/session.go`<br>`internal/agent/native/session_test.go`<br>`internal/application/repository/native_session.go`<br>`internal/application/repository/native_session_test.go` |
| P1.4 Memory generation 与删除防复活 | P1.2 | `internal/agent/native/memory.go`<br>`internal/agent/native/memory_test.go`<br>`internal/application/repository/native_memory.go`<br>`internal/application/repository/native_memory_test.go` |
| P1.5 CommitIntent、检查点屏障与可靠事件日志 | P1.3, P1.4 | `internal/application/repository/native_commit.go`<br>`internal/application/repository/native_commit_test.go`<br>`internal/application/repository/native_events.go`<br>`internal/application/repository/native_events_test.go` |

### Task P1.0: 关闭存储选型门槛，冻结业务契约

**Depends on:** P0。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `docs/superpowers/plans/trpc-native/storage-decision.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/interfaces.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/p0-decision.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 消费 P0 能力矩阵及已有 Session/Memory/identity 探针；输出经审查的 SDK root + 子模块版本/校验和、活跃方言、SDK Service 方法清单、事务边界、错误分类。不是产品接线任务。

- [ ] **Step 1：收集门槛证据。** 重新运行已提交的三个独立探针模块；不能用根 go test ./... 代替嵌套模块。SQLite/PostgreSQL 同一 Event.ID 不同 payload 必须拒绝，Memory clear 后旧 generation 写入必须拒绝。原生实现不满足时保留 incompatible，并批准最小存储扩展方案。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 证据检查。**

```bash
git diff --check
```

记录实际结果，未通过项不能改写为完成。

- [ ] **Step 3：实现最小规则与集成。**

```
GOWORK=off go test -race ./internal/agent/nativeprobe -count=20
rg --files tools/trpc-native-p1 | rg '(go.mod|session|memory|identity)'
rg -n 'module |trpc-agent-go' go.mod
GOWORK=off go test ./internal/application/service -run TestExecuteDurableRunPersistsBudgetExhaustionForNotification -count=1 -v
```

逐一读取探针模块 go.mod 并在各模块目录运行 go test ./...、go test -race ./...、go vet ./...；记录命令的 cwd。决策必须选择同一业务数据库内的受控 Session/Memory Service 或经探针满足契约的原生 SQL backend；前者不能重新实现推理。对跨存储采用 CommitIntent + barrier + reconcile，不宣称分布式原子提交。冻结 interfaces.md 完整类型到 P1.1 的 nativecontract；把六间隙各自恢复动作写进决策。预算通知失败单列修复任务，先复现、定位事件与终态提交顺序，修复后重复上述定向测试及 service 回归。存储决策完成不等于 P0 产品运行门打开；仅在原 P0 所列条件逐项有证据后更新其裁定。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。探针原生缺口有明确扩展与验收项；预算通知失败的修复有独立审查；准确记录仍未通过的 PostgreSQL/真实 Provider 条目。未选定 backend 时 P1.2 以后保持 blocked-design。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- docs/superpowers/plans/trpc-native/storage-decision.md docs/superpowers/plans/trpc-native/interfaces.md docs/superpowers/plans/trpc-native/p0-decision.md
git commit -m "docs: freeze native storage composition and gate evidence"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P1.1: 身份、错误与跨 Track 接口冻结

**Depends on:** P1.0。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/nativecontract/contracts.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/nativecontract/scope.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/nativecontract/scope_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 将 interfaces.md §3 的完整 Go 类型作为唯一来源；新增 SessionKey(scope Scope, sessionID string) (session.Key,error)、MemoryKey(scope Scope) (memory.UserKey,error)。tenant 非零，owner/subject/session 非空；认证 facade 仍负责权限，编码函数不构成授权。

- [ ] **Step 1：先写失败测试。** 同 owner/session 字符串跨两个空间；分隔符、Unicode、空身份、API key 与 external user 同名；Session owner 和 Memory subject 不可混用。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/nativecontract -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func TestSessionKeySeparatesTenants(t *testing.T) {
    a, err := SessionKey(Scope{TenantID: 1, SessionOwnerID: "same"}, "会话/a")
    require.NoError(t, err)
    b, err := SessionKey(Scope{TenantID: 2, SessionOwnerID: "same"}, "会话/a")
    require.NoError(t, err)
    require.NotEqual(t, a, b)
    require.Equal(t, "weknora/native-v1/tenant/1", a.AppName)
}
func TestSessionKeyRejectsEmptyOwner(t *testing.T) {
    _, err := SessionKey(Scope{TenantID: 1}, "s")
    require.Error(t, err)
}
```

使用 strconv.FormatUint 编码空间，base64.RawURLEncoding.EncodeToString([]byte(id)) 编码 owner/subject/session，分别加 owner/、subject/、session/ 前缀。创建新的 nativecontract 包，避免各 Track 扩写旧 runtime/contracts.go。逐字段迁移完整草案并加 SDK 接口编译检查；不把 Scope 从请求 body 解码。用现有 principal/context helper 解析 JWT、API key、IM、Embed 身份，恢复请求重新授权。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。空身份报 invalid_request；冲突身份键不同；同样输入可跨进程重建；错误类型与接口草案一致。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/nativecontract/contracts.go internal/agent/nativecontract/scope.go internal/agent/nativecontract/scope_test.go
git commit -m "feat: define scoped native agent contracts"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P1.2: 新命名空间、表约束与方言迁移

**Depends on:** P1.1。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/application/repository/native_schema.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_schema_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/schema-manifest.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 新增 NativeSchemaVersion = 1；迁移文件实际编号由串行迁移 owner 在当前 HEAD 分配并写入 schema-manifest.md，不在多个 Track 预占同一编号。数据表契约见本文件下方 Schema。

- [ ] **Step 1：先写失败测试。** 空库 upgrade；旧库 upgrade；连续执行两次；同空间 request_id 重复；跨空间同 request_id；非法外键；fixture downgrade 只回退本次新空表，含业务数据时拒绝破坏性回退。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test ./internal/application/repository ./internal/database -run 'NativeSchema|Migration' -count=1 -v
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
-- 在隔离数据库验证唯一性；第二条必须触发唯一键冲突。
INSERT INTO native_session_events(tenant_id, session_id, event_id, payload_hash, payload)
VALUES (1, 's', 'e', 'h1', '{}');
INSERT INTO native_session_events(tenant_id, session_id, event_id, payload_hash, payload)
VALUES (1, 's', 'e', 'h2', '{}');
```

在现有 migration runner 中添加实际编号的 up/down 文件；SQLite、PostgreSQL 的 SQL 分开使用现有 runner，不运行 PostgreSQL SQL 到 SQLite。为仍支持的 MySQL 增补等价迁移和测试，否则不能悄悄移除支持。native_schema.go 定义映射及校验，不另做启动时静默建表。唯一约束、范围索引与租约 CAS 列均建立；SQL 参数绑定，游标分页按稳定主键。新增表不复制旧会话/Memory。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。每个活跃方言跑真实数据库 migration/rollback fixture；记录最高编号与最终文件清单；生产回退依赖 P8 的备份流程，不自动 DROP 数据。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/application/repository/native_schema.go internal/application/repository/native_schema_test.go docs/superpowers/plans/trpc-native/schema-manifest.md
git commit -m "feat: add isolated native agent persistence schema"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P1.3: Session Service 持久化、幂等与授权 facade

**Depends on:** P1.2。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/session.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/session_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_session.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_session_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现选定 SDK 的完整 session.Service；新增 AppendStable(ctx context.Context, append nativecontract.SessionAppend) error。同 key+StableEventID+hash 返回成功，不同 hash 返回 ErrConflict。SDK Session CRUD/state/summary 方法不得漏项。 新增 StableAppendAction(existingHash,incomingHash string,exists bool) (string,error)，仅用于唯一键冲突后的结果分类，不替代数据库约束。

- [ ] **Step 1：先写失败测试。** 写入后重新打开数据库读取；重复事件计数不增；相同 ID 改 payload 拒绝；跨空间列表为空；撤销成员资格后读/summary/删除拒绝；append 故障后不得调用下一工具。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native ./internal/application/repository -run 'NativeSession|StableEvent|SessionScope' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func StableAppendAction(existingHash, incomingHash string, exists bool) (string, error) {
    if !exists { return "insert", nil }
    if existingHash == incomingHash { return "reuse", nil }
    return "", nativecontract.Failure{Code: nativecontract.ErrConflict}
}
func TestStableEventConflict(t *testing.T) {
    action, err := StableAppendAction("h1", "h1", true)
    require.NoError(t, err)
    require.Equal(t, "reuse", action)
    _, err = StableAppendAction("h1", "h2", true)
    require.Error(t, err)
}
```

在一个数据库事务内插入稳定事件；唯一键冲突后读取已有 hash 比较，不使用无条件 ON CONFLICT DO NOTHING。payload canonicalization 规则固定为版本化序列化，Event.ID 使用稳定业务 ID 而非恢复时重新生成。读取按范围过滤并复制可变 State/Events，禁止缓存 SDK Session 指针跨请求写共享内存。summary 绑定 through_event_id，不能总结未来或撤权会话。SDK append 返回错误必须传递至 P1.5 barrier。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。单元 hash 检查不能单独验收；真实 Service 重启、并发重复、权限撤销及事务回滚用例全部通过才 passed。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/session.go internal/agent/native/session_test.go internal/application/repository/native_session.go internal/application/repository/native_session_test.go
git commit -m "feat: persist scoped native sessions with stable event identity"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P1.4: Memory generation 与删除防复活

**Depends on:** P1.2。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/native/memory.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/native/memory_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_memory.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_memory_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现选定 SDK memory.Service 和 nativecontract.MemoryGovernance。新增 AcceptMemoryWrite(enabled bool, currentGeneration, jobGeneration int64) bool；作业持久化 MemoryJob.ID/Generation/PolicyRevision/ThroughEventID。

- [ ] **Step 1：先写失败测试。** 旧抽取 job 暂停，Clear 增加 generation 后释放 job；Delete 单条 tombstone 后旧 job 不得重建同条；关闭后召回为空且不继续写；重新开启不能接收旧 generation。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/native ./internal/application/repository -run 'NativeMemory|MemoryGeneration' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func TestMemoryWriteRejectsOldGeneration(t *testing.T) {
    require.False(t, AcceptMemoryWrite(true, 2, 1))
    require.False(t, AcceptMemoryWrite(false, 2, 2))
    require.True(t, AcceptMemoryWrite(true, 2, 2))
}
// 核心 predicate；SQL 必须在事务提交处重复判断，不能仅在入队时调用。
func AcceptMemoryWrite(enabled bool, currentGeneration, jobGeneration int64) bool {
    return enabled && currentGeneration == jobGeneration
}
```

Clear/disable 与 generation 递增同事务；Delete 写条目 tombstone 并使旧 job 失效；worker 执行前及提交时再次验证 generation、权限、enabled。查询也检查 enabled 与 scope。持久化只保存必要抽取输入引用，不能回读旧归档。失败作业有限重试；权限/generation 失效视为丢弃而非重试。删除策略沿用既有保留规则。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。SQL CAS 影响行数为零时显式拒绝；重启后 tombstone/generation 仍有效；SQLite 与 PostgreSQL 均运行延迟抽取竞态。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/native/memory.go internal/agent/native/memory_test.go internal/application/repository/native_memory.go internal/application/repository/native_memory_test.go
git commit -m "feat: prevent native memory resurrection after deletion"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P1.5: CommitIntent、检查点屏障与可靠事件日志

**Depends on:** P1.3, P1.4。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/application/repository/native_commit.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_commit_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_events.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/repository/native_events_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现 nativecontract.CommitCoordinator 与 EventReader。Barrier(ctx,Fence,intentID) 只有已应用 intent 才成功；Commit 同 ID 不同 hash 冲突。事件 seq 为十进制 int64 字符串，按 run 单调分配。

- [ ] **Step 1：先写失败测试。** 在结果→intent、intent→Session、Session→checkpoint、checkpoint→可见事件各间隙注入错误并重启；同 intent 重复 reconcile 不增加 Session/费用/事件；旧 epoch reconcile 拒绝；空洞游标返回 cursor_expired。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/application/repository -run 'NativeCommit|NativeEvent|Barrier|Reconcile' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
-- 唯一约束是幂等的底线，不能以应用层先查再写代替。
CREATE UNIQUE INDEX native_intent_identity ON native_commit_intents(tenant_id, run_id, intent_id);
CREATE UNIQUE INDEX native_event_sequence ON native_business_events(tenant_id, run_id, seq);
CREATE UNIQUE INDEX native_event_identity ON native_business_events(tenant_id, run_id, event_id);
-- stale worker 的更新必须同时匹配 owner、epoch、未过期 lease。
```

先事务保存真实工具结果和 intent，校验 fence；逐条幂等 append Session；写 checkpoint 前确认它引用的工具结果已可靠保存；applied 标记和可发布事件在最终事务提交。跨存储失败保留 intent，由新 fence 接管 reconcile；未通过 barrier 不得下一次 dispatch/成功终态。SSE 仅读已提交可见事件。投影丢失后由 Session 与业务日志重建，不能反向覆写权威历史。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。故障时外部工具下一次调用计数为零；恢复后结果、Session、checkpoint、事件一致；失效 fence 无一条有效写入。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/application/repository/native_commit.go internal/application/repository/native_commit_test.go internal/application/repository/native_events.go internal/application/repository/native_events_test.go
git commit -m "feat: coordinate native commits and replayable events"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。

## Schema：P1.2 必须覆盖的物理数据契约

| 表 | 唯一键/关键列 | 权威与删除边界 |
| --- | --- | --- |
| native_sessions | tenant_id/owner_id/session_id；schema/config version | Session Service 元数据；新命名空间 |
| native_session_events | tenant_id/session_id/event_id；payload_hash/payload/ordinal | Session 历史，所有查询还校验 owner；冲突 hash 拒绝 |
| native_session_state | tenant_id/owner_id/session_id/state_key；revision | SDK 状态与 summary/through_event_id；不存凭据 |
| native_memory_scopes | tenant_id/subject_id；enabled/generation/policy_revision | Memory 开关与并发删除权威 |
| native_memory_entries | tenant_id/subject_id/memory_id；generation/tombstone/payload | Memory Service，旧作业不能复活删除项 |
| native_memory_jobs | tenant_id/subject_id/job_id；generation/through_event_id/status | 抽取任务去重与恢复 |
| native_runs | tenant_id/run_id；owner/session/request/input_hash/status/revision/lease_owner/epoch/lease_until | 准入、调度与 fence；request 另按空间/owner/request 唯一 |
| native_attempts | tenant_id/run_id/attempt_id；logical_call_id/kind/epoch/status | 每次实际模型/工具尝试 |
| native_tool_plans | tenant_id/run_id/call_id；args_hash/tool_identity/policy | 外部调用前先可靠提交 |
| native_tool_results | tenant_id/run_id/call_id；effect/provider_request_id/payload | 已发生外部结果，不由 checkpoint 替代 |
| native_pending_decisions | tenant_id/run_id/pending_id；revision/status/decision_hash/detail | 审批/OAuth/unknown 等待及一次性消费 |
| native_commit_intents | tenant_id/run_id/intent_id；payload_hash/epoch/status/payload | 跨 Session/checkpoint/事件协调 |
| native_checkpoints | tenant_id/run_id/namespace/checkpoint_id；SDK/graph/schema/parent/pending_writes | 执行位置，不证明副作用成功 |
| native_business_events | tenant_id/run_id/seq 和 event_id 唯一；protocol/attempt/payload | 可重放业务事件，事务内分配 seq |
| native_usage | tenant_id/run_id/attempt_id/observation_id/revision；payload_hash/units/status | 商业观察输入；既有商业账本继续结算 |
| native_inputs | tenant_id/run_id/input_id；revision/ordinal/consumed | steer 有序消费 |
| native_config_bindings | source_kind/source_id/source_version；target_id/hash | 可重入配置绑定，不复制执行历史 |

P1.0 若选定原生 backend 的物理表，则在 schema-manifest 映射这些逻辑实体到真实表，并删除重复自建表设计；不得维持两份可写 Session/Memory 权威。列类型、DDL、索引及 SDK 状态序列化须由选定方言探针审核后固定。这是明确设计门槛，不授权猜测 backend 表结构。禁止级联删除旧附件/审计；新表 down 在有数据时拒绝破坏性回退。
