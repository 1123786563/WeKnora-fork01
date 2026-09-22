# Issue #45 — T15: 有证据的知识问答闭环

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #45 |
| 标题 | T15: 有证据的知识问答闭环 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/45 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:41:41Z |
| 更新时间 | 2026-09-20T13:44:10Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #33、#35、#36（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #47 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

知识问题 Task 只检索成员获准且 Tenant 允许发现的知识，答案带可打开的版本和时间证据。

## Acceptance criteria

- [ ] 事实、规则推导和模型推断明确区分。
- [ ] 撤权、无证据和多知识源场景均有端到端验证。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #33 — T03: Resource Shelf 展示当前空间可用资源
- #35 — T05: Task 详情 Snapshot、Timeline 与 SSE 恢复
- #36 — T06: 通用目标输入与耐久 Task 创建

## 任务清单（正文 checkbox 原文）

- [ ] 事实、规则推导和模型推断明确区分。
- [ ] 撤权、无证据和多知识源场景均有端到端验证。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
