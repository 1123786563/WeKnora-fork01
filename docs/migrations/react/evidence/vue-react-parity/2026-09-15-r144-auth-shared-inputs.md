# R144 auth shared inputs

- Scope: `apps/web/src/auth/LoginPage.tsx`
- Change: login and registration email, username, password, and confirmation fields now use shared `Input`, preserving autocomplete, localized placeholders, validation feedback, loading disable state, and submit behavior.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: browser same-condition auth visual comparison and live auth backend evidence remain open.
