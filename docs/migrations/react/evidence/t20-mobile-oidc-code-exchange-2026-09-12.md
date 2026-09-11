# T20 mobile OIDC code-exchange follow-up (2026-09-12)

- Commit `708132a` adds the native OIDC handoff boundary. Mobile requests the
  server callback URL plus the allowlisted `weknora://oidc` frontend target;
  the server signs that target into state, redirects only a short-lived
  provider code, and exposes `POST /auth/oidc/exchange` for the authenticated
  token exchange.
- Mobile rejects bearer-shaped legacy callback payloads, foreign schemes,
  missing state, and state mismatches. The server rejects an unallowlisted
  frontend redirect and validates the signed mobile state before exchanging.
- Verification passed: mobile OIDC tests 3/3, API-client auth tests 12/12,
  four `TestOIDCMobile*` handler tests, mobile typecheck, and
  route/middleware registration checks.
- No external IdP or physical-device callback was available. Provider success,
  cancellation, PKCE/nonce behavior at the IdP, and deployed deep-link
  registration remain live acceptance gates; T20 stays `review`.
