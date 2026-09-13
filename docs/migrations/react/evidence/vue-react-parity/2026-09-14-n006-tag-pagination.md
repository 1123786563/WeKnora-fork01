# N006 document tag pagination

Date: 2026-09-14

## Vue baseline

`KnowledgeBase.vue` loads tag options from the server in pages of 50, sends a
300ms-debounced keyword, preserves selected tags across page changes, displays
the server total, and exposes a loading-aware “load more” action.

## React implementation

The documents page now uses `tagsPage` at the api-client boundary, keeps the
existing `tags()` array API compatible for other consumers, and passes the
server page/search state to `TagFilterPanel`. The panel retains selected rows
that are not in the current response page and disables the load-more action
while the next page is loading.

## Verification

- API-client tag page parsing/path assertion passed.
- Documents tag UI tests: 19/19 passed.
- `typecheck:shared` passed.
- `typecheck:web` remains blocked by pre-existing unrelated exports missing in
  `apps/web/src/faq/FAQPage.test.tsx` (`pushListItem`, `removeListItem`, and
  related symbols); the changed tag files are included in the preceding
  focused tests.
- No authenticated Vue/React screenshot or production backend acceptance is
  claimed here.
