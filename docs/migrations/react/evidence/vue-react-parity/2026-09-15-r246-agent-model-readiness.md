# R246 — Agent creation model readiness

日期：2026-09-15

## Vue 基线

`frontend/src/composables/useTenantModelReadiness.ts` delegates agent readiness to
`evaluateTenantModelReadiness`, which counts tenant models with type `KnowledgeQA`.
`frontend/src/views/agent/AgentList.vue` opens the editor only when that readiness
flag is true; otherwise it warns and opens model settings.

## React 差异与修复

`apps/web/src/agents/AgentsPage.tsx` previously checked for the non-contract type
`llm`. With the parity account's `KnowledgeQA` model, React incorrectly treated
the tenant as unconfigured and navigated to `/platform/settings?section=models`.
The check now uses the explicit `KnowledgeQA` contract through
`hasAgentChatModel`; `llm` and `Embedding` do not satisfy agent readiness.

## 验证

- Focused Agents suite: 17/17 passed, including `KnowledgeQA`/`llm`/`Embedding`
  readiness cases.
- `pnpm typecheck:web`: passed.
- Browser, same account/locale/viewport/DPR: React remains on the agents route
  and opens the create editor; modal bounds are `127.5,54,1100,612`, matching
  the Vue `settings-modal` bounds exactly.
- Vue's currently reused tab retained contextual-guide/post-create state, so
  clean empty-submit validation is not claimed by this entry.

## 状态

已修复并通过本项；真实后端创建提交、失败回滚、响应式矩阵、Wails/native
渲染仍未由本项覆盖。
