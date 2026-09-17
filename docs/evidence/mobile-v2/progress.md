# MX 执行台账 · mobile-v2（唯一进度事实源）

仓库：1123786563/WeKnora-fork01 · 实施分支：codex/expo-mobile-v2（worktree .worktrees/expo-mobile-v2）
基线：main `6a70c35a`（2026-09-18）· 设计包：docs/design/mobile-v2（v2.0，task-index sha256 1c910fd8…）
模式：单实现者串行（D-001）+ 每任务独立 review 子代理。状态机：pending→ready→implementing→review→integrating→accepted（fixing/blocked-* 例外）。

| 任务 | 状态 | BASE→HEAD | 测试（cmd·exit·发现/通过） | Review | 证据 |
|---|---|---|---|---|---|
| MX-001 | accepted | 6a70c35a→04176329 | tsx --test mx-001.test.ts · RED exit1 → GREEN exit0 · 1/1（含基线 kind 不变量） | R1 FAIL(2×P1)→修复→复核 PASS | MX-001.md |
| MX-002 | accepted | 64b8da68→52c87b1c | test:mobile-v2 · RED exit1(13+真实偏差) → GREEN exit0 · 2/2；test:shared 579/579；prebuild 双平台 exit0 | 规格 PASS 6/6 · 质量 APPROVED（4×P2 非阻塞，见注） | MX-002.md / native-baseline.md |
| MX-003 | accepted | 52c87b1c→2bd4c682 | mv2 3/3 · shared 579/579 · go workbench+handler ok · RED=3 真实断言失败 | 规格 PASS 5/5 · 质量 APPROVED（3×P2 已承接，D-016） | MX-003.md |
| MX-004 | accepted | 2bd4c682→98002709 | mv2 5/5 · go handler 全绿 · RED=parser 对真实 v2 字节崩溃 | 规格 PASS 5/5 · 质量 APPROVED（4×P2：3 项由 MX-006 承接） | MX-004.md |
| MX-005 | accepted | 98002709→c9505e26 | mv2 7/7 · shared 589/589 | R1 FAIL→修复→复核 FAIL(注册表回归)→R2 修复→终核 PASS | MX-005.md |
| MX-006 | review | d8550bde→d9cfc83c(+fix) | mv2 8/8 · shared 589/589 · domain 5/5 · adapter 2/2 | R1 单点 FAIL(.gitignore 登记)→P2/P3 已修复→待 scoped 确认 | MX-006.md |
| MX-007 | review | c9505e26→5e6e3c29 | mv2 8/8 · shared 589/589 · 对比率 ACTIVE 全过(disabled 豁免) · typecheck 14=基线 | 审查进行中 | MX-007.md |
| MX-008 | accepted | 5e6e3c29→12a9c4da(含fix) | mv2 10/10 · 挂载 4/4 · typecheck 14=基线 | R1(2×P1)→修复→复核 PASS | MX-008.md |
| MX-009 | accepted | 08a5c9cb→12a9c4da | mv2 10/10 · 挂载 4/4 · happyAuthRequests=0 | 规格 PASS · 质量 APPROVED（P2 锁账澄清→D-023） | MX-009.md |
| MX-010 | accepted | 12a9c4da→95973585(+fix2) | mv2 13/13 · tsc 14=基线 | R1 FAIL→修复→复核 PASS（P2-1 probe 二修随 MX-012 R1 批次） | MX-010.md |
| MX-011 | accepted | 35ad2c0d→95973585(含fix) | mv2 12/12 · shared 590/590 · frozen B/false/0 | 规格 PASS 7/7 · 质量 APPROVED（P2 已处置，D-024） | MX-011.md |
| MX-012 | fixing→review | 95973585→202225bb(+fix) | mv2 13/13 · 挂载 6/6（含 single-flight 竞态回归） · tsc 14=基线 | R1 FAIL(P1 密钥竞态)→已修复→待复核 | MX-012.md |

## 首次更新（2026-09-18，MX-001）

- REPO/DESIGN 定位、基线与环境见 preflight.md；G01-G12 对账见 current-state-matrix.md。
- 首批就绪：MX-001（本任务）→ 完成后 MX-002、MX-003、MX-007 就绪（D-001 串行）。

