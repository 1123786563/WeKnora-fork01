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
| R03 | implementing | .worktrees/craft-r03 / codex/craft-r03 | 21bf52fc | 已派发（含 R02 审查 nit-2 owner 校验要求）；R04–O05 等待依赖 |

## 调度纪要

- 并行组 #1（R01+R02）：两项均审查 PASS_WITH_NITS 后串行合入（da6d169d、21bf52fc），合并后受影响包全绿（含 live 与真实 PG）。
- R03 已从集成 HEAD 21bf52fc 派发；ready 集合现为 {R03}（R04 需 R03 完成）。
- 事件留档：R01/R02 实现期间 worktree 曾被外部进程删除，实现者重建并复验；两次审查均核对提交与报告吻合。
