# React/Vue parity evidence — mobile configuration and identity race guards

Date: 2026-09-15

Mobile configuration catalog loads and identity capability loads now ignore superseded responses. Refreshes cannot reintroduce stale agent/model/MCP/skill rows or capability errors after a newer request has started.

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check`: passed.

Native configuration and identity interaction evidence remains pending.
