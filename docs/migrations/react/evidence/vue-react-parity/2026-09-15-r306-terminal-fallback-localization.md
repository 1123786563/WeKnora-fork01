# R306 Chat 终端 fallback 本地化（2026-09-15）

WebTerminalController 新增可选 `errorCopy`，ChatRoutePage 传入当前语言 `operationFailed`，覆盖 ticket 获取失败、WebSocket error frame 缺少消息和连接错误；服务端提供的错误消息仍优先。

- Terminal/Chat focused tests：4/4。
- `pnpm run typecheck:web`：通过。
- `git diff --check`：通过。

测试工具链仍有既有 `import.meta` CJS warning；无失败用例。真实沙箱 provider、浏览器 locale 和 native 证据仍待补齐。
