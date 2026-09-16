# R240 集成预览失败提示本地化（2026-09-15）

集成页路由层的预览会话失败提示改为复用 `embedPublish.previewUnavailable` 五语言词条，不再固定显示中文。预览失败仍在当前页面内提示，成功时继续打开同一内嵌预览抽屉。

验证：

- 集成预览回归：4/4
- 中文和英文提示均通过
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
