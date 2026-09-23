# T17：申请进展事件与阶段投影

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户按日期记录测评、笔试、面试、Offer、拒绝、撤回和更正，查看当前阶段及完整历史。

## Acceptance criteria

- [ ] 事件追加保存，纠错追加更正事件而非覆盖原记录
- [ ] 当前阶段由已确认事件投影，重新打开结果一致
- [ ] 同一请求重放不出现第二个事件
- [ ] 事件来源和确认者可追溯，跨申请记录不串联

## Blocked by

T14。

## Ownership and interfaces

- 主责边界：Career Application Event；Web 进展视图。
- 消费接口：求职申请与修订回执。
- 交付接口：追加进展事件与当前阶段投影。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 合同测试覆盖事件顺序、更正、幂等、阶段投影。
- Web E2E 录入面试后筛选并查看历史。

## Failure and unknown results

写入结果未知先查回执，不用新 ID 盲目重试。

## Parallel scheduling

- 理论前沿：G4；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
