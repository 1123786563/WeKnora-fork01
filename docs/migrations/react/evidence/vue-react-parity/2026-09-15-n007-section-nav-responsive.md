# N007 upload section navigation responsive parity

## Vue baseline

At widths below 800px, Vue changes the settings navigation to a horizontal, non-wrapping scroller: navigation rows become auto-width, keep their labels on one line, and no longer reserve the desktop row margin.

## React change

React removed the inline desktop display rule from `UploadSectionNav` and moved layout ownership to the shared upload-dialog stylesheet. The desktop rail remains a vertical grid; the responsive override switches the inner nav to a horizontal flex scroller with auto-width buttons and no row margin.

## Verification

- Focused upload-confirm tests: 37/37 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This slice is Vue source comparison plus React unit/type evidence. A live authenticated responsive browser capture remains open.
