# System API key copy action

Date: 2026-09-15  
Scope: `apps/web/src/settings/PlatformApiKeysPanel.tsx`

After creating a platform API key, the one-time token card now offers a localized copy action using the browser Clipboard API. Successful copy changes the action label to a localized confirmation; unsupported or failed clipboard writes preserve the token and show a localized manual-copy message. The existing close action and secret display behavior remain unchanged.

Validation: `pnpm typecheck:web` passed; `pnpm test:web` passed 895/895; `git diff --check` passed.
