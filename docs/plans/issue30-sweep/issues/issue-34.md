# Issue #34 — T04: 首页 Attention 与统一 Task 列表

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #34 |
| 标题 | T04: 首页 Attention 与统一 Task 列表 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/34 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:41:08Z |
| 更新时间 | 2026-09-20T13:44:05Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #32（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #38、#41、#42、#44、#68 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

首页展示需要我处理、正在运行和最近完成的 Task；统一列表支持搜索、筛选和归档，全部来自当前 Tenant 的授权聚合读模型。

## Acceptance criteria

- [ ] 首页和列表不做逐会话 N+1，也不泄露其他 Tenant 内容。
- [ ] 迟到查询、分页重复键和空/错/加载状态均可验证。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #32 — T02: Active Tenant 切换与 Scoped Vault 隔离

## 任务清单（正文 checkbox 原文）

- [ ] 首页和列表不做逐会话 N+1，也不泄露其他 Tenant 内容。
- [ ] 迟到查询、分页重复键和空/错/加载状态均可验证。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
