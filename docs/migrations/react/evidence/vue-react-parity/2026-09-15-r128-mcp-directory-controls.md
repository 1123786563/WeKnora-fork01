# R128 MCP directory shared controls

- Scope: `apps/web/src/settings/McpToolsDirectory.tsx`
- Change: migrated tool search to shared `Input` and enabled/approval policy toggles to shared `Switch`, preserving labels, disabled state, controlled values and mutation callbacks.
- Validation: `pnpm typecheck:web` and `pnpm test:web -- --runInBand` pass (895/895).
- Boundary: protected MCP backend mutation and same-session Vue visual comparison remain open.
