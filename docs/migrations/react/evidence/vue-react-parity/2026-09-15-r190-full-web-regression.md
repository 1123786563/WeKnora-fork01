# R190 full Web regression after organization and member alignment

- Validation: `pnpm test:web` 895/895, `pnpm test:shared` 469/469, `pnpm typecheck:web`, and `git diff --check` pass after the organization editor/avatar and tenant invitation empty-state changes.
- The test run emitted existing React `act(...)` warnings from unrelated tests but no failures; no new warning was introduced by the changed surfaces.
- Remaining acceptance: protected write flows, full responsive and locale matrix, Wails/Embed runtime, and iOS/Android native evidence.
