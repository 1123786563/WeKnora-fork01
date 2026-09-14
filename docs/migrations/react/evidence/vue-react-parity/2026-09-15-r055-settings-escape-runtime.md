# R055 Settings Escape runtime

- Runtime: authenticated React Web `/platform/settings?section=mcp`.
- Action: sent `Escape` while the settings drawer was open.
- Observed result: URL returned to `/platform/knowledge-bases`; after the loading transition the protected shell and knowledge-base list rendered normally.
- No configuration or backend data was changed.
