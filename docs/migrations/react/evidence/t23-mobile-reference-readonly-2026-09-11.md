# T23 mobile Wiki/FAQ read-only slice evidence — 2026-09-11

## Scope

The mobile capability matrix already declares Wiki and FAQ as `read-only`. This
slice makes that decision reachable instead of leaving a capability row without
an entry point:

- `KnowledgeDocumentsScreen` links to KB-scoped `/knowledge/:id/wiki` and
  `/knowledge/:id/faq` routes.
- Wiki rows use the shared `client.wiki.list` API and show title, slug fallback,
  summary, and revision marker.
- FAQ rows use the shared `client.knowledge.faq.list` API and show the question,
  enabled/recommended state, and the first answer when present.
- No mobile write control, offline queue, or unscoped route was added.

## Verification

- `pnpm --filter @weknora/mobile exec node --import tsx --test src/features/knowledge/reference-parity.test.ts`: 3/3, exit 0.
- `pnpm test:mobile`: 9/9, exit 0.
- `pnpm typecheck:mobile`: exit 0.
- `pnpm --filter @weknora/mobile exec expo export --platform ios`: exit 0, 1,113 modules, 3 MB Hermes bundle.
- `pnpm --filter @weknora/mobile exec expo export --platform android`: exit 0, 1,136 modules, 3.1 MB Hermes bundle.
- `node scripts/check-react-boundaries.mjs`: exit 0; `git diff --check`: exit 0.

These are source/test/typecheck checks only. A device or simulator with a real
backend was not available, so native navigation, authenticated Wiki/FAQ reads,
large lists, and role/tenant negatives remain unaccepted.
