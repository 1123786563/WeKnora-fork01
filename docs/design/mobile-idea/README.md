# WeKnora Expo Mobile Workbench — 设计交付包

日期：2026-09-17。仓库基线：`12737238aa7b9d6891e76b2397f6941024f45668`。

## 阅读顺序

1. `01-expo-mobile-architecture.md`：架构、技术栈、复用边界、状态恢复、安全和发布设计。
2. `02-source-review-and-gap.md`：源码事实与12项优先修复/核验点。
3. `wireframes.html`：18页可点击移动端线框；浏览器直接打开，无需安装依赖或启动后端。
4. `03-pages-and-implementation.md`：字段/按钮/状态、UX增量任务与W计划映射。
5. `04-api-contract-proposal.md`：已读接口、建议接口、尚未核验的旧计划接口，分开列示。
6. `sources.md` / `source-manifest.json`：固定提交的代码与文档阅读范围、官方技术资料。
7. `verification/prototype-report.json`：仅HTML原型验证结果；不代表真实Expo/Go系统验收。

## 原型使用

左侧选择页面，或点击手机内的按钮。顶部场景选择器可切换离线、审批冲突、无权限、启动未知。新任务会带入用户输入；审批有二次确认；空间切换、语音转写、通知、产物等只在演示内变化。所有数据都是模拟数据，不发送账号密码，不调用GitHub、WeKnora、Provider或支付接口。

`screenshots` 目录为原型页面预览；`wireframes-overview.png` 汇总六个核心页面。

## 限制与边界

通过GitHub连接器读取固定提交的目录和关键源码、15份指定参考文件的相关章节；完整git clone因当前容器DNS失败未完成。未安装项目依赖、未运行Go/Expo原生构建、数据库迁移或真实外部服务。没有修改/提交/推送原仓库；此包不包含仓库完整源码。

原型是新的产品信息架构，不是当前应用已实现功能的截屏。蓝图中的新路径与模型已标记为建议，避免被开发Agent误当成现有接口。未交付能力仍沿原W/H/T计划记录，不以禁用按钮掩盖完整性要求。
