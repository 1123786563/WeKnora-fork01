# R202 Cross-package chat regression (2026-09-15)

After the shared chat composer and empty-state spacing changes:

- Shared workspace tests: 469/469
- Desktop renderer tests/typecheck: 2/2, pass
- Embed tests/typecheck: 7/7, pass
- Mobile tests/typecheck: 190/190, pass

No cross-package contract or native runtime behavior was changed. Device visual and live backend chat evidence remain separate acceptance gates.
