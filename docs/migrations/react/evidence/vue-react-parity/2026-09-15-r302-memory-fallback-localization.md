# R302 个人记忆设置 fallback 本地化（2026-09-15）

个人记忆工作区设置保存失败时，未知异常现在复用 `common.operationFailed`；服务端异常对象的 message 仍优先显示。

- Settings focused tests：31/31。
- `pnpm run typecheck:web`：通过。
- `git diff --check`：通过。

真实后端写入失败、五语言浏览器截图和 native 验收仍待补齐。
