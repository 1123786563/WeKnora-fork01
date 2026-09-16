# r096 浏览器设置 MCP 路由验收

## 环境

- Vue：`http://localhost:5173/platform/settings?section=mcp`
- React：`http://localhost:5181/platform/settings?section=mcp`
- 两个本地服务 HTTP 200，使用 parity 认证夹具。

## 结果

两端均可打开设置抽屉，展示设置分组、MCP 服务管理标题、说明文案和“添加服务”入口；关闭设置按钮也可见。

React 页面在可访问性树中将设置分组暴露为按钮，并只保留一棵设置内容树。Vue 页面当前可见两棵重复的设置内容树（同一导航、标题和添加服务按钮重复出现），且多数导航项是图标/容器语义。这是当前可复现的 parity/结构差异，后续应优先修复 Vue 基准的重复挂载或在 React parity 记录中明确其来源。

## 结论

- 设置 MCP 深链接和首屏渲染通过浏览器夹具门禁。
- 发现 Vue 重复设置树问题；本项不宣称设置页面完整 parity，通过项保留为 review。
- 真实后端、权限变体、Wails、iOS/Android 仍未覆盖。
