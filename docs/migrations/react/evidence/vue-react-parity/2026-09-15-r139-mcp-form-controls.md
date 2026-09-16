# R139 MCP form shared controls

- Scope: `apps/web/src/settings/McpSettingsPanel.tsx`
- Change: OAuth scopes, API-key header/credential inputs, and usage-instructions editor now use shared `Input`/`Textarea`, preserving localized labels, secret handling, character limits, and mutation callbacks.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: protected MCP CRUD/OAuth runtime and same-session Vue visual comparison remain open.
