# Issue #41 — T11: 注册设备、行动通知与安全深链

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #41 |
| 标题 | T11: 注册设备、行动通知与安全深链 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/41 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:41:30Z |
| 更新时间 | 2026-09-20T13:44:11Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #32、#34、#35（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #67、#69、#70 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

设备在当前 Deployment 注册并可撤销；行动通知不含敏感正文，点击后重新鉴权并同步权威 Task。

## Acceptance criteria

- [ ] 推送只作同步 hint，不直接改变业务状态。
- [ ] 设备撤销、错误 deep link 和重复通知均有验证。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #32 — T02: Active Tenant 切换与 Scoped Vault 隔离
- #34 — T04: 首页 Attention 与统一 Task 列表
- #35 — T05: Task 详情 Snapshot、Timeline 与 SSE 恢复

## 任务清单（正文 checkbox 原文）

- [ ] 推送只作同步 hint，不直接改变业务状态。
- [ ] 设备撤销、错误 deep link 和重复通知均有验证。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
