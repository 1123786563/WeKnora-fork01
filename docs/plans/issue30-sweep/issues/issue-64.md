# Issue #64 — T34: Release 与依赖安全撤回传播

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #64 |
| 标题 | T34: Release 与依赖安全撤回传播 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/64 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:42:46Z |
| 更新时间 | 2026-09-20T13:44:27Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #60、#61（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #65 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

Release 或锁定依赖被撤回时，准确阻断受影响 Adoption/Variant 的新 Task/Run，并按风险处理在途执行。

## Acceptance criteria

- [ ] 依赖不按名称自动替换。
- [ ] 撤回保留历史、原因、影响范围和替代版本。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #60 — T30: Public Marketplace 审核与跨 Tenant Adoption
- #61 — T31: Agent Upgrade Proposal 与渐进升级

## 任务清单（正文 checkbox 原文）

- [ ] 依赖不按名称自动替换。
- [ ] 撤回保留历史、原因、影响范围和替代版本。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
