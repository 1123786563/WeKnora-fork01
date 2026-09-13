# N007 — Upload confirm dialog: graph section, URL list, add-source dropdown (React parity slice)

Date: 2026-09-13 · Branch: `codex/react-multiclient` · Worktree: `.worktrees/react-multiclient`

## Scope

Closed the remaining N007 gaps: the graph config section (Vue `GraphSettings` rendered by `UploadConfirmDialog`), the staged URL **list** (React had a single slot), the add-source **dropdown** affordance (React had a plain file+URL row), and the documents.css styling for the new sections.

## Vue references (authoritative, read-only)

`frontend/src/views/knowledge/components/UploadConfirmDialog.vue`

| Behavior | Vue lines |
| --- | --- |
| Graph section markup, rendered only when isGraphSectionAvailable | 526-533 |
| isGraphDatabaseEnabled / isGraphSectionAvailable gating | 861-868 |
| nav pushes graph with icon chart-bubble last, when available | 940-942 |
| getSectionNavStatus('graph'): off, then tag count, then on | 1009-1018 |
| UploadUIState.nodeExtractConfig + graphEnabled model | 600-615 |
| default state | 1057-1086 |
| initFromKbInfo extract seeding, enabled = extract.enabled AND indexing_strategy.graph_enabled | 1124-1137 |
| buildProcessOverrides: graph_enabled = enabled AND graphEnabled, extract_config passthrough | 1175-1183 |
| applyOverridesToState: extract_config + graph_enabled + clamp | 1231-1245 |
| handleNodeExtractUpdate mirrors config.enabled into graphEnabled | 1375-1378 |
| availability fallback watch, resets to default section | 1309-1313 |
| appendUrl / removeUrl (URL list) | 1338-1349 |
| URL rows in files panel | 80-96 |

`frontend/src/views/knowledge/settings/GraphSettings.vue` (822 lines): enable switch clears sample data but keeps custom instructions on disable (405-414), custom instructions textarea (45-59), creatable relation-type tags with admin generate button (62-96), sample text with generate button (99-132), entity rows with attribute sub-rows (135-203), relation rows (206-284), extraction actions incl. default/clear example (287-317), handlers (400-575), default Shakespeare example (549-565), admin gate canRunGraphExtract = hasRole('admin') (333-336), llm availability = !!modelId (362-366).

Extraction endpoints used by the admin buttons: POST /api/v1/initialization/extract/{text-relation,fabri-tag,fabri-text} — backend internal/router/routes_infra.go:123-125, handlers internal/handler/initialization.go:2407/2508/2582. **api-client has no bindings**, so the React port calls them via the client's public `request` method (packages/api-client/src/client.ts:292) — api-client itself untouched.

System info: `client.settings.system.info()` (packages/api-client/src/settings/index.ts:360-364). Live probe of GET /api/v1/system/info returns data.graph_database_engine = "Not Enabled" on this deployment; KB payloads carry indexing_strategy.graph_enabled and extract_config (null until graph configured).

## React changes

- `apps/web/src/documents/upload-pipeline.ts`
  - UploadGraphNodeState / UploadGraphRelationState / UploadNodeExtractState; UploadConfirmUIState.nodeExtract + .graphEnabled.
  - KB seeding mirrors Vue initFromKbInfo (incl. the enabled-AND-graph_enabled rule).
  - buildUploadConfirmOverrides emits graph_enabled + extract_config exactly like Vue.
  - applyUploadOverrides folds extract_config/graph_enabled with the Vue clamp (nodeExtract.enabled = enabled AND graphEnabled), copy-on-write.
  - UploadConfirmSectionKey type now includes graph; uploadSectionStatus('graph', ...) mirrors getSectionNavStatus('graph').
  - New helpers: graphDatabaseEnabled, graphSectionAvailable, defaultUploadConfirmSection, sectionAfterGraphAvailabilityChange, hasGraphAdminRole, GRAPH_EXTRACT_DEFAULT_EXAMPLE.
  - Copy table: graphSettings.* (49 keys), uploadConfirm.summaryGraphTagsValue, upload.uploadDocument, upload.uploadFolder, common.confirm — byte-exact from the five Vue locale files (zh/en/ja/ko/ru).
