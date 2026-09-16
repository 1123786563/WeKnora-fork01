# 2026-09-15 Desktop renderer production build evidence

- Scope: N033 Wails desktop renderer preflight.
- Commands: `pnpm --dir apps/desktop test`; `pnpm --dir apps/desktop run typecheck`; `pnpm --dir apps/desktop run build`.
- Result: desktop tests passed 2/2, TypeScript typecheck passed, and Vite production build completed after adding the shared `@weknora/views/*` subpath aliases to `apps/desktop/vite.config.ts`.
- The fix prevents Vite from treating the root `@weknora/views` file alias (`src/index.ts`) as a directory when resolving `@weknora/views/chat/mermaid` and other subpaths.
- Boundary: this is renderer build evidence; Wails embedded-backend interaction and per-feature desktop acceptance remain open.
