# Mobile workbench staged release report

- Generated: 2026-09-17 (W37 regeneration round — final state: all 37 tasks
  W01–W37 accepted in both ledgers), candidate baseline `46427a9e`
  (regenerated from the interim document generated at `f0f39fa0` before the
  last lanes closed)
- Acceptance document: `docs/evidence/mobile-workbench/acceptance.json`
  (gate: `node scripts/mobile-workbench/check-acceptance.mjs`)
- Ledgers: `docs/superpowers/plans/mobile-workbench-progress.md` (W ledger,
  coordinator-written) and `.superpowers/sdd/2026-09-12-mobile-ai-saas-workbench/progress.md`
  (SDD ledger). This report reflects the FINAL state after all 37 tasks were
  accepted; pass-row commands were re-run at the generation HEAD.
- Every "delivered" claim below is an accepted-with-blockers task unless
  stated otherwise: code-level evidence was independently reviewed AND its
  baseline commit is an ancestor of the candidate. Layers marked blocked-env
  were never executed here (no iOS/Android projects, no signing identities,
  no live Paseo/PG/deployment).

## Delivered (independently reviewed, code level, anchored in the candidate)

| Slice | Tasks | Strongest evidence pointer |
| --- | --- | --- |
| Core execution lane | W01–W07, W09–W12 | W10 final14 review (mounted send → durable read → product navigation, 28/28), W11 review (keyset list on real SQLite harness), W12 re-review (recovery 24/24 + mounted 10/10); W01–W06 survive as ledger-text acceptances (see blocked-env) |
| Native OIDC exchange | W08 | seam review (PASS/APPROVED): POST `/auth/mobile/exchange` verified in the test router; baseline re-anchored in the regeneration round to `bd624219` (the original `661a7b7c` is a dangling object); the router suite re-run at the final HEAD is green (the historical W20 compile blocker was repaired by `cbbc04b4`) |
| Remote lane (code) | W17–W23 | W17 static contract probe + W18–W23 ledger acceptances; W24 is NOT in this candidate (see Not delivered) |
| Resources | W25–W28 | W25 re-review (mounted upload chain), W26 re-review (immutable artifact versions + wiring), W27 re-review (preview CSP parity, bounded tickets, user-gesture external opens), W28 review (citation registrar 6/6 + component suites 27/27 — flipped to delivered in this round) |
| Voice | W29–W31 | W29 review (dictation 20/20 + mounted hold-to-talk), W30 rereview2 (exactly-once settlement under the real gate, honest settled semantics, 19/19 matrix — flipped in this round), W31 rereview1 (realtime controls, settle-on-fail five paths, dispose-on-unmount, live-progress port — flipped in this round) |
| Advanced interaction ports | W32 | review (PASS/APPROVED 0 C/I): seven-operation CAPABILITY_UNAVAILABLE gate with zero shell fallback, one-time terminal tickets, 78-row parity matrix honestly 12 harness / 66 open / 0 native, manifest 695 target correction with 0 hash changes |
| Governance | W33–W36 | W33 rereview7 (fenced purge, whole chain merged at `c6d78821`), W34 rereview1 (real-container egress/isolation/limits), W35 review (restore policy + harness drills), W36 review (compatibility window + perf harness) |
| Protocol compatibility wiring (W37 carry-forward) | W37 | `packages/domain/src/mobile/index.ts` re-export, `/system/capabilities` advertises `protocol_minimum`/`protocol_maximum` (config + env override), mobile handshake consumes `clientGate` and stops cancel/steer on unknown/upgrade verdicts |

## Not delivered in this candidate (explicit declarations, never counted as passed)

