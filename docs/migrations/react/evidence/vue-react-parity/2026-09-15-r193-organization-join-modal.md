# R193 organization join modal

- Scope: organization join flow, invite-code and searchable-space entry states.
- Vue baseline: at 1355x720 zh-CN, invite entry is a 480px modal (`h=325.59`); searchable entry is a 560px modal (`h=462.19`) with the Vue empty title `暂无数据` and description.
- Change: React join header line-height and footer spacing/button padding now match the Vue modal geometry; searchable empty state now includes the Vue title and description and uses the Vue-sized list surface.
- Browser evidence: same-session Chrome runtime measured React invite modal `480x324.49` at `x=437.5`, searchable modal `560x460.09` at `x=397.5`; footer controls match Vue at `60x32` and the same x positions. Remaining sub-pixel/inner vertical-flow difference is recorded, not claimed as zero-diff.
- Interaction: invite/search tabs are reachable; existing preview, request-role, note clamp and join branches remain covered by the organization tests.
- Validation: organization focused tests 26/26, Web typecheck and `git diff --check` pass.
- Boundary: no protected backend preview/join or persisted avatar fixture was available in this pass; preview detail, already-member action, responsive and desktop evidence remain open.
