# Craft 产品实施进度（协调器同步视图）

> 权威调度台账：`.superpowers/sdd/2026-09-10-craft-product-implementation/dag-state.json`（集成工作区）。
> 执行提示词：[2026-09-12-craft-code-agent-prompt.md](2026-09-12-craft-code-agent-prompt.md)；DAG：[2026-09-12-craft-execution-dag.json](2026-09-12-craft-execution-dag.json)。
> 集成分支：`codex/craft-integration`，BASE `83e2ef5c`。主工作区 `main@d0758b6c` 的 dirty 文档保留不动。

## G0 恢复现场结论（2026-09-13）

- 主工作区 `main@d0758b6c`：仅并行文档改动（mobile-workbench 计划），保留。
- `codex/craft-product@a4468c9c`：R01 候选实现（client.go/client_test.go/protocol.go），缺 lock 文件与真实二进制 fixture；作为候选检查补齐，不当已完成。
- `.worktrees/craft-r01`（`codex/craft-r01@83e2ef5c`）：中断的测试先行现场（更严格 protocol.go + 扩展测试 + 空 testdata），按"恢复现场"续做。
- 迁移编号修正：计划旧号 PG 000094/SQLite 000015 已过时；当前最大 PG 000120/SQLite 000040，R02 分配 **000121/000041**。
- 锁定 OpenCode 二进制：`~/.opencode/bin/opencode` = 1.18.4（SHA256 9449af91…8de3f98，darwin/arm64）；homebrew 的是 1.18.7，不得混用。

## 门禁状态

| 门禁 | 状态 | 证据（@83e2ef5c，2026-09-13） |
|---|---|---|
| G0 | passed | 本节；worktree/分支/dirty 清单已核验 |
| G1 主 Runtime | passed | `go test ./internal/agent/runtime/...` ok；`./internal/agent/trpc/...` ok；账目 13 轮浏览器/二进制崩溃证据为该基线祖先 |
| G2 React 基础 | unverified | apps/web @main 仅为迁移 seam（知识库/文档/连接器页）；完整对话/附件工作台在 react-multiclient 专项（未合入）。只阻塞 W04/W05（远端），不影响 R 链 |
| G3 恢复验收 | passed | SIGKILL 矩阵 SQLite 9/9 + PostgreSQL 8/8（本地 paradedb，DSN 经 env 注入）+ 双工作者竞争/epoch 拒绝双方言 PASS，全部 @83e2ef5c 真实 provider 二进制；遗留 `TestCrashAfterToolResult` 为已废弃变体（激活需 counter URL，验收文档注明 superseded） |
| G4 商业契约 | passed | `go test ./internal/commercial/... ./internal/application/service/commercial/...` 均 ok；saas-billing 已于 700ef410 合入并 PG 验证 |

## 任务状态（27 项）

