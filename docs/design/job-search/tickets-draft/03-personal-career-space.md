# T03：个人求职空间与已确认基础档案

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

求职者用 WeKnora 账号进入单成员个人求职空间，并确认毕业时间、学历、城市与意向；这些确认事实成为后续判断的唯一输入。

## Acceptance criteria

- [ ] 首次进入遵循现有 Tenant 开通策略，不能绕过数量与授权限制
- [ ] 未确认字段可保存为提案，但评估视图不得视为已确认
- [ ] 另一用户或 Tenant 不能读取档案，切换空间后旧读取结果不回填
- [ ] 一次重复确认不产生两条事实，冲突返回当前修订

## Blocked by

None (can start immediately)。

## Ownership and interfaces

- 主责边界：Career Office、Career Desk、Identity；Web 首条完整路径。
- 消费接口：现有身份与 Tenant 开通策略、认证后的用户与空间作用域。
- 交付接口：Career 最小公开意图与视图、确认事实回执、修订冲突合同。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Office 合同测试覆盖授权、确认、重复请求和修订冲突。
- Web 浏览器流程证明从登录到再次打开档案状态一致。

## Failure and unknown results

开通失败、权限不足和未知提交结果分别可见；未知结果先对账。

## Parallel scheduling

- 理论前沿：G0；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
