# R013 API Playground agents loading — 2026-09-14

## Vue baseline

`ApiIntegrationSettings.vue` loads `listAgents({ creator: 'all' })` during page initialization, exposes `agentsLoading` and `agentsError`, and passes both to the filterable TDesign select in the Playground drawer. The selected agent is re-seeded after the list resolves.

## React implementation

`IntegrationsRoutePage.tsx` now loads agents in parallel with API keys for the API tab, tracks loading and error state, and passes both through `ApiPlaygroundDrawer`. The selector exposes `aria-busy` and localized loading copy while the list is resolving; existing agent-error alert and selected-agent reconciliation remain intact.

## Verification

- Vue source comparison: complete for initialization and loading/error flow.
- React DOM regression verifies `aria-busy="true"` and `加载中...`.
- API Playground tests: 11/11.
- Web TypeScript check: pass.
- Real backend agent-list success/failure and browser computed-style evidence: pending.
