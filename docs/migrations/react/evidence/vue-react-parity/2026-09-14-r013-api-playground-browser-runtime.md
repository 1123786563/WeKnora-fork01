# R013 — API Playground 浏览器运行时复核

Date: 2026-09-14

## React browser evidence

- Target: `http://localhost:5181/platform/settings?section=integration-api`
- Browser: Chrome, authenticated owner account, zh-CN, viewport reported by the page as `1355 x 720`.
- Clicking `打开 Playground` produced an accessible `API Playground` dialog and a sibling `调整抽屉宽度` separator outside the settings dialog. The drawer was attached under `document.body`, matching the Vue `SettingDrawer` teleport boundary.
- DOM computed style for `.wk-api-playground-drawer`: width `640px`, height `720px`, right-side fixed placement (`left=715`, `right=1355`), white background, and left shadow `-12px 0 32px rgba(0, 0, 0, 0.18)`.
- The agent control exposed `role=combobox`, and the empty state remained localized. No API Key existed for this owner account, so running the real session/SSE request was correctly disabled; no credential was created for this verification.

## Discrepancy / acceptance boundary

The accessibility snapshot after the click contained the Portal drawer, but two subsequent Chrome screenshots still displayed only the underlying settings page and did not visually show the drawer. This is a runtime compositor/screenshot discrepancy requiring investigation; it is not accepted as same-condition screenshot parity. The browser DOM/computed-style evidence therefore remains partial, and R013/N028 stays `review`.

## Evidence layers

- Browser DOM/accessibility: passed for open state, Portal boundary, drawer geometry, and combobox presence.
- Browser screenshot comparison: blocked by the inconsistent screenshot result above.
- API backend success/failure/permission matrix: not run; the current account has no API Key and creating one is outside this verification scope.
- Vue same-condition screenshot, Wails, and native evidence: pending.
