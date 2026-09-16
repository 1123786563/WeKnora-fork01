# React/Vue parity evidence — auth and shell lazy chunk boundary

Date: 2026-09-15

The Web entry now lazy-loads PlatformShell, LoginPage, JoinPage, and WorkspaceOnboardingPage. Auth/onboarding routes render through a shared Suspense fallback, while protected routes keep the existing shell boundary and behavior.

Build comparison:

- Before: eager `index` approximately 2,735.9 kB (gzip 698.2 kB).
- After: eager `index` 2,531.25 kB (gzip 641.23 kB).
- Reduction: approximately 204.6 kB raw and 57.0 kB gzip.

Validation:

- Web tests: 895/895 passed.
- Web typecheck: passed.
- Web production build: passed.
- `git diff --check`: passed.
