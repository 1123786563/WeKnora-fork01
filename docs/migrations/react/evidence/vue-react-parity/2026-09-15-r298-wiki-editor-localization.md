# R298 Wiki 编辑器 fallback 本地化（2026-09-15）

`saveWikiPage` 现在接受调用方提供的文案，Wiki 页面将校验失败、保存冲突和未知保存失败接入当前 locale 的词条；默认英文文案保留给独立调用方和兼容行为。

- Wiki editor focused tests：4/4。
- `pnpm run typecheck:web`：通过。
- `git diff --check`：通过。

真实 Wiki 后端冲突/保存、五语言浏览器截图和 native 验收仍待补齐。
