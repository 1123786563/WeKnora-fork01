# Mobile workbench staged release report

- Generated: 2026-09-17 (W37 final regeneration round — five mobile lanes W13/W14/W15/W16/W24
  integrated into the candidate, all 37 tasks W01–W37 accepted in both ledgers),
  candidate baseline `72abc128` (regenerated twice: first at `f0f39fa0` before
  the last lanes closed, then at `46427a9e` before the five-lane integration;
  the lane merges `71294f7b`/`b6a701e9`/`061a442a`/`0d9baebd`/`05725468` are
  all ancestors of this HEAD — see `lane-integration-report.md`)
- Acceptance document: `docs/evidence/mobile-workbench/acceptance.json`
  (gate: `node scripts/mobile-workbench/check-acceptance.mjs`)
- Ledgers: `docs/superpowers/plans/mobile-workbench-progress.md` (W ledger,
  coordinator-written) and `.superpowers/sdd/2026-09-12-mobile-ai-saas-workbench/progress.md`
  (SDD ledger). This report reflects the FINAL state after all 37 tasks were
  accepted and the five lanes were merged; pass-row commands were re-run at
  the final HEAD.
- Every "delivered" claim below is an accepted-with-blockers task unless
  stated otherwise: code-level evidence was independently reviewed AND its
  baseline commit is an ancestor of the candidate. Layers marked blocked-env
  were never executed here (no iOS/Android projects, no signing identities,
  no live Paseo/PG/deployment).

## Delivered (independently reviewed, code level, anchored in the candidate)

