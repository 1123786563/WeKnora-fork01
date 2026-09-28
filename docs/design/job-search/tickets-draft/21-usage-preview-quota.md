# T21：搜索与生成的额度预估及阻断

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#161](https://github.com/1123786563/WeKnora-fork01/issues/161)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户在持续扫描和高用量材料生成前看到预估，额度不足时保留历史访问。

## Acceptance criteria

- [ ] 执行前展示将消耗的额度与触发条件
- [ ] 超额阻止新的收费 Run 而非删除既有档案或申请
- [ ] 重复请求不会重复预占或收费
- [ ] 付费状态不改变岗位排序或资格判定

## Blocked by

T13、T15。

## Ownership and interfaces

- 主责边界：Career Usage 与 Workbench admission；Web 额度提示。
- 消费接口：持续找岗、材料生成与 Workbench admission。
- 交付接口：预估用量、额度拒绝与只读保留行为。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- admission 合同测试额度边界、重复请求和只读访问。
- Web E2E 模拟超额后仍能打开旧申请。

## Failure and unknown results

预估不可得时不得先执行后补报，显示可理解的不可用原因。

## Parallel scheduling

- 理论前沿：G5；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
