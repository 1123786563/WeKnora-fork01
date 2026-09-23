# T07/#147 backend Review — fix round 1

Date: 2026-09-24 Asia/Shanghai. Code scope: `cd81dc222f31a07e5aad4a8f4022f055872f3cd2..074021e5676d0c2b5f3045ed6b991a82f16ed312`; report-only HEAD `dbca655f5`. Independent reviewer verdict: **Spec compliance FAIL; code quality FAIL**. Independent backend validator: DONE_WITH_CONCERNS; focused Go tests pass but do not exercise the cases below. No T07 code has been integrated.

| ID | Priority | Finding | Required result |
| --- | --- | --- | --- |
| T07-R1-F1 | high | `handler.go:153-171` now rejects `source.kind='user'`, while integrated T03 `apps/web/src/career/CareerPage.tsx:60,88-89` sends that value for all four mutation actions. | Preserve/normalize current Web wire alias, or update client and shared contract atomically; current profile actions continue working. |
| T07-R1-F2 | high | `handler.go:266,291` includes current profile revision in upload intent when optional `expectedRevision` is omitted. First success advances revision; exact retry falsely conflicts. | Resolve stored request before defaulting revision; omitted-revision exact retry returns same source and receipt. |
| T07-R1-F3 | high | `profile_intake.go:138` lease expires at now+30m; `handler.go:370` cleanup waits until lease is another 30m old. In that gap `ClaimUpload` can save a second raw blob and replace a known resource ref; old worker is unfenced. | Reconcile at actual expiry, preserve/release old ref, CAS-fence write and completion by claim token/epoch, test takeover. |
| T07-R1-F4 | high | `upload.go:77` ignores `Release` failure, then `FinishSource` clears the last resource ref; conflict path also ignores release error. | Keep a durable cleanup-pending ref until deletion succeeds; test failed release and retry. |
| T07-R1-F5 | high | `model_input.go:58` value redaction still uses a word boundary. `110105199001011234A` survives final JSON. | Apply robust in-token redaction to all serialized values and test adjacent ASCII. |

Disposition of R0: F1 distinct item keys and F6 migration rollback resolved. F2 source validation resolved for forged direct confirm but introduced R1-F1 Web compatibility regression. F3, F4 and F5 are partially resolved and correspond to R1-F5, R1-F2, R1-F3/F4. The independent reviewer ruled the generic FileService crash after physical write but before it returns a ref an infrastructure limitation outside T07's controllable window; it remains recorded, not a standalone #147 acceptance blocker. PostgreSQL live migration has not run. Fix round 2 must cover the five high findings without broadening #147's scope.
