# tRPC Native Agent P9 — 观察与旧执行代码退役 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在观察门槛通过后保留唯一原生执行入口与完整历史查询。

**Architecture:** 监控窗口和阈值发布前固定。按消费者清单逐符号移除旧执行与无消费者适配，保留共享非 Agent 模型、归档和审计。

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

1. 监控缺样本：在下面任务的 RED 场景和集成验收中分别验证。
2. 缩短窗口：在下面任务的 RED 场景和集成验收中分别验证。
3. 误删共享模型：在下面任务的 RED 场景和集成验收中分别验证。
4. 旧 fallback 残留：在下面任务的 RED 场景和集成验收中分别验证。
5. 清代码误清历史数据：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P9.1 固定观察窗口与退役资格 | P8.3 | `docs/superpowers/plans/trpc-native/observation-report.md`<br>`scripts/native-agent-observation.py`<br>`scripts/native-agent-observation_test.py` |
| P9.2 按消费者边界退役旧执行与适配器 | P9.1 | `docs/superpowers/plans/trpc-native/retirement-manifest.md`<br>`internal/application/service/agent_service.go`<br>`internal/application/service/session_agent_qa.go`<br>`internal/application/service/agent_run_graph.go`<br>`internal/container/agent_runtime.go`<br>`internal/agent/trpc/model.go`<br>`internal/agent/trpc/graph.go`<br>`internal/agent/trpc/checkpoint.go` |

### Task P9.1: 固定观察窗口与退役资格

**Depends on:** P8.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `docs/superpowers/plans/trpc-native/observation-report.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `scripts/native-agent-observation.py` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `scripts/native-agent-observation_test.py` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 消费 P7 performance-policy 和 P8 release manifest；窗口时长在发布前固定（建议审议 7 天，必须在 manifest 中批准），不得观察中缩短。

- [ ] **Step 1：先写失败测试。** 任一空间泄漏/重复副作用/重复收费立即 freeze；内部错误/恢复超阈值告警；窗口缺数据不能计通过；重启与 reconnect 仍正常。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
python3 -m unittest discover -s scripts -p 'native-agent-observation_test.py'
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
def retirement_allowed(hours, required_hours, severe_events, missing_samples):
    return hours >= required_hours and severe_events == 0 and missing_samples == 0

import unittest
class ObservationTests(unittest.TestCase):
    def test_missing_samples_block_retirement(self):
        self.assertFalse(retirement_allowed(168, 168, 0, 1))
    def test_severe_event_blocks_retirement(self):
        self.assertFalse(retirement_allowed(168, 168, 1, 0))
    def test_completed_clean_window(self):
        self.assertTrue(retirement_allowed(168, 168, 0, 0))

```

脚本从已有监控/业务记录读取数据，保留窗口、样本量与查询版本；不采集用户消息或秘密到报告。核对异常、waiting_user、预算结算、投影积压、租约接管、Memory 删除与归档访问。未达指标保持旧代码不可执行但不删除，修复后按批准规则重计观察。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。unittest 场景通过；真实观察数据完整、指标通过并独立审查才解锁删除。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- docs/superpowers/plans/trpc-native/observation-report.md scripts/native-agent-observation.py scripts/native-agent-observation_test.py
git commit -m "feat: implement p9.1 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P9.2: 按消费者边界退役旧执行与适配器

**Depends on:** P9.1。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `docs/superpowers/plans/trpc-native/retirement-manifest.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/agent_service.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/session_agent_qa.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/application/service/agent_run_graph.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/container/agent_runtime.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/trpc/model.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/trpc/graph.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/trpc/checkpoint.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 以上为删除审计起点，不是整文件删除授权；retirement-manifest 对每个符号列调用者、替代入口、保留原因/删除判据。共享非 Agent model 与 archive reader 不删除。

- [ ] **Step 1：先写失败测试。** 运行时无旧 fallback；archive/附件权限可用；非 Agent model 消费者回归；native tool/recovery/billing/client 全套不退化。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
git diff --check
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
rg -n 'CreateAgentEngine|ExecuteDurableRun|EngineType|internal/agent/trpc' internal client cli
GOWORK=off go test ./... -count=1 -timeout=20m
pnpm test:shared
pnpm test:web
pnpm test:desktop
pnpm test:embed
```

逐符号删除旧推理循环、只有旧执行消费的 graph/model bridge、切换旗标与无用依赖。有测试/归档/非 Agent 消费者则保留或先迁移，不能按目录批删。go.mod/go.sum 由共享 owner 唯一修改；数据表、历史、附件、审计和必要恢复材料不清除。更新架构/运维与适配器清单，执行 P7.3 同一完整测试矩阵及全分支最终审查。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。只删除 manifest 已审定的代码；全文搜索用于发现消费者，不构成运行验收；最终交付包含残留适配器理由和测试。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- docs/superpowers/plans/trpc-native/retirement-manifest.md internal/application/service/agent_service.go internal/application/service/session_agent_qa.go internal/application/service/agent_run_graph.go internal/container/agent_runtime.go internal/agent/trpc/model.go internal/agent/trpc/graph.go internal/agent/trpc/checkpoint.go
git commit -m "feat: implement p9.2 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。

