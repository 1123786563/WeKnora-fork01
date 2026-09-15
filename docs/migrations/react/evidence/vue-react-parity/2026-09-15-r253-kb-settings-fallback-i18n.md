# R253 — Knowledge-base settings fallback localization

日期：2026-09-15

## Vue 基线与差异

知识库设置的 Vue 编辑器使用共享 `knowledgeEditor.messages.*` 与
`knowledgeEditor.chunking.debug.*` 文案。React `KnowledgeSettingsPage` 的
正常字段已使用共享翻译，但加载失败、保存成功/失败和分块预览失败的
fallback 仍硬编码英文，导致 zh-CN 错误/成功状态泄漏英文。

## 修复

React 现在使用 `nameRequired`、`loadDataFailed`、`updateSuccess`、`common.error` 和
`chunking.debug.errorPrefix` 翻译键作为 fallback；服务端返回的具体错误
消息仍原样保留，API 请求和表单行为不变。

## 验证

- Knowledge-settings focused tests: 3/3 passed。
- zh-CN/en-US 翻译键解析回归通过。
- `pnpm typecheck:web`: passed。
- 当前 React/Vue 非空 KB 运行时对照仍受两端租户/API 数据不一致阻断，未将本项标记为完整页面视觉验收。
