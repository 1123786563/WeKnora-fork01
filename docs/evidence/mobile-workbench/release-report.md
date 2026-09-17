# Mobile workbench staged release report

- Generated: 2026-09-17 (W37 acceptance task), candidate baseline `f0f39fa0`
- Acceptance document: `docs/evidence/mobile-workbench/acceptance.json`
  (gate: `node scripts/mobile-workbench/check-acceptance.mjs`)
- Ledgers: `docs/superpowers/plans/mobile-workbench-progress.md` (W ledger,
  coordinator-written) and `.superpowers/sdd/2026-09-12-mobile-ai-saas-workbench/progress.md`
  (SDD ledger). This report reflects the state AT GENERATION TIME; the
  coordinator will require regeneration once all lanes close.
- Every "delivered" claim below is an accepted-with-blockers task unless
  stated otherwise: code-level evidence was independently reviewed, while
  the layers marked blocked-env were never executed here (no iOS/Android
  projects, no signing identities, no live Paseo/PG/deployment).

## Delivered (independently reviewed, code level)

| Slice | Tasks | Strongest evidence pointer |
| --- | --- | --- |
| Core execution lane | W01–W07, W09–W16 | W10 final14 review (mounted send → durable read → product navigation, 28/28), W11 review (keyset list on real SQLite harness), W12 re-review (recovery 24/24 + mounted 10/10) |
| Native OIDC exchange | W08 | seam review: POST `/auth/mobile/exchange` verified in the test router; focused Go was then blocked by the W20 compile defect (since repaired by `cbbc04b4`) |
| Notifications | W13–W16 | W13 final review (lifecycle fencing, deterministic Bind/Revoke races); W14/W15/W16 reviews per ledger text |
| Remote lane (code) | W17–W24 | W24 final3 review (trusted admission snapshots, durable late/replay settlement) |
| Resources | W25–W27 | W25 re-review (mounted upload chain), W26 re-review (immutable artifact versions + wiring), W27 re-review (preview CSP parity, bounded tickets, user-gesture external opens) |
| Voice capture | W29 | dictation state machine 20/20 + mounted hold-to-talk |
| Governance | W33–W36 | W33 rereview7 (fenced purge), W34 rereview1 (real-container egress/isolation/limits), W35 review (restore policy + harness drills), W36 review (compatibility window + perf harness) |
| Protocol compatibility wiring (W37 carry-forward) | W37 | `packages/domain/src/mobile/index.ts` re-export, `/system/capabilities` advertises `protocol_minimum`/`protocol_maximum` (config + env override), mobile handshake consumes `clientGate` and stops cancel/steer on unknown/upgrade verdicts |

## Not delivered (explicit declarations, never counted as passed)

| Item | State |
| --- | --- |
| W28 knowledge citations / specialized result registrar | Not implemented (a lane is in flight in the shared worktree; nothing accepted) |
| W30 settlement correctness fix | Independent review FAILED (Spec FAIL / Quality CHANGES_REQUIRED, `task-W30-review.md`): one actual usage can settle twice (I-1) and same-request replay is not idempotent under the real gate (I-2). Fix round not run. |
| W31 realtime voice / interruption / background progress | Not implemented |
| W32 advanced interaction capability ports | Not implemented |
| Complete Happy surface (full_happy profile) | Not in this release slice: W32 missing and the original H checklist is not closed |
| backend_model evidence kind | No task ever produced a reviewed model-calling-layer run; the core profile therefore stays open |

## blocked-env (machine-verified facts, not laziness)

- iOS/Android native: `apps/mobile/` has no `ios/`/`android/` project;
  `security find-identity -p codesigning -v` → 0 valid identities (re-verified
  by the W36 review). All ios_native / android_native acceptance stayed
  blocked-env across W07/W09/W11/W16/W25/W29/W31/W36.
- Live remote: no live Paseo daemon/provider credentials (Transport closed
  1006), no PostgreSQL DSN, no object storage — W17–W26 remote/live rows.
- Real deployment: W20–W24 control endpoints are placeholders, so W35's
  real-deployment fault injection and RTO/RPO measurement are blocked-env;
  only the in-process harness (15 records, 0 failures) ran.
- Early-session review files: the on-disk independent reviews for
  W01–W06, W14–W16, W18–W22 (round files) were lost with the early
  session; those tasks survive as ledger-text acceptances and are recorded
  `accepted-ledger-only`, which the gate deliberately does NOT count as
  pass evidence.

## Known risks

1. Six accepted-ledger-only rows (above) have no re-verifiable independent
   review artifact; a release claim should re-run their focused suites or
   accept the ledger text as provenance.
2. W30 billing defect is live in the candidate: voice usage may double-settle
   (I-1) — do not enable charged voice in production before the fix round.
3. Preview ticket store is process-internal (bounded at 65536 with
   full-table sweep, W27); multi-instance deployments need a shared store.
4. `internal/handler` has one pre-existing failing test
   (`TestOIDCMobileStartCallbackExchangeIsOneTime`, reproduced on clean
   `f0f39fa0`) unrelated to W37; SQLite migration 000055-family failures
   also pre-exist in repository/handler suites.
5. Shared worktree: concurrent lanes (W28 et al.) have in-flight edits; this
   report only covers accepted commits.

## Rollback commands (per reviewed capability)

```bash
# W34 capability switches — stop NEW work per lane, keep reads/cleanup:
WEKNORA_WORKBENCH_WORKER_DRAIN=true        # refuse NEW admissions everywhere
WEKNORA_WORKBENCH_PLATFORM_ADMISSION=false # stop NEW platform executions
WEKNORA_WORKBENCH_READ_ENABLED=false       # close read paths (last resort)
# Protocol compatibility window override (W37):
WEKNORA_WORKBENCH_PROTOCOL_MINIMUM=2
WEKNORA_WORKBENCH_PROTOCOL_MAXIMUM=3
# Revert a slice (example: the W37 acceptance/wiring commit):
git revert <commit-sha>
# Remote lane rollback precedent: cancel via W22 stop command + lease release;
# session deletion is tombstoned first (W33) — see deploy/mobile-workbench/README.md
```

## Traceable acceptance chains (ledger pointers, per volume-06 W37)

| Chain | Traceable evidence |
| --- | --- |
| Two platform agent chains | W10 final14 review (mounted NewSessionScreen → send → durable read → product navigation) + W11 review (workbench list/agent/space entries); real-device legs are blocked-env, honestly declared |
| Cross-endpoint approval | W05 ledger (typed commands, approval recovery) + W22 ledger (durable approval retry with exact provider args) |
| Reconnect | W12 re-review (inFlight clearing, W09 cursor resume, foreground/background lifecycle) |
| Paseo fault handling | W20 ledger (uncertain start recovery, payloadHash fixtures) + W35 harness drills (five scenarios, guard chain fail-closed) |
| Settlement after cancel | W24 final3 review (late/replay/unsupported-provider reconcile = one reservation/usage fact); the VOICE slice of this chain is NOT closed (W30 review FAIL) |
| Preview security | W27 re-review (CSP byte-parity, triple-scoped ticket issuance, 65536 bound + sweep, user-gesture external opens) |

## Gate result at generation time

Running `node scripts/mobile-workbench/check-acceptance.mjs` against the
current `acceptance.json` exits NON-ZERO. That is the correct, intended
behaviour at this staging point: native/live/billing kinds and the
early-session review gaps are real missing evidence, and this report
declares them rather than pretending them away. The gate flips to zero only
when every delivered/pending profile's required kinds carry independently
reviewed passing rows.
