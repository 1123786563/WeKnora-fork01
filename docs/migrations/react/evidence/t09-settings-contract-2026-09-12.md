# T09 settings contract follow-up — 2026-09-12

The shared knowledge-settings API now fails closed for unsafe Activity ids and
cursors, malformed optional parser fields, non-finite chunk statistics,
non-string parser tier entries, and invalid default storage identifiers.

```text
node --import tsx --test packages/api-client/src/knowledge/settings.test.ts  exit 0; 4/4
pnpm typecheck:shared                                                     exit 0
```

This closes the client-side settings response-validation gap. Live provider,
permission, and browser acceptance remain separate T09 evidence requirements.
