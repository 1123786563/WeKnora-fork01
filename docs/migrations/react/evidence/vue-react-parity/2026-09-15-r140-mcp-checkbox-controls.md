# R140 MCP test result checkbox controls

- Scope: `apps/web/src/settings/McpTestResultBody.tsx`, `packages/ui/src/checkbox.tsx`.
- Change: MCP tool enabled/approval policies now use shared `Checkbox`; the shared primitive imports React explicitly for the repository's server-render test transform, and the package exports the primitive for direct use.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895. The initial missing-React failure was reproduced and fixed before the green run.
- Boundary: protected MCP policy mutation and same-session Vue visual comparison remain open.
