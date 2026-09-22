# Issue #56 — T26: 可编辑语音转写草稿

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #56 |
| 标题 | T26: 可编辑语音转写草稿 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/56 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:42:20Z |
| 更新时间 | 2026-09-20T13:44:22Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #36（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #57、#69、#70 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

新建和 Task 输入支持录音转写为可编辑草稿；拒权、取消和迟到转写不破坏文字输入。

## Acceptance criteria

- [ ] 取消录音不取消 Task。
- [ ] 转写必须由用户确认后才提交。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #36 — T06: 通用目标输入与耐久 Task 创建

## 任务清单（正文 checkbox 原文）

- [ ] 取消录音不取消 Task。
- [ ] 转写必须由用户确认后才提交。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