| ID | 状态 | 工作区/分支 | BASE | 说明 |
|---|---|---|---|---|
| R01 | **done** | .worktrees/craft-r01 / codex/craft-r01 | 83e2ef5c | HEAD 542105ac 审查 PASS_WITH_NITS → 合入 21bf52fc；11/11 测试含 2 个 live（真实 1.18.4 二进制+本地 mock provider 两轮）；nit 留档：48 位 ID 回绕（R04+ 注意）、container_digest 待镜像 |
| R02 | **done** | .worktrees/craft-r02 / codex/craft-r02 | 83e2ef5c | HEAD 80b9fcbf 审查 PASS_WITH_NITS → 合入 da6d169d；迁移 000121/000041；SQLite+真实 PG 双绿；外键偏离（写时校验+sessions FK）经审查者独立实证 |
| R03 | **done** | .worktrees/craft-r03 / codex/craft-r03 | 21bf52fc | HEAD b9f28885 审查 PASS_WITH_NITS → 合入 86a651bb；真实镜像构建+复验（官方 release 摘要独立实测吻合）；nit 留档：ActiveRuns 生产接线无下游归属（协调器登记 W03/R05 装配）、lock 回填留发布链 |
| R04 | **done** | .worktrees/craft-r04 / codex/craft-r04 | 86a651bb | HEAD 8a387eb7 审查 PASS_WITH_NITS（23/23 规格）→ 合入 785ceeb8；41 测试含 live；exactly-once POST、15 案矩阵；契约留档：unknown 绝不落库、ErrUnknown 唯一 pending 信号 |
| R05 | **done** | .worktrees/craft-r05 / codex/craft-r05 | 785ceeb8 | HEAD b523c8f1 审查 PASS_WITH_NITS（16/16）→ 合入 9c02a7c4；ActiveRuns 真实接线闭合 R03 nit-1；fail-closed 生产执行器（W03 承接 dial）；契约留档：unknown 绝不落库、裸 idle 窄窗口 nit 转 W 链 |
| R06 | **done** | .worktrees/craft-r06 / codex/craft-r06 | 9c02a7c4 | HEAD fa51f671 审查 PASS_WITH_NITS → 合入 1b44296f；停止时序/决定时序 fail-closed 全有真库证据；通道分离设计经独立验证（C02 承接 outbox） |
| W01 | **done** | .worktrees/craft-w01 / codex/craft-w01 | 9c02a7c4 | HEAD f594e2b9 审查 PASS_WITH_NITS（21/21）→ 合入 47f1f4b7；真实 PG 迁移链验证；W02 提示：不能重 Publish 补 preview checks、.env 变体凭据加固 |
| O01 | **done** | .worktrees/craft-o01 / codex/craft-o01 | 9c02a7c4 | HEAD 0e4ecb37 审查 PASS_WITH_NITS（19/19）→ 合入 9608c6ac；真实 PG 双方言验证；O02 提示：网关单调序列、唯一记账入口、并发 Append 冲突信号收紧 |
| R07 | **done** | .worktrees/craft-r07 / codex/craft-r07 | 1b44296f | HEAD 6f1fb157 审查 PASS_WITH_NITS（18/18，审查者独立复跑 live PASS）→ 合入 b1e8950b；运行时链 R01–R07 全部完成 |
| W02 | **done** | .worktrees/craft-w02 / codex/craft-w02 | 1b44296f | 经 2 轮修复（conf 副本→真实 nginx→继承规则修复，捕获法实测剥凭据）→ 合入 e3835d82；审查 FAIL→定向复审→协调器复核闭环 |
| O02 | **done** | .worktrees/craft-o02 / codex/craft-o02 | 9608c6ac | HEAD a6709d34 审查 PASS_WITH_NITS（25/25）→ 合入 1c02390b；G4 缺口扩大记录（另 4 张商业表仅存 AutoMigrate），收费上线前 G4 须补（O05 门禁） |
| W03 | **done** | .worktrees/craft-w03 / codex/craft-w03 | 1c02390b | HEAD a6b30a09 审查 PASS_WITH_NITS → 合入 a8bfb241；API 表逐条过（幂等/Submit/gate/限流/身份注入拒绝）；fail-closed 边界确认；G2 复验通过（apps/web 构建+包导出） |
| W04 | **done** | .worktrees/craft-w04 / codex/craft-w04 | a8bfb241 | HEAD dcccb683 审查 PASS_WITH_NITS（14/14，与后端真实 SSE 协议交叉验证）→ 合入 8412e9a0；@weknora/core 由 W04 自建；C03 承接退避与快照合并 |
| W05 | **done** | .worktrees/craft-w05 / codex/craft-w05 | 8412e9a0 | 首轮 FAIL（turns 归档死代码）→修复 37d4433c→定向复审 PASS → 合入 befb5e48；发现并修复 .gitignore 未锚定 web/ 吞 apps/web 新文件问题（协调器锚定+产物 ignore，d9054bd0） |
| W06 | **done** | .worktrees/craft-w06 / codex/craft-w06 | d9054bd0 | HEAD 9e6481bf 审查 PASS_WITH_NITS（11/11，审查者独立复跑 6/6+DB 哈希核对）→ 合入 f931f02b。**B 阶段全部完成**（15/27）。留档：R02↔R04 messageID 上游立项、Scan 叙事修正、harness 进程组清理、decide 路由/preview 回写归 C 阶段决策 |
| C01 | **done** | .worktrees/craft-c01 / codex/craft-c01 | f931f02b | HEAD 072201a7 审查 PASS_WITH_NITS（14/14，真实 ACL 入口核验）→ 合入 3e05038b；Build 端点+挂载协调器裁定归 C06 生产装配（与 C05 协调文件） |
| C03 | **done** | .worktrees/craft-c03 / codex/craft-c03 | 3e05038b | HEAD 2e721e2f 审查 PASS_WITH_NITS（19/19+W04 三 nit 闭合）→ 合入 0302b8dd；128/128 复验；W05 待办：statusLabel 补 idle 措辞 |
| C02 | **done** | .worktrees/craft-c02 / codex/craft-c02 | 256fbb78 | 审查 PASS_WITH_NITS（18/18+SIGKILL 矩阵）→ 合入 2b9f7b30+冲突解 88065cfe（package.json 与 C03 脚本并集）；迁移 000127/000047；交互面板一行挂载留 C06 |
| D02 | **done** | .worktrees/craft-d02 / codex/craft-d02 | ae664bcb | HEAD 0f7d022a 审查 PASS_WITH_NITS（10/10，LO 重算 fixture 独立复核）→ 合入 de94676e（views exports 并集冲突解）；5 条开闸前接线清单留 D01/集成 |
| C06 | **done** | .worktrees/craft-c06 / codex/craft-c06 | 88065cfe | FAIL→修复 25f97360→复审 PASS（真实链路证据消除两阻断）→ 合入 da181e0b；可启动基线恢复；knowledge_scope 持久化/knowledge.built/CraftKnowledgeTool 消费方留后续 |
| O03 | implementing | .worktrees/craft-o03 / codex/craft-o03 | de94676e | 已派发（休眠/删除分策+Sweep CAS+worker 竞争测试） |
| D03 | **done** | .worktrees/craft-d03 / codex/craft-d03 | de94676e | HEAD ec03cf24 审查 PASS_WITH_NITS（14/14，钉扎镜像独立复现）→ 合入 92f23a10 |
| O03 | **done** | .worktrees/craft-o03 / codex/craft-o03 | de94676e | HEAD 8cd5dbe9 审查 PASS_WITH_NITS（13/13）→ 合入 4501f819+schema 常量 48（a543e439）；5 项接线清单+stranded-deleting 修复留 O04/集成 |
| D01 | review | .worktrees/craft-d01 / codex/craft-d01 | 92f23a10 | HEAD e0fc78e0：python-docx 镜像链+OOXML 校验+浏览器 4/4；五项类型接线全落地（EntryPath/Previewable/manifest 服务端门禁/NIT-1 词边界/预览归一/kind-aware specs）；审查排队中 |
| O04 | **done** | .worktrees/craft-o04 / codex/craft-o04 | a543e439 | 主体 23019793+O03 接线 0697597e 审查 PASS_WITH_NITS（11/11+接线 5/5）→ 合入 5c9f17aa；O05 提示：O02 网关未接线（账本空）/驻留无记录者//metrics 未暴露 |
| D01 | **done** | .worktrees/craft-d01 / codex/craft-d01 | 92f23a10 | HEAD e0fc78e0 审查 PASS_WITH_NITS（9/9+五接线验证，fixture 三重核验）→ 合入 6fbc0706。**26/27 完成** |
| O05 | **done** | .worktrees/craft-o05 / codex/craft-o05 | 6fbc0706 | HEAD 025687bc 审查 PASS_WITH_NITS（十步全过，门禁防篡改验证）→ 合入 3a4895f4。**27/27 全部完成**；全分支终审进行中 |

