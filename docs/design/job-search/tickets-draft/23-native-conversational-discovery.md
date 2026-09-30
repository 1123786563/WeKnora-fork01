# T23：Expo 移动端 C 找岗、建档与分享导入

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）完成档案确认、分享或粘贴导入、对话找岗与岗位评估。

## Acceptance criteria

- [ ] 复用同一 WeKnora 身份与 Tenant，不建本地权威副本
- [ ] 可上传已有简历或逐步建档，并逐项确认抽取事实
- [ ] 系统分享进入的链接/JD 可核对后导入
- [ ] 窄屏持续展示硬条件、依据、来源和待核实项
- [ ] 跨端修改档案后读到同一确认版本

## Blocked by

T05、T11。

## Ownership and interfaces

- 主责边界：Expo 移动 Career 呈现与分享 Adapter；复用 Career Desk。
- 消费接口：Career Desk 的档案、导入、搜索和评估合同。
- 交付接口：原生 C 页面与系统分享适配。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 合同测试 scope 切换与跨端版本。
- iOS、Android、鸿蒙各跑分享导入到岗位评估的设备流程。

## Failure and unknown results

离线输入只保存草稿，联网后需用户确认提交。

## Parallel scheduling

- 理论前沿：G4；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
