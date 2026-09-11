# T23 mobile capability matrix follow-up (2026-09-12)

- Commit `a56ab45` makes every mobile management capability declare an
  execution mode (`native-write`, `native-read`, `web-handoff`, or
  `unsupported`), a reason, and required roles.
- The management hub keeps the four existing native routes, while unsupported
  and Web-handoff entries are visibly non-actionable. Server capability
  projection fails closed and never invents a write action for a disabled
  surface.
- Verification passed: management-focused tests 11/11, full mobile tests
  71/71, and mobile typecheck. No real IdP/device or complete role-by-tenant
  matrix was run; those and provider-backed settings remain open, so T23 stays
  `review`.
