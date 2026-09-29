# Issue #72 Parallel Evidence Refresh — 2026-09-29

**Captured:** Authenticated GitHub enumeration at 2026-09-29T21:17:59+08:00 (Asia/Shanghai), read-only against `1123786563/WeKnora-fork01` using the native sub-issue API. A supplemental live inventory was captured at 2026-09-29 13:30:12 UTC (21:30:12 Asia/Shanghai). These ephemeral observations are not claims about later state.

## Scope and issue graph

Root #72 has 33 unique native direct children, #73–#105, with no nested children. #30 is not a child and is excluded. At capture, #72 was open; #74 and #81 were GitHub-open although internal implementation status is done based on recorded evidence. #73 and #75–#80 were closed. The existing [DAG](issue-72-dag.md) describes dependency edges separately from containment; this refresh does not change its graph or promote any task to verified.

## #86 evidence provenance and current-code boundary

The #86 evidence package is present in commits `c7b696b6a` and `65647cc55`. Both commits were verified as ancestors of candidate source checkpoint `85fd67f7f8be9d39d7f9f30b9a439524866fcffe` on ref `codex/issue-72-r8-runbook` and integration HEAD `8329b85d4dfff301d03f94406dfc829d87cb5b26` on ref `codex/issue-72-lago`; both refs were checked. The committed evidence artifacts match these SHA-256 values:

| Artifact | SHA-256 |
|---|---|
| [Evidence README](issue-72-flow-evidence-86/README.md) | `2b4b2676894abbceb718d91eedf857915707d1132841a59fa058ff0f86625979` |
| [Reconciliation output](issue-72-flow-evidence-86/reconcile-output.txt) | `e6adba29961141ab1388bec27a5b09892ae4288292f8d4ad68eb56fa989bc3bd` |
| [Acceptance record](../testing/lago/credits-order-acceptance.md) | `0b412731d4de338b3086e71624785370a7977ec5162b5c1b3beca704a18d36ab` |

