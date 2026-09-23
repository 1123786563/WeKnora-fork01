# T14：每份求职申请关联独立 Task

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#155](https://github.com/1123786563/WeKnora-fork01/issues/155)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户为一个确定岗位及招聘批次建立独立求职申请和 Task。

## Acceptance criteria

- [ ] 申请固定所用岗位快照、档案版本与资格评估
- [ ] 硬条件不符可显式继续，但警示常驻且不计合格申请指标
- [ ] 跨模块 Task 建立未知时保留 linking 状态并用原请求 ID 对账
- [ ] 同岗不同批次可分别申请，重复请求不创建第二个 Task

## Blocked by

T10。

## Ownership and interfaces

- 主责边界：Career Application 与 Workbench Task；Web 申请入口。
- 消费接口：岗位快照、资格评估与 Workbench Task 创建。
- 交付接口：求职申请、Task 关联及未知结果对账。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 公共 seam 测试覆盖正常、超时、重复、跨批次和警示。
- Web E2E 从岗位卡打开独立申请。

## Failure and unknown results

Task 创建失败保留可恢复申请，不将未关联状态显示为就绪。

## Parallel scheduling

- 理论前沿：G3；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
