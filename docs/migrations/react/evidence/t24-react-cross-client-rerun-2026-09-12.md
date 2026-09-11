# T24 React cross-client and candidate rerun (2026-09-12)

The current branch HEAD `3823ad6` was rebuilt as a React Web/Embed candidate.
`dist/react-web/web/BUILD_INFO.json` reports `renderer: react` and commit
`3823ad6`; the artifact contains both `config.js` and `favicon.svg`.

## Verification

| Check | Result |
| --- | --- |
| `pnpm test:shared` | 180/180 passed |
| `pnpm --filter @weknora/web test` | 94/94 passed |
| Web / shared typecheck | passed |
| Web and Embed production build | passed; 127 / 68 modules |
| Mobile test / typecheck | 61/61; passed |
| Desktop test / typecheck | 2/2; passed |
| Embed test / typecheck | 3/3; passed |
| React boundary and diff checks | passed |
| React desktop resource / artifact preflight scripts | passed |

The run is local static, Node, bundle, and package-preflight evidence. It does
not prove installed Wails Windows/Linux behavior, a complete device matrix,
registry/deployed proxy behavior, provider-backed model/RAG/tool flows, or
production rollback. T24 remains `review`; T25 remains gated and Vue source is
intentionally retained.
