# T11：C 对话入口的一次性真实找岗

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#152](https://github.com/1123786563/WeKnora-fork01/issues/152)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户用一句话寻找岗位，从至少一个核验来源得到带资格、证据、来源和下一步动作的真实结果。

## Acceptance criteria

- [ ] 找岗默认一次性 Task，指令不自动创建持续规则
- [ ] 首批来源明确实际可用方式与城市覆盖，不宣称全国完整
- [ ] 结果列出检查时间、资格状态、原始链接及不确定性
- [ ] 执行失败、未知回执和额度拒绝均可恢复，不复制搜索 Task

## Blocked by

T09、T10。

## Ownership and interfaces

- 主责边界：Career Search、Agent Runtime、Workbench Task；Web C 主界面。
- 消费接口：岗位来源 Adapter、评估视图、Workbench Task 准入。
- 交付接口：一次性找岗回执和 C 对话结果视图。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- 固定来源与真实允许来源各跑一次合同场景。
- Web E2E 从指令到岗位详情再到恢复历史结果。

## Failure and unknown results

外部来源不可用时说明范围与失败，不以演示数据替代。

## Parallel scheduling

- 理论前沿：G3；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
