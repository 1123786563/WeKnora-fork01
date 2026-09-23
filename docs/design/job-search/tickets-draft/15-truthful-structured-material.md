# T15：可信结构化材料与不可变版本

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户依据已确认档案和固定岗位快照生成、编辑、审阅简历及网申问答正文。

## Acceptance criteria

- [ ] 生成前冻结岗位与档案版本，主张链接到确认事实
- [ ] 缺失实习、证书、数字不得由模型补造，审阅指出风险
- [ ] 用户确认正文后形成新不可变版本，旧版本可比较
- [ ] 三端共同编辑的是结构化正文，修改后不得覆盖旧投递版

## Blocked by

T07、T14。

## Ownership and interfaces

- 主责边界：Career Material 与 Agent Runtime；Web 材料编辑。
- 消费接口：已确认档案、岗位快照与独立申请。
- 交付接口：结构化材料草稿、主张引用和不可变正文版本。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 公共 seam 测试覆盖未确认事实、伪造成果、版本冲突。
- Web E2E 修改正文并比较 V1/V2。

## Failure and unknown results

生成或审阅失败保留草稿与原因，不发布可投递版本。

## Parallel scheduling

- 理论前沿：G4；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
