# React/Vue parity evidence — resource mention compatibility payload

Date: 2026-09-15

## Change

Web chat stream requests now preserve Vue's resource-specific compatibility fields in addition to the typed `mentioned_items` array: file mentions emit `knowledge_ids`, tag mentions emit `tag_ids`, and Agent-mode MCP/Skill mentions emit `mcp_service_ids` and `skill_names`. MCP and Skill fields are intentionally omitted from quick-answer knowledge mode, matching the Vue request gate.

## Validation

- Agent selection tests: 12/12 passed.
- `pnpm run test:web` — 895/895 passed.
- `pnpm run typecheck:web` — passed.
- `git diff --check` — passed.
