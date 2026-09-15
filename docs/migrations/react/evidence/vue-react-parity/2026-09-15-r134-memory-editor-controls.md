# R134 personal memory editor controls

- Scope: `apps/web/src/settings/PersonalMemorySettingsPanel.tsx`
- Change: migrated memory-item edit/create textareas and create-kind selector to shared `Textarea` and `Select`, retaining Vue-shaped labels, validation, controlled state, keyboard save, and create callbacks.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: protected memory CRUD runtime and same-session Vue visual comparison remain open.
