# React/Vue parity evidence — mobile graph request deduplication

Date: 2026-09-15

## Change

The mobile knowledge graph screen no longer refetches the same graph twice when a node is opened. Node presses invoke the load directly; the effect now reacts to knowledge base, mode, depth, and type changes without treating the newly stored center as a second trigger.

## Validation

- Mobile tests: 189/189 passed.
- Mobile typecheck: passed.
- `git diff --check` — pending final commit check.

## Remaining evidence

Native authenticated graph interaction still requires a device fixture with a successful graph payload; the local deployment currently exposes only the disabled-feature gate.
