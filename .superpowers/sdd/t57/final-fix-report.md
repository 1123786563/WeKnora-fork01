# T27 #57 整计划最终修复批次报告（t57 final-fix round）

- 分支：`codex/issue30-t57`（worktree `.worktrees/issue30-sweep-t57`）
- 基线：`7b949f985`
- 范围：整计划最终审查的 5 项发现（5 minor），一次批次全部处置（4 修复 + 1 如实说明）
- 工作流：严格 RED→GREEN——先写回归测试并实跑确认失败（6 项失败全部指向实现缺口），再最小实现，全量回归零失败

## 发现 1（minor）：ask 指定的 `.superpowers/sdd/t57/final-pkg.md` 缺席 → 无法修复（审查方交付物），如实说明

`final-pkg.md` 是**审查方**的审查包交付物，在审查发生前即应存在；修复方不能事后伪造一份「审查包」来抹平缺席（伪造证据违反本计划「绝不伪造通过」的诚实性约束）。修复方能动的事实只有两个：

1. 本报告按 ask 指定路径写入 `.superpowers/sdd/t57/final-fix-report.md`——`t57` 目录因此建立，且与 `.superpowers/sdd/t52|t56|t60|t67/final-fix-report.md` 的入库先例同型（本报告随本批次 commit 入库）；
2. 缺席事实本身如实转记于此，供编排方知晓：审查包内容当时由编排方以实际存在的 `.superpowers/sdd/plan-t57/{progress.md,task-1-brief.md-report.md}` 与 `docs/plans/issue30-sweep/plans/plan-t57.md-report.md` 替代补齐，审查结论已覆盖。

**状态**：不可由修复方修复（非代码缺陷）；已转记 + 目录建立 + 报告入库。

## 发现 2（minor）：charging-unconfigured 分支 AC1 证据字段未经真实动作背书 → 已修复

**问题**：`runVoiceRoomIntegration` 在语音计价未配置分支（open 503）硬编码 `evidence.disconnect='ended-settled'`、`evidence.resume='new-session'`（修复前 `apps/mobile/src/voice-room-integration-smoke.ts:211-212`），而该分支从未执行 `leave()`/`resume()`（无会话可结、resume 必再 503）——字段契约（AC1 leave/resume 事实）与真实动作不对齐。

**改动**（`apps/mobile/src/voice-room-integration-smoke.ts`）：
1. `VoiceRoomIntegrationEvidence` 契约扩展：`disconnect` 增加 `'not-exercised'`、`resume` 增加 `'not-exercised'`，接口注释注明「分支未行使该动作时如实标 not-exercised，不冒充真实回执」；
2. 分支体抽为**可单测的纯函数** `markChargingUnconfigured(evidence, handle)`（导出）：`turn='charging-unconfigured'`、`confirmSteer='skipped-no-transcript'`、`disconnect='not-exercised'`、`resume='not-exercised'`、`voiceNeverDecided` 结构断言保留、`errorReason='deployment has no voice pricing configured (honest 503); AC1 leave/resume not exercised'`；
3. 调用点（503 分支）改为 `return markChargingUnconfigured(evidence, handle)`；`runVoiceRoomIntegration` 诚实性要点注释补第 (3) 条。

transcribed 主分支行为零变化：两字段仍是真实 `leave()`/`resume()` 回执（`lastLeave.settled` / 新会话判定）。

**回归测试**：
- `voice-room-integration-smoke.test.ts` 新增 `charging-unconfigured branch marks AC1 fields not-exercised instead of fake leave/resume receipts`——对纯函数直测 6 项断言（含 `disconnect/resume === 'not-exercised'` 与 errorReason 披露 `not exercised`）；
- 既有 live 用例补分支钉子：`turn === 'charging-unconfigured'` 时断言两字段为 `'not-exercised'`（有凭据环境实跑时生效；本环境无凭据以 `t.skip` 诚实跳过）。

## 发现 3（minor）：缺 taskId 深链的文案误归因为登录问题 → 已修复

