# 2026-09-14 R012 agents page state coverage (dual-end)

States captured (screenshots/r012-states-20260914/): normal / loading
(4s deferred API) / error (500) / empty (empty list), both ends.

## Findings
1. REDEFECT (React): list-fetch failure leaks the raw error payload —
   {"message":"mock failure"} rendered as text on the agents page. Vue
   renders the clean empty state with no error surface (AgentList.vue
   fetchList has no error UI). Fix queued: drop the raw Status banner,
   match Vue's silent fallback.
2. Empty-state CTA color divergence: Vue 创建智能体 (empty state) is
   purple/indigo; React renders it brand green. Registered as a visual
   slice item (needs the Vue empty-state component's exact token).
3. Loading state: both ends show skeletons; Vue shows the section
   skeleton within the card grid, React within the same grid — visually
   equivalent at capture time.
4. Normal state: 4 built-in agent cards match (names, descriptions,
   capability chips, purple/green icon chips).

## Limits
- zh-CN only, one account, 1440x900. Editor drawer states and
  create/success flows are covered by the FAQ/R012 slice records.
