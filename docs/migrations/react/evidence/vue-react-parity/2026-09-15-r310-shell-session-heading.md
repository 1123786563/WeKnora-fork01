# R310 会话区域标题语义对齐（2026-09-15）

PlatformShell 会话区域移除额外可见“我的对话”标题，保留 `aria-label` 语义和会话日期/行内容，与 Vue `menu.vue` 的 session submenu 结构一致；键盘快捷键与租户身份显示保持不变。

- PlatformShell/session focused tests：10/10。
- `git diff --check`：通过。

这是 DOM/交互结构证据；双端像素、响应式和 Wails/native 运行证据仍待补齐。
