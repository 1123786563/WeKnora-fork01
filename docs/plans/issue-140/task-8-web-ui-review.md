# T08 Web UI independent review and integration

- Reviewed isolated UI code `fbc3afac310a40e2e661722f49fd97318d068dd7` and fix `3fd9c19369c45cef5be104a11f67f1cce02165eb`, based on the reviewed Web contract. Integrated as `03fd933d6` and `04fca5bab`.
- Initial independent reviewer: Spec FAIL/quality FAIL with two medium findings. Repeated Save after success created a fresh request ID and duplicate Opportunity; `opportunity.css` copied 466 global CSS lines.
- Fix: saved state blocks another POST until explicit new draft, with focused test asserting exactly one request; the stylesheet contains only 71 Opportunity-specific lines. Final independent reviewer: **Spec PASS/quality PASS**, no blocking findings.
- Final independent frontend validator: focused 36/36, Web typecheck, full Web suite 2329/2329, build, diff check PASS at reviewed source SHA. Browser E2E and screenshot after CSS deletion were outstanding at review time, so validator reported `DONE_WITH_CONCERNS`. The controller owns that acceptance gate.
- At integrated `04fca5bab`, `pnpm typecheck:web` and the focused Opportunity/chat/router tests passed 36/36. The full Web run and build at the same source code were passed by the implementation and validator; copied reports contain exact commands and warnings.
