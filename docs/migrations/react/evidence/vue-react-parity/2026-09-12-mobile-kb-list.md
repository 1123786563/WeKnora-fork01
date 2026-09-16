# Mobile knowledge-base list parity slice

## Scope

Compared Vue `frontend/src/views/knowledge/KnowledgeBaseList.vue` with the native React implementation at `apps/mobile/src/features/knowledge/KnowledgeBaseListScreen.tsx`.

Implemented in this slice:

- workspace-role-gated create action and native modal with required-name validation, optional description, submit guard, refresh, and server error presentation;
- All / Mine / Favorites / Recent scopes using the shared `@weknora/domain/knowledge/list` scope and grouping semantics;
- per-card favorite and recent-visit state, grouped section labels, document/FAQ counts, description fallback, loading/error/empty states.
- native knowledge-graph route with server-backed overview/ego queries, node search, type/depth filters, bounded-result messaging, retry, and neighbor expansion.
- native document detail now gates preview by shared processing status, uses the authenticated preview endpoint, renders text/Markdown safely and images natively, while keeping PDF/Office formats download-only.

## Evidence

| Gate | Result | Evidence |
| --- | --- | --- |
| Focused native knowledge tests | pass | `node --import tsx --test apps/mobile/src/features/knowledge/*.test.ts` — 20/20 |
| Mobile TypeScript | pass | `pnpm typecheck:mobile` |
| Graph route contract/type integration | pass | `apps/mobile/app/(app)/knowledge/[id]/graph.tsx`, `KnowledgeGraphScreen.tsx`, `client.wiki.graph` |
| Native preview contract integration | pass | `KnowledgeDocumentDetailScreen.tsx`, shared `@weknora/domain/knowledge/preview`, authenticated `documents.previewPath` |
| Static whitespace check | pass | `git diff --check` |
| iOS native runtime | missing | requires a configured simulator/device and authenticated fixture |
| Android native runtime | missing | requires a configured emulator/device and authenticated fixture |

## Remaining parity

This slice does not claim acceptance. Vue still has richer persistent pin/recents storage, upload-progress aggregation, shared-space sidebar data, localized six-language copy, and desktop card menus; mobile upload progress, exact graph visualization styling, graph capability gating, and iOS/Android runtime evidence remain open in N031.
