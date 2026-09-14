# React/Vue parity evidence — mobile editor reload race guard

Date: 2026-09-15

Wiki and FAQ editor reloads now use a generation token. A late response from an earlier reload cannot overwrite the current draft, conflict state, error, or loading indicator.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native editor interaction and conflict-flow evidence remain pending.
