# R137 model debug shared textareas

- Scope: `apps/web/src/settings/ModelDebugPanel.tsx`
- Change: debug input, rerank documents, and system-prompt fields now use shared `Textarea`, preserving labels, rows, controlled state, and request payload behavior.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: live model debug success/error and same-session Vue visual comparison remain open.
