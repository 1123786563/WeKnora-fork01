# R161 agent editor controls

- Scope: `apps/web/src/agents/AgentEditorModal.tsx`.
- Change: agent name/description, prompt textareas, numeric configuration fields and selectors now use shared `Input`/`Textarea`/`Select`, preserving validation, disabled built-in state, controlled updates and payload construction. Range slider semantics remain native.
- Validation: agent focused suite passed 68/68; Web typecheck and diff check passed.
- Boundary: full protected agent CRUD and same-session Vue visual comparison remain open.
