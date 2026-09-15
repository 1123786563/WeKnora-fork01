# Shared UI parity slice

Date: 2026-09-15

## Scope and source of truth

Implemented only the shared design-token and `packages/ui` slice. Vue is the visual authority. Facts were extracted from `frontend/src/assets/theme/theme.css` and `frontend/src/assets/dropdown-menu.less`: 14px/20px body text, 12px small text, green brand `#07c05f` with hover `#08dd6e`, 6px controls, 10px menus, 12px panels, 148px menu minimum width, 4px menu padding, 0.18s easing, popup shadow, and overlay z-index 3000/3500.

## Changed files

- `packages/design-tokens/package.json`
- `packages/design-tokens/src/index.ts`
- `packages/design-tokens/src/styles.css`
- `packages/design-tokens/src/index.test.ts`
- `packages/ui/package.json`
- `packages/ui/src/index.tsx`
- `packages/ui/src/styles.css`
- `packages/ui/src/styles.d.ts`
- `packages/ui/src/index.test.tsx`
- `pnpm-lock.yaml` (workspace dependency link update from `pnpm install --offline`)

The existing dirty business/app files were not modified.

## Tests and checks

- `node --import tsx --test packages/design-tokens/src/index.test.ts packages/ui/src/index.test.tsx` — PASS, 3/3.
- `pnpm exec tsc --noEmit --jsx react-jsx --target ES2022 --module NodeNext --moduleResolution NodeNext --strict packages/design-tokens/src/index.ts packages/ui/src/index.tsx` — PASS.
- TDD RED was observed before implementation: missing token module/UI exports caused the new tests to fail; a follow-up Tabs test caught the nested trigger active-state bug before final GREEN.

## Not verified / remaining evidence

- No browser/Playwright or Vue-vs-React screenshot comparison was run in this slice.
- No live DOM test environment was available, so focus trap, initial focus, Escape dispatch, Portal placement, backdrop dismissal, and focus restoration are source-level implementations rather than runtime acceptance evidence.
- Radix packages were not introduced; wrappers use native ARIA/keyboard-compatible primitives and `createPortal`. Radix-specific integration remains unverified.
- Select/Menu controlled state, arrow-key roving behavior, and full Tabs keyboard navigation need browser-level tests in the next UI slice.
