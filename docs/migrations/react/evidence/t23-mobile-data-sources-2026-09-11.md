# T23 mobile data-source inventory evidence

Date: 2026-09-11
Branch: `codex/react-multiclient`

## Implemented boundary

- `apps/mobile/app/(app)/knowledge/[id]/data-sources.tsx` adds a native route linked from the knowledge-base Files screen.
- The route reads the shared `dataSources.list()` and `dataSources.types()` APIs and shows source name, connector type, status, schedule, sync mode, and declared connector metadata.
- It is explicitly read-only on mobile. `config`, credentials, validation, sync, pause/resume, and raw error payloads are not rendered or mutated, preventing secret-shaped configuration from entering the native UI.

## Automated evidence

```text
pnpm test:mobile -- --testNamePattern='data source'  # 33/33, exit 0
pnpm typecheck:mobile                              # exit 0
pnpm exec expo export --platform ios                # exit 0, 3.1 MB Hermes bundle
pnpm exec expo export --platform android            # exit 0, 3.2 MB Hermes bundle
```

The focused helper test was first red because the module did not exist, then passed after the implementation.

## Evidence boundary

This proves typed native route wiring, secret-safe read-only rendering, iOS/Android JavaScript bundleability, and local status/type handling. It does not claim a live connector credential, sync, pause/resume, or native device acceptance; those remain Web/production-management scope.
