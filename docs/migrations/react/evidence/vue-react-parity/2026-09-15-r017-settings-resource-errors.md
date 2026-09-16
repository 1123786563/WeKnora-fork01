# Vue ↔ React parity evidence — settings resource error copy

- Scope: `CloudSettingsPanel` status refresh and `ResourceSettingsPanel` resource-list/default-resource failures.
- Change: added five-locale local fallback copy for cloud status loading, resource list loading, and default-resource assignment; removed migration TODOs that documented missing dedicated labels.
- Validation: `pnpm run test:web` (891/891), `pnpm run typecheck:web`, `git diff --check`.
- Limitation: this is static/unit evidence; live backend failure injection and browser interaction for each settings section remain pending.
