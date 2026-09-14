# R013 MCP settings runtime evidence — 2026-09-15

- Target: authenticated React Chrome at `http://localhost:5181/platform/settings?section=mcp`.
- Accessibility tree shows localized `MCP服务` heading, `MCP 服务管理` description and `添加服务` action.
- No MCP service row was present in this session, so the destructive confirmation dialog was not opened. Its copy is covered by the shared five-locale key and focused tests.
- Vue comparison remains open because the available Vue tab is unauthenticated and redirects to `/login`.
