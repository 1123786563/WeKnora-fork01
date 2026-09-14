# R051 Chat mention capability-aware failure handling

- Scope: aggregate mention failure detection now distinguishes unavailable optional resource methods from real requests. Test doubles or reduced clients no longer turn fallback empty promises into false success.
- Behavior: a real fulfilled KB/document/MCP/Skill request keeps the picker usable; when all available primary requests fail, localized retryable error state is exposed.
- Verification: Web tests 892/892, `pnpm run typecheck:web`, and `pnpm run build:web` passed.
