# N031 mobile FAQ/Wiki reference copy

Date: 2026-09-14

## Change

`KnowledgeReferenceScreen` now keeps the native reference surface aligned with
the migrated Vue copy boundary: FAQ and Wiki empty states use the shared
locale catalog, empty answers/summaries do not invent placeholder content, and
the extra mobile-only role/knowledge-base diagnostic line was removed.

## Verification

- `pnpm --filter @weknora/mobile test`: 146/146 passed.
- `pnpm --filter @weknora/mobile typecheck`: passed.

This is static/native-source and mobile unit evidence only. No iOS or Android
device launch was performed in this environment, and no authenticated Vue
same-account comparison is claimed.

## Follow-up graph surface

The native Wiki graph surface now uses shared Vue-derived labels for the graph
title, search, filters, loading, retry, empty-filter, refresh, and neighbor
expansion controls. Unknown server page types remain visible using their
server value rather than being dropped.
