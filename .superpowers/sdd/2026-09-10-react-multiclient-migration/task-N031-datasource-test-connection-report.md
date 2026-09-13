# N031 mobile data-source test connection

## Scope

- `apps/mobile/src/features/knowledge/DataSourcesScreen.tsx`
- `apps/mobile/src/features/knowledge/DataSourcesScreen.test.tsx`
- `docs/migrations/react/vue-react-parity-progress.md`

The change is limited to the native editor's pre-save connection test. It
reuses the existing typed `dataSources.validateCredentials` API and does not
add an endpoint or persist a draft.

## Evidence

- RED: the native-host component test initially failed because the editor had
  no `Test connection` action.
- GREEN: the action validates the current draft, calls the existing transport,
  reports `Connection successful`, surfaces failures, clears stale success when
  fields change, and disables test/save/cancel while pending.
- Focused component test: 3/3 pass.
- Full mobile suite: 92/92 pass.
- Mobile typecheck: pass.

## Review boundary

This is implementation-level evidence only. Connector-specific credential
fields, resource selection, localized copy, real backend responses, browser or
Vue screenshot comparison, Wails, iOS, and Android evidence remain open.
N031 therefore remains `implementing`.
