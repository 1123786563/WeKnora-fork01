# R131 Web production build after settings controls

- Command: `pnpm --dir apps/web run build`
- Result: Vite production build passed; emitted `index-YuyGgC6q.js` (2,538.60 kB, gzip 642.88 kB).
- Advisory: existing large-chunk warning remains for the main bundle and Mermaid/Cynefin chunks; this is a performance follow-up, not a build failure.
