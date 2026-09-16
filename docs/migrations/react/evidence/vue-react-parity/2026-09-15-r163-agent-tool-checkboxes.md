# R163 agent tool checkboxes

- Scope: `apps/web/src/agents/AgentEditorModal.tsx`.
- Change: allowed-tool and skill-selection toggles now use shared `Checkbox`, preserving data attributes, disabled capability gating, checked state and payload updates.
- Validation: agent focused suite passed 68/68; Web typecheck and diff check passed.
- Boundary: protected agent mutation flows and same-session Vue visual comparison remain open.
