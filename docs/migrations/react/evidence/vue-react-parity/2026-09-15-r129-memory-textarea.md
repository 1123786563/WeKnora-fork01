# R129 personal memory textarea control

- Scope: `apps/web/src/settings/PersonalMemoryPanel.tsx`
- Change: migrated the extraction-instructions field to shared `Textarea`, retaining localized label/placeholder, max length, rows, disabled state, controlled updates, and blur save behavior.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: protected memory save runtime and same-session Vue visual comparison remain open.
