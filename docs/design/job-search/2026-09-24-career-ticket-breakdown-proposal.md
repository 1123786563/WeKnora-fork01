# 求职专业 Agent Ticket DAG 提案（Expo + TDesign Miniprogram）

状态：用户已确认，33 个子 Issue 已发布到父规格 #140；GitHub 原生阻塞边已建立。父规格为 [#140](https://github.com/1123786563/WeKnora-fork01/issues/140)。每个 Ticket 的独立验收清单、接口、验证与失败处理见 [Brief 索引](tickets-draft/README.md)。

技术约束：Web 沿用 React/TDesign；iOS／Android 沿用 Expo／React Native，原生组件参照 TDesign Mobile React 视觉规范；鸿蒙原生 App 必须先通过 Expo 兼容性闸口；微信小程序沿用 Taro 4 并接入 TDesign Miniprogram。移动 Web 组件不直接用于 React Native。[ADR-0018](../../adr/0018-expo-tdesign-career-clients.md)记录依据与阻塞条件。

## 规则

- 每个 Ticket 是可演示、可验证的纵向切片；平台补齐 Ticket 使用共同服务，但仍需真实目标的端到端行为。
- Blocked by 只含真正的前置条件；前置实现须通过验证并集成，后续才能开始。
- 同一理论前沿不等于可以在同一工作区同时改代码。独立 Worktree、模块写权限、共享契约冻结与串行集成是实际并行条件。
- Ticket 不指定生产文件路径或实现代码；Code Agent 在详细计划里固定文件所有权、命令和检查点。

## Ticket 与直接阻塞边

| ID | 标题 | Blocked by | 可观察交付 | 主责边界 |
| --- | --- | --- | --- | --- |

