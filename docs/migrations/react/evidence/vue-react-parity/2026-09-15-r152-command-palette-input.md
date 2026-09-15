# R152 global command palette shared input

- Scope: `apps/web/src/platform/GlobalCommandPalette.tsx`.
- Change: command palette search now uses shared `Input` with forwarded ref, preserving keyboard shortcut handling, query state, focus behavior and localized placeholder.
- Validation: direct shortcut suite passed 11/11; existing Web typecheck/full regression remain green.
- Boundary: browser focus/compositor comparison remains open.
