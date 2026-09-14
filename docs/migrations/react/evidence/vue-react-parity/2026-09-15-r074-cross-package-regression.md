# React/Vue parity evidence — cross-package regression after mobile request guards

Date: 2026-09-15

The mobile data-source request guards were followed by a cross-package regression sweep.

- Shared: 465/465 passed.
- Web: 895/895 passed.
- Embed: 7/7 passed.
- Desktop renderer: 2/2 passed.
- Mobile: 189/189 passed.
- Mobile and shared typechecks passed.
- `git diff --check` passed.

Browser computed-style parity and authenticated iOS/Android runtime acceptance remain separate gates.
