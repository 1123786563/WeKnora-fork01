# R315 WeChat QR fallback localization (2026-09-15)

The IM integration wizard now resolves the unknown WeChat QR-generation failure through the five-locale integration fallback table. Provider/server error messages remain authoritative; only the unknown branch is localized.

Verification:

- Integrations view tests: 12/12 passed.
- Web full suite: 911/911 passed; failed/cancelled/skipped: 0.

Protected provider QR generation, callback/polling, browser locale screenshots and native evidence remain open.
