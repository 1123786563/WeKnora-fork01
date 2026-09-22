# Issue #31 — T01: 原生客户端登录并进入受支持的 Deployment

## 元数据

| 字段 | 值 |
|---|---|
| 编号 | #31 |
| 标题 | T01: 原生客户端登录并进入受支持的 Deployment |
| URL | https://github.com/1123786563/WeKnora-fork01/issues/31 |
| 状态 | open |
| 关闭时间 | - |
| 标签 | ready-for-agent |
| 作者 | 1123786563 |
| 创建时间 | 2026-09-20T13:40:58Z |
| 更新时间 | 2026-09-21T02:30:40Z |
| 评论数 | 3（API 字段 3，timeline commented 事件 3，三者一致） |
| 原生 sub-issues | 无（嵌套子 Issue 为空） |
| 声明式 Parent | #30（正文 `## Parent` 段） |
| 声明式 Blocked by | None（正文声明可立即开始）（与原生 blocked-by 关系一致） |
| 原生 blocking（被本 Issue 阻塞） | #32 |
| 正文其它 #引用 | 无 |
| 关联 PR | 无（timeline 中无来自 PR 的 cross-referenced 事件；全仓库仅 PR #1、#2，均为 miniprogram QA 修复，与本 Issue 无关） |
| 任务清单 | 3 条 checkbox，已勾选 0 条 |

## 正文全文

## Parent

#30

## What to build

建立最小原生 App、Mobile Runtime 和真实 WeKnora Adapter。用户可选择官方云或 HTTPS 自托管 Deployment，完成登录/OIDC 与 capability 协商；协议不兼容时只出现安全升级说明。

## Acceptance criteria

- [ ] 真实登录成功后只进入该 Deployment 的授权 surface。
- [ ] 不兼容或未知 capability 时不发送业务命令，并有 Interface-level 与登录流程证据。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## Blocked by

- None (can start immediately).

## 任务清单（正文 checkbox 原文）

- [ ] 真实登录成功后只进入该 Deployment 的授权 surface。
- [ ] 不兼容或未知 capability 时不发送业务命令，并有 Interface-level 与登录流程证据。
- [ ] 端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。

## 评论摘要

### 评论 1 — 1123786563 @ 2026-09-20T19:38:54Z

- 链接: https://github.com/1123786563/WeKnora-fork01/issues/31#issuecomment-5752167194
- 摘要: 实现已集成到 feature/mobile-office 分支（HEAD 0c5a6bdc）。任务级与全分支代码评审发现的缺陷已修复；focused 测试、shared/Web/mobile 类型检查、全量 Go 测试、shared/Web/mobile 测试与 iOS/Android Expo 导出通过。验收仍开放：缺少部署/凭据与 Android 工具，未在授权 HTTPS staging 执行安装后的 iOS/Android 密码/OIDC/capability 检查；移动工作流 React 边界门禁存在 227 个既有违规（与 pre-Ticket 基线 ae41e271 的输出一致，未新增违规）；CI 基线提案在评审中。

### 评论 2 — 1123786563 @ 2026-09-21T02:30:14Z

- 链接: https://github.com/1123786563/WeKnora-fork01/issues/31#issuecomment-5754630443
- 摘要: 集成 SHA 0c5a6bdc 上的 iOS Simulator 探测发现原生启动阻塞：Xcode 27 / iOS 27 构建可安装并启动进程但渲染黑屏；UIKit 要求 Scene 生命周期（Expo SDK 55 生成的 AppDelegate/Info.plist 不含，官方指引称后续 SDK 才引入）。按已批准计划的 Expo 55 约束未私自手改生命周期或升级 SDK。T01 保持 OPEN，等待 SDK/scene lifecycle 裁定 + 真实 HTTPS staging 登录/OIDC 与 Android 设备证据；模拟器进程启动不计为成功的屏幕验收。

### 评论 3 — 1123786563 @ 2026-09-21T02:30:40Z

- 链接: https://github.com/1123786563/WeKnora-fork01/issues/31#issuecomment-5754633228
- 摘要: 更正上一条评论的日志排版：确切的 UIKit 运行时消息为 "Application failed to launch: UIScene life cycle is required for apps built with this SDK."；因屏幕保持黑屏，已安装应用未被接受为成功 surface。


## 关联 PR

无。本 Issue timeline 中 cross-referenced 事件来源均为 Issue（非 PR）；仓库全部 PR（#1、#2，MERGED）与 #30 移动 AI Office 系列无关。
