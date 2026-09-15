# R143 React release bundle after shared UI migration

- Command: `pnpm build:react-bundle`
- Result: Web and Embed TypeScript/Vite production builds passed; Web bundle copied to `dist/react-web/web`.
- Embed output: `index-Cy6P08OG.js` 2,639.25 kB (gzip 682.02 kB).
- Advisory: existing >500 kB chunk warning remains and is tracked as a performance follow-up.
