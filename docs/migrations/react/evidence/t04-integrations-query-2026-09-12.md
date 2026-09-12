# T04 Web integrations query compatibility follow-up

Date: 2026-09-12

The React integrations entry now consumes the legacy `section`/`tab` query
aliases. `integrationKeyFromQuery` maps canonical `integration-*` keys, bare
`api`/`im`/`embed` and external tabs (`cli`, `chrome`, `claw`) to the concrete
React integrations tab; the legacy `section=integrations` default maps to IM.
Unknown values fail closed to the existing Embed tab. The Web route passes the
resolved tab to the shared `IntegrationsPage`.

TDD evidence: the new alias test failed with a missing export, then passed
after the resolver and Web wiring were added. Verification passed with
`pnpm --filter @weknora/views exec node --import tsx --test
src/integrations/registry.test.ts` (3/3), `pnpm test:web` (103/103),
`pnpm test:shared` (187/187), both typechecks, `pnpm build:web`, the React
boundary check, and `git diff --check` all exiting 0. This is static/Node and
bundle evidence; it does not claim browser click-through or provider-backed
integration mutations.