- `apps/web/src/documents/KnowledgeDocumentsPage.tsx`
  - UploadGraphSettings: full Vue GraphSettings port (enable switch semantics, custom instructions, creatable tags + admin generate, sample text + admin generate, entity rows with attributes, relation rows, add entity/relation, start extraction / default example / clear example, embedded disabled-DB alert).
  - UploadConfirmSections: gated data-section="graph" fieldset slot (graphAvailable + graphSettings props) — Vue v-if parity.
  - pendingUrl: string became pendingUrls: string[] (Vue localUrls): staging, dedupe notice, per-row removal, sequential per-URL upload that keeps the failed URL plus unattempted ones staged on error.
  - UploadSourceDropdown (Vue KbUploadSourceDropdown): file/folder/URL menu, hidden multiple + webkitdirectory inputs, URL import sub-dialog with urlLabel/urlPlaceholder/urlTip and invalidURL validation.
  - Section nav: graph entry pushed last when available with chart-bubble icon glyph, active state (aria-current) and the Vue watch(isGraphSectionAvailable) fallback via sectionAfterGraphAvailabilityChange.
  - Page fetches systemInfo (client.settings.system.info()) and stores auth/me for the admin gate; updateNodeExtract mirrors enabled into graphEnabled.
- `apps/web/src/documents/documents.css`: graph section styles (setting rows, node/relation cards, action buttons), URL row polish, dropdown + nav active + URL sub-dialog styles, ported from the Vue scoped LESS onto wk-* variables.
- `apps/web/tsconfig.n007.json` (NEW, verification-only): scoped tsconfig used to typecheck just the slice files; see typecheck note below. Safe to drop.

## Tests (TDD)

Red first (new tests in upload-confirm-dialog.test.tsx +8, upload-pipeline.test.ts +4; the pipeline file initially failed to load on the missing GRAPH_EXTRACT_DEFAULT_EXAMPLE export): **tests 19 / pass 11 / fail 8** (the pipeline file-level failure masked its 4 new tests). One of the new dialog tests (URL list rows) passed immediately because UploadFilesPanel was already list-capable — it documents the panel contract; the actual single-slot bug lived in the page state and is covered by the state/payload tests.

After implementation:

```
pnpm --filter @weknora/web exec tsx --test src/documents/upload-confirm-dialog.test.tsx src/documents/upload-pipeline.test.ts
  tests 45 / pass 45 / fail 0   (baseline before slice: 33/33)
all documents tests (regression): tests 57 / pass 57 / fail 0
```

Typecheck: `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` currently fails in src/agents/list.ts(214,3) (pre-existing, another agent's WIP file — not touched by this slice). Scoped check of this slice's files is clean:

```
pnpm --filter @weknora/web exec tsc -p tsconfig.n007.json --noEmit   # exit 0
# includes src/documents/**, src/knowledge/permissions.ts, src/i18n.ts, src/App.tsx
```

## I18n keys

Shared `packages/i18n` already had knowledgeEditor.sidebar.graph and the knowledgeBase.importURL* family. It has **no** graphSettings.* block, no uploadConfirm.* block, and no upload.uploadDocument/uploadFolder / common.confirm — the dialog therefore uses the established local byte-exact port table in upload-pipeline.ts (per its own header note). Nothing in packages/i18n was modified. If shared i18n later gains these keys, uploadConfirmT resolves them first automatically; the local table rows can then be deleted.

## Backend observations

- GET /api/v1/system/info works and returns graph_database_engine ("Not Enabled" on this deployment) — matches the api-client SystemInfo contract.
- The three extraction routes exist and are admin-gated server-side (internal/router/routes_infra.go:123-125); no contradiction with the client.
- api-client lacks bindings for the extraction endpoints (see above) — worked around via client.request; a follow-up slice may want proper bindings.

## Remaining gaps

- The extraction generate/extract buttons are rendered per the Vue gates but the underlying endpoints are only verified by curl + route checks (graph DB is "Not Enabled" here, so the section is hidden end-to-end in a live run); error paths surface as a notice line, Vue uses toasts.
- Vue's dialog shows one section at a time (v-show + activeSection); React renders all sections in a scrollable panel with an active-nav highlight (the pre-existing structural model of the prior slice). Nav order, gating and the unavailable-fallback match Vue.
- common.cancel for the sub-dialog uses the ported uploadConfirm.cancel (byte-identical copy); common.cancel itself is not in shared i18n.