| Slice | Tasks | Strongest evidence pointer |
| --- | --- | --- |
| Core execution lane | W01–W07, W09–W12 | W10 final14 review (mounted send → durable read → product navigation, 28/28), W11 review (keyset list on real SQLite harness), W12 re-review (recovery 24/24 + mounted 10/10); W01–W06 survive as ledger-text acceptances (see blocked-env) |
| Native OIDC exchange | W08 | seam review (PASS/APPROVED): POST `/auth/mobile/exchange` verified in the test router; baseline re-anchored in the regeneration round to `bd624219` (the original `661a7b7c` is a dangling object); the W08 handler test family (`go test ./internal/handler -run 'TestOIDCMobile' -count=1`, 5/5) is green at the final HEAD — the full-chain test was repaired by the final-review F1 fix (see Known risks #5) |
| Notifications server lane | W13–W16 | Lane integrated in the final round (merges `71294f7b`/`b6a701e9`/`061a442a`/`0d9baebd`, ancestor-verified): W13 final review (device registration, lifecycle fencing, deterministic races; repository suite green at the final HEAD), W14 scoped retry-error review (Spec PASS / Quality APPROVED — the ledger's cited review; outbox migrations renumbered 000146, repository suites green), W15 empty-token review (provider lifecycle/pause/batch retry/revoke revision; `-race ./internal/notification` green at the final HEAD — the package never compiled on its lane), W16 rereview (deep-link parser 2/2; the Vitest suites the review could not execute were made genuinely passing by the post-integration fix `96efa90c`, 11/11) |
| Remote lane (code) | W17–W24 | W17 static contract probe + W18–W23 ledger acceptances; W24 final3 review (trusted target policy, platform/BYOK/parent admission, durable budget, late/replay settlement) — lane merged in the final round (`05725468`, usage-binding migrations renumbered 000071/000149, admission coordinator fused with the W34 capability gate; persistence suites re-run green at the final HEAD) |
| Resources | W25–W28 | W25 re-review (mounted upload chain), W26 re-review (immutable artifact versions + wiring), W27 re-review (preview CSP parity, bounded tickets, user-gesture external opens), W28 review (citation registrar 6/6 + component suites 27/27 — flipped to delivered in this round) |
| Voice | W29–W31 | W29 review (dictation 20/20 + mounted hold-to-talk), W30 rereview2 (exactly-once settlement under the real gate, honest settled semantics, 19/19 matrix — flipped in this round), W31 rereview1 (realtime controls, settle-on-fail five paths, dispose-on-unmount, live-progress port — flipped in this round) |
| Advanced interaction ports | W32 | review (PASS/APPROVED 0 C/I): seven-operation CAPABILITY_UNAVAILABLE gate with zero shell fallback, one-time terminal tickets, 78-row parity matrix honestly 12 harness / 66 open / 0 native, manifest 695 target correction with 0 hash changes |
| Governance | W33–W36 | W33 rereview7 (fenced purge, whole chain merged at `c6d78821`), W34 rereview1 (real-container egress/isolation/limits), W35 review (restore policy + harness drills), W36 review (compatibility window + perf harness) |
| Protocol compatibility wiring (W37 carry-forward) | W37 | `packages/domain/src/mobile/index.ts` re-export, `/system/capabilities` advertises `protocol_minimum`/`protocol_maximum` (config + env override), mobile handshake consumes `clientGate` and stops cancel/steer on unknown/upgrade verdicts |

## Not delivered in this candidate (explicit declarations, never counted as passed)

| Item | State |
| --- | --- |
| backend_model evidence kind | No task ever produced a reviewed model-calling-layer run; the core profile therefore stays open (unchanged in the final round — a true plan-coverage hole). |
| Complete Happy surface (full_happy profile) | Not in this release slice: W32 delivered the advanced-operation ports at the core-capability scope (accepted), but the H27–H33 management screens and the H33 WebSocket bridge are not implemented; 66 of the 78 parity-matrix rows are honestly open. |
| Real-device native layers | All ios_native / android_native acceptances stay blocked-env (see below) — W31/W32 flipped to accepted at code level, the device layers did not. |

## blocked-env (machine-verified facts, not laziness)

- iOS/Android native: `apps/mobile/` has no `ios/`/`android/` project;
  `security find-identity -p codesigning -v` → 0 valid identities (re-verified
  by the W36 review). All ios_native / android_native acceptance stayed
  blocked-env across W07/W09/W11/W16/W25/W29/W31/W32/W36.
- Live remote: no live Paseo daemon/provider credentials (Transport closed
  1006), no PostgreSQL DSN, no object storage — W17–W26 remote/live rows.
- Real deployment: W20–W24 control endpoints are placeholders, so W35's
  real-deployment fault injection and RTO/RPO measurement are blocked-env;
  only the in-process harness (15 records, 0 failures) ran.
- Real voice providers: W30's OpenMeter/PG migration execution and W31's
  realtime media face have no provider environment here; the server-side
  hold sweeper for W31 remains a declared Cannot-verify boundary (client
  paths fully covered by five-path settlement tests).
- Early-session review files: the on-disk independent reviews for
  W01–W06 and W18–W22 (round files) were lost with the early
  session; those tasks survive as ledger-text acceptances and are recorded
  `accepted-ledger-only`, which the gate deliberately does NOT count as
  pass evidence. (W14–W16 originally shared this state; their accepting
  reviews were recovered from the lane worktrees / the W15 lane merge during
  the final integration round.)

## Known risks

1. Lane-integration gap: RESOLVED (integrated). The notifications server
   lane (W13–W16) and the W24 usage-binding lane were accepted on their
   branches and, at the previous regeneration, had never merged — that gap
   is now closed: all five lanes merged (W13 `71294f7b`, W14 `b6a701e9`,
   W15 `061a442a`, W16 `0d9baebd`, W24 `05725468`; integration fixes
   `a6adf7a1` voice-route/workbenchservice restore, `44710c6e` FK fixture,
   `96efa90c` W16 vitest repair), merge-base verified as ancestors of this
   HEAD. Residual risk: lane tests that had never executed (Vitest was
   blocked-env on the lanes) surfaced failures on their first real run and
   were repaired — the missing `@/` alias resolution in the mobile vitest
   setup remains a latent trap for future alias-mocked tests (flagged for
   the platform lane).
2. Eleven accepted-ledger-only rows have no re-verifiable independent
   review artifact (W01–W06, W18–W22); a release claim should re-run their
   focused suites or accept the ledger text as provenance.
3. W30 voice settlement is now accepted (double-settle I-1 and replay I-2
   fixed and re-reviewed), but a rate-version drift window remains by
   design: deferred re-settlement under a changed rate version is
   conservatively rejected and needs manual reconciliation (no mischarge,
   possible under-report until reconciled).
4. Preview ticket store is process-internal (bounded at 65536 with
   full-table sweep, W27); multi-instance deployments need a shared store.
5. RESOLVED (attribution erratum, final-review F1): the previously reported
   `internal/handler` failure (`TestOIDCMobileStartCallbackExchangeIsOneTime`)
   was described as "pre-existing ... unrelated to the mobile-workbench
   lanes" — that attribution was wrong. Both the test and the
   signed-state contract it exercises were introduced by the W08 lane
   (first W08 commit `708132ae`), and the test had never been green on any
   compilable point of the candidate (the handler package could not compile
   until the W20 `remote_dispatch` defect was repaired by `cbbc04b4`; the
   first compilable point already failed). Root cause: the test stub
   returned a bare provider state while `decorateOIDCMobileAuthorization`
   requires a signed state, and the test router never mounted the real
   `/auth/mobile/exchange` route — a test defect, not an implementation
   defect. Fixed in the final review wave: signed-state stub + real route
   mount; the family is 5/5 green at the fix HEAD. Separately, the SQLite
   000055-family failures (repository 2 + session 19 = 21 tests) are
   W02-lane legacy debt (migration `000055_workbench_runs` was introduced
   by the W02 merge `391a4ac2`, inside the recovery baseline): zero new
   failures appeared in `edc853b2..HEAD`, but under the full-plan scope
   this is W02-domain red debt that has never been fixed — tracked below
   as proposed issue M-2.
6. Shared worktree: a concurrent parity automation lane still has in-flight
   edits; this report only covers accepted commits.

## Known debt & proposed issues (issue creation pending authorization)

Recorded per the final review (M-2 + four B-table items); issues should be
filed once the coordinator authorizes creation.

1. M-2 — SQLite 000055-family red tests (21: repository 2 + handler/session
   19). W02-lane legacy: `000055_workbench_runs` was introduced by the W02
   merge `391a4ac2` and predates the recovery baseline `edc853b2`. Zero new
   failures in `edc853b2..HEAD`, but the debt has been carried as
   "pre-existing baseline" since the plan's second week and pollutes CI
   signal. Proposed issue: fix the 000055-family migration semantics (down
   path / numbering) or re-anchor the suites.
2. W25 M-1 — cancel / late-success race can orphan attachments (no
   permission or correctness risk; slow storage leak only). Proposed issue:
   reference-counting TTL sweeper as the architectural backstop.
3. W30 — rate-version drift window: deferred re-settlement under a changed
   rate version is conservatively rejected (no mischarge, possible
   under-report until reconciled). Proposed issue: a reconciliation
   adjudication flow.
4. W33 — production ingest lacks the `deletion_revision` field (out of
   scope at W33); purge stops at `cleanup_pending`, which is the fail-closed
   direction. Proposed issue: assign to the W21/W22 domain owners.
5. W31 — real-provider ownership of the realtime media face remains a
   blocked-env boundary (no provider environment existed in this plan);
   accepted as the B-table Ruling stands. Proposed issue only if/when a
   provider environment lands.


## Rollback commands (per reviewed capability)

```bash
# W34 capability switches — stop NEW work per lane, keep reads/cleanup:
WEKNORA_WORKBENCH_WORKER_DRAIN=true        # refuse NEW admissions everywhere
WEKNORA_WORKBENCH_PLATFORM_ADMISSION=false # stop NEW platform executions
WEKNORA_WORKBENCH_READ_ENABLED=false       # close read paths (last resort)
# Protocol compatibility window override (W37):
WEKNORA_WORKBENCH_PROTOCOL_MINIMUM=2
WEKNORA_WORKBENCH_PROTOCOL_MAXIMUM=3
# Revert a slice (examples):
git revert e9da7f44   # W30 voice settlement round (billing slice)
git revert df26fb3b   # W31 realtime voice fix round
git revert b2b83d43   # W32 advanced interaction ports
# Remote lane rollback precedent: cancel via W22 stop command + lease release;
# session deletion is tombstoned first (W33) — see deploy/mobile-workbench/README.md
```

## Traceable acceptance chains (ledger pointers, per volume-06 W37)

| Chain | Traceable evidence |
| --- | --- |
| Two platform agent chains | W10 final14 review (mounted NewSessionScreen → send → durable read → product navigation) + W11 review (workbench list/agent/space entries); real-device legs are blocked-env, honestly declared |
| Cross-endpoint approval | W05 ledger (typed commands, approval recovery) + W22 ledger (durable approval retry with exact provider args); both are ledger-text acceptances in this candidate |
| Reconnect | W12 re-review (inFlight clearing, W09 cursor resume, foreground/background lifecycle) |
| Paseo fault handling | W20 ledger (uncertain start recovery, payloadHash fixtures) + W35 harness drills (five scenarios, guard chain fail-closed) |
| Settlement after cancel | Both slices CLOSED at code level in this candidate: VOICE via W30 rereview2 (exactly-once deferred settlement under the real gate, 19/19) and REMOTE via the integrated W24 usage-binding lane (merge `05725468`; persistence suites re-run green at the final HEAD). Live-provider legs remain blocked-env |
| Preview security | W27 re-review (CSP byte-parity, triple-scoped ticket issuance, 65536 bound + sweep, user-gesture external opens) |
| Advanced operation safety | W32 review (seven-operation gate, CAPABILITY_UNAVAILABLE before any call, one-time terminal tickets that cannot become access tokens, no shell RPC passthrough) |

## Gate result at generation time

Running `node scripts/mobile-workbench/check-acceptance.mjs` against the
final `acceptance.json` exits NON-ZERO with 6 profile-level blocking
findings (core: backend_model/ios_native/android_native; oidc: ios_native;
remote: remote/recovery; resources: ios_native/android_native; voice:
ios_native; governance: recovery) and ZERO evidence-record violations —
every pass row (now 21, including the five restored/upgraded lane rows and
the W16 supplementary vitest row) carries a candidate-ancestor baseline, an
on-disk independent review, exit code 0 from a re-run at the final HEAD and
a fresh artifact hash. The remaining gaps are real missing evidence (native
device layers, live remote, real-deployment recovery, the plan-wide
backend_model hole), and this report declares them rather than pretending
them away: that non-zero exit is the correct, intended behaviour of a staged
gate. The gate flips to zero only when every delivered/pending profile's
required kinds carry independently reviewed passing rows.
