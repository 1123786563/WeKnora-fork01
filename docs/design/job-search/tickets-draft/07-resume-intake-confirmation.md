# T07：简历上传、逐步建档与事实确认

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#147](https://github.com/1123786563/WeKnora-fork01/issues/147)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

求职者上传已有简历或逐步填写，审阅带来源的事实提案后逐项确认或拒绝。

## Acceptance criteria

- [ ] 教育、经历、项目、技能、成果数字与证书均有来源和确认状态
- [ ] 拒绝或未确认的提案不能用于评估或材料生成
- [ ] 上传失败不覆盖已确认档案，修改保留版本和来源
- [ ] 送模型前遮蔽与当前目的无关的证件号码等字段

## Blocked by

T03。

## Ownership and interfaces

- 主责边界：Career Profile Intake；Web 建档页。
- 消费接口：个人求职空间、事实确认意图。
- 交付接口：带来源的事实提案与确认版本。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 测试改变提案确认状态并观察后续可用事实。
- Web 用含缺失与冲突事实的样例简历完成审阅。

## Failure and unknown results

抽取失败允许手动继续；模型输出不能直接写权威事实。

## Parallel scheduling

- 理论前沿：G1；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
