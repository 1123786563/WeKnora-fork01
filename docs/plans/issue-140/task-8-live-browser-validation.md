# T08/#146 live browser and SQLite acceptance

Date: 2026-09-24 (Asia/Shanghai). Integration code checkpoint: `2fc613c1bcf691acb3a889f635d86840d8015a06` (T08 backend, guard, client and Web UI integrated; latest changes after UI code were documentation only). A disposable SQLite Lite API ran on loopback `127.0.0.1:57444`; Vite on `127.0.0.1:57446` proxied `/api` to it. The browser used a disposable local account and a synthetic JD. No personal data or real recruiting website was used.

Input: four lines of synthetic Chinese JD text describing a quality engineer, Shanghai location and test skills, followed by a deliberately inert instruction to call a deletion tool. Source label was `本地合成 JD`; reference was an `example.invalid` URL stored as metadata. The UI clearly stated that the paste would not be sent to chat or execute instructions.

## Observations

| Step | Browser/API result |
| --- | --- |
| Open `/platform/creatChat` as authenticated user | Visible `保存职位描述` panel with raw-text, optional source label/reference and disabled Save before text entry. |
| Save synthetic JD | Button became disabled `已保存`; request ID and `查看已保存的 JD 证据` link appeared. Link contained both immutable opportunity ID and `snapshotId`. |
| Open result link | Evidence page showed exact four-line JD as plain text, source label/reference, server acquisition time, fixed snapshot ID, `待确认`, and five extracted fields explicitly `未知`. The instruction line was displayed literally and did not trigger an action. |
| Reload evidence URL | The same snapshot ID, raw text, source, acquisition time and unknown fields reappeared from API storage. |
| SQLite catalog check | Exactly one row in each of `career_opportunities`, `career_opportunity_observations`, `career_opportunity_snapshots`, `career_opportunity_receipts` after the single UI save. |
| Sign out, then navigate directly to fixed evidence URL | App redirected to `/login` and displayed no JD content. |

The in-app browser screenshot at the loaded evidence page was inspected: source/time/status and extracted-field cards rendered visibly without copied global styles; the raw text section began below them. The full URL and UUIDs were transient test IDs; no screenshot artifact was exported. The browser evidence directly covers paste → result action → fixed evidence → reload and protected-route logout. Same-request replay/conflict, cross-tenant denial, transient commit recovery and late scope-response fencing remain covered by reviewed Office/HTTP and focused Web tests rather than this single browser run.

The `browser-use` CLI named in the optional browser skill was unavailable (`command not found`), so the app's in-app browser control was used for this local interactive acceptance. Disposable API/Vite processes and SQLite directory are stopped/removed after this report is saved.
