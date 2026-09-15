# R138 model settings shared inputs

- Scope: `apps/web/src/settings/ModelSettingsPanel.tsx`
- Change: Ollama combobox, model name, base URL, provider credentials, region and custom-header fields now use shared `Input`, preserving combobox attributes, labels, placeholders, controlled draft updates and credential handling.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: protected model CRUD/debug runtime and same-session Vue visual comparison remain open.
