# T27：Expo 移动端申请时间线与按需准备

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#167](https://github.com/1123786563/WeKnora-fork01/issues/167)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）可记录、更正并筛选进展，按实际投递版本准备面试与求职信。

## Acceptance criteria

- [ ] 事件历史不可覆盖，阶段与 Web 同源
- [ ] 跨端新增事件后 Expo 移动端重新同步显示新阶段
- [ ] 面试准备显示所引用的实际投递版
- [ ] 未知版本不推断招聘方看到最新材料

## Blocked by

T19、T25。

## Ownership and interfaces

- 主责边界：Expo 移动 Career 进展与准备呈现。
- 消费接口：进展事件、阶段投影、面试准备和原生申请页。
- 交付接口：原生时间线、纠错及按需准备。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 事件投影与版本引用测试。
- iOS、Android、鸿蒙各演示投递后面试与更正。

## Failure and unknown results

离线事件草稿需联网后确认，未知回执不重复提交。

## Parallel scheduling

- 理论前沿：G8；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
