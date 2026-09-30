# T31：Expo 移动端导出与删除

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）下载完整求职数据并发起受权删除，旧内容随授权失效。

## Acceptance criteria

- [ ] 导出包包含档案、岗位快照、申请事件和材料
- [ ] 删除说明外部平台内容不受影响
- [ ] 完成删除后缓存不可读，旧材料下载授权失效
- [ ] 权限或保留策略阻断时显示确切状态

## Blocked by

T22、T23。

## Ownership and interfaces

- 主责边界：Expo 移动 Career 隐私与文件 Adapter。
- 消费接口：完整导出与删除合同、原生空间作用域。
- 交付接口：原生数据迁出与删除后的本地清理。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk scope/删除测试。
- iOS、Android、鸿蒙各演示导出、删除和旧链接拒绝。

## Failure and unknown results

下载中断不显示完整导出；删除未知先对账。

## Parallel scheduling

- 理论前沿：G7；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
