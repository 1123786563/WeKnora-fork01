# N007 upload-confirm dialog shell

## Scope

Port the Vue `UploadConfirmDialog.vue` outer geometry into the React upload confirmation flow while preserving the existing upload state machine and controls.

## Vue baseline

`frontend/src/views/knowledge/components/UploadConfirmDialog.vue` defines a 92vw / 1160px wide, 85vh / 750px high modal with a 220px files panel, a 216px settings navigation rail, independently scrolling configuration content, and a 14px/20px footer with 12px action gap. Below 800px the columns stack and navigation becomes horizontally scrollable.

## React change

`apps/web/src/documents/KnowledgeDocumentsPage.tsx` now uses the project `Dialog` with dedicated upload-confirm layout, files/settings columns, section-navigation rail, config scroll panel, and footer classes. `packages/ui/src/styles.css` owns the shared geometry and responsive overrides instead of the default narrow dialog surface.

The navigation rail keeps `UploadSectionNav` as the only semantic `nav`; the outer sizing wrapper is non-semantic, avoiding nested navigation landmarks while preserving the Vue three-column structure.

## Verification

- Focused upload-confirm tests: 37/37 passed.
- `pnpm typecheck:web`: passed.
- `pnpm test:web`: 911/911 passed; 0 failed, cancelled, or skipped.

## Evidence boundary

This is Vue source comparison plus React unit/type/full-suite evidence. Same-condition authenticated Vue/React screenshots, computed-style capture, real upload/parse backend behavior, and Wails/native runtime evidence remain open; no visual or backend acceptance is claimed here.
