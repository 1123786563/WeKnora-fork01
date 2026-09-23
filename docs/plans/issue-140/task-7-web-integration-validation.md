# T07/#147 Web integration and browser validation

Date: 2026-09-24. Isolated Web code range: `85f950529..b80bf7ec30f4e0b61e38a8705cd2b8dd35b3368f`. Three code/report commits were cherry-picked in order into integration as `bc418ac43`, `9388c5fa6`, and `01f4143ef`. Independent R2 reviewer concluded Spec PASS and code-quality PASS, no remaining valid high/medium Web issue; frontend validator PASS for R2. The initial R0 high and R1 medium findings were fixed in these commits, with source reports and plans saved in this directory and the task worktree.

At integration code HEAD `01f4143ef`, `pnpm typecheck:web && pnpm test:web && pnpm build:web` completed with exit 0. The full Web suite reported **2319 passed, 0 failed, 0 cancelled**. The build completed after transforming 6930 modules. It emitted existing CSS `@import` order, invalid `calc()` whitespace and large chunk warnings; these did not fail the build and were outside this Career page change.

## Browser path

An in-app browser opened Vite on `127.0.0.1:57346`, proxied to the disposable SQLite Lite API on `127.0.0.1:57344`. A synthetic account logged in. The API stored bytes locally; its document-reader HTTP protocol used the isolated text-only test double on `127.0.0.1:57345`. The file contained six explicit labeled categories, two conflicting experience claims, no graduation year and an unrelated synthetic identity line. No real user resume, credential, or government identifier was uploaded.

Observed through the browser accessibility tree and rendered page text:

1. Opened `/platform/career` at revision 0, with no confirmed facts or sources. Uploaded the `.txt` fixture through the visible file chooser and “开始上传”. The UI reported seven pending proposals, source revision 1, missing `education.graduation_year`, and the multiple-experience review flag. All seven proposals showed exact evidence lines and remained pending; confirmed facts were still zero.
2. Confirmed the education proposal. The UI advanced to profile revision 2 and displayed one confirmed fact with the resume source ID; the other proposals stayed pending. Dismissed one of the conflicting experience proposals; revision advanced to 3 without creating a second confirmed fact.
3. Reloaded the page. Source version, review flags, one confirmed education fact, five pending proposals and revision 3 remained visible. The dismissed experience item did not reappear as pending.
4. Uploaded an invalid `.pdf`. The UI showed a failed source revision 2 and a retry/manual-entry message, while the prior confirmed fact remained visible and the file picker remained enabled for a new attempt.
5. Signed out through the account menu. The browser navigated to `/login` and no private Career content remained in the accessibility tree.

The separate live HTTP report verifies exact request replay, changed-intent 409, cross-Tenant 403 and preservation of confirmed facts after invalid upload. This browser run did not inject a transport timeout or same-scope 403 race; focused page tests and independent reviewer/validator covered those cases. It also did not exercise a production DocReader service or live PostgreSQL. The local browser tab, Vite/API/test-double services and disposable files are cleaned up after evidence capture.
