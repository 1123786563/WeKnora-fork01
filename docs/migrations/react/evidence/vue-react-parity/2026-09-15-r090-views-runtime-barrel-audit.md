# React/Vue parity evidence — views runtime barrel audit

Date: 2026-09-15

The Web/Embed/desktop runtime audit now finds no remaining value imports from the aggregate `@weknora/views` entrypoint. Remaining matches are test-only imports or a type-only `ChatSubmission` import, so production runtime modules no longer depend on the full views barrel through these consumers.

- `rg` runtime audit: no remaining value imports in `apps/web/src`, `apps/embed/src`, or `apps/desktop/src`.
- Web tests: 895/895 passed.
- Web typecheck/build: passed.
- Embed tests/typecheck: passed.

The eager index chunk remains approximately 2.74 MB because other transitive route consumers still share common modules; chunk-size reduction needs a separate route-boundary analysis.
