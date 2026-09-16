# React/Vue parity evidence — mobile graph loading accessibility

- Scope: `apps/mobile/src/features/knowledge/KnowledgeGraphScreen.tsx`
- Change: graph loading indicator now announces the shared localized `common.loading` message instead of the unrelated empty-data label.
- Validation: `pnpm run typecheck:mobile`, `pnpm test:mobile` (189/189), `git diff --check`.