| 01 | [#143 · Expo 鸿蒙原生兼容性闸口](https://github.com/1123786563/WeKnora-fork01/issues/143) · [Brief](tickets-draft/01-expo-harmony-compatibility-gate.md) | None (can start immediately)。 | 验证现有 Expo／React Native 工程是否能以可维护的适配方式在鸿蒙原生设备运行受认证 Task；形成可复现的技术裁定，供后续移动求职 Ticket 使用。 | Expo 鸿蒙适配可行性、原生构建与端能力探针；不接管 iOS／Android 页面或 Career 后端 |
| 02 | [#145 · Expo iOS／Android 受认证 Task 薄切片](https://github.com/1123786563/WeKnora-fork01/issues/145) · [Brief](tickets-draft/02-expo-ios-android-task-proof.md) | None (can start immediately)。 | 复用仓库现有 Expo／React Native 工程，让同一位已登录用户在 iOS、Android 打开同一条获授权的现有 Task，固定可供求职页面复用的原生运行边界。 | 现有 Expo／React Native 移动运行面、iOS／Android Task 入口及平台 Adapter；不改 Career 后端业务规则 |
| 03 | [#141 · 个人求职空间与已确认基础档案](https://github.com/1123786563/WeKnora-fork01/issues/141) · [Brief](tickets-draft/03-personal-career-space.md) | None (can start immediately)。 | 求职者用 WeKnora 账号进入单成员个人求职空间，并确认毕业时间、学历、城市与意向；这些确认事实成为后续判断的唯一输入。 | Career Office、Career Desk、Identity；Web 首条完整路径 |
| 04 | [#142 · 固定版本 Artifact 授权下载](https://github.com/1123786563/WeKnora-fork01/issues/142) · [Brief](tickets-draft/04-versioned-artifact-grant.md) | None (can start immediately)。 | 已授权用户以固定产物身份取得短时下载授权，旧链接在撤销或删除后失效，为求职材料版本提供通用安全能力。 | Workbench Artifact；现有 Task 产物路径 |
| 05 | [#144 · Expo 移动端 Task Office 可进入与恢复](https://github.com/1123786563/WeKnora-fork01/issues/144) · [Brief](tickets-draft/05-native-task-office-entry.md) | T01、T02。 | iOS、Android、鸿蒙用户可在已授权的活动空间中打开、恢复一个现有 Task，并看到服务端权威状态。 | Expo 移动 Task Office 与 Mobile Runtime |
| 06 | [#148 · 微信小程序（Taro 4 + TDesign Miniprogram）现有 Task 产物下载](https://github.com/1123786563/WeKnora-fork01/issues/148) · [Brief](tickets-draft/06-mini-task-artifact-download.md) | None (can start immediately)。 | 微信小程序（Taro 4 + TDesign Miniprogram）用户可从已拥有的现有 Task 获取真实 PDF 或 DOCX，而不被迫转到 Web。 | 微信小程序（Taro 4 + TDesign Miniprogram） Task 呈现与 Workbench 下载 Adapter |
| 07 | [#147 · 简历上传、逐步建档与事实确认](https://github.com/1123786563/WeKnora-fork01/issues/147) · [Brief](tickets-draft/07-resume-intake-confirmation.md) | T03。 | 求职者上传已有简历或逐步填写，审阅带来源的事实提案后逐项确认或拒绝。 | Career Profile Intake；Web 建档页 |
| 08 | [#146 · 粘贴 JD 形成岗位机会与快照](https://github.com/1123786563/WeKnora-fork01/issues/146) · [Brief](tickets-draft/08-paste-jd-snapshot.md) | T03。 | 用户粘贴招聘要求，得到可打开的岗位机会、原文快照、来源和获取时间。 | Career Opportunity；Web C 对话入口 |
| 09 | [#149 · 链接导入与不完整来源回退](https://github.com/1123786563/WeKnora-fork01/issues/149) · [Brief](tickets-draft/09-url-import-incomplete-fallback.md) | T08。 | 用户粘贴岗位链接时能看到来源取得状态；页面无法获得完整 JD 时清楚请求补充文本。 | Career Source Adapter；Web 对话导入 |
| 10 | [#150 · 三值资格与证据化匹配](https://github.com/1123786563/WeKnora-fork01/issues/150) · [Brief](tickets-draft/10-eligibility-and-evidence.md) | T08。 | 用户先看到硬性资格结论，再看到技能、项目与意向的证据化匹配和待核实项。 | Career Evaluation；Web C 岗位结果卡 |
| 11 | [#152 · C 对话入口的一次性真实找岗](https://github.com/1123786563/WeKnora-fork01/issues/152) · [Brief](tickets-draft/11-one-shot-conversational-search.md) | T09、T10。 | 用户用一句话寻找岗位，从至少一个核验来源得到带资格、证据、来源和下一步动作的真实结果。 | Career Search、Agent Runtime、Workbench Task；Web C 主界面 |
| 12 | [#151 · 多来源去重、岗位更新与覆盖说明](https://github.com/1123786563/WeKnora-fork01/issues/151) · [Brief](tickets-draft/12-source-reconciliation.md) | T11。 | 用户能区分同一岗位的多个来源、不同招聘批次及后来变化的 JD。 | Career Source Observation；Web 岗位证据视图 |
| 13 | [#154 · 可控的持续找岗规则](https://github.com/1123786563/WeKnora-fork01/issues/154) · [Brief](tickets-draft/13-controlled-search-rule.md) | T11。 | 用户明确开启持续找岗，能查看、修改、暂停与恢复条件和频率。 | Career Search Rule、Workbench admission；Web 规则页 |
| 14 | [#155 · 每份求职申请关联独立 Task](https://github.com/1123786563/WeKnora-fork01/issues/155) · [Brief](tickets-draft/14-application-owned-task.md) | T10。 | 用户为一个确定岗位及招聘批次建立独立求职申请和 Task。 | Career Application 与 Workbench Task；Web 申请入口 |
| 15 | [#153 · 可信结构化材料与不可变版本](https://github.com/1123786563/WeKnora-fork01/issues/153) · [Brief](tickets-draft/15-truthful-structured-material.md) | T07、T14。 | 用户依据已确认档案和固定岗位快照生成、编辑、审阅简历及网申问答正文。 | Career Material 与 Agent Runtime；Web 材料编辑 |
| 16 | [#158 · 同版 PDF/DOCX 生成与验证](https://github.com/1123786563/WeKnora-fork01/issues/158) · [Brief](tickets-draft/16-validated-pdf-docx.md) | T04、T15。 | 用户从一份已确认材料版本取得内容一致的 PDF 和可编辑 DOCX。 | Career Material Export、Workbench Artifact；Web 文件获取 |
| 17 | [#157 · 申请进展事件与阶段投影](https://github.com/1123786563/WeKnora-fork01/issues/157) · [Brief](tickets-draft/17-progress-event-timeline.md) | T14。 | 用户按日期记录测评、笔试、面试、Offer、拒绝、撤回和更正，查看当前阶段及完整历史。 | Career Application Event；Web 进展视图 |
| 18 | [#159 · 本人投递确认与实际材料绑定](https://github.com/1123786563/WeKnora-fork01/issues/159) · [Brief](tickets-draft/18-manual-submission-binding.md) | T16、T17。 | 用户在招聘平台亲自投递后，记录渠道、时间和实际使用的材料版本。 | Career Application Submission；Web 申请详情 |
| 19 | [#156 · 求职信与基于投递版的面试准备](https://github.com/1123786563/WeKnora-fork01/issues/156) · [Brief](tickets-draft/19-cover-letter-interview-prep.md) | T18。 | 用户按需起草求职信，并按招聘方实际收到的材料准备面试。 | Career Preparation、Agent Runtime；Web 按需入口 |
| 20 | [#160 · 站内待办与隐私通知](https://github.com/1123786563/WeKnora-fork01/issues/160) · [Brief](tickets-draft/20-actionable-private-reminders.md) | T13、T17。 | 用户能看到申请下一步和新岗位待办，并按意愿接收平台提示。 | Career Attention、Workbench Inbox；Web 待办 |
| 21 | [#161 · 搜索与生成的额度预估及阻断](https://github.com/1123786563/WeKnora-fork01/issues/161) · [Brief](tickets-draft/21-usage-preview-quota.md) | T13、T15。 | 用户在持续扫描和高用量材料生成前看到预估，额度不足时保留历史访问。 | Career Usage 与 Workbench admission；Web 额度提示 |
| 22 | [#162 · 求职数据导出与完整删除](https://github.com/1123786563/WeKnora-fork01/issues/162) · [Brief](tickets-draft/22-career-export-deletion.md) | T16、T17。 | 用户能导出完整求职事实与历史，并按清楚的保留规则删除个人求职资料。 | Career Data Lifecycle、Identity、Workbench；Web 隐私入口 |
| 23 | [#163 · Expo 移动端 C 找岗、建档与分享导入](https://github.com/1123786563/WeKnora-fork01/issues/163) · [Brief](tickets-draft/23-native-conversational-discovery.md) | T05、T11。 | iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）完成档案确认、分享或粘贴导入、对话找岗与岗位评估。 | Expo 移动 Career 呈现与分享 Adapter；复用 Career Desk |
| 24 | [#164 · 微信小程序（Taro 4 + TDesign Miniprogram）C 找岗、建档与分享导入](https://github.com/1123786563/WeKnora-fork01/issues/164) · [Brief](tickets-draft/24-mini-conversational-discovery.md) | T06、T11。 | 微信小程序（Taro 4 + TDesign Miniprogram）用户绑定同一 WeKnora 用户，完成档案确认、分享或粘贴导入、对话找岗与评估。 | 微信小程序（Taro 4 + TDesign Miniprogram） Career 呈现与微信分享 Adapter；复用 Career Desk |
| 25 | [#165 · Expo 移动端申请、材料与本人投递](https://github.com/1123786563/WeKnora-fork01/issues/165) · [Brief](tickets-draft/25-native-application-material-submission.md) | T18、T23。 | iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）从岗位建立申请，编辑结构化材料、比较版本、取得 PDF/DOCX 并记录本人投递。 | Expo 移动 Career 申请和材料呈现；复用 Career Desk |
| 26 | [#166 · 微信小程序（Taro 4 + TDesign Miniprogram）申请、材料与本人投递](https://github.com/1123786563/WeKnora-fork01/issues/166) · [Brief](tickets-draft/26-mini-application-material-submission.md) | T06、T18、T24。 | 微信小程序（Taro 4 + TDesign Miniprogram）用户从岗位建立申请，编辑、比较、下载两种文件并记录本人投递。 | 微信小程序（Taro 4 + TDesign Miniprogram） Career 申请和材料呈现；复用 Career Desk |
| 27 | [#167 · Expo 移动端申请时间线与按需准备](https://github.com/1123786563/WeKnora-fork01/issues/167) · [Brief](tickets-draft/27-native-progress-preparation.md) | T19、T25。 | iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）可记录、更正并筛选进展，按实际投递版本准备面试与求职信。 | Expo 移动 Career 进展与准备呈现 |
| 28 | [#169 · 微信小程序（Taro 4 + TDesign Miniprogram）申请时间线与按需准备](https://github.com/1123786563/WeKnora-fork01/issues/169) · [Brief](tickets-draft/28-mini-progress-preparation.md) | T19、T26。 | 微信小程序（Taro 4 + TDesign Miniprogram）用户可记录、更正并筛选进展，按实际投递版本准备面试与求职信。 | 微信小程序（Taro 4 + TDesign Miniprogram） Career 进展与准备呈现 |
| 29 | [#168 · Expo 移动端持续规则、额度与提醒](https://github.com/1123786563/WeKnora-fork01/issues/168) · [Brief](tickets-draft/29-native-rules-usage-reminders.md) | T13、T20、T21、T23。 | iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）能管理持续找岗、查看额度与站内待办，并自主选择推送。 | Expo 移动 Career 规则、用量与通知 Adapter |
| 30 | [#170 · 微信小程序（Taro 4 + TDesign Miniprogram）持续规则、额度与提醒](https://github.com/1123786563/WeKnora-fork01/issues/170) · [Brief](tickets-draft/30-mini-rules-usage-reminders.md) | T13、T20、T21、T24。 | 微信小程序（Taro 4 + TDesign Miniprogram）用户能管理持续找岗、查看额度与站内待办，并自主选择订阅消息。 | 微信小程序（Taro 4 + TDesign Miniprogram） Career 规则、用量与订阅消息 Adapter |
| 31 | [#171 · Expo 移动端导出与删除](https://github.com/1123786563/WeKnora-fork01/issues/171) · [Brief](tickets-draft/31-native-export-deletion.md) | T22、T23。 | iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）下载完整求职数据并发起受权删除，旧内容随授权失效。 | Expo 移动 Career 隐私与文件 Adapter |
| 32 | [#173 · 微信小程序（Taro 4 + TDesign Miniprogram）导出与删除](https://github.com/1123786563/WeKnora-fork01/issues/173) · [Brief](tickets-draft/32-mini-export-deletion.md) | T22、T24。 | 微信小程序（Taro 4 + TDesign Miniprogram）用户下载完整求职数据并发起受权删除，旧内容随授权失效。 | 微信小程序（Taro 4 + TDesign Miniprogram） Career 隐私与文件 Adapter |
| 33 | [#172 · 五环境真实闭环与发布门槛](https://github.com/1123786563/WeKnora-fork01/issues/172) · [Brief](tickets-draft/33-cross-platform-launch-gate.md) | T12、T25、T26、T27、T28、T29、T30、T31、T32。 | Web、iOS、Android、鸿蒙和微信小程序分别从建档走到找岗、材料、本人投递、跟进、导出与删除，形成发布证据。 | 跨端验收与发布负责人；不代替前置 Ticket 的 Review |

## 理论并行前沿

先满足单项真实阻塞边即可启动，不用等待整行全部完成；下表是按依赖拓扑分层的最大并行候选。

| 层 | 可同时就绪的 Ticket | 共享状态注意 |
| --- | --- | --- |
| G0 | 01、02、03、04、06 | 鸿蒙原生可行性、Expo 双端、Career/Identity、Artifact、小程序文件入口可分开；T01 和 T02 的 Mobile Runtime 变更由主控串行集成。 |
| G1 | 05、07、08 | 使用独立 Worktree；对共享接口、导航、迁移、构建目录与测试资源先冻结所有权。 |
| G2 | 09、10 | 使用独立 Worktree；对共享接口、导航、迁移、构建目录与测试资源先冻结所有权。 |
| G3 | 11、14 | 使用独立 Worktree；对共享接口、导航、迁移、构建目录与测试资源先冻结所有权。 |
| G4 | 12、13、15、17、23、24 | 使用独立 Worktree；对共享接口、导航、迁移、构建目录与测试资源先冻结所有权。 |
| G5 | 16、20、21 | 使用独立 Worktree；对共享接口、导航、迁移、构建目录与测试资源先冻结所有权。 |
| G6 | 18、22、29、30 | 使用独立 Worktree；对共享接口、导航、迁移、构建目录与测试资源先冻结所有权。 |
| G7 | 19、25、26、31、32 | 使用独立 Worktree；对共享接口、导航、迁移、构建目录与测试资源先冻结所有权。 |
| G8 | 27、28 | 使用独立 Worktree；对共享接口、导航、迁移、构建目录与测试资源先冻结所有权。 |
| G9 | 33 | 五环境统一收口；鸿蒙闸口未通过时不能宣称完整发布。 |

## 父规格故事覆盖

| 父规格用户故事 | 覆盖 Ticket |
| --- | --- |
| 1～6：统一身份、个人空间、建档与事实确认 | 01、02、03、07、23、24 |
| 7～9：链接、JD 与分享导入 | 08、09、23、24 |
| 10～16：指令找岗、持续规则、来源与更新 | 09、11、12、13、23、24 |
| 17～21：硬条件、匹配证据与用户覆盖 | 10、14、23、24 |
| 22～23：独立求职申请与固定快照 | 14、25、26 |
| 24～29：可信材料、编辑、版本、PDF/DOCX | 07、15、16、25、26 |
| 30～35：求职信、本人投递、进展事件 | 17、18、19、27、28 |
| 36～39：提醒、隐私通知与额度 | 13、20、21、29、30 |
| 40～43：导出删除、恢复、隔离与审计 | 03、04、05、12、14、16、17、22、31、32、33 |

## 对 Code Agent 的交接

1. 读取父规格 #140、批准规格、CONTEXT.md 与 ADR-0015～0018；用真实 Issue 关系重建 DAG。
2. T01 鸿蒙原生兼容性与 T02 Expo iOS／Android Task 可并行；Career/Identity、Artifact、小程序 TDesign 文件入口也可独立开工。鸿蒙若无可维护原生路径，只阻塞鸿蒙及含鸿蒙验收的移动任务，保留证据并请求技术裁定。
3. 每个 Ticket 固定端到端验收、消费/交付合同、文件所有权、验证命令、独立 Review 和恢复检查点。
4. 并行实现流各用独立 Worktree；共享迁移、导航、构建目录、端口和测试数据库视为写冲突。
5. 以 T33 的 Web、iOS、Android、鸿蒙原生和微信小程序五环境证据结束；原型 C 提供信息架构与 TDesign 配色。
