# R288 — Chat operation fallback localization

- Chat session, agent, message/history, resume, artifact download, session creation, pin, delete, and clear-message fallback strings now use the localized `operationFailed` copy from the shared chat catalog.
- Server-provided `Error.message` values remain authoritative and visible.
- Verification: chat focused tests pass; Web typecheck passes; latest full Web regression is 910/910; `git diff --check` passes.
- Boundary: provider/network failure paths and browser locale screenshots remain open for the broader parity matrix.
