# Task 3 local browser acceptance, 2026-09-24

- Source: Issue #141 acceptance and `docs/plans/2026-09-24-issue-140-implementation.md`, Task 3.
- Code checkpoint: `5dd7fb62f7b02a8d458b29f3669f3edc94470140` in the issue-140 integration worktree.
- Environment: isolated SQLite Lite API on `127.0.0.1:49242`, Vite Web UI on `127.0.0.1:49253`, disposable local account and database. Browser interactions were observed through the in-app browser accessibility tree and screenshots.

## Observed path

1. Registered and signed in through the Web UI. The TDesign platform shell loaded.
2. Opened the “求职档案” sidebar entry at `/platform/career`. The initial profile showed revision 0, no confirmed facts, and no pending proposals.
3. Entered `毕业时间` = `2026-06` and selected “保存为提案”. Revision advanced to 1. The value appeared only as a pending proposal with “本人填写” provenance; confirmed facts remained empty.
4. Selected “确认”. Revision advanced to 2. The pending list became empty and the confirmed fact displayed `毕业时间` = `2026-06` with “本人填写” provenance.
5. Reloaded `/platform/career`. Revision 2 and the confirmed fact remained visible.
6. Signed out using the account menu. The browser reached `/login`, displaying the sign-in form without private profile content.

## Related checks and limits

The separate live HTTP acceptance report (`task-3-live-http-validation.md`) records unauthenticated denial and owner A's token with owner B's tenant header receiving HTTP 403. This browser run did not exercise a second tenant UI session. The normal full Web test script is still under a separate runner-stability correction; this record is limited to the observed local browser path. The isolated API and Vite services were stopped after observation; the disposable database was removed.
