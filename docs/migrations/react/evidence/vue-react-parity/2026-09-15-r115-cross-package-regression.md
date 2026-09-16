# Cross-package regression after mobile and platform parity slices

Date: 2026-09-15  
Branch: `codex/react-multiclient`

## Commands

| Surface | Command | Result |
|---|---|---|
| Shared contracts/domain/API/views | `pnpm test:shared` | 469/469 passed |
| Web | `pnpm test:web` | 895/895 passed |
| Mobile | `pnpm test:mobile` | 190/190 passed |
| Desktop renderer | `pnpm test:desktop` | 2/2 passed |
| Embed | `pnpm test:embed` | 7/7 passed |

All commands exited with code 0. This is focused static/unit evidence; it does not close the remaining browser, live-backend, Wails feature-interaction, or native authenticated-flow rows in the parity matrix.
