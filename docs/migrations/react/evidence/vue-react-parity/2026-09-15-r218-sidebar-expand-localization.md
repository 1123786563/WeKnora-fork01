# R218 侧栏展开按钮本地化（2026-09-15）

## 修正

PlatformShell 收起状态的展开按钮不再写死英文 `Expand sidebar`，改用已有 `menu.expandSidebar` 五语言消息，并同时设置 aria-label 与 title。展开/收起行为和视觉结构保持不变。

## 验证

- `shell-sessions-header.test.tsx`：4/4 通过。
- 测试确认收起侧栏隐藏会话区，并在 zh-CN 下暴露“展开侧边栏”。
- menu i18n 已覆盖 zh-CN、en-US、ja-JP、ko-KR、ru-RU。
