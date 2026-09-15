# R246 集成抽屉关闭控件本地化（2026-09-15）

集成预览抽屉、嵌入渠道创建抽屉和 API Playground 的关闭按钮不再硬编码中文 `aria-label/title`，统一使用集成翻译表中的五语言 `common.close`。关闭行为、焦点和抽屉布局不变。

验证：

- `EmbedPreviewModal` 与集成预览回归：5/5
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
