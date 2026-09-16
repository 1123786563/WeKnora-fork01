# R309 侧栏 scope 控件去重（2026-09-15）

PlatformShell 不再渲染与知识库/组织页面 rail 重复的 scope 子菜单；scope 查询参数继续由页面自身读取，侧栏保留主导航与会话区域。这样避免 Vue 页面 rail 与 shell 同时出现两套筛选入口。

- PlatformShell scope/会话 focused tests：12/12。
- `pnpm test:desktop`：2/2。
- `pnpm test:web`：911/911。

测试中存在既有 React act 警告，但无失败用例。真实双端像素、响应式和 native 运行证据仍待补齐。
