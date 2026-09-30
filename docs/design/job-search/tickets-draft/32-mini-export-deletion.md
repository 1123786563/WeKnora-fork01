# T32：微信小程序（Taro 4 + TDesign Miniprogram）导出与删除

**状态：** 待批准草案；发布时加 ready-for-agent 标签。

## Parent

GitHub Spec #140：WeKnora 求职专业 Agent（三端完整闭环）。

## What to build

微信小程序（Taro 4 + TDesign Miniprogram）用户下载完整求职数据并发起受权删除，旧内容随授权失效。

## Acceptance criteria

- [ ] 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证
- [ ] 导出内容与其他端一致
- [ ] 删除说明保留规则和外部平台边界
- [ ] 删除后微信账号重进不恢复旧私有资料
- [ ] 旧材料链接和本地缓存不可访问

## Blocked by

T22、T24。

## Ownership and interfaces

- 主责边界：微信小程序（Taro 4 + TDesign Miniprogram） Career 隐私与文件 Adapter。
- 消费接口：完整导出与删除合同、微信身份作用域。
- 交付接口：微信小程序（Taro 4 + TDesign Miniprogram）数据迁出与删除后的本地清理。
- 修改共享契约时先公开变更、完成契约测试，再放行并行分支。

## Verification evidence

- Career Desk 删除与重新登录测试。
- 微信小程序（Taro 4 + TDesign Miniprogram）真实环境检查导出、删除、重进和旧授权。

## Failure and unknown results

导出能力受平台限制时提供等效可取得的文件流程，不静默截断。

## Parallel scheduling

- 理论前沿：G7；只在阻塞 Ticket 已验证并集成后进入该前沿。
- 可与同前沿的其他 Ticket 使用独立 Worktree 推进；共享接口、导航、迁移、构建目录或测试资源冲突时先冻结所有权并串行集成。
