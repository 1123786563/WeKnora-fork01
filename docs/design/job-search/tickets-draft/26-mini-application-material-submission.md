# T26：微信小程序（Taro 4 + TDesign Miniprogram）申请、材料与本人投递

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

微信小程序（Taro 4 + TDesign Miniprogram）用户从岗位建立申请，编辑、比较、下载两种文件并记录本人投递。

## Acceptance criteria

- [ ] 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证
- [ ] 与 Web、移动 App 使用同一申请和结构化正文版本
- [ ] PDF/DOCX 实际下载且内容对应确定版本
- [ ] 硬条件不符时警示常驻，用户显式继续
- [ ] 投递确认不触发招聘平台自动提交

## Blocked by

T06、T18、T24。

## Ownership and interfaces

- 主责边界：微信小程序（Taro 4 + TDesign Miniprogram） Career 申请和材料呈现；复用 Career Desk。
- 消费接口：申请、材料、导出、投递合同与微信小程序（Taro 4 + TDesign Miniprogram）C 页面。
- 交付接口：微信小程序（Taro 4 + TDesign Miniprogram）申请到本人投递完整路径。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 版本冲突测试。
- 微信小程序（Taro 4 + TDesign Miniprogram）真实环境完成岗位到文件下载和投递确认。

## Failure and unknown results

下载授权过期可重取；结果未知先对账。

## Parallel scheduling

- 理论前沿：G7；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