| Item | State |
| --- | --- |
| W13–W16 notifications server lane | Lane-only integration: all implementation commits live on `codex/mobile-w13`…`codex/mobile-w16` and never merged into the candidate (verified by merge-base in the regeneration round); the candidate tree contains no mobile_device repository, registration intents, outbox or provider-delivery code. The independent W13 final review (PASS/APPROVED, on disk) verified real work — on its lane. The W13 evidence row is downgraded to `blocked`; W14–W16 remain ledger-text acceptances with the same lane caveat. |
| W24 usage-binding final round | Lane-only integration: all seven commits (`a9404191`…`d19b0b1a`) live on `codex/mobile-w24` and are non-ancestors of the candidate; `TestExecutionTargetStorePersistsServerUsageBinding` and the target.go usage parsing are absent from the candidate tree. Some W24-domain surfaces reached the candidate via later work (`cbbc04b4` rename fix, W30 commercial settlement consumption), but the reviewed persistence evidence is lane-only. The W24 evidence row is downgraded to `blocked`. |
| backend_model evidence kind | No task ever produced a reviewed model-calling-layer run; the core profile therefore stays open (unchanged in the regeneration round — a true plan-coverage hole). |
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
  W01–W06, W14–W16, W18–W22 (round files) were lost with the early
  session; those tasks survive as ledger-text acceptances and are recorded
  `accepted-ledger-only`, which the gate deliberately does NOT count as
  pass evidence.

## Known risks

1. Lane-integration gap (regeneration finding): the notifications server
   lane (W13–W16) and the W24 usage-binding lane were accepted on their
   branches but never merged. A deployment of this candidate gets NEITHER
   the notifications server code NOR the W24 usage-binding persistence,
   despite ledger acceptances — merge `codex/mobile-w13`…`w16`/`w24`
   (or re-port them) before claiming those capabilities.
2. Thirteen accepted-ledger-only rows have no re-verifiable independent
   review artifact (W01–W06, W14–W16, W18–W22); a release claim should
   re-run their focused suites or accept the ledger text as provenance.
3. W30 voice settlement is now accepted (double-settle I-1 and replay I-2
   fixed and re-reviewed), but a rate-version drift window remains by
   design: deferred re-settlement under a changed rate version is
   conservatively rejected and needs manual reconciliation (no mischarge,
   possible under-report until reconciled).
4. Preview ticket store is process-internal (bounded at 65536 with
   full-table sweep, W27); multi-instance deployments need a shared store.
5. `internal/handler` has one pre-existing failing test
   (`TestOIDCMobileStartCallbackExchangeIsOneTime`, reproduced on the clean
   BASE `f0f39fa0` snapshot) unrelated to the mobile-workbench lanes;
   SQLite migration 000055-family failures also pre-exist in
   repository/handler suites.
6. Shared worktree: a concurrent parity automation lane still has in-flight
   edits; this report only covers accepted commits.

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
| Settlement after cancel | VOICE slice CLOSED in this candidate: W30 rereview2 (exactly-once deferred settlement under the real gate, 19/19). The REMOTE slice (W24 usage binding) is lane-only and NOT in this candidate — the chain closes for voice only |
| Preview security | W27 re-review (CSP byte-parity, triple-scoped ticket issuance, 65536 bound + sweep, user-gesture external opens) |
| Advanced operation safety | W32 review (seven-operation gate, CAPABILITY_UNAVAILABLE before any call, one-time terminal tickets that cannot become access tokens, no shell RPC passthrough) |

## Gate result at generation time

Running `node scripts/mobile-workbench/check-acceptance.mjs` against the
regenerated `acceptance.json` exits NON-ZERO with 6 profile-level blocking
findings (core: backend_model/ios_native/android_native; oidc: ios_native;
remote: remote/recovery; resources: ios_native/android_native; voice:
ios_native; governance: recovery) and ZERO evidence-record violations —
every pass row now carries a candidate-ancestor baseline, an on-disk
independent review and a fresh artifact hash. The remaining gaps are real
missing evidence (native device layers, live remote, real-deployment
recovery, the plan-wide backend_model hole), and this report declares them
rather than pretending them away: that non-zero exit is the correct,
intended behaviour of a staged gate. The gate flips to zero only when every
delivered/pending profile's required kinds carry independently reviewed
passing rows.
