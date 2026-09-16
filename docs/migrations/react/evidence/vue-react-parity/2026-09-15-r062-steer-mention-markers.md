# React/Vue parity evidence — steer mention markers

Date: 2026-09-15

## Change

The in-stream steer composer now uses the same five resource markers and `data-mention-type` hooks as the main chat composer. Selected chips and listbox options preserve the resource type for KB, file, tag, MCP, and skill mentions while keeping the existing keyboard and removal behavior.

## Validation

- Focused shared chat renderer tests — 63/63 passed.
- `pnpm run test:web` — 895/895 passed.
- `pnpm run typecheck:web` — passed.
- `git diff --check` — passed.

## Remaining evidence

Live steer interaction still depends on an authenticated streaming session fixture; the current tenant does not expose a running stream or non-KB mention resources.
