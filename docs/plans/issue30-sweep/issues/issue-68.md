# Issue #68 — T38: Taro Adapter 复用深 Module 与维护模式门槛

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #68 |
| 标题 | T38: Taro Adapter 复用深 Module 与维护模式门槛 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/68 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:42:59Z |
| 更新时间 | 2026-09-20T13:44:35Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #34、#35、#36、#38、#46（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #71 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

小程序通过同一 Task/Resource/Material Interface 跑关键 scenario，并形成进入维护模式的可验证门槛。

## Acceptance criteria

- [ ] 业务 Module 不依赖 Expo 或 WeChat。
- [ ] 新旧编排器不长期并存，迁移采用 replace-dont-layer。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #34 — T04: 首页 Attention 与统一 Task 列表
- #35 — T05: Task 详情 Snapshot、Timeline 与 SSE 恢复
- #36 — T06: 通用目标输入与耐久 Task 创建
- #38 — T08: Attention Inbox 与类型化审批闭环
- #46 — T16: Task Material：Artifact、引用、预览与分享

## 任务清单（正文 checkbox 原文）

- [ ] 业务 Module 不依赖 Expo 或 WeChat。
- [ ] 新旧编排器不长期并存，迁移采用 replace-dont-layer。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
