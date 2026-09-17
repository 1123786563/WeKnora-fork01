# WeKnora 小程序 v0.1 提交验证说明

状态：**待联调源码版本**。本文是 `apps/miniprogram/README.md` 引用的验证状态入口；
测试证据、缺陷台账与页面矩阵在 [qa/](./qa/) 目录维护。

## 验证层级声明

以下结论只声明已有对应证据的层级，不混用：

| 层级 | 状态 | 证据 |
| --- | --- | --- |
| 干净环境依赖安装（frozen） | 通过 | [qa/baseline.md](./qa/baseline.md)（隔离检出 29.5s，`@tarojs/shared` 链接齐全） |
| 设计令牌检查 / 单元与装配测试 / 类型检查 / weapp 构建 | 通过 | [qa/baseline.md](./qa/baseline.md)、[qa/test-matrix.md](./qa/test-matrix.md) |
| 共享包回归（含 mini-regression） | 通过 | 同上 |
| 真实后端联调（OrbStack 本地 Go 后端） | 部分完成 | 既往提交 9e326603 的 curl 登录验证；本轮未重新执行带副作用链路 |
| 微信开发者工具模拟器 | 首轮渲染验证完成（9e326603：未登录首页、登录页、表单、键盘输入）；20 页逐页导航截图待补 | [qa/report.md](./qa/report.md) |
| 真机 / 微信审核 | 未执行 | 明确未验证，不得宣称 |

## 边界（not-implemented，入口保持关闭）

以下能力后端或客户端未实现，入口保持关闭并展示原因，不以 mock/空数据/假成功替代：

- `GET /api/v1/workbench/executions`（任务集合读模型）无 Go 路由（仅 `POST` Start 与
  `GET /requests/:id` 存在，见 `internal/router/routes_workbench.go`）；任务页把 404 诚实
  展示为"当前基线尚缺任务集合读模型"。
- 微信快捷登录/绑定、本人任务集合、待办聚合、审批动作详情（安全拒绝之外的决策）、
  支付渠道、产物下载定位（会话级索引缺 message_id）。

缺陷修复记录见 [qa/bug-ledger.md](./qa/bug-ledger.md)。
