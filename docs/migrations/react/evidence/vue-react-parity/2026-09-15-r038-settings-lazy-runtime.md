# Runtime evidence — settings lazy-loading boundary

- Browser: Chrome extension session, React dev server `http://localhost:5181`.
- Flow: authenticated `parity-test@local.dev` session opened `/platform/settings?section=platform-api-keys` after the lazy-loading change.
- Observed: settings dialog rendered successfully; platform API key section loaded its localized heading and showed the expected role-denied state for the non-system-admin account. No `lazy` initialization error or blank page remained.
- Validation: `pnpm run build:web`, `pnpm run test:web` (891/891), `pnpm run typecheck:web`, `git diff --check`.
- Limitation: successful system-admin API-key mutation requires a separate account/permission fixture.
