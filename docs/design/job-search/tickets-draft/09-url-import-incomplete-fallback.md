# T09：链接导入与不完整来源回退

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户粘贴岗位链接时能看到来源取得状态；页面无法获得完整 JD 时清楚请求补充文本。

## Acceptance criteria

- [ ] 只接入经核验允许读取的来源，不绕过登录或反爬限制
- [ ] 保留原链接、取得时间、完整性及失败原因
- [ ] 摘要不足以推断届别、学历等硬条件
- [ ] 用户补充 JD 后产生新的固定快照，原始来源仍可追溯

## Blocked by

T08。

## Ownership and interfaces

- 主责边界：Career Source Adapter；Web 对话导入。
- 消费接口：岗位快照与导入意图。
- 交付接口：来源完整性分类、原链接保留和 JD 回退。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- 固定来源响应契约覆盖完整、登录阻断、摘要、不存在和超时。
- Web 演示链接失败后粘贴 JD 成功。

## Failure and unknown results

来源未知时不标为已核验；重试不覆写历史快照。

## Parallel scheduling

- 理论前沿：G2；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
