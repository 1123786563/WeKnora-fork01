# R201 Chat composer regression after metric alignment (2026-09-15)

The full Web suite passes after aligning the shared chat textarea metrics and preserving the 56px empty-state spacing.

- Web: 895/895
- Shared chat/view focused suites: 63/63
- Web chat page focused suite: 27/27
- Existing non-fatal React `act(...)` and DOM-property warnings remain informational.
- No backend, route, or cross-platform API contract changed.
