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
| D01 | implementing | .worktrees/craft-d01 / codex/craft-d01 | 92f23a10 | 实现中（python-docx+OOXML 校验+浏览器+五项类型接线收尾） |
| O04 | implementing | .worktrees/craft-o04 / codex/craft-o04 | a543e439 | 已派发（用量视图+指标+排障手册+O03 五项接线收尾含 stranded-deleting 修复）；仅余 O05 等待 |
| C04 | **done** | .worktrees/craft-c04 / codex/craft-c04 | f931f02b | HEAD 7d0aebc4 审查 PASS_WITH_NITS（19/19+矩阵复验）→ 合入 256fbb78；容器装配裁定归 C05（含 nit-2/3） |
| C05 | **done** | .worktrees/craft-c05 / codex/craft-c05 | 256fbb78 | HEAD faf500e9 审查 PASS_WITH_NITS（21/21，安全闭环无伪造路径）→ 合入 ae664bcb；迁移 000126/000046；C04 装配落实；C06 接线提示与 6 卫生 nit 留档 |
| D02 | implementing | .worktrees/craft-d02 / codex/craft-d02 | ae664bcb | 已派发（openpyxl+LibreOffice 重算+XLSX 导出+浏览器）；C06/D01/D03/O03/O04/O05 等待依赖或席位 |
| C02 | ready（排队） | — | — | 等实现席位（C03 完成后即派） |

## 调度纪要

- 并行组 #1（R01+R02）：两项均审查 PASS_WITH_NITS 后串行合入（da6d169d、21bf52fc），合并后受影响包全绿（含 live 与真实 PG）。
- R03 已从集成 HEAD 21bf52fc 派发；ready 集合现为 {R03}（R04 需 R03 完成）。
- 事件留档：R01/R02 实现期间 worktree 曾被外部进程删除，实现者重建并复验；两次审查均核对提交与报告吻合。
