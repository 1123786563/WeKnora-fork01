# WeKnora Taro 微信小程序 v0.1

状态：**待联调源码版本，非生产就绪版本**。本工程复用现有 Go 后端、共享 API Client、contracts 与 domain；不另建账号、任务或计费账本。

## 代码与设计

四个主 Tab 为工作台、任务、知识、我的，共 20 个页面入口。平台网络、流、文件、存储和导航放在 `src/platform`，页面在 `src/features`，路由包装在 `src/pages` 和 `src/subpackages`。

设计资料见仓库中的 [详细设计与任务](../../docs/WeKnora-Miniapp-Implementation-Kit/) 和 [原始技术方案与线框](../../docs/WeKnora-Taro-Design/)。本版使用 `src/styles/tokens.json` 中的 125 项令牌，逻辑画板宽 375，令牌导出为最终 rpx；运行时安全区尺寸不再次缩放。

## 配置与开发

在仓库根目录安装依赖。Node 至少 22.16；pnpm 版本沿用根 packageManager。Taro 4.2.1 / React 18.3.1 是本源码锁定的候选组合，尚未经过完整安装和原生构建验证。根锁文件需要在受信任开发环境安装后更新并审查，当前提交不伪造锁文件。

配置 `WEKNORA_API_ORIGIN` 为固定 HTTPS origin（不带 `/api/v1`），`WEKNORA_WEAPP_APPID` 为公开小程序 AppID。可参考 `.env.example`，实际构建使用 shell/CI 环境变量。AppSecret、访问令牌、租户 API Key 和支付密钥不得写入客户端。

```bash
pnpm install
pnpm --filter @weknora/miniprogram test
pnpm --filter @weknora/miniprogram tokens:check
pnpm --filter @weknora/miniprogram typecheck
pnpm --filter @weknora/miniprogram build:weapp
```

构建命令会生成本工程的 `project.config.json` 与 `dist/`；导入微信开发者工具后仍需完成域名配置、基础库与真机验证。

## 本版本边界

已有源码覆盖平台账号登录、刷新、空间切换、Agent 浏览、聊天与引用、临时附件、知识上传、执行启动/恢复/命令、用量和订单双状态查询。接口失败不会回退到虚构业务数据。

微信快捷登录/绑定、本人任务集合与待办聚合的 Go 增量、完整审批风险摘要、可靠产物下载定位、小程序支付渠道及通知仍未完成。缺少动作详情时批准按钮关闭，支付入口关闭；不宣称这些能力已可用。

验证结果和未验证项目见 [本次提交说明](../../docs/miniprogram/submission-v0.1.md)。不要跳过 Taro 构建、共享包与 Web/Expo 回归后直接合并或上线。
