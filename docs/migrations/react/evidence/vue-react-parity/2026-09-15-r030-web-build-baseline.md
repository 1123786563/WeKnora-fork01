# React Web production build baseline

- Command: `pnpm run build:web`
- Result: Vite production build completed successfully after TypeScript project build.
- Current output: main application chunk about 4.9 MB minified; Mermaid/Cytoscape/Cynefin remain separately emitted chunks and are already dynamically imported by the shared Mermaid renderer.
- Decision: a manual vendor split experiment was reverted because it created a circular chunk warning and did not reduce the initial main chunk. Further reduction requires route-level or feature-level dynamic imports with browser evidence.
