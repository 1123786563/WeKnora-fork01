# N005 share dialog follow-up fix

## Scope

- `apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx`
- `apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.test.tsx`
- `docs/migrations/react/vue-react-parity-progress.md`

## Evidence

- Added request-generation protection so stale knowledge-base loads cannot
  overwrite the current dialog or clear its loading state.
- Connected existing organization-share translations for the dialog title,
  form, list, permissions, feedback, and confirmation copy.
- Mutation success notices and `onChanged` now require a successful reload;
  reload failures remain visible and do not claim success.
- Focused tests: 10/10 pass.
- Full Web suite: 298/298 pass.
- Web typecheck: pass.

## Review boundary

The earlier independent review findings about stale loads, localization, and
reload side effects are addressed by code and tests. Exact Vue structure,
metadata/avatar/action parity, browser/Vue screenshots, real backend,
Wails, iOS, and Android evidence are still missing, so N005 remains `review`.
