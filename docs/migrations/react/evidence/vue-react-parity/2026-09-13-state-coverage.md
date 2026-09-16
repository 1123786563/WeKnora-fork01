# 2026-09-13 state-coverage evidence sweep (R002 + R009)

Dual-end browser captures of state branches that prior sweeps did not
cover. No data mutation (wrong-password attempts only produce auth
errors). Script: .parity-tools/state-coverage.cjs (gitignored by repo
convention). Output: screenshots/state-coverage-20260913/ (10 pairs).

## R002 /login
- empty form: submit-enabled on both ends (submitDisabled=false) — the
  forms rely on submit-time validation, consistent behavior; both
  screenshots saved (login-empty.png).
- failure (wrong password): REAL divergence found — Vue presents a
  top-center MessagePlugin toast with the backend message
  (Login.vue:695-697); React rendered an inline red banner inside the
  form card. FIXED same day (commit 0f6f4d12): LoginPage now renders a
  top-center .auth-toast (error/success tones, 3s dismiss), localized
  fallback aligned to auth.loginError, register success also toasts
  (Login.vue:727/742). jsdom pin test added (login-page.test.tsx,
  red->green). Re-shot after the fix: /tmp/react-login-toast.png
  matches the Vue presentation (top-center white card, red text).

## R009 /platform/knowledge-bases
- loading state (API deferred 3s): both ends captured mid-load.
- error state (API 500 forced): both ends captured; presentation of the
  failure state to be compared in a dedicated slice (not yet reviewed
  pixel/text-wise — listed for the acceptance batch).

## Limits
- One locale (zh-CN), one account, 1440x900. States beyond
  loading/empty/error/failure for these two rows (permissions, submit
  flows) are NOT covered by this sweep.
- The kb-list 500-forcing route mock intercepts only knowledge-bases
  API calls; login itself used the real backend.
