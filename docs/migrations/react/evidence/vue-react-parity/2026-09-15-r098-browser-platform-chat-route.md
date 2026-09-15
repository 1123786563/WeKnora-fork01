# r098 浏览器平台 Shell / 新对话路由验收

## 环境

- Vue：`http://localhost:5173/platform/creatChat`
- React：`http://localhost:5181/platform/creatChat`
- 两个本地服务 HTTP 200，认证夹具为 `paritytester / parity-test@local.dev`。

## 双端可见状态

两端都渲染新对话入口、知识库/智能体/共享空间导航、空会话列表、用户区、欢迎语和输入区。

React 额外将平台 Shell 和 composer 暴露为可访问性语义：折叠侧栏按钮、导航链接、批量管理工具栏、输入框、智能体下拉、上传附件、知识库选择和发送按钮；Vue 当前主要是容器/文本/图标语义。React fixture 显示 `mock-stream-model 200K`，Vue 显示“未配置”，属于运行时夹具配置差异，不能据此宣称模型状态 parity。

## 结论

- 两端路由和首屏结构可达。
- React Shell/composer 的 ARIA 语义覆盖已得到浏览器证据。
- Vue/React 的模型配置差异、完整视觉 computed-style、真实后端与多端原生证据仍开放。
