# R048 Chat mention aggregate error state

- Scope: the multi-resource mention loader now distinguishes partial success from total failure. When every primary resource request rejects, it resets the loaded guard and exposes the localized knowledge-base load error so retry can issue a fresh request.
- Partial success: any fulfilled resource keeps the picker usable; tag fan-out remains independently tolerant.
- Verification: Web tests 892/892, `pnpm run typecheck:web`, and `pnpm run build:web` passed.
