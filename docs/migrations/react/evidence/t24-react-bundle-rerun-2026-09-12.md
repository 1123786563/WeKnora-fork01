# T24 React bundle and route regression rerun — 2026-09-12

## Scope

This rerun validates the current React candidate build inputs and Go static
frontend route test after the latest committed mobile/auth and evidence
changes. It is a local static/build check, not a replacement for deployed
registry, installed-client, cross-OS, browser-performance, or full role-matrix
acceptance.

## Results

| Command | Result |
|---|---|
| `python3 -m unittest scripts/test_generate_react_migration_baseline.py -v` | exit `0`, 4/4 |
| `python3 scripts/generate_react_migration_baseline.py` | exit `0` |
| `pnpm test:shared` | exit `0`, 164/164 |
| `pnpm test:web` | exit `0`, 64/64 |
| `pnpm test:embed` | exit `0`, 3/3 |
| `pnpm test:desktop` | exit `0`, 2/2 |
| `pnpm test:mobile` | exit `0`, 57/57 |
| `pnpm typecheck:shared` | exit `0` |
| `pnpm typecheck:web` | exit `0` |
| `pnpm typecheck:embed` | exit `0` |
| `pnpm typecheck:desktop` | exit `0` |
| `pnpm typecheck:mobile` | exit `0` |
| `node scripts/check-react-boundaries.mjs` | exit `0` |
| `pnpm build:web` | exit `0`, 120 modules |
| `pnpm build:embed` | exit `0`, 68 modules |
| `pnpm build:react-bundle` | exit `0`, Web/Embed candidate emitted under `dist/react-web/web` |
| `pnpm build:desktop-renderer` | exit `0`, 121 modules |
| `go test ./internal/router -run 'TestFrontendStatic' -count=1` | exit `0` |
| `git diff --check` | exit `0` |

The candidate Web bundle emitted `index.html`, a 464.73 kB JavaScript asset
(127.92 kB gzip), and a 15.30 kB CSS asset (3.26 kB gzip). The candidate
Embed bundle emitted a 242.10 kB JavaScript asset (75.74 kB gzip) and a
2.87 kB CSS asset (0.97 kB gzip). These are artifact-size observations only;
they are not a performance threshold pass.

## Boundary

The working tree still contains the user-provided untracked authority docs
and the pre-existing user modification to `apps/mobile/expo-env.d.ts`; this
rerun did not edit or stage them. T24 remains `review` until all release and
runtime gates in the ledger are independently evidenced, and T25 remains
gated behind T24.
