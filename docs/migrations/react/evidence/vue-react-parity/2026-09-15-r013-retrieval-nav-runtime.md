# R013 retrieval settings navigation runtime evidence

Date: 2026-09-15

- Authenticated React Chrome reached `/platform/settings?section=retrieval`.
- The sidebar label rendered `搜索设置` instead of the previous English `Retrieval`; the content heading and description remained localized.
- The section still enforced its admin-only visibility and the retrieval controls remained reachable.
- The available Vue tab had no authenticated session, so same-session pixel comparison remains open.
