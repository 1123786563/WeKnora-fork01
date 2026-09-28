# T13：可控的持续找岗规则

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#154](https://github.com/1123786563/WeKnora-fork01/issues/154)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户明确开启持续找岗，能查看、修改、暂停与恢复条件和频率。

## Acceptance criteria

- [ ] 未开启不后台运行，暂停后下一次触发被取消或不再入队
- [ ] 启用前展示条件、频率与预计消耗
- [ ] 同一新岗位只产生一个发现待办
- [ ] 重复触发和结果未知经同一请求身份对账

## Blocked by

T11。

## Ownership and interfaces

- 主责边界：Career Search Rule、Workbench admission；Web 规则页。
- 消费接口：一次性找岗能力与 Workbench admission。
- 交付接口：可暂停的规则与幂等触发回执。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 定时触发测试覆盖开关、重复与未知状态。
- Web E2E 修改规则后显示下次运行计划。

## Failure and unknown results

预算不足或来源不完整形成可见状态，不静默跳过。

## Parallel scheduling

- 理论前沿：G4；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
