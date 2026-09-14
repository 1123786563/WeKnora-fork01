# R046 Chat resource mention payloads

- Scope: React chat mention models and loading now support `kb`, `file`, `tag`, `mcp`, and `skill` items. The protected chat host loads available KBs plus recent documents, MCP services, and skills with partial-failure tolerance.
- Payload: stream and steer serialization preserves the resource type and resource-specific identifiers (`kb_id`, `kb_name`, `skill_name`) so backend routing can consume the same contract as Vue.
- Verification: Web tests 892/892, `pnpm run typecheck:web`, and `pnpm run build:web` passed. Added regression coverage for all five resource types.
- Limitation: tag discovery still depends on an upstream selected-KB/tag provider; this slice does not invent tag data when the API does not expose it. Browser evidence for non-KB options requires tenant fixtures containing those resources.
