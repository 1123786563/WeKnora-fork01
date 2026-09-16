# 2026-09-15 React release bundle evidence

- Command: `pnpm build:react-bundle`.
- Result: Web TypeScript/Vite build passed, Embed TypeScript/Vite build passed, and the script copied the output to `dist/react-web/web`.
- Related gates: shared typecheck passed; Embed tests 7/7; desktop tests 2/2 and typecheck passed.
- Boundary: runtime protected-route and backend acceptance remain separate from bundle generation.
