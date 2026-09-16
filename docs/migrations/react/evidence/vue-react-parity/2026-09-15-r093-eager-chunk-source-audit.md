# React/Vue parity evidence — eager chunk source audit

Date: 2026-09-15

The post-lazy-load build was inspected for remaining heavyweight runtime markers. Mermaid is dynamically imported by the renderer, and route-level modules remain split; the eager index is approximately 2,531.5 kB (gzip 641.4 kB). The remaining size is shared application/runtime code rather than a residual views barrel import. Further reduction requires route-boundary or shared dependency graph work, not another blind import rewrite.

- Web production build: passed.
- Runtime barrel audit: no remaining value imports.
- `git diff --check`: passed.
