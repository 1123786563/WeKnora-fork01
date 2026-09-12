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
| W02 | fixing | .worktrees/craft-w02 / codex/craft-w02 | 1b44296f | 审查 FAIL（阻断：preview.conf 是 Go 源副本非 nginx 配置；其余 24/26 项达标）；已退回实现者修复重审 |
| O02 | implementing | .worktrees/craft-o02 / codex/craft-o02 | 9608c6ac | 已派发（G4 映射+受控网关）；W03–O05（除 O02 外）等待依赖 |

## 调度纪要

- 并行组 #1（R01+R02）：两项均审查 PASS_WITH_NITS 后串行合入（da6d169d、21bf52fc），合并后受影响包全绿（含 live 与真实 PG）。
- R03 已从集成 HEAD 21bf52fc 派发；ready 集合现为 {R03}（R04 需 R03 完成）。
- 事件留档：R01/R02 实现期间 worktree 曾被外部进程删除，实现者重建并复验；两次审查均核对提交与报告吻合。
