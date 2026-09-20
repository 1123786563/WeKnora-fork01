# Mobile AI Office Specification

状态：正式规格，已确认产品与测试边界。本文自包含地汇总已确认的产品决策和工程方向，不表示任何能力已经实现、发布或完成验收。

## Problem Statement

用户需要在 iOS 与 Android 上发起、监督和收取 AI 办公任务成果，但任务、文件、Git、终端和审批必须由 WeKnora 在可信云端统一治理。现有移动、同步和远程执行来源不能各自成为新的会话、权限、执行或加密权威。用户还需要在不中断任务上下文的前提下处理持续运行、断线恢复、只读共享、版本发布和受控 Git 写回。

## Solution

首版采用 Paseo 作为移动 UI 与适配底座，借鉴 Happy 的交互、同步、版本和多设备设计，由移动适配层调用 WeKnora。WeKnora 是唯一后端和治理权威：Task 以持续会话的 Session ID 为唯一权威，一个 Task 可包含多个 Run；WeKnora Workbench/Run 编排可信云端沙箱中的文件、Git 与终端操作。

首版交付空间切换、Agent 会话、云端任务、审批、文件与产物、知识资源，管理配置随后补齐，并进行 iOS/Android 真实验收；Paseo Web 只保持可构建，完整 Web 继续由 WeKnora 提供。任务默认私有，可显式只读共享。发布采用固定版本独立副本；交互终端受 Run 停止互斥和后端权限约束，只有 Git 远端写入逐次审批。

## User Stories

1. As an 空间成员, I want 在可信 WeKnora 云端发起任务, so that 文件、Git 和终端操作不会依赖个人主机。
2. As an 空间成员, I want 所有任务资源按当前空间归属, so that 不同空间的数据和权限保持隔离。
3. As a 移动用户, I want 使用 Paseo 风格的移动交互, so that 可以在原生端高效处理办公任务。
4. As a 移动用户, I want 借鉴 Happy 的同步和多设备设计, so that 切换设备时能理解任务进度。
5. As a 用户, I want 在一个入口使用知识问答、通用 Agent 和专业 Agent, so that 不必在不同应用间切换。
6. As a 用户, I want 云端只读取我为任务授权的内容, so that 可信云执行仍有明确的数据边界。
7. As a 用户, I want 知道该链路不承诺云端不可读 E2EE, so that 我能据此决定是否授权内容。
8. As a 用户, I want 一个 Task 对应一个持续会话, so that 任务上下文不会在每次执行时丢失。
9. As a 用户, I want 在同一 Task 内进行多次 Run, so that 可以继续或重试而保留上下文。
10. As a 用户, I want 跨会话聚合的大型项目任务留到后续能力, so that 首版的会话边界保持清晰。
11. As a 用户, I want 云端 Run 在客户端离开后持续执行, so that 我不必保持应用前台运行。
12. As a 用户, I want 审批持久等待并在完成、失败或待审批时收到状态, so that 我能在合适时机处理任务。
13. As an 离线用户, I want 只读缓存和草稿在恢复连接后可用, so that 网络中断不会丢失我已查看的内容或输入。
14. As an 离线用户, I want 自己决定恢复连接后是否发送草稿, so that 客户端不会自动执行或自动审批。
15. As a Task Owner, I want 新任务默认私有, so that 任务内容不会默认暴露给空间成员。
16. As a Task Owner, I want 显式向空间成员授予只读共享, so that 他们可以协作查看而不获得执行权。
17. As a 只读共享成员, I want 查看获准的对话、Run、文件和产物, so that 我能了解任务结果。
18. As a 只读共享成员, I want 被拒绝继续执行、终端和审批操作, so that Owner 的执行边界不会被绕过。
19. As a Task Owner, I want 同一 Task 的多个 Run 复用持久工作区, so that 文件和 Git 状态可以延续。
20. As a Task Owner, I want 不同 Task 默认没有共享可写目录, so that 任务之间不会意外互相修改文件。
21. As a Task Owner, I want 显式导入文件到另一个 Task, so that 跨任务复用可被追踪和授权。
22. As a 用户, I want 任务结束后文件保留而计算可休眠, so that 我可以在后续续用任务。
23. As a 空间成员, I want 配额超限时仍能查看、导出和清理文件, so that 我能恢复可写容量。
24. As a 用户, I want 首版不按天自动删除任务文件, so that 任务产物不会意外消失。
25. As a Task Owner, I want 选择固定版本发布, so that 接收方看到确定的成果副本。
26. As a 发布目标读者, I want 依据目标权限读取发布副本, so that 原任务的私有性不被改变。
27. As a Task Owner, I want 修改后必须重新发布, so that 已发布版本不会被自动覆盖。
28. As a Task Owner, I want 撤销任务共享不影响已发布副本, so that 发布副本的目标权限保持独立。
29. As a 移动用户, I want iOS 和 Android 都经真实验收, so that 首版在两类设备上可用。
30. As a Web 用户, I want 继续使用现有 WeKnora Web, so that 首版不会出现第二套不完整 Web。
31. As a 移动用户, I want 锁屏通知只显示完成、失败或待审批且不含标题正文, so that 敏感内容不会出现在锁屏上。
32. As a 用户, I want 缓存按 backend、账号和空间隔离加密并能退出清理, so that 本地数据不跨身份混用。
33. As a 用户, I want 联网发现撤权后清除缓存并可关闭离线缓存, so that 缓存风险可被控制。
34. As a Task Owner, I want 在云工作区 clone、修改、查看 Diff 并创建本地 Git 提交, so that 远端写入前能审查精确变更。
35. As a Task Owner, I want clone 前检查个人代码平台 Connection 授权, so that 云端不能擅自使用我的凭据。
36. As a Task Owner, I want 每次 push 分支或创建 PR 展示 repo、branch 和具体变更并审批, so that 每次远端写入都有明确授权。
37. As a Task Owner, I want 禁止 force push 和远端删除分支, so that 首版不会执行高风险 Git 操作。
38. As an 活跃 Run 观察者, I want 只观察终端输出, so that 自动运行不会和人工写入竞争。
39. As a Task Owner, I want 交互 PTY 或手工编辑先停止并确认 Run, so that 工作区写入不会产生竞态。
40. As a Task Owner, I want 副作用结果未知时 Run 保持阻断, so that 系统不会假装已停止或自动重复操作。
41. As a 用户, I want 发布目标仅为当前空间资源或指定知识库, so that 首版不会跨空间或公开到互联网。
42. As a 发布目标读者, I want 下载和导出获准发布内容, so that 可以在授权范围内使用成果。
43. As a 通知打开者, I want 打开通知后先重验权限再恢复任务, so that 已撤权用户不会看到旧状态。
44. As a 移动用户, I want 在任务内进入 Conversation、Files、Git、Terminal 和 Artifacts, so that 工作区能力围绕任务而不是空间目录组织。
45. As a Task Owner, I want 未知结果的远端 Git 写入先保持阻断并对账, so that 系统不会重复 push 或重复创建 PR。
46. As a Task Owner, I want 启动 Run 前收回既有 PTY 写权限且停止确认前不恢复写入, so that Run 与人工终端写入双向互斥。

