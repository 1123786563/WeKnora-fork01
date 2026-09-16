# R206 agent collapsed rail

- Scope: `/platform/agents` collapsed resource rail.
- Vue baseline: `ListSpaceSidebar.vue` collapsed `.icon-strip` renders each icon and label only; counts are included in tooltip content and are not rendered as a third visible text row.
- Finding: React rendered counts below every collapsed rail label, changing item height and visual hierarchy.
- Change: React `AgentRail` keeps count values in the button title for tooltip-equivalent access and removes visible count spans.
- Browser evidence: live React rail buttons now expose text `全部/收藏/最近/本空间` with titles `全部 (4)` etc., and no visible count text nodes.
- Validation: Agents focused tests 36/36, Web typecheck and `git diff --check` pass.
- Boundary: expanded-rail drag/resize and protected agent mutations remain separate acceptance gates.
