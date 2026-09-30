# Issue #65 — T35: Marketplace Evaluation、隐私指标与 Publisher Custody

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #65 |
| 标题 | T35: Marketplace Evaluation、隐私指标与 Publisher Custody |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/65 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:42:49Z |
| 更新时间 | 2026-09-20T13:44:26Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #60、#63、#64（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #71 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

Catalog 展示审核、兼容性、结构化 Evaluation 和去标识指标；Publisher 消失后仅保管已有 Adoption 必需包。

## Acceptance criteria

- [ ] 指标不能反推出 Tenant 或 Task 内容。
- [ ] Custody 不转移 Publisher 身份，也不开放新 Adoption。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #60 — T30: Public Marketplace 审核与跨 Tenant Adoption
- #63 — T33: Variant 退役、Adoption 终止与 Listing 生命周期
- #64 — T34: Release 与依赖安全撤回传播

## 任务清单（正文 checkbox 原文）

- [ ] 指标不能反推出 Tenant 或 Task 内容。
- [ ] Custody 不转移 Publisher 身份，也不开放新 Adoption。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
