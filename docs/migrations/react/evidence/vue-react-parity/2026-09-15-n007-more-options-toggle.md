# N007 upload more-options toggle

## Vue baseline

The Vue chunking section exposes a compact green `更多处理选项` toggle with a 6px icon gap, 6px vertical padding, and a chevron that rotates when expanded.

## React change

React now uses the same green text treatment, spacing, hover underline, keyboard focus ring, and an inline SVG chevron with an expanded rotation. The toggle still controls the existing advanced settings group and keeps its `aria-expanded` state.

## Verification

- Focused upload-confirm tests: 37/37 passed.
- Web typecheck and `git diff --check` passed.

## Evidence boundary

This is Vue source comparison plus React unit/type evidence. Authenticated post-change computed-style/screenshots, real upload processing and Wails/native evidence remain open.
