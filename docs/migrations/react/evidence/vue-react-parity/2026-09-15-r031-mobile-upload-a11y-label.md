# React/Vue parity evidence — mobile upload progress accessibility label

- Scope: `apps/mobile/src/features/knowledge/KnowledgeBaseListScreen.tsx`
- Change: upload progress container now exposes a localized progress label using the existing five-locale upload progress message instead of the hardcoded English `upload progress` label.
- Validation: `pnpm run typecheck:mobile`, `pnpm test:mobile` (189/189), `git diff --check`.