## 最终交付状态（2026-09-13）

- **27/27 任务 done**：每项独立实现（RED→GREEN）→ 独立规格+质量审查（3 次 FAIL 打回修复后复审通过：W02×2、C06×1、含 D01/C05/C02 等冲突协调解决）→ 串行合入 `codex/craft-integration`（HEAD 3a4895f4，基线 83e2ef5c，108 提交，262 文件 +53329 行）。
- **门禁**：G0/G1/G3/G4 passed（G3 双方言 SIGKILL 矩阵真实复跑）；G2 passed-for-W04-scope（apps/web 构建+包导出核验；chat 工作台在 react-multiclient 专项未合入，见 limitations）。
- **发布证据**：`docs/testing/craft/release-evidence.json` + 门禁脚本 `scripts/check-craft-release.py`（篡改反向验证全拒）；十场景故障演练证据 `docs/testing/craft/o05/`；付费发布 blocked（Billing=false，G4 商业库缺口如实入 limitations）。
- **外部缺口留档**（不阻塞非收费交付）：G4 商业表迁移缺口（owner 列+4 表 AutoMigrate-only）；O02 网关未接线（用量账本空、端点诚实返 0）；/metrics 未暴露；驻留事件无生产记录者。
- **回退**：craft.enabled 默认 false；发布手册 `docs/operations/craft-release.md`（四阶段开闸、三红线回退、DB 升级顺序、停止准入命令）。部署实施是后续明确动作，本次交付可审阅发布包。
- **全分支终审**：PASS_WITH_FOLLOWUPS（8/8 清单过：双方言迁移 17 表对齐、capabilities 与代码事实一致、builtin 回归、前端构建、真实全栈 6/6、资源保留四态保护、回退三层 fail-closed、跨任务一致性 27 项+26 依赖 SHA 全为祖先）。跟进项：F1 发布切割时重生成 evidence head；F2 四项外部缺口跟踪；F3 /tmp 清理。报告：final-branch-review.md。
| C04 | **done** | .worktrees/craft-c04 / codex/craft-c04 | f931f02b | HEAD 7d0aebc4 审查 PASS_WITH_NITS（19/19+矩阵复验）→ 合入 256fbb78；容器装配裁定归 C05（含 nit-2/3） |
| C05 | **done** | .worktrees/craft-c05 / codex/craft-c05 | 256fbb78 | HEAD faf500e9 审查 PASS_WITH_NITS（21/21，安全闭环无伪造路径）→ 合入 ae664bcb；迁移 000126/000046；C04 装配落实；C06 接线提示与 6 卫生 nit 留档 |
| D02 | implementing | .worktrees/craft-d02 / codex/craft-d02 | ae664bcb | 已派发（openpyxl+LibreOffice 重算+XLSX 导出+浏览器）；C06/D01/D03/O03/O04/O05 等待依赖或席位 |
| C02 | ready（排队） | — | — | 等实现席位（C03 完成后即派） |

