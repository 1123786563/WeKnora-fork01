# 预检记录 · mobile-v2 实施轮

记录时间：2026-09-18 · 记录者：MX-001 协调者

## 仓库与工作区

| 项 | 值 |
|---|---|
| REPO_ROOT（主 checkout） | /Users/wuyongjun/trea/WeKnora-fork01（branch main） |
| remote origin | https://github.com/1123786563/WeKnora-fork01.git（upstream=Tencent/WeKnora） |
| 基线 HEAD | `6a70c35a19c58511b8372b5a68b5147ab577ad14`（docs: 整理移动端v2设计文档资源…） |
| git status（主 checkout） | MX-001 记录时点 clean；2026-09-18 复核出现 7 项 apps/miniprogram 未提交改动（auth.ts、polyfills.ts、4 个测试文件 + helpers/，属用户其他任务）——保留不动，本轮不触碰 miniprogram |
| 本轮实施工作区 | `.worktrees/expo-mobile-v2`（branch `codex/expo-mobile-v2`，基于基线 HEAD 创建） |
| 既有 worktree | react-multiclient（codex/react-vue-parity-align，其他任务所有，不动）+ 若干 /tmp 临时 review worktree + .sdd-worktrees/w08-w24（历史 W 轮） |

## 设计材料

| 项 | 值 |
|---|---|
| DESIGN_ROOT | `docs/design/mobile-v2/`（已在基线 commit 中入库） |
| task-index.json SHA-256 | `1c910fd82eb4595e709773cb93a23d0daffd4f0d5a833feae529379f9aa74c66` |
| pnpm-lock.yaml SHA-256 | `1365039155aa833449f78423179ffb3eff00b5fc15e31af076d7ce85b89dcd2f` |
| 设计包版本识别 | README 自述 v2.0（2026-09-17，源码基线 12737238a）；task-index 顶层 `status: design_only_not_executed`，36 任务全 pending |

## 环境可用性

| 环境 | 状态 |
|---|---|
| go1.26.3 / node v26.7.0 / pnpm 10.28.2 / git 2.54 | 可用 |
| Docker 后端栈 | **运行中**：WeKnora-app、WeKnora-postgres-dev、WeKnora-redis-dev、WeKnora-docreader-dev、parity-mock |
| TRPC_TEST_POSTGRES_DSN | 未设置（可用容器内 PG 构造，使用前先核对容器凭据，不写入证据） |
| Xcode 27.0 + iOS 26.5 模拟器 | 可用 |
| adb / Android 模拟器 | adb 不在 PATH（Android 链路 blocked-env 候选，MX-002/034 再核） |
| 真机 iOS/Android | 未授权（live-service 证据按 blocked-env 记录） |

## 已有台账盘点（不重置历史）

| 台账 | 性质 | 状态 |
|---|---|---|
| docs/superpowers/plans/2026-09-12-mobile-workbench-task-index.json + mobile-workbench-progress.md | W 计划原始索引 | 全 pending，**从未随执行更新** |
| .superpowers/sdd/2026-09-12-mobile-ai-saas-workbench/progress.md | W 轮真实执行台账 | W01-W07/W09/W17-W21 终审通过；W08/W10 止于修复循环；W13/W25/W34 code-ready；W36/W37 未派发 |
| docs/superpowers/plans/2026-09-12-open-connector-progress.md | OC T01-T18 | 全 passed；T18 provider_write/billing 永久 blocked-env |
| docs/upstream-parity/LEDGER.md | 上游对照 | 独立事项，与 MX 无冲突 |
| **docs/evidence/mobile-v2/**（本目录） | **MX 轮唯一事实源** | MX-001 建立 |

## 规则文档已读

根 AGENTS.md、docs/agents/domain.md（CONTEXT.md 词汇表）、docs/agents/issue-tracker.md、设计包 README/详设/页面规格/设计系统/API 契约/总计划/S00 分册/references（G01-G12）。
apps/mobile 与 packages/ 下无 AGENTS.md/CONTEXT.md。

## 首批就绪任务

MX-001 无前置（本任务）。完成后按 DAG 就绪：MX-002、MX-003、MX-007（三者写集合无冲突，锁文件仅 MX-002 持有；按 D-001 串行执行）。