**问题**：`/tasks/voice` 无 `taskId` 参数时渲染 `<VoiceRoomScreen state={undefined}/>`（修复前 `apps/mobile/src/app/tasks/voice.tsx:69-71`），屏上固定显示「请先登录并激活空间，再加入语音房。」——把缺参数误报为登录问题。

**改动**（`apps/mobile/src/app/tasks/voice.tsx`）：缺 `taskId` 分支改为传入携带归因文案的真实状态：
`state={{ phase: 'idle', taskId: '', turns: [], lastSubmitError: '链接缺少 taskId：语音房需从任务详情页进入。' }}`。
文案归因于缺参数并引导正确入口（与同文件既有 `lastSubmitError` 承载环境类提示的先例一致）；`state={undefined}` 分支从路由消失，屏的 undefined 兜底文案保留为纯防御路径。

**回归测试**：
- 路由源级守卫（仓库既有手法）`the voice route attributes a missing taskId deep link to the missing parameter, never to login`：断言路由源含「缺少 taskId」「任务详情页进入」且**不含** `state={undefined}`；
- 真渲染测试（经典 JSX + 全局 React stub，与 app-smoke 同型）`screen render: missing-taskId deep link copy ...`：`state=undefined` 仍显示登录兜底文案；携带缺参文案的状态上屏显示「缺少 taskId」且**不出现**登录文案。

## 发现 4（minor）：确认后 act 失败无用户可及的重试入口（UX 缺口）→ 已修复（缺口闭合，非回滚语义原样保留）

**问题**：`confirmTranscript` 在 `onConfirmIntent` 失败后轮次保持 confirmed、`pendingTurnId` 清空、仅呈现 `lastSubmitError`（`apps/mobile/src/voice-room-view.ts:95-107`），该条确认文字无法经控制器再次提交，用户须重新说话或去详情页手输。

**改动**：
1. `voice-room-view.ts`：提交序列抽为 `submitIntent(intent)`；失败时把意图留在 `lastFailedIntent`；接口与实现新增 `retrySubmit()`——无失败意图或提交中时是 no-op，否则把**同一条已确认文字**经同一 `onConfirmIntent`（宿主 act 通道，服务端 act 幂等）再提交；成功后清错误与失败意图。**不回滚语义零变化**：轮次终态仍由模块在 confirm 时钉死为 confirmed，重试只作用于提交通道；
2. `screens/VoiceRoomScreen.tsx`：`lastSubmitError` 存在时在错误文案下渲染「重试写入任务」按钮（提交中 disabled），新增 `onRetrySubmit?` 回调；无错误时不渲染；
3. `app/tasks/voice.tsx`：`onRetrySubmit={() => { void controllerRef.current?.retrySubmit(); }}` 接线。

**回归测试**（`voice-room-view.test.ts`）：
- `a failed act keeps a user-reachable retry entry: retrySubmit resubmits the same confirmed text without rollback`——首次 act 抛 `TASK_OFFICE_COMMAND_UNAVAILABLE` 后 `retrySubmit()` 二次提交同文字、错误清除、轮次仍 confirmed（无回滚钉子）；
- `retrySubmit without a prior failure (or after success) is a no-op`——无失败不提交、成功后不重复提交；
- 渲染测试 `screen render: lastSubmitError offers a retry button wired to onRetrySubmit`——有错误出按钮且接到回调，无错误无按钮；
- 既有钉子 `controller surfaces submit failure honestly without discarding the confirmed intent` 原样通过（非回滚语义未被破坏）；
- 路由接线：既有源级测试补断言 `retrySubmit` 已接入路由。

## 发现 5（minor）：执行报告未入库 → 已修复

`docs/plans/issue30-sweep/plans/plan-t57.md-report.md` 由 untracked 转为 git 跟踪（`git add`，随本批次 commit 入库），与姊妹报告 plan-t43/t48/t52.md-report.md 的入库惯例对齐，TDD 证据链留痕。

## TDD 证据（本会话实跑）

### RED（先写测试，实跑确认失败——6 项失败全部指向实现缺口）

命令：`pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts`

