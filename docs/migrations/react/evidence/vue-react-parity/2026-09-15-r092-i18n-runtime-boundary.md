# React/Vue parity evidence — lightweight i18n runtime boundary

Date: 2026-09-15

Added `@weknora/i18n/runtime`, a dependency-free entry containing locale validation and the loading fallback needed before route chunks load. The Web entry no longer imports the full locale catalog during bootstrap; route components continue using the full catalog where their UI needs it.

- Web tests: 895/895 passed.
- Web typecheck: passed.
- Web production build: passed.
- `git diff --check`: passed.

The measured eager index remains approximately 2,531.5 kB (gzip 641.4 kB), so the catalog was not the dominant remaining source; no further claim of size reduction is made for this slice.
