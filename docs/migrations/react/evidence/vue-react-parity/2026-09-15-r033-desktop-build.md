# Desktop renderer production build evidence

- Command: `pnpm --filter @weknora/desktop-renderer build`
- Result: Vite production build completed successfully after transforming 2447 modules.
- Note: the renderer emits Mermaid/Cytoscape/Cynefin chunks separately; the current bundle warning is reported for visibility and needs a measured route-level split before changing chunk policy.
- Limitation: this is a build artifact check, not native Wails launch or authenticated interaction evidence.