```
✖ charging-unconfigured branch marks AC1 fields not-exercised instead of fake leave/resume receipts
✖ a failed act keeps a user-reachable retry entry: retrySubmit resubmits the same confirmed text without rollback
✖ retrySubmit without a prior failure (or after success) is a no-op
✖ screen render: lastSubmitError offers a retry button wired to onRetrySubmit
✖ the voice route attributes a missing taskId deep link to the missing parameter, never to login
✖ the voice route is an Expo Router screen wired through composition only   ← 新增 retrySubmit 接线断言失败
ℹ tests 14  pass 7  fail 6  skipped 1
```

（过程如实记录：RED 期间渲染测试曾因**测试 helper 自身**的无限递归/children 存放位置 bug 而失败，属测试代码缺陷，修正 helper 后重跑 RED，失败原因全部收敛为实现缺口，才进入 GREEN。）

### GREEN（实现后，目标两文件 14 测试）

命令：`pnpm exec tsx --test apps/mobile/src/voice-room-view.test.ts apps/mobile/src/voice-room-integration-smoke.test.ts`

```
✔ integration stays opt-in: missing credentials skip and a private/localhost host is invalid, never a pass
﹣ live voice room end to end: open → turn → confirm steer → leave settled → resume new session (opt-in)   # missing WEKNORA_MOBILE_TEST_* （诚实跳过）
✔ charging-unconfigured branch marks AC1 fields not-exercised instead of fake leave/resume receipts
✔ the integration runner is total: a failing transport still yields evidence, not a rejection
✔ voice room copy covers every phase, notice reason, and never leaks wire detail
✔ controller wraps the handle: pending-turn identity never escapes, confirm routes through onConfirmIntent
✔ controller surfaces submit failure honestly without discarding the confirmed intent
✔ a failed act keeps a user-reachable retry entry: retrySubmit resubmits the same confirmed text without rollback
✔ retrySubmit without a prior failure (or after success) is a no-op
✔ the screen renders state + callbacks only, and the view module never imports wire packages
✔ screen render: missing-taskId deep link copy attributes the parameter, not login; login copy stays undefined-state-only
✔ screen render: lastSubmitError offers a retry button wired to onRetrySubmit
✔ the voice route attributes a missing taskId deep link to the missing parameter, never to login
✔ the voice route is an Expo Router screen wired through composition only
ℹ tests 14  pass 13  fail 0  skipped 1
```

### 全量回归（apps/mobile 全套件）

命令：`pnpm --dir apps/mobile test`

```
ℹ tests 240  pass 228  fail 0  skipped 12  todo 0
```

12 个 skip 逐一核对均为既有 opt-in live 冒烟（缺 `WEKNORA_MOBILE_TEST_*` 凭据诚实跳过：attention-inbox / blind-push / device-inbox / knowledge-qa / legacy / material / offline / task-budget / task-intervention / task-start / dictation / voice-room），无新增 skip、零失败。

### 类型检查

命令：`pnpm --dir apps/mobile typecheck`（`tsc --noEmit`，严格模式，expo tsconfig）

```
typecheck exit: 0（无输出即无错误）
```

## 范围与提交

改动文件（6 代码/测试 + 1 文档报告 + 本报告）：
- `apps/mobile/src/voice-room-integration-smoke.ts`（发现 2）
- `apps/mobile/src/voice-room-integration-smoke.test.ts`（发现 2 回归）
- `apps/mobile/src/voice-room-view.ts`（发现 4）
- `apps/mobile/src/screens/VoiceRoomScreen.tsx`（发现 4）
- `apps/mobile/src/app/tasks/voice.tsx`（发现 3 + 4 接线）
- `apps/mobile/src/voice-room-view.test.ts`（发现 3/4 回归）
- `docs/plans/issue30-sweep/plans/plan-t57.md-report.md`（发现 5 入库）
- `.superpowers/sdd/t57/final-fix-report.md`（本报告，按 t52/t56/t60/t67 先例入库）

未触碰：mobile-core、api-client、Go 服务端（发现 2 的语义修正仅涉及 apps/mobile 冒烟证据层）。
