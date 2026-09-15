# R142 cross-package UI regression

- Mobile: `pnpm test:mobile` passed 190/190; `pnpm typecheck:mobile` passed.
- Desktop: `pnpm test:desktop` passed 2/2.
- Embed: `pnpm test:embed` passed 7/7.
- Scope: validates shared UI primitive and settings-control changes across package consumers.
- Boundary: device/browser visual parity and protected backend flows remain separate acceptance gates.
