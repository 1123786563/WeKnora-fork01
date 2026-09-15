# R199 Chat composer input metrics (2026-09-15)

## Change

`packages/views/src/chat/composer.tsx` now uses a fixed 72px composer textarea, 16px horizontal padding, 16px font size, 24px line height and Vue-compatible vertical padding.

## Source basis

Vue `frontend/src/components/Input-field.vue` applies `min-height: 72px`, `font-size: 16px`, `line-height: 24px`, and `padding: 12px 16px` to the textarea inner control. The React control now preserves the same rendered height and typography while retaining controlled draft state, disabled behavior, and resize policy.

## Verification

- Shared chat/view suites: 63/63
- Web chat page suite: 27/27
- `git diff --check`: pass
- Cross-package and full Web regressions remain green from the preceding chat slice; no API or backend behavior changed.
