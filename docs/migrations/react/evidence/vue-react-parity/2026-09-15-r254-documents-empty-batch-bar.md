# R254 — Empty document state batch-bar parity

日期：2026-09-15

## Vue 基线

`frontend/src/views/knowledge/KnowledgeBase.vue` only displays
`DocumentBatchBar` while batch mode is active or a document is selected. In the
initial empty-document state there is no batch toolbar; the page shows the
upload empty state and source action only.

## React 差异与修复

React rendered the select-all/selected-count/reparse/move/tag/delete/cancel
toolbar even after a successful empty document response, with all controls
disabled. The toolbar now renders during loading or when documents exist, and
is absent for the successful empty and error states, matching the Vue empty
surface without changing document selection or mutation handlers.

## 验证

- Same authenticated account, KB `Parity KB Demo`, zh-CN, Chrome: Vue and React
  both show the empty upload state; React no longer exposes `全选` or `本页已选`.
- Document tag/batch focused suite: 9/9 passed.
- `pnpm typecheck:web`: passed.
- Current runtime fixture is empty; non-empty row selection and batch-mode
  interaction remain separate open evidence items.
