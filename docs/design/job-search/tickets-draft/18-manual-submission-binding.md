# T18：本人投递确认与实际材料绑定

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#159](https://github.com/1123786563/WeKnora-fork01/issues/159)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户在招聘平台亲自投递后，记录渠道、时间和实际使用的材料版本。

## Acceptance criteria

- [ ] 系统不点击外部提交或自动发信
- [ ] 实际版本可选择已验证版，未知时明确记录未确认
- [ ] 投递事件绑定渠道、时间、版本或未知标记
- [ ] 重复确认不生成第二次投递记录，后续准备不引用错误版本

## Blocked by

T16、T17。

## Ownership and interfaces

- 主责边界：Career Application Submission；Web 申请详情。
- 消费接口：验证通过的材料版本与申请进展事件。
- 交付接口：本人投递事件及实际版本绑定。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office seam 测试已知/未知版本与重复回执。
- Web E2E 记录投递并回看版本及时间线。

## Failure and unknown results

外部提交是否成功由用户确认；产品不得从点击下载推断投递。

## Parallel scheduling

- 理论前沿：G6；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
