# tRPC Native Agent P7 — 故障、全功能、性能与集成验收 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提供逐功能、逐环境可核对的完整验收证据。

**Architecture:** 进程级故障矩阵检查真实持久化与外部效果，全端操作单独记录。CI 必需证据缺失即失败，单元/mock/build 与真实验收分层。

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

1. skip 伪绿：在下面任务的 RED 场景和集成验收中分别验证。
2. 旧 commit 证据：在下面任务的 RED 场景和集成验收中分别验证。
3. 未知副作用重复：在下面任务的 RED 场景和集成验收中分别验证。
4. 缺失消费者：在下面任务的 RED 场景和集成验收中分别验证。
5. 性能阈值事后放宽：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P7.1 六间隙 SIGKILL 与双 Worker 矩阵 | P3.4, P4.6, P5.7, P5.6, P5.5, P6.3 | `internal/agent/recoverytest/native_matrix_test.go`<br>`internal/agent/recoverytest/native_worker_test.go`<br>`internal/agent/recoverytest/native_external_test.go` |
| P7.2 全功能/全客户端与性能验收 | P7.1 | `tests/native-agent/acceptance_manifest.json`<br>`tests/native-agent/acceptance_test.go`<br>`docs/superpowers/plans/trpc-native/acceptance-report.md`<br>`docs/superpowers/plans/trpc-native/performance-policy.json` |
| P7.3 完整测试、CI 门禁与最终集成审查 | P7.2 | `scripts/check-native-agent-evidence.py`<br>`scripts/check-native-agent-evidence_test.py`<br>`.github/workflows/native-agent-acceptance.yml`<br>`docs/superpowers/plans/trpc-native/integration-report.md` |

### Task P7.1: 六间隙 SIGKILL 与双 Worker 矩阵

**Depends on:** P3.4, P4.6, P5.7, P5.6, P5.5, P6.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `internal/agent/recoverytest/native_matrix_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/recoverytest/native_worker_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `internal/agent/recoverytest/native_external_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 复用现有 recoverytest 子进程 harness；新增故障点 model_plan/approval_call/external_result/result_checkpoint/checkpoint_session/event_send；每一行指定数据库、恢复动作与外部调用计数。

- [ ] **Step 1：先写失败测试。** SQLite/PostgreSQL × 六点 × 重启；旧 worker 复活；非幂等未知效果 hold；可查询工具不重复；批准撤权；预算根并发；replay 不重复收费。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test -race ./internal/agent/recoverytest -run 'NativeTwoWorker|NativeDecision' -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
GOWORK=off go test -json ./internal/agent/recoverytest -run Native -count=1 -timeout=20m > /tmp/native-recovery.json
python3 - <<'PY'
import json
rows=[json.loads(x) for x in open('/tmp/native-recovery.json') if x.strip()]
bad=[r for r in rows if r.get('Action') in ('skip','fail')]
assert not bad, [(r.get('Test'),r['Action']) for r in bad]
assert any(r.get('Action')=='pass' and r.get('Test') for r in rows), 'no tests ran'
PY
```

进程 kill 前由 durable marker/barrier 通知父进程，禁止用 sleep 猜测窗口。重启新进程读取同数据库，查询外部测试服务的 operation ID、次数与结果；断言 Session/Run/checkpoint/events/usage 一致。数据库不可用或 provider 未配置时记录 blocked-env；发布门禁不得把 skip 视为成功。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。每个场景具名、有执行记录，零 skip/未解释失败；以 stable feature ID 关联证据。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- internal/agent/recoverytest/native_matrix_test.go internal/agent/recoverytest/native_worker_test.go internal/agent/recoverytest/native_external_test.go
git commit -m "feat: implement p7.1 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P7.2: 全功能/全客户端与性能验收

**Depends on:** P7.1。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `tests/native-agent/acceptance_manifest.json` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `tests/native-agent/acceptance_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/acceptance-report.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/performance-policy.json` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** acceptance manifest 每行 id/required/layer/environment/command/status/evidence/commit；status=pending/pass/fail/blocked-env；P0 features.tsv 每行至少一对应验收。

- [ ] **Step 1：先写失败测试。** 真实 provider 配置全矩阵；工具族、权限、Memory/压缩、归档、商业；Web/Desktop/Flutter/Expo/CLI/Embed/小程序/DSH；性能基线与新系统同负载。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test ./tests/native-agent -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
python3 - <<'PY'
import json
from pathlib import Path
rows=json.loads(Path('tests/native-agent/acceptance_manifest.json').read_text())
assert rows
for r in rows:
    if r['required']:
        assert r['status']=='pass', r['id']
        assert Path(r['evidence']).is_file(), r['id']
PY
```

先固定性能 policy 再压测：相同硬件、数据、并发、provider 与 warmup；记录 TTFT p50/p95、完成耗时、事件吞吐、恢复耗时、DB 连接/锁等待。发布阈值由基线+业务 SLO 审定并保存，不能在失败后放宽。建议初始审议值为 TTFT/完成 p95 不高于基线 1.2 倍、内部错误率 <1%、恢复 p95≤2×leaseTTL+30s；这些建议不自动成为已批准 SLO。安全项重复副作用/跨空间读取/重复收费容忍度始终为零。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。每端真实操作与 Provider 证据分层；性能 policy 未审定或缺 required 项阻止 P8。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- tests/native-agent/acceptance_manifest.json tests/native-agent/acceptance_test.go docs/superpowers/plans/trpc-native/acceptance-report.md docs/superpowers/plans/trpc-native/performance-policy.json
git commit -m "feat: implement p7.2 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P7.3: 完整测试、CI 门禁与最终集成审查

**Depends on:** P7.2。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `scripts/check-native-agent-evidence.py` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `scripts/check-native-agent-evidence_test.py` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `.github/workflows/native-agent-acceptance.yml` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/integration-report.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 新增证据校验脚本退出码：0 全必需项通过，1 缺失/失败/skip/commit 不匹配。CI 调用完整矩阵，不能依赖只覆盖旧路径的 agent-recovery.yml 绿色。

- [ ] **Step 1：先写失败测试。** 缺一个 required feature、伪造 evidence 路径、旧 commit、skip、空清单都失败；完整清单通过；主仓库与全部嵌套模块均执行。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
python3 -m unittest discover -s scripts -p 'check-native-agent-evidence_test.py'
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
GOWORK=off go test ./... -count=1 -timeout=20m
GOWORK=off go test -race ./internal/agent/... ./internal/application/repository ./internal/application/service -count=1
pnpm test:shared
pnpm typecheck:shared
pnpm test:web
pnpm typecheck:web
pnpm build:web
pnpm test:desktop
pnpm typecheck:desktop
pnpm test:embed
pnpm typecheck:embed
pnpm build:desktop-renderer
pnpm build:embed
```

再执行总索引列出的 Flutter/Expo/小程序/Go client/DSH/独立探针模块完整测试；记录每条 exit code 和日志，不用最后一个命令覆盖前面失败。基线已知失败与新失败都列入报告，未修复时不得宣称 full green。最终 reviewer 审查 spec coverage、原生化、六间隙、身份和财务，不负责代替运行测试。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。最终 integrated SHA 全套通过+最终审查通过才能 P8；集成后改代码需重跑受影响及发布必需门禁。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- scripts/check-native-agent-evidence.py scripts/check-native-agent-evidence_test.py .github/workflows/native-agent-acceptance.yml docs/superpowers/plans/trpc-native/integration-report.md
git commit -m "feat: implement p7.3 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。

