# R287 — Knowledge page fallback localization

- FAQ mutation/load/import/export fallback errors now use `t('common.error')`, preserving server-provided messages while removing hardcoded English UI copy.
- Wiki page loading uses the shared `wikiBrowser.loading` key.
- Verification: FAQ focused suite 48/48; Web full regression 910/910; Web typecheck and `git diff --check` pass.
- Boundary: this verifies source and automated rendering behavior. Protected backend failures, same-session Vue pixels, responsive and native evidence remain open.
