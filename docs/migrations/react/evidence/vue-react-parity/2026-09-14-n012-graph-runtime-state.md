# N012 图谱运行时状态证据

日期：2026-09-14

## React 浏览器

- 地址：`http://localhost:5181/platform/knowledge-bases/kb-1?tab=graph`
- 条件：已认证本地 Chrome、默认语言 zh-CN、当前后端服务 `localhost:8080`。
- 结果：页面加载态显示“加载图谱中...”；请求失败后显示后端的知识库不存在状态和“重试”，搜索/深度/类型/帮助控制在失败态隐藏。
- 运行时原因：当前后端 `GET` 图谱请求返回 `knowledge base not found`，知识库列表也显示“暂无知识库”，因此没有可用于节点、邻居、拖拽、缩放和 status-card 的真实数据。

## 结论

该记录证明 React 图谱 loading/error 条件渲染及重试文案已在浏览器实际出现；不证明 Vue/React 同数据画布视觉、真实 graph API、节点交互、Wails 或移动端验收。后续需要可用知识库 fixture 或真实后端数据后补采同视口对照。

## Source parity correction

Vue `WikiBrowser.vue` has no visible graph-depth selector; depth is an internal default used by ego/bloom graph loading. React no longer renders its migration-only native 1/2/3 depth selector and keeps the internal default at `1`, so this control does not remain as an unverified React-only surface.
