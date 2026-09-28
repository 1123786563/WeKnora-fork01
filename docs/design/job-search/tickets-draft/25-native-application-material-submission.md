# T25：Expo 移动端申请、材料与本人投递

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#165](https://github.com/1123786563/WeKnora-fork01/issues/165)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

iOS、Android、鸿蒙用户（鸿蒙以 T01 可运行路径为前置）从岗位建立申请，编辑结构化材料、比较版本、取得 PDF/DOCX 并记录本人投递。

## Acceptance criteria

- [ ] 同岗不同批次申请分别展示
- [ ] 材料修改产生新版本且两种导出可下载
- [ ] 记录实际投递版或未知，警示硬条件冲突
- [ ] 离线草稿不在重联网后静默确认投递

## Blocked by

T18、T23。

## Ownership and interfaces

- 主责边界：Expo 移动 Career 申请和材料呈现；复用 Career Desk。
- 消费接口：申请、材料、导出、投递合同与原生 C 页面。
- 交付接口：原生申请到本人投递完整路径。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 跨端版本与未知回执测试。
- iOS、Android、鸿蒙各完成岗位到投递确认设备流程。

## Failure and unknown results

下载失败或提交结果未知保留原版本和待核对状态。

## Parallel scheduling

- 理论前沿：G7；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
