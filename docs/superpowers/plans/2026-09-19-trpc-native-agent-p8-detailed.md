# tRPC Native Agent P8 — 切换演练、回退与具体发布 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付演练过的一次切换流程及经批准的生产执行记录。

**Architecture:** 先在生产等价隔离环境执行全流程与两类回退，再准备固定版本发布包。生产批准只在结果和具体操作可审阅后请求。

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

1. 排空期间丢结果：在下面任务的 RED 场景和集成验收中分别验证。
2. 未知外部结果放行：在下面任务的 RED 场景和集成验收中分别验证。
3. 未验证备份：在下面任务的 RED 场景和集成验收中分别验证。
4. 旧端误建任务：在下面任务的 RED 场景和集成验收中分别验证。
5. 开放后盲恢复数据库：在下面任务的 RED 场景和集成验收中分别验证。

---

## 文件职责和阅读顺序

本文件列出的新增 native 文件是目标设计，不声称已经存在。旧入口只作迁移参考，所有新增测试由本阶段实施时创建。精确的跨阶段业务类型来自 interfaces.md §3，P1.1 冻结为 `nativecontract`；使用前必须读取完整定义。代码块给出最小规则、SQL 或验收命令，不构成已经编译的产品实现。存储构造器/SDK 装配必须使用 P1.0 的版本与方法证据；未获证据时保持 blocked-design，不能自行猜测 API。

| 任务 | 前置 | 文件职责 |
| --- | --- | --- |
| P8.1 隔离生产等价环境全流程演练 | P7.3 | `docs/runbooks/native-agent-cutover.md`<br>`scripts/native-agent-cutover-check.sh`<br>`docs/superpowers/plans/trpc-native/rehearsal-report.md` |
| P8.2 开放前/开放后回退演练 | P8.1 | `docs/runbooks/native-agent-rollback.md`<br>`tests/native-agent/rollback_test.go`<br>`docs/superpowers/plans/trpc-native/rollback-report.md` |
| P8.3 发布包、生产批准与一次切换 | P8.2 | `docs/superpowers/plans/trpc-native/release-manifest.json`<br>`docs/superpowers/plans/trpc-native/release-record.md` |

### Task P8.1: 隔离生产等价环境全流程演练

**Depends on:** P7.3。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `docs/runbooks/native-agent-cutover.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `scripts/native-agent-cutover-check.sh` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/rehearsal-report.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 输入固定 release manifest（代码 SHA、镜像 digest、SDK/backend、migration、所有客户端版本、backup ID、环境、窗口、阈值）；输出逐步操作记录/耗时/校验/退出条件。

- [ ] **Step 1：先写失败测试。** 停止准入仍允许结果写入；排空超时转取消；unknown effect 未解决阻止继续；备份可恢复；旧协议请求不会建新旧 Run。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
bash -n scripts/native-agent-cutover-check.sh
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
# 演练 gate：必须显式提供环境与批准的 manifest 路径。
: "${NATIVE_RELEASE_MANIFEST:?release manifest required}"
: "${NATIVE_TARGET_ENV:?target environment required}"
test "$NATIVE_TARGET_ENV" = "rehearsal"
python3 scripts/check-native-agent-evidence.py --manifest "$NATIVE_RELEASE_MANIFEST"
```

手册按顺序写实际部署环境命令：冻结新准入→记录在途→排空/取消→外部未知核对→备份并验证恢复→归档/配置 apply+verify→部署匹配后端及各端→权限/模型/工具审批/费用/流式/归档冒烟→开放准入。脚本默认检查模式，不偷偷部署。环境命令只有选定部署目标后才固化到 release manifest，未经实际演练的命令不得当作已验证。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。在隔离环境完整跑通，不只是 shell syntax；保存 backup restore 校验与外部操作账。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- docs/runbooks/native-agent-cutover.md scripts/native-agent-cutover-check.sh docs/superpowers/plans/trpc-native/rehearsal-report.md
git commit -m "feat: implement p8.1 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P8.2: 开放前/开放后回退演练