## 调度纪要

- 并行组 #1（R01+R02）：两项均审查 PASS_WITH_NITS 后串行合入（da6d169d、21bf52fc），合并后受影响包全绿（含 live 与真实 PG）。
- R03 已从集成 HEAD 21bf52fc 派发；ready 集合现为 {R03}（R04 需 R03 完成）。
- 事件留档：R01/R02 实现期间 worktree 曾被外部进程删除，实现者重建并复验；两次审查均核对提交与报告吻合。

## G0 再恢复与当前基线复验（2026-09-17）

执行提示词要求的现场恢复再次执行，本轮为第二次全量复核。与 2026-09-13 记录相比的关键变化与结论：

- **历史现场已清理**：`.worktrees/craft-*` 与 `codex/craft-*` 任务分支均不存在（2026-09-16 分支清理）；权威 `dag-state.json` 随集成工作区删除。本文件与 Git 历史（合并 `e064a1f7`，为当前 `main` 祖先）成为完成事实的权威记录。
- **27/27 任务维持 done**：DAG JSON 程序化校验（27 任务+5 门禁、引用完整、无环）；`release-evidence.json` 的 26 个任务 SHA + O05 集成 3a4895f4 + 终合并 e064a1f7 + 基线 83e2ef5c 全部为当前 HEAD（3e767d66）祖先；craft 迁移 000125–000132、`docs/testing/craft/` 证据文件均在当前树中。
- **复核纠错（重要）**：第一轮复核的 "`go build ./...` PASS" 为管道假象（`go build | head && echo` 吞掉退出码），据此得出的"无集成破坏"结论错误。第二轮以真实退出码复验，发现 Craft 合并后（+2156 提交，paseo/mobile 专项）main 上存在四类真实破坏，全部由 paseo 合并链引入且未跑全量构建即落库：
  1. **编译破坏 A**：`e9b024d3`（fix(paseo)）在 `service/workbench/remote_dispatch.go` 新增私有方法 `dispatch` 与既有字段 `dispatch` 重名，包不编译，连锁 `handler/session`、`container`、`application/repository` 构建失败。修复：方法改名 `dispatchFenced`。
  2. **编译破坏 B**：`207cb66d`（fix(container)）在 `container/agent_runtime.go` 用 `r.Enabled`（*bool）直接做布尔运算、`worker, err =` 双分支赋值未声明 `err`；`container.go` 遗留未用导入。修复：改用 `r.RecoveryEnabled()`、补 `var err error`、删冗余导入。
  3. **迁移器常量过期**：`internal/database/migration.go` 的 `sqliteWorkbenchRunsMigrationVersion=16` 未随迁移重编号（合并 391a4ac2 后文件为 000055）更新，导致生产迁移路径在 v55 处 dirty（`internal/database` 10 项测试失败）。修复：常量 16→55 + 注释。
  4. **测试助手未适配 000055 自管事务**：12 处 `sqlite3migrate.Config{}`（11 个 craft 期测试助手 + `recoverytest/provider/main.go`）在 000055（BEGIN IMMEDIATE 自管事务）下报 "cannot start a transaction within a transaction"，使 `application/service`、`agent/opencode` live、`agent/recoverytest`（G3 双工作者竞争）失败。修复：全部加 `NoTxWrap: true`（与 paseo 已适配助手同模式）。
  另修复四处过期测试期望：`workbench_migration_test.go` DownUp 步数 19→31（迁移编号存在 17-18/22-29 空洞）；`agent_run_worker_test.go` 三个 WorkerConfig 补 `Driver:"platform"`；`agent/tools`+`agent/trpc` 手工 schema 助手按 000055 精确差异补 `driver/target_id/budget_ref` 三列（ALTER 字面量，与重建迁移等价）；`repository/execution_dispatch_test.go` 迁移 head 断言 20→57 与 `repository/craft_version_test.go` W01 回滚目标 41→45（craft 家族现居 45–52，两处均为 open-connector 重编号后的过期编号）。
