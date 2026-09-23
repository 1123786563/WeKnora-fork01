# T19：求职信与基于投递版的面试准备

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#156](https://github.com/1123786563/WeKnora-fork01/issues/156)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户按需起草求职信，并按招聘方实际收到的材料准备面试。

## Acceptance criteria

- [ ] 求职信与回答只引用已确认事实和岗位快照
- [ ] 面试准备优先固定实际投递版本，版本未知必须提示
- [ ] 生成结果可审阅、修订且有来源
- [ ] 不自动发送或替用户承诺事实

## Blocked by

T18。

## Ownership and interfaces

- 主责边界：Career Preparation、Agent Runtime；Web 按需入口。
- 消费接口：确认事实、岗位快照与实际投递版。
- 交付接口：可审阅求职信及面试准备草稿。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 测试证明 V2 已投递时引用 V2、未知时不猜 V3。
- Web E2E 查看来源并修订草稿。

## Failure and unknown results

模型失败保留请求和可恢复状态，不展示空白成功产物。

## Parallel scheduling

- 理论前沿：G7；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
