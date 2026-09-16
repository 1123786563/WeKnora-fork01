# R178 knowledge-base list layout

- Scope: `apps/web/src/App.tsx` knowledge-base list page.
- Change: removed conflicting page-level max-width/padding utilities and moved content spacing to the list content column, allowing the Vue-style rail and responsive card grid to use the full shell width without duplicated margins.
- Validation: knowledge-base focused suite passed 40/40; full Web suite passed 895/895; Web typecheck passed.
- Browser evidence (Chrome, 1355x720, zh-CN, same seeded account/data): Vue content column `x=316,y=0,w=1039`, padding `20px 0 0 28px`, heading `x=344,y=20`; React returned the same values after the fix. Vue and React shell both expose a 26x26 search affordance at `x=195,y=20`; React search opened the command palette (`role=dialog`, 640x314) and Escape closed it.
- Boundary: responsive widths beyond this viewport, protected backend mutations, and desktop/mobile-native rendering remain separate acceptance gates.
