# R244 Agents create entry parity

## Runtime baseline

Vue `frontend/src/views/agent/AgentList.vue` renders a 28px icon-only header action beside the title. The text `创建智能体` button is reserved for the Vue empty-state variants. React's populated list rendered both the icon action and an additional right-aligned green text button, which was a React-only surface.

## Change

`apps/web/src/agents/AgentsPage.tsx` now keeps only the Vue-shaped icon header action while cards are present. The existing empty-state text button remains unchanged, and viewer/create permission gating is preserved.

## Verification

- Agents focused suite: `pnpm exec tsx --test apps/web/src/agents/AgentsPage.test.tsx` — 16/16 passed.
- Web typecheck: `pnpm run typecheck:web` — passed.
- Same-session Chrome runtime at 1355x720, DPR2, zh-CN: populated React header exposes only one 28px icon at `x=424,y=22`, matching Vue; React body no longer contains a populated-list `创建智能体` button, while the four cards and section remain present.

Full Web regression and protected agent mutation/responsive/native evidence remain tracked separately.
