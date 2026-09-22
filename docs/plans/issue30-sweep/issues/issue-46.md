# Issue #46 — T16: Task Material：Artifact、引用、预览与分享

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #46 |
| 标题 | T16: Task Material：Artifact、引用、预览与分享 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/46 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:41:44Z |
| 更新时间 | 2026-09-20T13:44:15Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #35（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #47、#48、#52、#68 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

Task 可列出和打开 Evidence、Artifact、Files、Diff、测试报告和只读 Terminal，并通过短期授权下载/分享。

## Acceptance criteria

- [ ] Artifact 版本不可原地覆盖，签名 URL 不长期缓存。
- [ ] 不支持、大文件、grant 过期和 Terminal 输入尝试均正确处理。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #35 — T05: Task 详情 Snapshot、Timeline 与 SSE 恢复

## 任务清单（正文 checkbox 原文）

- [ ] Artifact 版本不可原地覆盖，签名 URL 不长期缓存。
- [ ] 不支持、大文件、grant 过期和 Terminal 输入尝试均正确处理。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
