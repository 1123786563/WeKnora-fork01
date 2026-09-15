# R323 login typography and control geometry

## Baseline and change

At a shared 1440x900 viewport with zh-CN, Vue `Login.vue` uses a 24px semibold title, 13px subtitle, 14px medium field labels, and 40px large inputs. React now uses the same title/label tokens and 40px input controls; the existing login card, carousel and language-menu behavior remains intact.

## Live evidence

- Vue screenshot: `artifacts/parity-20260915/vue-login-final.png`.
- React screenshot: `artifacts/parity-20260915/react-login-final.png`.
- Playwright computed geometry: Vue form y=323.30, height=336; React form y=327.80, height=331.5; both form bottoms align at 659.30. React input controls measure 40px.
- Login component tests: 7/7 passed; `pnpm typecheck:web` and `git diff --check` passed.

## Evidence boundary

This is a live unauthenticated browser screenshot/computed-geometry comparison plus component tests. It does not establish all auth states, every locale/theme/responsive breakpoint, backend/OIDC, Wails, or mobile acceptance.
