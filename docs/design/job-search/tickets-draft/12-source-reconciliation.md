# T12：多来源去重、岗位更新与覆盖说明

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#151](https://github.com/1123786563/WeKnora-fork01/issues/151)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户能区分同一岗位的多个来源、不同招聘批次及后来变化的 JD。

## Acceptance criteria

- [ ] 仅有充分岗位编号、企业、地点与批次证据时合并
- [ ] 不确定重复并列，所有原始链接和检查时间保留
- [ ] 过期、下架和要求变化显式标注，旧申请仍展示旧快照
- [ ] 可查看已接入来源和实际覆盖城市

## Blocked by

T11。

## Ownership and interfaces

- 主责边界：Career Source Observation；Web 岗位证据视图。
- 消费接口：来源观察与找岗结果。
- 交付接口：谨慎去重、更新差异与来源覆盖视图。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 合同测试覆盖同岗双来源、同名不同批次和 JD 更新。
- Web 展示变化前后差异且历史可访问。

## Failure and unknown results

来源检查失败保留最后成功观察并显示陈旧时间。

## Parallel scheduling

- 理论前沿：G4；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
