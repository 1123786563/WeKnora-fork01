# N010 FAQ card and editor anatomy

Date: 2026-09-14

## Vue baseline

`FAQEntryManager.vue` renders selectable FAQ cards with a question header and
more menu, optional collapsible similar/negative sections, an always-present
answers section, and a footer tag/status control. Its editor drawer uses a
520px right drawer, field descriptions, list add/remove controls, caps of 10
similar/negative questions and 5 answers, and required-field ordering.

## React implementation

`FAQPage.tsx` now renders that card anatomy, gates selection/more/status
controls by contributor permission, preserves collapsed sections locally, and
implements the Vue editor list fields, draft controls, validation helpers and
inline error slot.

## Verification

- FAQ view regression: 35/35 passed.
- Shared typecheck passed before unrelated parallel integration test changes.
- Web typecheck is currently blocked by the unrelated dirty
  `integrations/embedWizardRender.test.tsx` input-signature mismatch.
- No authenticated Vue/React screenshot or real-backend acceptance is claimed.