The README identifies historical live source `dd089662becc43edb5a44dfb6f2a29f5b343ae37`. Using the same method (extract each revision's file with `git show`, then SHA-256 the bytes), representative `internal/modules/commercial/commercialplatform/lago.go` and `lago_credits_order_integration_test.go` have matching hashes at that historical source and integration HEAD: `lago.go` is `f1fe9f972f3bcc7e9a701aa35ea3f37aedf624aa47a65722c1957e6fd53c04de`; the integration test is `4553d135253b6392ce4bbfc0b1faa08bd0574d6f13a41fcda9beabea784c673d`. They match in these representative files; the revisions differ in 81 files overall. The README does not record a fresh live replay at the exact current integration checkpoint or stable-identity verification. The committed historical artifacts remain hashable, but do not establish fresh exact-checkpoint live acceptance. #86 remains acceptance-unverified.

## #87–#89 readiness and rulings

- **#87 Task 0 remains blocked-env.** The [`probe-results-r7-wallet-consumption.json`](../../../lago-int/docs/migrations/lago/t15-admission-pricing-r7-evidence/probe-results-r7-wallet-consumption.json) records the initial blocked-environment result, started at `2026-09-28T14:50+08:00` with setup `./deploy/lago/lago.sh up`. The later explicit-isolation rerun, API health HTTP `000`, and resulting zero-call conclusion are documented in the sibling integration worktree's [`README.md`](../../../lago-int/docs/migrations/lago/t15-admission-pricing-r7-evidence/README.md); they are not fields in that JSON. This zero-call observation is specific to that later R7 rerun. Earlier R2 CNY and R6 wallet probes made authenticated API calls and remain separate observations; none establishes wallet debit, settlement, negative-balance, or over-limit behavior. Fix4 evidence remains offline-only, and the script/result hash mismatch prevents treating the R7 result as exact-script runtime evidence. Task 0 remains unverified; no live contract or acceptance claim is made.
- **R-6 is settled and unchanged.** The approved ruling in [user rulings](issue-72-user-rulings.md) assigns each billable dimension through the published Plan Version's explicit Billable Metric mapping. Ambiguous ownership fails closed only for the affected billable dimension; other uniquely owned dimensions may continue. Paid-plan purchases carrying Lago Usage Charges remain excluded. This audit does not reopen those decisions or imply code compliance.
- **#88 and #89 remain blocked** on #87. Their separate readiness audits found no safe independent implementation slice. No #72 node is promoted to ready or verified by this refresh.

## Observed work and resource state

At the supplemental inventory time, the Paseo registry showed no #72 implementation agent; its only listed agent was the unrelated idle `deepseek-harness`. The read-only #72 audit subagents in the parent collaboration had completed. Separately, Issue #140 OCR PID 25829 was still live after about 23 minutes; the other OCR operation was not live. No matching #86/#87 probe, test, or Docker command was visible in `ps`. Root-cwd terminal shells could not be mapped to an agent or task, so process ownership remains partly unresolved. The existing shared `lago-int` integration worktree had 41 status entries and remains untouched. Shared database/Docker ownership was unresolved; these observations do not establish that those resources were free or released.

The safe frontier is read-only planning and evidence reconciliation that does not claim the shared database/Docker slot. Implementation readiness must be reassessed only when the #86/#87 acceptance gates and resource ownership have current evidence.

## Read-only TTY/session correlation — 2026-09-29 22:36 CST

At this capture, read-only `ps` showed four live `codex` processes: `ttys001`/PID `78779` (started 13:50:56 CST), `ttys002`/PID `81039` (13:51:37), `ttys003`/PID `84486` (13:55:00), and `ttys004`/PID `88833` (13:55:50). `lsof -a -p PID -d cwd` reported `/Users/wuyongjun/trea/WeKnora-fork01` for each. Rollout `session_meta` records map one-to-one by start time: PID 78779 → session `01a0ebb7-72be-7cb2-a48e-bddb059ec1a5` (13:51:03); PID 81039 → `01a0ebb8-01aa-7201-9632-1cbe61427b36` (13:51:39); PID 84486 → `01a0ebbb-3fa1-7b30-9be0-85351aeb9463` (13:55:12); PID 88833 → `01a0ebbb-eb35-7453-b985-f9c0ceaef7b7` (13:55:56). Each rollout record has repository-root cwd and branch `main`.

Verified rollout-goal attachments map TTY001/PID 78779/session `01a0ebb7-72be-7cb2-a48e-bddb059ec1a5` to Issue #30 (attachment `b10249b8...`); TTY002/PID 81039/session `01a0ebb8-01aa-7201-9632-1cbe61427b36` to Issue #72 (attachment `f247...`); TTY003/PID 84486/session `01a0ebbb-3fa1-7b30-9be0-85351aeb9463` to Issue #140 (attachment `6c1...`); and TTY004/PID 88833/session `01a0ebbb-eb35-7453-b985-f9c0ceaef7b7` to Craft #107, based on its rollout goal and T14 answer. This establishes issue-level mapping only: no specific child/task or dedicated worktree is established. TTY002 for #72 remains live, so no #72 lane is marked released. This correlation is observational only; no shell was acted on.

## Sources and limits

- Authenticated native-child enumeration captured at `2026-09-29T21:17:59+08:00`; root and child statuses above are a point-in-time snapshot.
- [Issue #72 DAG](issue-72-dag.md), [execution ledger](issue-72-execution-ledger.md), [#86 ledger](issue-72-ledger-86.md), [user rulings](issue-72-user-rulings.md), [approved billing spec](../specs/2026-09-20-lago-billing-migration-design.md), and [#86 acceptance record](../testing/lago/credits-order-acceptance.md).
- Commit ancestry, artifact hashes, and source hashes were checked in the isolated refresh worktree against the recorded revisions. Paseo/process state is ephemeral and limited to the stated observation time.