## 注记（2026-09-18，MX-002 审查 P2 处置）

- P2-3「test:shared 偶发失败」已定位：审查代理运行时段恰逢 MX-003 并发修改 snapshot 契约与消费方 fixture（同一批失败断言随后在 2bd4c682 修复）。非 flaky，登记澄清。
- P2-1 probe 自研 satisfies 的 ^0.x 语义：当前矩阵无 ^0.x range 不触发；若未来出现 0.x 期望再修（记录于本注）。
- P2-2 libsodium 包名精确为 @more-tech/react-native-libsodium@1.5.6；P2-4 RED 阶段平台顺序为过程笔误。均不阻塞。

## 注记（2026-09-18，注册表回归教训）

mx-005 fix 轮把 interactions.ts 的 kind 误改 modify，违反 mx-001 基线守护（该文件相对基线 6a70c35a 恒为 create）；复核代理实跑抓获。R2 已修复并复跑 7/7。纪律固化：**任何 file-ownership.json 变更后必须重跑 test:mobile-v2**（守护测试就是为此存在）。

| MX-013 | accepted | 6d94a204→0ff12076 | frozen Go ok（含派生断言）· mv2 16/16 | R1 FAIL(P1 虚构列)→修复→复核 PASS | MX-013.md |
| MX-014 | review | cd10655d→(待提交) | mv2 15/15 · shared 590/590 · tsc 14=基线 | 待独立review | MX-014.md |

| MX-014 | accepted | cd10655d→964bf811(+fix2) | mv2 18/18（含真实翻页/重放去重断言） | R1→修复→PASS/APPROVED（证据缺口 R2 补齐：frozen 补真实翻页+重放页） | MX-014.md |
| MX-017 | fixing→review | 336b0ffb→d782cc4f(+fix) | mv2 18/18 · tsc 11=基线 | 规格 PASS · 质量 CR(P2 屏造 request_id)→已修→待复核 | MX-017.md |
| MX-016 | review | 964bf811→424cfe97 | mv2 17/17 · shared 590/590 · tsc 11=基线 | 待独立review | MX-016.md |
| MX-022 | review | 424cfe97→620ddcb8 | mv2 18/18 · frozen 撤权零可见/禁问 | 审查进行中（与 016/017 复核合并） | MX-022.md |
| MX-030 | review | 620ddcb8→(待提交) | mv2 19/19 · shared 590/590 · tsc 11=基线 | 待独立review | MX-030.md |

## 接续说明（2026-09-18 本轮收口）

- 分支 codex/expo-mobile-v2（worktree .worktrees/expo-mobile-v2），HEAD `202225bb`，工作区干净，未 push。
- MX-001–012 全部提交；accepted：001–009、011；**review 在途**：MX-010（P1 修复待复核）、MX-012（待审查）——复核代理已派发，结论到达后按 R1 模式处置并更新本表。
- 下一就绪（依赖全满足）：**MX-013**（工作台聚合读模型+首页+入口接线：Go service/handler/路由/DI + SDK + HomeScreen + 按 D-023 锁转移完成 _layout 入口切换）；随后 MX-014/016/017/022/030。
- 常用命令：`pnpm run test:mobile-v2`（13/13 基线）、`pnpm run test:shared`（590/590）、`cd apps/mobile && pnpm exec tsc --noEmit | grep -c "error TS"`（14=基线）、挂载套件 `pnpm --filter @weknora/mobile exec vitest run -c vitest.mobile-v2.config.ts`（4/4）、Go `go test ./internal/workbench/ ./internal/handler/session/ ./internal/application/service/workbench/`。
- 纪律提醒：file-ownership.json 任何变更后必须重跑 test:mobile-v2（mx-001 基线守护）；typecheck 证据用全量 grep -c 禁止 tail；提交被 Mimosa 全项目误报间歇拦截时按 D-022 处置（重试，不改文案换绿灯）。
- 证据索引：本目录 progress.md（唯一进度事实）、decisions.md D-001–D-024、current-state-matrix.md、file-ownership.json（~200 锁）。
