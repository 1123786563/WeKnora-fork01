# T22：求职数据导出与完整删除

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户能导出完整求职事实与历史，并按清楚的保留规则删除个人求职资料。

## Acceptance criteria

- [ ] 导出包含档案、原岗位快照、申请事件和材料版本
- [ ] 删除前说明空间内与外部平台资料的边界
- [ ] 删除使旧 Task、Artifact 授权和客户端缓存不可再访问
- [ ] 若保留策略要求延迟或例外，显示范围与状态

## Blocked by

T16、T17。

## Ownership and interfaces

- 主责边界：Career Data Lifecycle、Identity、Workbench；Web 隐私入口。
- 消费接口：求职档案、快照、申请、材料及授权。
- 交付接口：完整导出包、删除回执与授权撤销。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office/Identity/Workbench 联合合同测试导出完整性和旧链接失效。
- Web E2E 导出后发起删除并检查不可读取。

## Failure and unknown results

局部删除失败保留可恢复状态与审计，不声称已完全删除。

## Parallel scheduling

- 理论前沿：G6；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
