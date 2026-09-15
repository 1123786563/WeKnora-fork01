# R292 — Integrations fallback localization

- Embed channel, IM channel, tenant API key, and agent option load fallback messages now use the shared localized `common.error` translator.
- Server-provided errors remain visible; preview-unavailable behavior remains in-place and unchanged.
- Verification: integrations focused suite 53/53; Web full regression 910/910; Web typecheck and `git diff --check` pass.
- Boundary: provider callbacks, protected integration mutations, same-session Vue visual comparison and native evidence remain open.
