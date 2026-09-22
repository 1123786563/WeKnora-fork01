# Issue #70 — T40: Android 安装包核心工作流验收

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #70 |
| 标题 | T40: Android 安装包核心工作流验收 |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/70 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:43:07Z |
| 更新时间 | 2026-09-20T13:44:35Z |
| 评论数 | 0（API 字段 0，timeline commented 事件 0，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | #40、#41、#56、#66（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #71 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

真实 Android Release 包完成核心流程，并验证系统返回、后台限制、通知、文件 URI、Keystore、麦克风和弱网恢复。

## Acceptance criteria

- [ ] 证据来自可安装 Release 包。
- [ ] 与 iOS 的领域行为一致，平台差异仅在 Adapter。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- #40 — T10: 加密离线缓存、草稿与联网确认
- #41 — T11: 注册设备、行动通知与安全深链
- #56 — T26: 可编辑语音转写草稿
- #66 — T36: 多 Deployment 切换与兼容性降级

## 任务清单（正文 checkbox 原文）

- [ ] 证据来自可安装 Release 包。
- [ ] 与 iOS 的领域行为一致，平台差异仅在 Adapter。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

无评论。

## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
