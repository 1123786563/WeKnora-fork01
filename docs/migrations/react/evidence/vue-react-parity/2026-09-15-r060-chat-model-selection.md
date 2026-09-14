# React/Vue parity evidence — chat model selection

Date: 2026-09-15

## Change

The React chat composer now exposes the loaded model catalog as an accessible model selector. The selected model is carried through `ChatSubmission.modelId` and serialized as `summary_model_id` for both knowledge and agent chat streams. Empty model ids are omitted, and the first loaded model is selected when no prior selection remains valid.

## Validation

- `pnpm run typecheck:web` — passed.
- `pnpm run test:web` — 894/894 passed.
- `pnpm run build:web` — passed.
- `git diff --check` — pending final commit check.

## Remaining evidence

The browser tenant currently exposes no configured model catalog, so a live visual interaction with multiple selectable models remains gated on a fixture containing at least two enabled models. The request serialization and component behavior are covered by the Web regression suite.