## Implementation Decisions

- Paseo 只作为移动 UI 和适配底座；Happy 只提供交互、同步、版本和多设备设计参考。首版不采用 Paseo daemon，也不采用 Happy 的云端或加密协议。
- WeKnora 保持会话、Task、Run、权限、审批、Connection、知识、产物和云端执行的唯一权威。Task 直接使用 Session ID，不引入独立 Task 状态机。
- 云端在用户授权范围内读取任务内容；传输和云端存储均加密。该可信云边界不等同于云端不可读 E2EE。
- 移动端建设 app-owned 的窄云工作区客户端，仅消费 WeKnora 的查询与命令契约，不在客户端维护第二个执行状态机。
- Task/Run 映射复用持续会话与执行记录概念。查询返回 snapshot、scope、sequence、cursor 和项目集合；SSE 使用 scope、sequence 和 cursor 去重，cursor 过期时以 snapshot 全量回补。
- 命令携带请求幂等键和期望版本。后端以 compare-and-set 处理冲突并返回当前 snapshot；未知副作用进入等待用户状态，先对账再允许重试。
- 审批绑定 Run、交互标识、操作标识、获批参数哈希、凭据版本和期望版本。任一获批内容或凭据版本变化均要求重新批准。
- 私有、共享只读和 Owner 写入权限由后端在每次访问时判定。工作区标识只定位资源，不能代替空间和 Owner 鉴权。
- 发布以固定版本和独立 ACL 建模。发布目标只能是当前空间的产物资源或指定知识库；发布权限不随任务共享级联，撤销任务共享也不撤销既有发布副本。
- 每个 Task 使用独立持久工作区；同一 Task 的 Run 复用，跨 Task 必须显式导入。闲置计算可以休眠而文件保留；空间存储配额超限时禁止新增写入但保留查看、导出和清理。
- 云沙箱需要持久卷和 snapshot provider。休眠续用只有在 provider 真实验证后才可对外承诺。
- 活跃 Run 仅提供终端观察。启动 Run 前收回任何既有 PTY 写权限；交互 PTY 或人工编辑须先通过带期望版本的停止命令并确认停止，结果未知保持阻断。
- 代码平台 Connection 属于 Task Owner 个人使用且由 WeKnora 管理，凭据不进入聊天或产物。clone 先验权，云工作区的修改、Diff 和本地 Git 提交不另审批；每次远端写入均绑定操作类型、精确 commit、PR base/head、repo、branch 和变更摘要，字段变化后重新批准，合并由代码平台完成。
- 移动缓存 key 包含 backend、account 和 tenant；scope 变化立即丢弃旧流。缓存内容按账号和空间隔离加密，退出清理，联网发现撤权后清除；锁屏通知不含标题和正文。
- 离线状态不能即时获知撤权，用户可关闭离线缓存；恢复连接后由用户决定是否发送草稿，客户端不得自动发送或自动审批。
- 建议的移动信息架构为 Home、Tasks、Agents、Me 四个主入口，Workspace 作为任务内入口；这是工程信息架构建议，不代表页面已实现。
- 需要扩展的持久化概念包括：发布副本及其 ACL、Task 工作区绑定与休眠状态、缓存 scope 版本、远端写入审批快照，以及 Run/PTY 排他状态。具体字段和迁移以实现前的现有 schema 核对为准。

