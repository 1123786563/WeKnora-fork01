# Issue #35 — T05: Task 详情 Snapshot、Timeline 与 SSE 恢复

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #35 |
| 标题 | T05: Task 详情 Snapshot、Timeline 与 SSE 恢复 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/35 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:41:11Z |
| 更新时间 | 2026-09-20T13:44:04Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #32（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #36、#37、#38、#40、#41、#42、#44、#45、#46、#57、#68 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

用户可以打开 Task，看到结果优先详情、Task/Run/Attention 三层状态和规范 Timeline，并通过 Snapshot、游标 SSE、补洞和重同步恢复。

## Acceptance criteria

- [ ] 重复事件幂等，缺口或游标裁剪不会被静默忽略。
- [ ] App 重启、断线和终态 drain 均有 Interface-level scenario 证据。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #32 — T02: Active Tenant 切换与 Scoped Vault 隔离

## 任务清单（正文 checkbox 原文）

- [ ] 重复事件幂等，缺口或游标裁剪不会被静默忽略。
- [ ] App 重启、断线和终态 drain 均有 Interface-level scenario 证据。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