- **修复后当前 main 复验全绿**（真实退出码）：`go build ./...` exit 0；`go test` craft / workbench / agent 全树（含 recoverytest G3 矩阵 39.9s、tools、trpc）/ sandbox / execution / database / application/service 全部子包 / application/repository / handler/session / container 全 ok；`CRAFT_LIVE=1 TestLiveCraftTwoTurns` PASS（41.3s，含真模型独立证据通道：真实模型写入 monthly-summary.md 且汇总正确）；builtin 回归子集 PASS；`apps/web` tsc+vite 构建与 1280/1280 单测 PASS（主工作区需先 `pnpm install`，非代码问题）；`check-craft-release.py --commit 6fbc0706` evidence verified。
- **发布门禁按设计拒发当前 HEAD**：evidence head=6fbc0706 ≠ 当前 HEAD，须在发布切割时重新生成（终审 F1 原样有效，属发布时动作，非代码缺口）。
- **外部缺口复核仍与留档一致（F2 继续有效）**：`NewCraftModelGateway` 无生产装配调用方（O02 网关未接线）；commercial 订阅/预算/订单/恢复/结算仍走 AutoMigrate（G4 迁移缺口）；`internal/metrics` 无 router/cmd 注册（/metrics 未暴露）；驻留事件无生产记录者。以上仅阻塞收费发布，不影响非收费交付。
- **F3 已满足**：`/tmp` 无 craft 残留目录。
- `craft.enabled` 回退路径复核：`CraftFeatureGate.Enabled` 零值 false，fail-closed 成立。
- 本节修复（22 个代码文件）为协调器在 main 工作区的直接修复——均为根因已完全定位的机械修复（改名/常量/配置位/列对齐/过期编号），逐项经修复后定向复跑验证；未提交，待用户审阅。
