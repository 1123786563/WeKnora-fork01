# 求职专业 Agent Ticket Brief 索引

状态：33 份 Ticket 已发布为 GitHub 子 Issue，全部带 ready-for-agent 标签和原生阻塞关系。父规格：[#140](https://github.com/1123786563/WeKnora-fork01/issues/140)。

技术边界：Expo iOS／Android、鸿蒙原生兼容性闸口、Taro 4 + TDesign Miniprogram 微信小程序；详细决策见 [ADR-0018](../../../adr/0018-expo-tdesign-career-clients.md)。

## 并行前沿

| 层 | Ticket |
| --- | --- |
| G0 | T01、T02、T03、T04、T06 |
| G1 | T05、T07、T08 |
| G2 | T09、T10 |
| G3 | T11、T14 |
| G4 | T12、T13、T15、T17、T23、T24 |
| G5 | T16、T20、T21 |
| G6 | T18、T22、T29、T30 |
| G7 | T19、T25、T26、T31、T32 |
| G8 | T27、T28 |
| G9 | T33 |

## Ticket 文件

- [T01：Expo 鸿蒙原生兼容性闸口](01-expo-harmony-compatibility-gate.md) · [GitHub #143](https://github.com/1123786563/WeKnora-fork01/issues/143) — 阻塞：None (can start immediately)。
- [T02：Expo iOS／Android 受认证 Task 薄切片](02-expo-ios-android-task-proof.md) · [GitHub #145](https://github.com/1123786563/WeKnora-fork01/issues/145) — 阻塞：None (can start immediately)。
- [T03：个人求职空间与已确认基础档案](03-personal-career-space.md) · [GitHub #141](https://github.com/1123786563/WeKnora-fork01/issues/141) — 阻塞：None (can start immediately)。
- [T04：固定版本 Artifact 授权下载](04-versioned-artifact-grant.md) · [GitHub #142](https://github.com/1123786563/WeKnora-fork01/issues/142) — 阻塞：None (can start immediately)。
- [T05：Expo 移动端 Task Office 可进入与恢复](05-native-task-office-entry.md) · [GitHub #144](https://github.com/1123786563/WeKnora-fork01/issues/144) — 阻塞：T01、T02。
- [T06：微信小程序（Taro 4 + TDesign Miniprogram）现有 Task 产物下载](06-mini-task-artifact-download.md) · [GitHub #148](https://github.com/1123786563/WeKnora-fork01/issues/148) — 阻塞：None (can start immediately)。
- [T07：简历上传、逐步建档与事实确认](07-resume-intake-confirmation.md) · [GitHub #147](https://github.com/1123786563/WeKnora-fork01/issues/147) — 阻塞：T03。
- [T08：粘贴 JD 形成岗位机会与快照](08-paste-jd-snapshot.md) · [GitHub #146](https://github.com/1123786563/WeKnora-fork01/issues/146) — 阻塞：T03。
- [T09：链接导入与不完整来源回退](09-url-import-incomplete-fallback.md) · [GitHub #149](https://github.com/1123786563/WeKnora-fork01/issues/149) — 阻塞：T08。
- [T10：三值资格与证据化匹配](10-eligibility-and-evidence.md) · [GitHub #150](https://github.com/1123786563/WeKnora-fork01/issues/150) — 阻塞：T08。
- [T11：C 对话入口的一次性真实找岗](11-one-shot-conversational-search.md) · [GitHub #152](https://github.com/1123786563/WeKnora-fork01/issues/152) — 阻塞：T09、T10。
- [T12：多来源去重、岗位更新与覆盖说明](12-source-reconciliation.md) · [GitHub #151](https://github.com/1123786563/WeKnora-fork01/issues/151) — 阻塞：T11。
- [T13：可控的持续找岗规则](13-controlled-search-rule.md) · [GitHub #154](https://github.com/1123786563/WeKnora-fork01/issues/154) — 阻塞：T11。
- [T14：每份求职申请关联独立 Task](14-application-owned-task.md) · [GitHub #155](https://github.com/1123786563/WeKnora-fork01/issues/155) — 阻塞：T10。
- [T15：可信结构化材料与不可变版本](15-truthful-structured-material.md) · [GitHub #153](https://github.com/1123786563/WeKnora-fork01/issues/153) — 阻塞：T07、T14。
- [T16：同版 PDF/DOCX 生成与验证](16-validated-pdf-docx.md) · [GitHub #158](https://github.com/1123786563/WeKnora-fork01/issues/158) — 阻塞：T04、T15。
- [T17：申请进展事件与阶段投影](17-progress-event-timeline.md) · [GitHub #157](https://github.com/1123786563/WeKnora-fork01/issues/157) — 阻塞：T14。
- [T18：本人投递确认与实际材料绑定](18-manual-submission-binding.md) · [GitHub #159](https://github.com/1123786563/WeKnora-fork01/issues/159) — 阻塞：T16、T17。
- [T19：求职信与基于投递版的面试准备](19-cover-letter-interview-prep.md) · [GitHub #156](https://github.com/1123786563/WeKnora-fork01/issues/156) — 阻塞：T18。
- [T20：站内待办与隐私通知](20-actionable-private-reminders.md) · [GitHub #160](https://github.com/1123786563/WeKnora-fork01/issues/160) — 阻塞：T13、T17。
- [T21：搜索与生成的额度预估及阻断](21-usage-preview-quota.md) · [GitHub #161](https://github.com/1123786563/WeKnora-fork01/issues/161) — 阻塞：T13、T15。
- [T22：求职数据导出与完整删除](22-career-export-deletion.md) · [GitHub #162](https://github.com/1123786563/WeKnora-fork01/issues/162) — 阻塞：T16、T17。
- [T23：Expo 移动端 C 找岗、建档与分享导入](23-native-conversational-discovery.md) · [GitHub #163](https://github.com/1123786563/WeKnora-fork01/issues/163) — 阻塞：T05、T11。
- [T24：微信小程序（Taro 4 + TDesign Miniprogram）C 找岗、建档与分享导入](24-mini-conversational-discovery.md) · [GitHub #164](https://github.com/1123786563/WeKnora-fork01/issues/164) — 阻塞：T06、T11。
- [T25：Expo 移动端申请、材料与本人投递](25-native-application-material-submission.md) · [GitHub #165](https://github.com/1123786563/WeKnora-fork01/issues/165) — 阻塞：T18、T23。
- [T26：微信小程序（Taro 4 + TDesign Miniprogram）申请、材料与本人投递](26-mini-application-material-submission.md) · [GitHub #166](https://github.com/1123786563/WeKnora-fork01/issues/166) — 阻塞：T06、T18、T24。
- [T27：Expo 移动端申请时间线与按需准备](27-native-progress-preparation.md) · [GitHub #167](https://github.com/1123786563/WeKnora-fork01/issues/167) — 阻塞：T19、T25。
- [T28：微信小程序（Taro 4 + TDesign Miniprogram）申请时间线与按需准备](28-mini-progress-preparation.md) · [GitHub #169](https://github.com/1123786563/WeKnora-fork01/issues/169) — 阻塞：T19、T26。
- [T29：Expo 移动端持续规则、额度与提醒](29-native-rules-usage-reminders.md) · [GitHub #168](https://github.com/1123786563/WeKnora-fork01/issues/168) — 阻塞：T13、T20、T21、T23。
- [T30：微信小程序（Taro 4 + TDesign Miniprogram）持续规则、额度与提醒](30-mini-rules-usage-reminders.md) · [GitHub #170](https://github.com/1123786563/WeKnora-fork01/issues/170) — 阻塞：T13、T20、T21、T24。
- [T31：Expo 移动端导出与删除](31-native-export-deletion.md) · [GitHub #171](https://github.com/1123786563/WeKnora-fork01/issues/171) — 阻塞：T22、T23。
- [T32：微信小程序（Taro 4 + TDesign Miniprogram）导出与删除](32-mini-export-deletion.md) · [GitHub #173](https://github.com/1123786563/WeKnora-fork01/issues/173) — 阻塞：T22、T24。
- [T33：五环境真实闭环与发布门槛](33-cross-platform-launch-gate.md) · [GitHub #172](https://github.com/1123786563/WeKnora-fork01/issues/172) — 阻塞：T12、T25、T26、T27、T28、T29、T30、T31、T32。

同一前沿仅表示依赖就绪；实际并行仍需独立 Worktree、文件所有权和共享接口冻结。鸿蒙原生闸口的失败必须被保留为阻塞证据。
