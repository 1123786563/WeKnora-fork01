# R301 文档详情 Trace 操作闭环（2026-09-15）

文档详情 Trace Sheet 现在支持按解析状态轮询 spans、手动刷新、失败重建、处理中取消，并在操作后重新读取文档和 trace 状态；失败响应展示服务端最后错误。

- 文档/预览/处理聚焦测试：21/21。
- `pnpm run typecheck:web`：通过。
- `git diff --check`：通过。

真实后端 reparse/cancel、RBAC、双端浏览器和 native 证据仍待补齐。
