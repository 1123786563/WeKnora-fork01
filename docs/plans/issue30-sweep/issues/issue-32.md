# Issue #32 — T02: Active Tenant 切换与 Scoped Vault 隔离

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #32 |
| 标题 | T02: Active Tenant 切换与 Scoped Vault 隔离 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/32 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:41:02Z |
| 更新时间 | 2026-09-20T13:44:04Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #31（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #33、#34、#35、#40、#41、#66 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

用户可以选择和切换 Active Tenant；Token、Scope Lease、缓存、草稿和在途请求按 Deployment、用户、Tenant 隔离，旧订阅与迟到响应不会污染新空间。

## Acceptance criteria

- [ ] 跨 Tenant 缓存和请求均 fail closed。
- [ ] 切换、退出和撤权的加密 key 失效路径有自动化证据。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #31 — T01: 原生客户端登录并进入受支持的 Deployment

## 任务清单（正文 checkbox 原文）

- [ ] 跨 Tenant 缓存和请求均 fail closed。
- [ ] 切换、退出和撤权的加密 key 失效路径有自动化证据。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
