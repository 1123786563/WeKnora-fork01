# T10/#150 live browser and SQLite acceptance

Date: 2026-09-24 Asia/Shanghai. Integrated code checkpoint: `d8f7b19b5` (T10 Office, HTTP, typed Web contract, UI, confirmed key fix and responsive shell integrated). The browser used a disposable local account, synthetic graduation years and a synthetic JD. SQLite database and local files were isolated under a unique `/tmp/weknora-t10-browser.*` directory; API and Vite listened only on loopback ports 57544 and 57546. No personal data or recruiting site was used.

## Observations

| Step | Observed result |
| --- | --- |
| Confirm manual `毕业时间=2026` in shipped Career form | Profile revision 1 and one confirmed fact appeared. SQLite `career_facts` stored the original Chinese key and value. |
| Save standalone `仅限2027届` as JD | Conversation card displayed an immutable evidence link with fixed opportunity and snapshot IDs. |
| Evaluate before the confirmed-key fix | Historical evaluation at profile revision 1 showed `待确认` and `confirmed_graduation_year_missing`; this live result exposed the English-only fact-key integration defect. |
| Restart API with reviewed fix; reopen historical URL | The old `待确认` evaluation remained readable and unchanged. |
| Re-evaluate same fixed JD at revision 1 | New ID showed `不符合` immediately on the stable page. Detail showed JD citation `仅限2027届` (raw byte span 0–13) and confirmed `毕业时间=2026`, profile revision 1, fact version 1. |
| Confirm `毕业时间=2027`, re-evaluate same JD | Career profile advanced to revision 2; the new evaluation showed `符合已识别条件` with `毕业时间=2027`, revision 2, fact version 2. The previous `不符合` URL still showed revision 1 and 2026. |
| SQLite catalog | Three evaluation rows: profile revisions/statuses `1/unknown`, `1/ineligible`, `2/eligible`. Current Career fact row was `毕业时间=2027` revision 2. No old evaluation was overwritten. |
| Sign out, open fixed evaluation URL | Browser redirected to `/login` and displayed no JD or profile data. |

## Narrow viewport

At a 390×844 in-app browser viewport after the reviewed shell fix, the page screenshot showed the verdict, re-evaluation action and hard-rule card with wrapping text. Read-only DOM measurement gave `innerWidth=390`, document/body scroll width 390, shell width 390, collapsed sidebar width 60 and evaluation main width 330. The pre-fix measurement was document width 600 and main width 540, visibly clipped. With desktop preference expanded, resizing 1024→390 collapsed the sidebar and kept document width 390; explicit narrow expansion overlaid a 260px sidebar while main remained 390 (no content squeeze); resizing back restored the expanded desktop sidebar at width 260 with document width 1024. Browser viewport override was reset afterward.

## Limits and rulings

- The full Web suite passed 2356/2356 at the reviewed narrow-shell code SHA. An earlier T10 UI checkpoint had an unrelated debounce timing failure and a stalled agent-editor rerun; those earlier failures are retained in the task reports, not claimed as passing. The final integrated source has not yet had a separate full-suite run after documentation-only commits.
- The full repository Go suite earlier timed out in an unrelated opencode test and failed an environment-dependent payment assertion; affected Career/router/database/handler/container suites and architectureguard passed at their reviewed code checkpoints. PostgreSQL runtime migration was not exercised without an isolated DSN.
- The shell reviewer recorded a low risk drag/resize edge case: a drag already in progress during a breakpoint change can use a stale breakpoint and may leave listeners on unmount. Ordinary resize/toggle behavior and live viewport measurements passed; this low item is deferred and does not change T10 qualification data.
- This live run covered real auth, fixed JD, profile changes, immutable old/new evaluations, narrow rendering and logout. Same-ID timeout recovery, cross-owner denial, 403 clearing and late-response fencing are covered by focused reviewed tests, not by an induced network fault in this browser run.
