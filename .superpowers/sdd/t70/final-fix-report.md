# T40 / #70 整计划最终修复批次报告（t70 final-fix round）

- 分支：`codex/issue30-t70`（worktree `.worktrees/issue30-sweep-t70`）
- 范围：整计划最终审查 3 项发现（全部 minor），一次批次全部处置
- 修复前状态：`322c57d01` HEAD，工作区仅 1 个 untracked 文件（发现 2 的对象）

## 发现 1（minor）：审查包 `.superpowers/sdd/t70/final-pkg.md` 不存在 → 已补生成

**事实核对**：`.superpowers/sdd/` 下确无 `t70/` 目录（仅 `plan-t70/progress.md`）；仓库全量 `git ls-files | grep final-pkg` 与文件系统 `find . -name final-pkg.md` 均为 0 命中——该路径从未被任何任务产出过，最终审查只能绕道 plan/issue/diff/实跑完成。

**处置**：新建 `.superpowers/sdd/t70/final-pkg.md`（审查任务的指定消费物）。内容全部来自本会话可复核事实：git `branch`/`log`/`show --stat` 核实的 8 提交任务映射（§二）、本会话实跑的计划级验证整链输出（§三）、`android-acceptance.ts` 逐行号核对的七工作流矩阵（§四）、plan 原文 blocked-env B1-B3（§五）、Task 7 报告的已知事项（§六）。主控可直接按原路径消费，无需再改任务路径。

**覆盖验证**：报告内引用的每个文件路径（issue-70.md / plan-t70.md / plan-t70.md-report.md / android-evidence/android-release.md / progress.md / android-acceptance.ts）均已 `ls`/`find`/`grep -n` 逐一核实存在且行号属实（见本报告各验证段命令）。

## 发现 2（minor）：Task 7 报告 untracked 未入库 → 已补提交

**事实核对**：`git status` 显示 `docs/plans/issue30-sweep/plans/plan-t70.md-report.md` untracked；对照惯例 `git ls-files .superpowers/sdd/t56 .superpowers/sdd/t60 .superpowers/sdd/t67` 三者均有入库的 `final-fix-report.md`。

**处置**：本批次以独立 docs 提交将 Task 7 报告（169 行：任务定位、交付物、初版与修复轮 RED/GREEN 全证据、计划级验证、worktree 重建环境事件、Concerns 清单）显式路径入库。

**关于「Task 1-6 无任务级实施报告」**：如实处置为**不伪造**——Task 1-6 的会话级 RED/GREEN 记录不在本会话上下文，凭空补写将违反 Evidence over claims；其可复核证据已代偿汇总进 `final-pkg.md` §二的提交映射（每任务提交 SHA + 文件清单 + 测试规模，git 历史随时可验）+ `plan-t70/progress.md` 的计划 gate PASS 台账。此取舍在 final-pkg.md §二末段明示。

**覆盖验证**：提交后 `git ls-files docs/plans/issue30-sweep/plans/ | grep t70` 确认入库（见下文提交记录）。

## 发现 3（minor）：注释措辞与行为不符 → 已修正

**事实核对**：`apps/mobile/src/adapters/notification-permission.ts:30` 原注释 `// 权限面不可用：如实上抛，不伪造结论`，但所在 catch 块的实际行为是 `return 'unavailable'`（非 throw）。审查给出的行号 :34 与现文件行号有偏差，但引文文本唯一命中 :30，即本处。同函数第二个 catch（requestPermissionsAsync 失败路径，:35-36）无注释且行为同型。

**改动**（1 行，仅注释、零行为变化）：

```diff
       } catch {
-        return 'unavailable'; // 权限面不可用：如实上抛，不伪造结论
+        return 'unavailable'; // 权限面不可用：如实返回 'unavailable'，不伪造结论（与下方 requestPermissionsAsync 失败路径同型）
       }
```

**覆盖测试与输出**：该行为的契约本就被 `notification-permission.test.ts:49-54` 钉住——`a broken permissions surface reports unavailable and never fabricates granted` 用 `failGet`/`failRequest` stub 让权限面抛错，断言 `ensure()` **返回** `'unavailable'`（若实现真按旧注释"上抛"，此测试即 fail）。本批次修正注释后实跑：

```
$ pnpm exec tsx --test apps/mobile/src/adapters/notification-permission.test.ts
ok 1 - the native factory is unavailable in the Node test chain (fail closed)
ok 2 - an already-granted permission never re-prompts the user
ok 3 - an ungranted permission prompts exactly once and reports the honest outcome
ok 4 - a broken permissions surface reports unavailable and never fabricates granted
# tests 4 / # pass 4 / # fail 0
EXIT=0
```

## 回归证据（本会话实际执行的命令与结果）

1. `pnpm exec tsx --test apps/mobile/src/adapters/notification-permission.test.ts` → **4/4 pass，EXIT=0**（发现 3 定向）。
2. 计划级验证命令（`plan-t70.md:1279` 整链逐字执行：六文件定向 + app-smoke + mobile 全量 + typecheck）→ 六文件 **41/41 pass/0 fail**、app-smoke **67/67 pass/0 fail**、`pnpm --filter @weknora/mobile test` **244 tests / 233 pass / 0 fail / 11 skipped**（skip 为 `WEKNORA_MOBILE_TEST_*` opt-in 集成冒烟，无凭据如实 skip）、`pnpm --filter @weknora/mobile typecheck`（tsc --noEmit）通过、**PLAN_LEVEL_EXIT=0**。与本批次前 Task 7 修复轮记录的 244/233/0/11 完全一致——注释级改动零行为漂移。

## 提交记录（分项独立提交）

1. 发现 2 → docs：`docs(issue30-sweep): t70 Task 7 实施报告补入库（修复轮全证据 + 环境事件 + Concerns）`
2. 发现 3 → 代码：`fix(mobile): 通知权限 Adapter 注释措辞对齐实际行为——如实返回 unavailable 而非上抛（#70）`
3. 发现 1 + 本报告 → docs：`docs(sdd): t70 final-pkg 审查包补生成 + 最终修复批次报告`

## 未做/不在范围

- 未补写 Task 1-6 会话级实施报告（无会话证据可依，伪造违反 Evidence over claims；以 final-pkg.md §二提交映射代偿，已明示）。
- 未处置 `internal/types/vectorstore_test.go` 既有凭据（历史提交 `5cf093706` 引入，非本计划文件——留主控专门任务，Task 7 报告 Concerns ②已有记录）。
- 未把 t70 分支再合并进 `codex/issue30-mobile-office`（合并属主控职责；final-pkg.md §六第 1 条已提请）。
