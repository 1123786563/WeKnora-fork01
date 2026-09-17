# WeKnora Expo 移动工作台 · 高保真开发交付 v2.0

2026-09-17 · 基于上一轮18页线框与WeKnora固定源码阅读基线 `12737238aa7b9d6891e76b2397f6941024f45668`。本轮交付设计、任务拆分、令牌与HTML原型；不改仓库、不发布、不进行真实业务操作。

## 从这里开始

**[打开高保真交互工作台](index.html)** · **[设计系统与组件样式](design-system.html)** · **[开发交付手册（HTML）](handoff.html)**

18页可导航，支持明暗主题和异常场景。桌面左侧选页面，右侧看字段/API/任务编号；移动宽度用顶部选择器。`pages/`每个文件自包含，可以独立打开。HTML是本地演示，不是已连接后端的Expo App。

## 设计文档

| 文件 | 内容 |
|---|---|
| [详细设计](docs/01-detailed-design.md) | 模块、原生适配、身份/空间、提交幂等、SSE、SQLite、审批、资源、通知、远程、语音、用量和发布 |
| [18页规格](docs/02-screen-specifications.md) | 页面字段、布局、交互、校验、状态、组件和任务对应 |
| [视觉系统](docs/03-design-system.md) | 暖白/深青绿、明暗语义色、字阶、间距、圆角、动效、组件和原生映射 |
| [API契约](docs/04-api-contracts.md) | A/B/C接口分类、聚合读模型提案、执行事件、决定union与错误行为 |
| [开发交接](docs/05-development-handoff.md) | 原型使用、文件作用、生成命令、接入次序与验证边界 |

## 任务与令牌

**[实施总计划](plans/00-implementation-master.md)**：36项MX增量任务，5个阶段，精确文件锁/角色/输入输出/测试场景/验收及原W/T/H映射。不会重置旧计划状态。

**[机器可读任务索引](plans/task-index.json)** · **[依赖图源码](plans/task-dag.mmd)** · **[页面与缺口追溯](plans/06-traceability.md)** · **[Code Agent交接](plans/07-code-agent-handoff.md)**

**[令牌JSON源](tokens/tokens.json)** · **[Web CSS](tokens/tokens.css)** · **[原生TypeScript](tokens/native-tokens.ts)** · **[类型契约提案](contracts/mobile-proposal.ts)**

## 视觉预览

[浅色六屏](overview-light.png) · [深色六屏](overview-dark.png) · [关键状态](overview-states.png)。18页×2主题单页截图在screenshots；并未打包字体文件。

## 已执行检查

252组浏览器布局检查、18个独立页初始化、设计系统默认入口、31项交互、8项原型状态机测试、85项结构/依赖/令牌检查，以及2份TypeScript文件的静态类型检查。详细结果见[验证说明](verification/README.md)。浏览器检查使用Chromium set_content；不代表本地文件导航、真机、后端或真实外部服务验收。

完整业务能力在正式实施后按profile验证。原型金额/资料/登录/上传/录音/审批均为演示；“下载示例”只生成标记为模拟的本地Markdown文件。
