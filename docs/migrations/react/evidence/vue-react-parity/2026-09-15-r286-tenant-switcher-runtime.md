# R286 — Tenant switcher runtime evidence

- Environment: authenticated React Web page `http://localhost:5181/platform/knowledge-bases` in Chrome.
- Action: opened the user menu, expanded `切换空间`, and inspected its accessible list.
- Observed: the menu exposes a `list` named `切换空间`; the current membership is marked selected and labeled `当前`.
- The authenticated account exposed one membership, so no destructive or cross-tenant switch was attempted. Provider switch callback and post-switch protected data refresh remain open.
