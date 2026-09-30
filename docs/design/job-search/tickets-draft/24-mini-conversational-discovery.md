# T24：微信小程序（Taro 4 + TDesign Miniprogram）C 找岗、建档与分享导入

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

微信小程序（Taro 4 + TDesign Miniprogram）用户绑定同一 WeKnora 用户，完成档案确认、分享或粘贴导入、对话找岗与评估。

## Acceptance criteria

- [ ] 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证
- [ ] 微信身份只作为关联方式，不生成第二份求职档案
- [ ] 可上传已有简历或逐步建档，并逐项确认抽取事实
- [ ] 分享导入先展示可核对内容再提交
- [ ] 资格冲突、来源和待核实项在窄屏可见
- [ ] 同一用户 Web 的档案变更可在微信小程序（Taro 4 + TDesign Miniprogram）同步看到

## Blocked by

T06、T11。

## Ownership and interfaces

- 主责边界：微信小程序（Taro 4 + TDesign Miniprogram） Career 呈现与微信分享 Adapter；复用 Career Desk。
- 消费接口：Career Desk 合同与微信身份绑定。
- 交付接口：微信小程序（Taro 4 + TDesign Miniprogram）C 页面与分享适配。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 合同测试身份绑定与版本变化。
- 微信小程序（Taro 4 + TDesign Miniprogram）真机或官方环境跑完整找岗流程。

## Failure and unknown results

分享负载缺失或授权失效时展示恢复入口，不用演示数据冒充结果。

## Parallel scheduling

- 理论前沿：G4；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
