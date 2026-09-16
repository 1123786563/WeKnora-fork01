# Vue/React browser parity evidence — 2026-09-15

## Conditions

- Vue: `http://127.0.0.1:5180/login`
- React: `http://127.0.0.1:5181/login`
- Browser viewport: 1355 x 776 CSS pixels
- Locale: zh-CN
- Backend: unavailable for authenticated data loading

## Observations

- Both login pages rendered the same public header, language control, product
  statement, feature list, login heading, subtitle, email/password controls,
  login action, and registration entry.
- The Vue runtime did not show visible required-field stars. React had shown
  them, so the React labels were changed to match the Vue runtime.
- Navigating to React `/platform/apps` while unauthenticated redirected to
  `/login?next=%2Fplatform%2Fapps`; the login surface remained usable.

## Classification

- Evidence type: browser runtime, public unauthenticated states.
- Not proven here: authenticated app catalog/connection/authorization/action
  states, real backend mutations, permission variations, desktop rendering,
  and mobile rendering. Those remain review or blocked-env in the ledger.
