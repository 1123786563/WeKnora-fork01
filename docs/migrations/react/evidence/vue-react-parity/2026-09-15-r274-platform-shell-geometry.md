# R274 平台侧栏展开与收起几何验收（2026-09-15）

在 Chrome 受保护 React 知识库页面上执行真实侧栏收起/展开操作，并读取渲染后的 DOM 几何：

- 展开态侧栏：`x=0,y=0,w=260,h=720`
- 展开态内容区：`x=260,w=1095,h=720`
- 收起态侧栏：`x=0,y=0,w=60,h=720`
- 收起态内容区：`x=60,w=1295,h=720`
- 收起态导航保留四个图标入口，并以本地化 `title` 提供名称；会话区隐藏。

Vue `frontend/src/components/menu.vue` 的基线 CSS 同样定义展开 `260px`、收起 `60px`，因此当前 React 运行时几何与 Vue 壳层契约一致。

验证：
- PlatformShell 聚焦测试：32/32
- Chrome 展开/收起交互：通过
- `pnpm run typecheck:web`：通过
