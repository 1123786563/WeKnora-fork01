# R297 文档详情 Trace 入口（2026-09-15）

文档详情页现在在成功加载后提供 Vue 对齐的“解析进度”入口，右侧 Sheet 展示阶段时间线、可展开 trace 节点、耗时和原始 JSON 详情，并复用可调宽度与持久化能力。

- 文档 preview/processing focused tests：7/7。
- `pnpm run typecheck:web`：通过。
- `git diff --check`：通过。

真实后端 spans、失败/重试/取消动作、Vue 同条件浏览器截图、响应式和 Wails/native 证据仍待补齐。
