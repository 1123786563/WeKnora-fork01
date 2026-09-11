# T23 mobile API key evidence

Date: 2026-09-11
Branch: `codex/react-multiclient`

## Implemented boundary

- `apps/mobile/app/(app)/management/api-keys.tsx` exposes a native workspace API-key route linked from the management hub and administration screen.
- The route uses the shared `administration.tenantApiKeys` client and the backend's actual Owner-only route contract. Admin, contributor, viewer, and unknown roles remain read-only.
- Scoped keys expose only the bounded mobile capability set (`retrieve`, `chat`, `read_agents`, `ingest`, and `manage_knowledge_bases`). Blank names and empty capability selections are rejected locally.
- Newly returned `token` values are shown once in memory and are not written to SecureStore or included in later list rows. Revoke waits for the server response and reloads; failures preserve the visible list.
- `integrations.api` deployment capability is projected into the mobile matrix; unsupported deployments remain visibly unsupported rather than presenting a writable form.

## Automated evidence

```text
pnpm test:mobile       # 32/32, exit 0
pnpm typecheck:mobile  # exit 0
```

Focused TDD evidence: the API-key helper test first failed because the new module was absent; after the implementation it passed as part of the 32-test mobile suite.

## Evidence boundary

This proves the native route wiring, Owner-only client-side affordance, scoped input validation, token non-persistence boundary, shared API client usage, and type-safe mobile bundle source. It does not claim a live API-key create/revoke, server 403 matrix, native device interaction, or production secret-management acceptance; those remain open T23/T24 evidence.
