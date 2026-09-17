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
| MX-005 | fixing→review | 98002709→c679c089(+fix) | mv2 7/7 · shared 589/589 · go 三包 ok · probe 解析真实 CAS 观测 | R1 FAIL(P1-1 pending wire)→已修复待复核 | MX-005.md |

## 首次更新（2026-09-18，MX-001）

- REPO/DESIGN 定位、基线与环境见 preflight.md；G01-G12 对账见 current-state-matrix.md。
- 首批就绪：MX-001（本任务）→ 完成后 MX-002、MX-003、MX-007 就绪（D-001 串行）。

## 注记（2026-09-18，MX-002 审查 P2 处置）

- P2-3「test:shared 偶发失败」已定位：审查代理运行时段恰逢 MX-003 并发修改 snapshot 契约与消费方 fixture（同一批失败断言随后在 2bd4c682 修复）。非 flaky，登记澄清。
- P2-1 probe 自研 satisfies 的 ^0.x 语义：当前矩阵无 ^0.x range 不触发；若未来出现 0.x 期望再修（记录于本注）。
- P2-2 libsodium 包名精确为 @more-tech/react-native-libsodium@1.5.6；P2-4 RED 阶段平台顺序为过程笔误。均不阻塞。
