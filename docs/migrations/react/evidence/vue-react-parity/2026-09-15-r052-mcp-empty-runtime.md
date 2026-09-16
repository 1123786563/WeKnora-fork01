# R052 MCP settings empty runtime

- Runtime: authenticated React Web `/platform/settings?section=mcp`.
- Observed: settings drawer and MCP section load through the lazy boundary; the page shows the MCP management heading, localized description, and `添加服务` entry after the loading state resolves.
- The tenant has no configured MCP service, so synced/stale tool-directory states are not reachable in this fixture.
- No service was created or modified.