**Depends on:** P8.1。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `docs/runbooks/native-agent-rollback.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `tests/native-agent/rollback_test.go` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/rollback-report.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 两条独立回退 runbook；输入 backup ID、release manifest、Run/tool/usage 外部核对清单。

- [ ] **Step 1：先写失败测试。** 开放前恢复旧版本与备份；开放后先 freeze，再核对新动作/收费；不明结果阻止盲恢复；恢复库不能触发工具重放或退款。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 RED。**

```bash
GOWORK=off go test ./tests/native-agent -run Rollback -count=1
```

先加入测试再运行，预期因目标行为未实现而 FAIL；缺工具、依赖下载或数据库不可用只记 blocked-env，不能当行为 RED。不得接受 no tests to run；确认输出含本任务新增测试名。

- [ ] **Step 3：实现最小规则与集成。**

```
GOWORK=off go test ./tests/native-agent -run 'RollbackBeforeAdmission|RollbackAfterAdmission' -count=1 -v
# 实际演练验收：
# before-open: restored_hash == backup_hash, admitted_new_runs == 0
# after-open: every new run has effect/accounting reconciliation,
# duplicate_external_actions == 0, duplicate_charges == 0
```

开放后优先评估前向修复；确需恢复时保留新 journal/事件/账务副本与外部 operation IDs，明确已执行动作如何显示及核对。不能将“回退数据库”写成撤销付款/发送/外部文件修改。任何无法核对项目进入人工处置并保持准入冻结。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。两条演练独立证据；回退达到目标服务恢复但不声称消除外部效果。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- docs/runbooks/native-agent-rollback.md tests/native-agent/rollback_test.go docs/superpowers/plans/trpc-native/rollback-report.md
git commit -m "feat: implement p8.2 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

### Task P8.3: 发布包、生产批准与一次切换

**Depends on:** P8.2。依赖表示“测试并审查通过且已集成”，不表示仅代码写完。

**Files:**
- Create/Modify: `docs/superpowers/plans/trpc-native/release-manifest.json` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。
- Create/Modify: `docs/superpowers/plans/trpc-native/release-record.md` — 只允许本任务 owner 修改；执行前核实存在性，现有文件保留其他职责。

**Interfaces:** 生产动作输入必须包含用户批准的具体环境/版本/时段/影响/回退方案；既往规格确认不等于生产部署批准。

- [ ] **Step 1：收集门槛证据。** required gate 缺失、客户端未就绪、unknown effect、备份验证失败时禁止放行；冒烟失败保持冻结。 将每个场景写为具名测试，逐一断言错误码、持久化终态、外部调用次数与允许的数据范围；测试文件使用 Files 中对应的 test 文件，不依赖执行顺序。

- [ ] **Step 2：执行 证据检查。**

```bash
git diff --check
```

记录实际结果，未通过项不能改写为完成。

- [ ] **Step 3：实现最小规则与集成。**

```
python3 scripts/check-native-agent-evidence.py --manifest docs/superpowers/plans/trpc-native/release-manifest.json
# 退出 0 只代表资料门禁；不代表生产动作已批准或已执行。
```

先完成并展示可审阅 release 包，最后请求具体生产动作批准。批准后严格依 P8.1 实测步骤执行，逐步记录时刻/操作者/结果；失败走 P8.2 对应阶段，不临时放宽阈值。切换只一次，新任务全部进入 native；旧接口给升级提示。

- [ ] **Step 4：执行 GREEN 与边界验收。** 重跑 Step 2 命令，要求所有新增测试 PASS；再执行对应包完整回归。生产 smoke 与开放事件有证据；未获生产批准时状态 awaiting-release-approval，不阻塞已经授权的文档/演练工作。 对数据库任务逐活跃方言执行，对外部能力保存真实授权测试资源的状态/调用身份。没有所需环境保持 blocked-env。

- [ ] **Step 5：审查、修复与限定提交。** 审查者核对本任务 Interfaces、所有 RED 场景、规格对应功能行及 Review Focus；发现问题返回同一 owner 修复再复核。共享文件需持有总索引中的串行修改权。

```bash
git diff --check
git add -- docs/superpowers/plans/trpc-native/release-manifest.json docs/superpowers/plans/trpc-native/release-record.md
git commit -m "feat: implement p8.3 native agent boundary"
```

实际新增迁移/接线文件必须先加入本任务文件 manifest，再用精确路径 stage；禁止 `git add .`。将 commit、命令、cwd、exit code、日志、环境、审查与未解决项记入 `trpc-native/p1-p9-progress.tsv`，由集成人串行汇总。

## 阶段完成门槛

- 所有任务完成 scoped tests 和独立需求/质量审查；阻塞项原样记录。
- 合入集成分支后跑本阶段与上游消费方回归，不能把 worktree 单独 PASS 当作集成 PASS。
- 功能清单每条有新入口、具体测试与证据；没有删减原功能来换取完成。
- 当前文件是计划交付，所有实施任务初始 pending/blocked-design；没有声称本轮执行了这些测试或实现。

