# R300 Chat 附件 fallback 本地化（2026-09-15）

ChatRoutePage 的附件状态刷新、上传、移除和清理路径在未知异常时统一复用五语言 `operationFailed`，服务端/异常对象已有 message 仍原样保留。

- Chat focused tests：38/38。
- `pnpm run typecheck:web`：通过。
- `git diff --check`：通过。

测试中的 `import.meta` CJS warning 为既有测试工具链提示；无失败用例。真实 provider 附件失败、多语言浏览器截图和 native 验收仍待补齐。
