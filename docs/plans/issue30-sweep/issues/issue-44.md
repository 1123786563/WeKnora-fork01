# Issue #44 — T14: 旧 Session 投影为 Legacy Task

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #44 |
| 标题 | T14: 旧 Session 投影为 Legacy Task |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/44 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:41:38Z |
| 更新时间 | 2026-09-20T13:44:12Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #34、#35（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #71 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

现有 Session 以相同身份作为 Legacy Task 查看和继续普通追问，只投影历史能证明的状态。

## Acceptance criteria

- [ ] 不创建第二 task ID，不伪造 Grant、预算、审批或 Agent Version。
- [ ] 需要新安全语义的行为必须显式升级或新建 Run。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #34 — T04: 首页 Attention 与统一 Task 列表
- #35 — T05: Task 详情 Snapshot、Timeline 与 SSE 恢复

## 任务清单（正文 checkbox 原文）

- [ ] 不创建第二 task ID，不伪造 Grant、预算、审批或 Agent Version。
- [ ] 需要新安全语义的行为必须显式升级或新建 Run。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
