# CFT-S01-T012 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- `packages/views/src/craft/interaction-card.tsx`（新增）：`CraftDecisionCard`——送达状态分层的决策卡：
  - `decisionDeliveryLabel`：recorded（已记录·等待送达）/ delivery_pending（等待送达）/ delivered（已确认送达）/ unknown（送达不明·需核对）各自成层，**任何一层都不显示"已执行/执行已继续"**；
  - question 卡只提供 回答+拒绝（答题不授任何权限）；permission 卡只提供 允许一次（无永久授权措辞）；unknown 只拒绝——R06 fail-closed 词汇延续；
  - 终态卡（resolved）**不渲染任何决定控件**——迟到批准无法复活已取消请求；卡片身份可追溯。
- `packages/views/src/craft/status-notice.tsx`（新增）：`CraftStatusNotice`——连接/授权生命周期横幅（委托 T004 CraftNotice 的 aria-live 与 kind 样式）：reconnect="连接中断，正在重新同步…（任务仍在执行）"、expired="预览票据已过期，可重新授权同一版本"——均不把任务画成失败。
- `packages/views/src/craft/interaction-card.test.tsx`（新增，4 tests）。

## 验收断言对照

- 批准 response 仅 recorded 不展示执行已继续 ✓（测试 1：四层 delivery 各自文案 + 明确的否定断言——recorded 层不出现"已确认送达/已执行"等）
- 问题回答不增加权限 ✓（测试 2：question 卡无批准控件；permission 卡"允许一次"且无永久授权措辞）
- 取消 accepted 仍显示取消中 ✓（测试 4：statusLabel 投影——stopping→'正在停止'、canceled→'已停止'，受理未终态期间永不显示终态词；状态由权威 Run 投影拥有，浏览器不自判）
- 断线和票据到期不显示任务失败 ✓（测试 4：reconnect/expired 横幅无"失败"字样、kind 不落 failed 色；测试 3：过期交互卡零决定控件）
- 附加：过期请求按钮行为可测 ✓（测试 3：终态卡 0 个决定按钮 + 身份保留）

## 与既有实现的关系

- W05 workbench 内 InteractionCardView 与 C02/R06 interaction.tsx（decide 客户端 + 409/410 处理 + deliveryUnknown 文案）继续承担事件流投影的交互呈现；CraftDecisionCard 是任务卡要求的**独立可复用决策卡组件**（四层送达状态与终态只读为新增性），组装层可在后续视觉轮（T034）统一替换挂载。
- 取消中/已取消的权威语义来自服务端 Run 终态（W04 语义，Go 侧取消链路在 S02/T023 复核）。

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `pnpm exec tsx --test packages/views/src/craft/interaction-card.test.tsx` | 0 | 4 pass / 0 fail |
| `pnpm run test:craft:shared` | 0 | **108 pass / 0 fail**（104→108） |
| `pnpm run typecheck:web` | 0 | 通过 |
| `bash e2e/craft-stack.sh up mock && run mock` | 0 | **6 passed (26.2s)** 无回归 |

## 回退

revert 本提交（三新文件，纯增量）。
