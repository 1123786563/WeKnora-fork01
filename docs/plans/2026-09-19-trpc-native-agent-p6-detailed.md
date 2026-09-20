# tRPC Native Agent P6 — 历史只读归档与业务配置迁移 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 保留全部历史查询和业务配置，使新执行数据从零开始。

**Architecture:** 旧数据由明确 archive API 读取，迁移优先保留业务配置身份并建立新绑定。可重入 CLI 提供 dry-run、批次 checkpoint 与校验报告。

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

1. 新旧同 ID：在下面任务的 RED 场景和集成验收中分别验证。
2. 归档撤权：在下面任务的 RED 场景和集成验收中分别验证。
3. 附件失效/误删：在下面任务的 RED 场景和集成验收中分别验证。
4. 迁移中断重跑：在下面任务的 RED 场景和集成验收中分别验证。
5. 报告泄露秘密：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P6.1 归档清单、附件与只读权限 API | P1.5, P5.1 | `internal/application/service/native_archive.go`<br>`internal/application/service/native_archive_test.go`<br>`internal/handler/session/native_archive.go`<br>`internal/handler/session/native_archive_test.go`<br>`internal/router/routes_native_archive.go` |
| P6.2 可重入配置迁移与校验报告 | P6.1 | `cmd/native-agent-migrate/main.go`<br>`internal/application/service/native_migration.go`<br>`internal/application/service/native_migration_test.go` |
| P6.3 归档/迁移集成与恢复核对 | P6.2, P5.4 | `tests/native-agent/archive_migration_test.go`<br>`docs/superpowers/plans/trpc-native/archive-manifest.md` |

### Task P6.1: 归档清单、附件与只读权限 API

**Depends on:** P1.5, P5.1。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/application/service/native_archive.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_archive_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/handler/session/native_archive.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/handler/session/native_archive_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/router/routes_native_archive.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 实现 nativecontract.ArchiveReader；GET /api/v1/agent-archive/sessions、/records/:id、/artifacts/:id，任何 resume/continue/write 返回 archive_read_only。

- [ ] **Step 1：先写失败测试。** 旧/new 同 ID 不串读；撤销成员资格后列表/详情/附件均拒绝；签名 URL 过期；旧 Memory/checkpoint 不被新 Runner 读取；归档不触发附件垃圾回收。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/application/service ./internal/handler/session -run NativeArchive -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
func ArchiveMutationError() nativecontract.Failure {
    return nativecontract.Failure{Code: nativecontract.ErrArchiveReadOnly, Message: "历史记录仅供查询"}
}
func TestArchiveRejectsMutation(t *testing.T) {
    require.Equal(t, nativecontract.ErrArchiveReadOnly, ArchiveMutationError().Code)
}
```

旧表保留为只读命名空间，由新的明确 archive 路由读取；在维护窗口封旧写入，不能提前冻结现有生产流量。归档索引记录 sessions/messages/tools/decisions/audit/memory/checkpoint/artifact 关联；附件仍通过原有权限与保留逻辑。禁止旧 engine 默认值决定读取新旧数据。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。真实旧数据副本核对数量和附件下载；新会话上下文为空，旧会话不可恢复。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/application/service/native_archive.go internal/application/service/native_archive_test.go internal/handler/session/native_archive.go internal/handler/session/native_archive_test.go internal/router/routes_native_archive.go
git commit -m "feat: implement p6.1 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P6.2: 可重入配置迁移与校验报告

**Depends on:** P6.1。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `cmd/native-agent-migrate/main.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_migration.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/native_migration_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** CLI 子命令 plan/apply/verify；参数 --manifest、--database-ref、--dry-run；默认 dry-run，apply 必须匹配 manifest source fingerprint。报告字段 counts/relations/permission_errors/credential_resolution_errors/checksum/checkpoint。

- [ ] **Step 1：先写失败测试。** 中途退出重新运行；同 source/config version 不重复；配置冲突显式失败；密钥不输出；未指定隔离目标拒绝测试迁移；配置不丢模型/Agent/persona/Skills/MCP/知识/商业账户。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test ./cmd/native-agent-migrate ./internal/application/service -run NativeMigration -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
type MigrationIdentity struct { SourceKind, SourceID, SourceVersion string }
func MigrationKey(v MigrationIdentity) string {
    b, _ := json.Marshal(v)
    h := sha256.Sum256(b)
    return hex.EncodeToString(h[:])
}
func TestMigrationVersionSeparatesKeys(t *testing.T) {
    require.NotEqual(t, MigrationKey(MigrationIdentity{"agent","a","1"}),
        MigrationKey(MigrationIdentity{"agent","a","2"}))
}
```

优先保留原业务配置表与 ID，建立 native binding，而非重复复制商业/知识业务。必须转存时 mapping 表保存 source/version/target/hash。批次事务与 checkpoint 同提交；验证权限关系、外键与凭据引用可解析性，报告仅布尔状态/脱敏 ID。dry-run 无任何持久化写入；apply 输出每批 checkpoint。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。在隔离副本做两次 apply、一次中断恢复；最终数量/关联/hash 相同；旧执行数据绝不导入新 Session/Memory。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- cmd/native-agent-migrate/main.go internal/application/service/native_migration.go internal/application/service/native_migration_test.go
git commit -m "feat: implement p6.2 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P6.3: 归档/迁移集成与恢复核对

**Depends on:** P6.2, P5.4。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `tests/native-agent/archive_migration_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/archive-manifest.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 消费迁移 CLI、ArchiveReader、Web 归档入口；输出 release 使用的 manifest 格式及副本恢复证据。

- [ ] **Step 1：先写失败测试。** 迁移前后配置与附件 hash、权限撤销、只读 API、重复迁移；恢复到备份后归档仍可读；商账与外部动作不因恢复重做。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test ./tests/native-agent -run ArchiveMigration -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
GOWORK=off go test ./tests/native-agent -run ArchiveMigration -count=1 -v
# 必须使用测试生成的隔离 database-ref，记录 manifest 与 verify 报告。
# 断言：before.config_count == after.config_count
# 断言：new_session_count == 0 && new_memory_count == 0
# 断言：missing_artifacts == 0 && permission_errors == 0
```

从 P0 每一旧数据类别建立清单，验证保存与查询路径。故障注入在批次提交前后，恢复必须读 checkpoint 而非人工删 mapping 重跑。把不可解析凭据、失效附件分别列为阻塞，不默默跳过。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。此为隔离恢复证据；生产备份/维护窗口执行属于 P8。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- tests/native-agent/archive_migration_test.go docs/superpowers/plans/trpc-native/archive-manifest.md
git commit -m "feat: implement p6.3 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。

