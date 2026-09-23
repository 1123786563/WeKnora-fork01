# T06：微信小程序（Taro 4 + TDesign Miniprogram）现有 Task 产物下载

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

微信小程序（Taro 4 + TDesign Miniprogram）用户可从已拥有的现有 Task 获取真实 PDF 或 DOCX，而不被迫转到 Web。

## Acceptance criteria

- [ ] Taro 4 小程序中的文件入口与可见控件使用 TDesign Miniprogram，通过页面 usingComponents 接入并验证组件注册、样式 token、事件回调、构建产物和真机交互；该实现仅承诺微信小程序目标

- [ ] 产物列表只显示当前用户与空间可访问项
- [ ] 短时授权过期时可重新取得，越权时不可下载
- [ ] PDF 和 DOCX 均可打开或保存，失败给出可操作提示
- [ ] 不在微信小程序（Taro 4 + TDesign Miniprogram）持久缓存明文敏感文件

## Blocked by

None (can start immediately)。

## Ownership and interfaces

- 主责边界：微信小程序（Taro 4 + TDesign Miniprogram） Task 呈现与 Workbench 下载 Adapter。
- 消费接口：现有 Task 产物列表与短时授权。
- 交付接口：微信小程序（Taro 4 + TDesign Miniprogram）文件获取 Adapter 和过期恢复。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- 微信小程序（Taro 4 + TDesign Miniprogram）真机或官方开发环境下载两种文件并检查内容。
- 服务合同测试过期、撤销和越权。

## Failure and unknown results

下载未知或中断不显示完成状态。

## Parallel scheduling

- 理论前沿：G0；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
