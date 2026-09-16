# R270 文档文件夹卡片外壳对齐（2026-09-15）

根据 Vue `DocumentCardView.vue` 的 `folder-card`、`folder-card__body` 和 `folder-card__footer` 样式，React 文档卡片网格完成结构收敛：

- 网格最小列宽由 220px 调整为 Vue 的 240px。
- 文件夹卡片固定 `136px` 高度、`8px` 圆角、border-box、溢出隐藏及 Vue 对齐阴影。
- 文件夹主体采用 `12px 14px 10px` 内边距，图标 28px，标题 14px/500/20px。
- 数量栏独立为底部边框区域，使用 `8px 14px` 内边距和 12px 文本。
- 保留键盘可见焦点和原有进入文件夹点击行为。

验证：
- 文档聚焦测试：56/56
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过

真实非空文件夹双端截图、响应式和 Wails/native 证据仍待补齐。
