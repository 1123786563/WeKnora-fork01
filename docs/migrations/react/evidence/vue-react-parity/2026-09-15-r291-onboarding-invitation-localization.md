# R291 — Workspace onboarding invitation fallback localization

- Workspace onboarding now uses dedicated localized keys for invitation-list loading failure and accept/decline action failure.
- Server-provided error messages remain authoritative.
- Verification: onboarding focused tests 4/4; Web full regression 910/910; Web typecheck and `git diff --check` pass.
- Boundary: authenticated provider invitation lifecycle and native onboarding evidence remain open.
