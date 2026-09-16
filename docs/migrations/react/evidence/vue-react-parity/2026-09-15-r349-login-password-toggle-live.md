# R349 login password-toggle live comparison

- Runtime: backend `:8080`, Vue `http://localhost:5180`, React `http://localhost:5181`, Chrome native UI, same browser window and `zh-CN` locale.
- State: unauthenticated `/login?next=%2F`; no credentials were submitted in this capture.
- Vue baseline: password control exposes the trailing eye affordance; login card, labels, hint, CTA and three feature rows are visible.
- React before repair: same structure and geometry, but the password input had no trailing visibility affordance.
- Repair: `apps/web/src/auth/LoginPage.tsx` now wraps the password input with a project-styled visibility button and toggles `password`/`text` without changing the form contract.
- Browser recheck: React now exposes the password control plus a `密码` button in the accessibility tree; the live screenshot shows the eye affordance in the same trailing position as Vue.
- Automated evidence: `apps/web/src/auth/login-page.test.tsx` password-toggle test passes; focused login tests 8/8.
- Boundary: this is an unauthenticated browser/UI comparison. It does not prove authenticated page, backend mutation, Wails or native acceptance.
