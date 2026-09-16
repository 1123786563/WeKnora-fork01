# R215 聊天区域无障碍标签本地化（2026-09-15）

## 修正

聊天共享视图中两个此前写死英文的标签已纳入五语言 `chat-copy` 表：

- 消息列表 `Messages`
- 会话分页 `Conversation pages`

React 组件现在使用当前 locale 的 `messagesLabel` 与 `conversationPagesLabel`，不改变视觉结构或分页行为。

## 验证

- `chat-copy.test.ts`：12/12 通过，确认五语言 key 集合一致。
- `message-list.test.tsx`：1/1 通过。
- `pnpm run typecheck:web` 和完整 Web 回归待本切片提交后执行。
