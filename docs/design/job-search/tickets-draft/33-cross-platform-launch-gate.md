# T33：五环境真实闭环与发布门槛

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#172](https://github.com/1123786563/WeKnora-fork01/issues/172)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

Web、iOS、Android、鸿蒙和微信小程序分别从建档走到找岗、材料、本人投递、跟进、导出与删除，形成发布证据。

## Acceptance criteria

- [ ] 五个实际环境均跑完整链路和失败恢复
- [ ] 分享导入、PDF/DOCX 下载、通知权限、跨端同步与越权检查有可复现记录
- [ ] 公布真实岗位来源、覆盖城市、使用条件与数据处理说明
- [ ] 来源、模型、微信能力、个人信息和收费流程的运营核验完成

## Blocked by

T12、T25、T26、T27、T28、T29、T30、T31、T32。

## Ownership and interfaces

- 主责边界：跨端验收与发布负责人；不代替前置 Ticket 的 Review。
- 消费接口：全部已审查并集成的纵向切片。
- 交付接口：五环境验收证据和发布阻塞清单。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- 保存各端操作记录、服务端合同结果与产物检查摘要。
- 最终差异审查与父规格 43 条故事覆盖核对。

## Failure and unknown results

任何真实阻塞缺口保留未完成，不以 Web 通过代表其他目标通过。

## Parallel scheduling

- 理论前沿：G9；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
