# T08 Web contract independent review and integration

- Source code commits: `efd18d67feaac09d8ee6347df509f537ee0cf10e` and review fix `9fbf9f5ce73eada47fc357e01da1b4d08dfd5ba3`, based on `d830e261dc465ac9b734040922d0979cf7acfffc`.
- Integrated as `676cd8cd2` and `f37c7e961` after independent review and validation; the same Web task worktree retains the original source commits for Task 2.
- First reviewer found one medium malformed-wire gap: evidence decoder accepted blank JD text. A low impossible-date gap also affected `acquiredAt`. The fix added RED tests and rejected both malformed values while preserving valid JD whitespace/CRLF and valid offset timestamps unchanged.
- Final independent reviewer: **Spec PASS, quality PASS**, no remaining critical/high/medium finding. Final frontend validator: focused 9/9 tests, Web typecheck and diff check PASS at reviewed code SHA.
- Integrated `pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts` passed 9/9 and `pnpm typecheck:web` passed after cherry-pick. Opportunity UI and browser acceptance remain outstanding.
