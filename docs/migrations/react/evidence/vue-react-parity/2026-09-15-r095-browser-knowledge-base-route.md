# r095 浏览器知识库路由验收

## 环境

- Vue 基准：`http://localhost:5173/platform/knowledge-bases`
- React 实现：`http://localhost:5181/platform/knowledge-bases`
- 两个本地服务均返回 HTTP 200。
- 浏览器：Chrome 扩展会话，认证夹具用户 `paritytester / parity-test@local.dev`。

## 可见状态

Vue 与 React 均渲染知识库页面，并显示：

- 顶部/侧栏导航：新对话、知识库、智能体、共享空间。
- 知识库筛选：全部、收藏、最近、本空间。
- 页面标题“知识库”。
- 说明文案与模型未初始化提示。
- “我创建的”分组、数量 `1`。
- `Parity KB Demo`、`parity test data`、数量 `0`。

React 额外提供了可访问性语义：折叠侧栏按钮、导航链接、筛选 checkbox、设置弹出按钮和“新建知识库”按钮；Vue 页面对应图标/容器已可见，但部分控件未暴露同等 ARIA 角色。该差异记录为后续可访问性与视觉一致性修正项，不宣称整页 parity 完成。

## 结论

- 路由可达、React 页面完成首屏渲染，并与 Vue 共享同一 parity 数据夹具。
- 本证据只覆盖浏览器首屏可见状态；真实后端、Wails、iOS/Android 原生交互仍需分别验收。
