# T30：微信小程序（Taro 4 + TDesign Miniprogram）持续规则、额度与提醒

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

微信小程序（Taro 4 + TDesign Miniprogram）用户能管理持续找岗、查看额度与站内待办，并自主选择订阅消息。

## Acceptance criteria

- [ ] 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证
- [ ] 规则与 Web、移动 App读写同一版本
- [ ] 超额不阻断历史访问
- [ ] 订阅消息只提醒同步，不携带敏感岗位详情
- [ ] 拒绝订阅后站内待办仍可用

## Blocked by

T13、T20、T21、T24。

## Ownership and interfaces

- 主责边界：微信小程序（Taro 4 + TDesign Miniprogram） Career 规则、用量与订阅消息 Adapter。
- 消费接口：持续规则、额度、站内待办与微信订阅能力。
- 交付接口：微信小程序（Taro 4 + TDesign Miniprogram）规则、用量与订阅消息选择。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 规则冲突测试。
- 微信小程序（Taro 4 + TDesign Miniprogram）真实环境检查订阅授权、提醒与隐私文案。

## Failure and unknown results

订阅消息不可用时显示站内待办，不伪造已送达。

## Parallel scheduling

- 理论前沿：G6；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