## Testing Decisions

- 主测试 seam 采用既有 WeKnora Workbench/Session/Run 外部 API：测试请求、快照、SSE、权限结果、审批状态和可见成果，而不测试客户端内部实现细节。
- 移动适配层以合同测试验证查询、命令、snapshot/cursor 回补、scope 变化丢弃、请求幂等和期望版本冲突。合同必须覆盖私有、只读共享、发布副本和撤权负例。
- Run/工作区测试覆盖同 Task 多 Run 复用、跨 Task 显式导入、配额超限、停止确认、未知副作用等待用户、既有 PTY 与新 Run 的双向互斥。
- 审批和 Git 测试覆盖参数哈希、凭据版本、审批后字段变化重审、Owner Connection、只读成员拒绝、精确 commit/PR 目标与逐次远端写入审批。
- 通知和缓存测试覆盖锁屏最小内容、设备作用域、账号/空间隔离、退出清理、在线撤权清理和离线不自动审批。
- 真实验收与静态/fixture 验证分开记录：iOS 与 Android 需要真实设备、真实 WeKnora 云端和真实沙箱验证；静态、mock 或合同测试不能替代这些证据。
- 用户已确认测试 seam 为既有 Workbench/Session/Run 对外接口、移动云工作区客户端合同测试和真实双端云 E2E；实施和验收以这三层边界组织。
- 测试先例复用现有 Run 持久恢复、Workbench 交互决策与 CAS、以及 AppConnector 的 owner/scope 校验模式；测试其对外行为，不复制内部实现断言。

## Out of Scope

- 第二套完整 Web、Paseo daemon、Happy wire 协议消费者、Happy 云端服务及其端到端加密协议。
- 跨空间发布、公开互联网发布链接、force push、远端删除分支和自动离线审批。
- 跨会话聚合的大型项目任务、自动按天删除任务文件，以及未获真实验证的任意沙箱休眠恢复承诺。

## Further Notes

Q1–Q20 覆盖摘要：Q1–Q6 确认 WeKnora 可信云、Tenant 归属、Paseo/Happy 取舍和首版范围；Q7–Q12 确认 Session/Run、持续执行、只读共享和持久工作区；Q13–Q17 确认配额、固定版本发布、双端验收、缓存通知和 Git 基线；Q18–Q20 确认 Run/PTY 互斥、Owner Connection 逐次远端写审批及同空间/知识库发布边界。

三仓源码基线：WeKnora-fork01 为 `f7802c47c82835514e4a5d30457222b6c0e891d9`，Paseo 为 `d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b`，Happy 为 `ac64b9b4677870f7b7a9eacfd0780959229717f1`。WeKnora-fork01 与 Happy 工作区在查阅时存在未提交修改，因此 SHA 不能独自重现当时工作区；Paseo 工作区当时干净。

本规格由本地完整设计、访谈台账、ADR 和领域词汇综合而成；这些本地设计依据尚未推送，不能被当作远程可访问证据。本文已保留作 GitHub 规格阅读所需的决策、边界、接口候选、测试计划和基线信息，不依赖读者访问本地文件。本轮规格不代表已实施；后续由拆票和实现阶段交付。审查通过后由主控发布到 `1123786563/WeKnora-fork01`，不得按默认 upstream 推断目标仓库。
