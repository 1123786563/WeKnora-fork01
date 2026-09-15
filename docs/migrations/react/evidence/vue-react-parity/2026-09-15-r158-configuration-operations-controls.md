# R158 configuration operation controls

- Scope: `apps/web/src/configuration/ConfigurationOperations.tsx`.
- Change: agent sharing, model debug and skill catalog fields now use shared `Input`, `Select` and `Textarea` controls; file upload remains a native file input for browser file selection.
- Validation: configuration focused suite passed 40/40; Web typecheck and diff check passed.
- Boundary: protected operation success/error flows and same-session Vue visual comparison remain open.
