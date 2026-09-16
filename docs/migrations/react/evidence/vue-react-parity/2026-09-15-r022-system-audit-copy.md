# React/Vue parity evidence — system audit copy

- Scope: `apps/web/src/settings/SystemAuditLogPanel.tsx`
- Change: added locale-aware copy for audit title, description, refresh/retry/loading controls, table/detail labels, empty state, system actor fallback and load errors across five supported locales.
- Validation: `pnpm run test:web` (891/891), `pnpm run typecheck:web`, `git diff --check`.
- Limitation: authenticated runtime evidence for non-Chinese locales and paginated audit data is still pending.
