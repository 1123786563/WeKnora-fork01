# T36 最终审查修复报告 — 修复批次 1/1

Worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t36`（分支 `codex/issue30-t36`）
基线：`ee1d458d1`（T07 完成态）

## 发现 F1（important）：统一 New 入口目标 TextInput 无 maxLength —— R1 裁决第三层未落地

### 根因确认（本次修复前亲验）

- `apps/mobile/src/screens/NewTaskScreen.tsx:29`（修复前行号）的目标 TextInput 仅有 `value/onChangeText/placeholder/multiline` 四个 props，无 `maxLength`。
- 修复前 `grep -rn "maxLength" apps/mobile/src/` 仅命中 `adapters/intent-log.ts:12` 的注释（该注释声明「单条超限由 New 表单的 maxLength 约束在源头拦截，不在本层截断」），代码中 0 处实际约束——与 finding 的 grep 实证一致。
- `.superpowers/sdd/plan-t36/progress.md:28`（R1 裁决原文）：「Task 5 表单 maxLength=500（物理约束工程映射）……maxLength 数值为工程默认」；`progress.md:29`：「第三层（Task 5 maxLength=500）由主控 dispatch 传达，本任务不实现」——该 dispatch 从未落地，R1 三层处置（progress.md:31 显示后续任务全部 complete）中第三层成为唯一缺口。
- 破坏链路（本次以 node 精确计算复核）：Android SecureStore 单值约 2048 字节；单条意图记录（`SubmissionIntentRecord`，`packages/mobile-core/src/task-office/task-office.ts:189-195`）固定开销（UUID 形态 requestId/sessionId + goal 包装 + scope + persistedAt）+ 中文 UTF-8 每字 3B——**无上限时 560+ 中文字即单条超 2048B** → `createSecureIntentLog` 的 `setItemAsync`（`adapters/intent-log.ts:71`）抛错 → `office.start` 上抛 → 提交永久失败且错误为底层异常；每次重试生成新 requestId 再失败。iOS keychain 无此限制（平台差异破损）。

### 修复内容

`apps/mobile/src/screens/NewTaskScreen.tsx`：

1. 新增导出常量 `GOAL_TEXT_MAX_LENGTH = 500`（附注释链回 R1 裁决与 intent-log 字节预算层），落实主控已裁决数值。
2. 目标 TextInput 追加 `maxLength={GOAL_TEXT_MAX_LENGTH}`（现为 `NewTaskScreen.tsx:39`）——R1 第三层源头拦截落地，且不与 1536B 预算层冲突：intent-log 层的 `serializeWithinBudget` 在仅剩 1 条时不裁剪（保最新意图耐久落盘），源头 maxLength 恰好保证该「不被截断的单条」最坏情形仍在 Android 2048B 信封内（见下）。
3. 配套超长提示（finding 可选项）：`state.draft.text.length >= GOAL_TEXT_MAX_LENGTH` 时渲染 `目标文本已达 500 字上限，超出部分不会保存`，替代 native 静默截断的零反馈。

草稿保留语义（AC2）不受影响：maxLength 只限制输入长度，不触碰草稿通道。

### 回归测试（`apps/mobile/src/app-smoke.test.tsx`，紧随既有 New 入口用例之后新增 1 个 test 块）

`the universal New entry caps goal text at the source so one intent record stays inside the SecureStore envelope`，四段断言：

1. `GOAL_TEXT_MAX_LENGTH === 500`（裁决数值锁定，防回归漂移）；
2. 500 字草稿渲染时，placeholder 为 `今天想完成什么？` 的 TextInput props 上 `maxLength === 500`（源头约束真实挂载），且超长提示出现；
3. 499 字时提示不出现（不打扰未触界输入）；
4. 字节预算交叉验证：最坏情形单条记录（500 个中文字 × 3B + UUID requestId/sessionId + scope 固定开销）经 `JSON.stringify` + `TextEncoder` 序列化后 **≤ 2048 字节**（实测 1791B，node 精确计算）——直接封死 finding 描述的「单条超限 → save 抛错 → 提交永久失败」链路。

### 验证证据（本次全部亲跑）

| 命令 | 结果 |
| --- | --- |
| `npx tsx --test src/app-smoke.test.tsx`（apps/mobile 下） | 39/39 pass，含新增用例 ✔（`the universal New entry caps goal text at the source so one intent record stays inside the SecureStore envelope (1.137ms)`） |
| `pnpm test`（@weknora/mobile，计划 gate 同款） | 98 tests：97 pass + 1 skip（opt-in live 用例缺 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 按设计跳过，`ee1d458d1` 引入的机制）+ 0 fail，exit 0 |
| `pnpm typecheck`（@weknora/mobile） | 0 错误，exit 0 |
| `npx tsx --test src/adapters/intent-log.test.ts` | 8/8 pass（1536B 预算层关联回归不受影响） |
| `grep -rn "maxLength" apps/mobile/src/` | 命中 `NewTaskScreen.tsx:39` 实码 + 常量/测试引用——finding 的「grep 实证 0 处」已反转 |

## 结论

1 项发现全部修复，无「无法安全修复」项。#40 真机验证前该 Android 平台破损已由源头封死。
