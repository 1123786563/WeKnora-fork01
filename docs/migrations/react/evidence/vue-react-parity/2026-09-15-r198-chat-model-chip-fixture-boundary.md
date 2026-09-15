# R198 Chat model chip fixture boundary (2026-09-15)

## Scope

The paired `/platform/creatChat` screenshots showed React rendering `mock-stream-model 200K` while the Vue tab rendered `未配置` under the same empty-chat layout. This slice checks whether the difference is caused by React selection logic or by the live fixture state.

## Source contract comparison

- Vue `frontend/src/stores/chatResources.ts:59` filters the model list to `type === 'KnowledgeQA'`.
- Vue `frontend/src/components/Input-field.vue:982-994` selects the persisted last pick, then the first available model. `selectedModelDisplayName` returns `input.notConfigured` only when no selected model resolves.
- React `apps/web/src/chat/model-chip.ts` applies the same KnowledgeQA filter and selection rule. `apps/web/src/chat/ChatRoutePage.tsx` loads the model list, restores the scoped local selection, and falls back to the first available model.
- React tests cover the first-model fallback, empty-list fallback, and agent-model-missing fallback.

## Finding

No source-level parity defect was identified. The two live tabs did not expose equivalent model-list/local-storage state: React had a resolved model row, while Vue had no resolved selected model at capture time. A model chip change would therefore conflate a runtime fixture difference with a product behavior change.

## Acceptance boundary

- Static contract: passed by source comparison and focused model-chip tests.
- React/Web regression: already green (895/895) with the current implementation.
- Same-session backend parity: open until both tabs are captured after the same authenticated model-list response and cleared scoped local selection.
- No protected model mutation or successful stream claim is made by this evidence.
