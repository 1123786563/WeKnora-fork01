# R181 shared font stack

- Scope: `packages/ui/src/theme.css`.
- Change: shared UI theme now follows the application font preference variable and uses the Vue-compatible system/CJK fallback stack, keeping shared primitives typographically aligned with the shell.
- Validation: shared suite passed 469/469; Web typecheck and diff check passed.
- Boundary: cross-platform font rendering remains subject to native screenshot acceptance.
