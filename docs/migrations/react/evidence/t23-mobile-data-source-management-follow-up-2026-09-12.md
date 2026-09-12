# T23 mobile data-source management follow-up — 2026-09-12

This increment replaces the earlier native read-only data-source inventory
with a bounded management surface. It does not claim full T23/T24 acceptance.

## Implemented boundary

- `DataSourcesScreen` now supports native create/edit, connector credential
  validation, synchronization, pause/resume, sync-log inspection, and
  confirmation-gated deletion through the existing typed SDK methods.
- New/edit forms use the server connector type list and keep schedule, sync
  mode, conflict strategy, and deletion policy explicit. A failed mutation
  leaves the current list unchanged until a later successful reload.
- Existing credentials are never reconstructed into the form. New credentials
  are sent only when explicitly entered, are validated before save, and remain
  platform input rather than server-rendered text.
- The implementation stays knowledge-base scoped and uses the existing
  authenticated transport; no new endpoint or offline queue was introduced.

## Verification

| Command | Result |
|---|---|
| `pnpm exec tsx --test apps/mobile/src/features/knowledge/data-source-form.test.ts` | 3/3 passed after the intentional missing-module RED run |
| `pnpm test:mobile` | 82/82 passed |
| `pnpm --filter @weknora/mobile typecheck` | exit 0 |
| `pnpm exec expo export --platform ios --output-dir /tmp/weknora-react-mobile-datasource-ios` | exit 0; worktree mobile bundle exported |
| `pnpm exec expo export --platform android --output-dir /tmp/weknora-react-mobile-datasource-android` | exit 0; worktree mobile bundle exported |
| `node scripts/check-react-boundaries.mjs` | exit 0 |
| `git diff --check` | exit 0 |

## Evidence boundary

This proves native source wiring, pre-submit validation, credential non-prefill,
type safety, and iOS/Android JS bundleability. It does not prove live provider
credentials, sync completion, 403/409 permutations, or native device
interaction. T23/T24 remain `review`.
