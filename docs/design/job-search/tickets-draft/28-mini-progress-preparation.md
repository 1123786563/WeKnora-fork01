# T28：微信小程序（Taro 4 + TDesign Miniprogram）申请时间线与按需准备

**状态：** 已发布；ready-for-agent。

**GitHub Issue：** [#169](https://github.com/1123786563/WeKnora-fork01/issues/169)。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

微信小程序（Taro 4 + TDesign Miniprogram）用户可记录、更正并筛选进展，按实际投递版本准备面试与求职信。

## Acceptance criteria

- [ ] 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证
- [ ] 事件列表与其他端共享权威顺序
- [ ] 跨端更正后旧事件仍可追溯
- [ ] 准备内容引用确定投递版，未知时提示
- [ ] 微信小程序（Taro 4 + TDesign Miniprogram）切换账号不显示前一用户缓存

## Blocked by

T19、T26。

## Ownership and interfaces

- 主责边界：微信小程序（Taro 4 + TDesign Miniprogram） Career 进展与准备呈现。
- 消费接口：进展事件、阶段投影、面试准备和微信小程序（Taro 4 + TDesign Miniprogram）申请页。
- 交付接口：微信小程序（Taro 4 + TDesign Miniprogram）时间线、纠错及按需准备。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 事件与 scope 测试。
- 微信小程序（Taro 4 + TDesign Miniprogram）真实环境录入、修改并查看面试准备。

## Failure and unknown results

网络断开只保留本地草稿，不静默改申请状态。

## Parallel scheduling

- 理论前沿：G8；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
