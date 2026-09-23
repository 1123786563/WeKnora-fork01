# T05：Expo 移动端 Task Office 可进入与恢复

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

iOS、Android、鸿蒙用户可在已授权的活动空间中打开、恢复一个现有 Task，并看到服务端权威状态。

## Acceptance criteria

- [ ] Expo 移动运行面替换占位首页，提供可用 Task 入口
- [ ] 刷新或重启后不创建新 Task，只恢复原 Task
- [ ] 活动空间切换时旧 Task 数据和迟到响应不显示
- [ ] 离线显示受控缓存或明确不可用状态，不暗示任务成功

## Blocked by

T01、T02。

## Ownership and interfaces

- 主责边界：Expo 移动 Task Office 与 Mobile Runtime。
- 消费接口：Mobile Runtime 的活动空间租约与 Task 公共视图。
- 交付接口：Expo 移动 Task 进入、恢复与 scope 失效呈现。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Mobile Runtime/Task Office 公共 seam 测试覆盖恢复和 scope 失效。
- iOS、Android 开发构建与鸿蒙可用构建各演示一条现有 Task；鸿蒙不支持时按 T01 判定阻塞。

## Failure and unknown results

网络超时保持未知并允许重新同步，不自动重复执行。

## Parallel scheduling

- 理论前沿：G1；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
