# T20：站内待办与隐私通知

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户能看到申请下一步和新岗位待办，并按意愿接收平台提示。

## Acceptance criteria

- [ ] 同一机会和事件只产生一条待办
- [ ] 站内记录是权威，推送只提醒重新同步
- [ ] 通知正文无公司、岗位、面试细节
- [ ] 取消订阅后不再发送，已存在待办仍可读取

## Blocked by

T13、T17。

## Ownership and interfaces

- 主责边界：Career Attention、Workbench Inbox；Web 待办。
- 消费接口：持续规则发现结果与申请进展事件。
- 交付接口：去重站内待办和最小内容通知事实。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Inbox/通知契约测试去重、隐私文案和取消订阅。
- Web E2E 从待办进入权威申请。

## Failure and unknown results

通知送达失败不丢站内事实，也不将推送视为状态更新。

## Parallel scheduling

- 理论前沿：G5；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
