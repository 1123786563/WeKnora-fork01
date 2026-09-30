# Wave 3 — T02 补缺：F3 迟到响应运行时证据 + 截图19 定性更正（fix-resume, validation）

你是 frontend_validator。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 你只补证据与更正报告，**不改任何生产代码**；若补证过程中发现新缺陷，只记录。

## 1. Worktree 与基线

```bash
cd /Users/wuyongjun/.codex/worktrees/issue-140-t02-live-validation/WeKnora-fork01
git status --short   # 必须 clean（调度员 2026-09-25 实核为空，HEAD 086d93ed1）
git checkout --detach d88cd513a   # 换基到当前集成 HEAD；你的报告文件已随 70360810f 入库，在此之上继续
```
报告文件：`docs/plans/issue-140/task-2-live-ios-validation.md`（本 worktree 内）。本地 commit 允许；绝不 push；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 两个待修项（集成台账 Wave 2 载明，T02 维持 running 的原因）

1. **F3 迟到响应仍缺**：R1 已补"空间切换 seam"（switch-tenant 后旧 Task 空列表/404，PASS），但"**切换空间后迟到响应不可见**"（late/stale response arriving after scope switch must not backfill）的**运行时证据**仍缺。要求：在真实 iOS 模拟器运行中的应用构造一次"慢响应"场景（例如：发起 Task 读取后立即切换空间/登出，让响应在网络层晚到），捕获证据证明旧空间数据未被回填到 UI（截图/storage dump/网络时序，如实声明构造方式与观察层级）。可用手段自选：本地 Lite 服务器人为延迟（端口 **57807**）或应用内触发；禁止改生产代码来制造时序。
2. **截图19 定性更正**：R1 评审载明"新增截图 19 被错误定性为登录门（实为全白屏），需更正一处次要证据声明"。在报告中更正该截图的定性描述，保持其余评审过的内容不动（只改这一处事实性描述，不重写报告）。

## 3. 环境（沿用你上一轮的现场）

- iOS 27 模拟器 iPhone 18 Pro（UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`）；构建方式与命令沿用报告既有记载（expo run:ios；API origin 构建期环境变量注入）。
- 服务器/fixture：沿用 `docs/plans/issue-140/task-2-live-ios-validation.md` 自己的复现提纲；本轮端口 **57807**（57802-57806 已被占用）。
- 若迟到响应场景在真实施动下确实无法构造（例如应用已在 UI 层同步取消），如实记录技术原因与替代证据（单测/静态 seam 指针），并明确声明"运行时迟到响应证据不可构造"——**这是可接受的诚实结论，不伪造时序证据**。

## 4. 全局约束（verbatim 摘录）

- validator 生产代码零改动；唯一可写文件是证据/报告文档。
- 凭据/密钥 disposable，结束清理，报告不保留 token；测试端口隔离（57807）。
- 严禁 push、merge、deploy、GitHub 操作；禁止派发子 agent。
- 报告只写事实与实测输出；受阻如实 blocked 附证据。

## 5. 产出

更新 `docs/plans/issue-140/task-2-live-ios-validation.md`（追加 R2 节：F3 迟到响应证据或不可构造结论 + 截图19 更正说明），本地提交 `docs(mobile): close T02 round-2 evidence gaps`。最终消息报告 HEAD SHA、两项处置结果（补证 PASS / 不可构造结论 + 更正完成）。T02 终态（verified 或维持 running/blocked 口径）由主控依据证据裁决。
