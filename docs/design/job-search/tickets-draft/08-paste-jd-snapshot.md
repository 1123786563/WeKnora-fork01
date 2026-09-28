# T08：粘贴 JD 形成岗位机会与快照

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#146](https://github.com/1123786563/WeKnora-fork01/issues/146)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户粘贴招聘要求，得到可打开的岗位机会、原文快照、来源和获取时间。

## Acceptance criteria

- [ ] 原文与抽取字段分开保存，缺字段标为未知
- [ ] 同一请求 ID 重放返回同一机会或回执
- [ ] 岗位原文中的指令不能获得 Agent 工具权限
- [ ] 页面可从对话结果进入岗位证据详情

## Blocked by

T03。

## Ownership and interfaces

- 主责边界：Career Opportunity；Web C 对话入口。
- 消费接口：Career 最小意图与对话入口。
- 交付接口：岗位机会、来源观察和不可改写的 JD 快照。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 合同测试覆盖原文快照、重复提交和恶意 JD。
- Web E2E 证明粘贴到再次打开内容一致。

## Failure and unknown results

提取失败保留原文并提示补充，不生成虚构岗位条件。

## Parallel scheduling

- 理论前沿：G1；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
