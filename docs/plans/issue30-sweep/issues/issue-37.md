# Issue #37 — T07: 运行中调整、排队下一 Run 与停止重启

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #37 |
| 标题 | T07: 运行中调整、排队下一 Run 与停止重启 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/37 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:41:17Z |
| 更新时间 | 2026-09-20T13:44:07Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #35、#36（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | 无 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

Task Owner 可明确调整当前 Run、排队下一 Run或停止后重启，并看到指令实际绑定的 Run。

## Acceptance criteria

- [ ] 停止请求、停止确认、结果未知分别呈现。
- [ ] 未知结果阻止冲突写 Run，所有控制命令带真实 revision。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #35 — T05: Task 详情 Snapshot、Timeline 与 SSE 恢复
- #36 — T06: 通用目标输入与耐久 Task 创建

## 任务清单（正文 checkbox 原文）

- [ ] 停止请求、停止确认、结果未知分别呈现。
- [ ] 未知结果阻止冲突写 Run，所有控制命令带真实 revision。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
