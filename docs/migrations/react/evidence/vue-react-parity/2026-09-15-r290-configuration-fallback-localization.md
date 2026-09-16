# R290 — Configuration fallback localization

- The compatibility configuration page now localizes agent, model, MCP, skills load failures and remove failures through the shared translator.
- Server-provided error messages remain visible when present.
- Verification: configuration focused tests 3/3; Web full regression 910/910; Web typecheck and `git diff --check` pass.
- Boundary: the legacy configuration route still has broader visual and runtime parity work beyond this fallback slice.
