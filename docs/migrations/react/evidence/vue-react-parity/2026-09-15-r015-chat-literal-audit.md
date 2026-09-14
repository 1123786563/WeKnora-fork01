# R015 Chat literal audit（2026-09-15）

## 审计范围

扫描 `packages/views/src/chat`、`apps/web/src/chat` 中的英文字符串，并逐项区分渲染文案、协议值、测试 fixture 和用户输入示例。

## 结果

- `tool-result.tsx` 的工具标题、空态、命中统计、Shell 状态和详情字段已由 `ChatCopyTable` 覆盖五种语言。
- `page.tsx` 的发送异常回退已使用活动语言 `copy.sendFailed`；工具状态值（如 `streaming`、`completed`、`idle`）保留服务端/状态机枚举，不翻译后再回传。
- `ChatPage` 和 `MessageList` 中剩余的 `Live response`、`Sandbox terminal`、`Suggested questions`、`Messages` 属于 aria 兼容标签；当前测试和既有 DOM 合同依赖这些稳定标识，未将其混入可见文本。
- 测试中的英文问题、工具名和 fixture 是测试数据，不属于页面泄漏。
- `Clipboard is unavailable` 属于底层能力异常，尚未接入页面 copy 注入；该路径需下一轮确认 Vue 对应错误语义后再改。

## 判定

本轮没有发现新的可见英文泄漏。剩余项属于无障碍稳定标识或待确认底层异常路径，记录为后续审查项；不改变协议枚举和用户输入数据。
