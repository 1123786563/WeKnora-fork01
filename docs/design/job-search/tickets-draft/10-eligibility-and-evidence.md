# T10：三值资格与证据化匹配

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#150](https://github.com/1123786563/WeKnora-fork01/issues/150)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

用户先看到硬性资格结论，再看到技能、项目与意向的证据化匹配和待核实项。

## Acceptance criteria

- [ ] 2026 届对仅限 2027 届为不符合，缺毕业日期为待确认
- [ ] 每项资格指向岗位快照与已确认档案版本
- [ ] 总分不表述为录用概率，不符合不能被其他高分掩盖
- [ ] 用户改档案后可生成新评估，旧评估仍可读取

## Blocked by

T08。

## Ownership and interfaces

- 主责边界：Career Evaluation；Web C 岗位结果卡。
- 消费接口：已确认档案版本与岗位快照。
- 交付接口：三值资格判断与证据化匹配评估。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 公共 seam 场景测试覆盖符合、不符合、未知和版本变化。
- Web E2E 检查警示、证据与来源在窄布局可见。

## Failure and unknown results

模型无法结构化解释时显示评估待复核，不假装符合。

## Parallel scheduling

- 理论前沿：G2；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
