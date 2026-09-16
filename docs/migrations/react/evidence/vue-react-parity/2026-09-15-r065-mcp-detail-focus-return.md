# React/Vue parity evidence — MCP tool detail focus return

Date: 2026-09-15

## Change

The MCP tool detail popup now restores focus to its triggering Details button after Escape or outside-pointer dismissal. Scheduling uses `requestAnimationFrame` when available and a timer fallback for jsdom/native environments, avoiding a runtime dependency on browser-only globals.

## Validation

- MCP settings/tool directory tests: 16/16 passed.
- `pnpm run typecheck:web` — passed.
- `git diff --check` — passed.
