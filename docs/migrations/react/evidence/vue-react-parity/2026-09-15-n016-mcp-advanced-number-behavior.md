# N016 MCP advanced number behavior

## Vue baseline

Vue advanced timeout/retry inputs use text-backed number fields: users may clear or type intermediate values, and `onAdvancedNumberBlur` applies the field default and min/max bounds on blur. The suffix units remain passive visual labels.

## React change

React advanced fields now preserve a transient empty value during editing and normalize only on blur; payload construction applies the same fallback and bounds as a final guard. Existing unit suffixes and API field mapping remain unchanged.

## Verification

- MCP settings tests: 15/15 passed, including transient-empty and normalization cases.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

The normalization contract is unit-tested but authenticated browser keyboard/blur capture, live MCP mutation, and Wails/native evidence remain open.
