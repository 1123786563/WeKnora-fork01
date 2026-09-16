# R145 agent editor shared selection controls

- Scope: `apps/web/src/agents/AgentEditorModal.tsx`
- Change: agent editor model, knowledge-base and scope selectors now use the shared selection controls while preserving Vue-shaped option lists, selected state, validation and payload updates.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895 after commit `c55a143f`.
- Boundary: full Agent Editor browser visual parity and protected agent CRUD remain open.
