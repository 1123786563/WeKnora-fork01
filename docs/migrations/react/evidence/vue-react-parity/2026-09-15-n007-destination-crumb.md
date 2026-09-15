# N007 upload destination breadcrumb

## Vue baseline

The Vue upload confirmation dialog renders the destination selector as a compact text breadcrumb: muted label, emphasized current folder, and a small disclosure marker. It has no bordered button chrome and remains ellipsis-safe at narrow widths.

## React change

React `UploadDestinationPicker` now uses the same borderless inline breadcrumb anatomy, with explicit text hierarchy, truncation, hover color and keyboard focus feedback. The existing destination picker open/close behavior and folder selection callbacks are unchanged.

## Verification

- Focused upload-confirm tests: 37/37 passed.
- Web typecheck and `git diff --check` passed.

## Evidence boundary

This is Vue source comparison plus React unit/type evidence. Authenticated post-change responsive screenshots, real folder mutation/upload behavior and Wails/native evidence remain open.
