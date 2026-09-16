# Shared settings controls migration

Date: 2026-09-15  
Scope: `apps/web/src/settings/ConfigSettingsPanel.tsx`, `apps/web/src/settings/ResourceSettingsPanel.tsx`

Retrieval, chat-history, parser, resource and connector settings now use the shared UI controls (`Input`, `NumberInput`, `Select`, `Switch`, `Textarea`) while retaining the Vue-shaped layout, labels, validation and API payload behavior. This removes duplicated native-control styling and keeps settings forms consistent with the shared design system.

Validation: `pnpm typecheck:web` passed; `pnpm test:web` passed 895/895; `pnpm --dir apps/web run build` passed. The production build still reports the known large-chunk advisory.
