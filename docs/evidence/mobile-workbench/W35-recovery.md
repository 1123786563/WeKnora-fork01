# W35 Event retention, backup restore and fault drills

Date: 2026-09-17
Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`
Base: dispatched `1cff65f8`; worktree HEAD had already advanced to
`f4f8a7f9` (W26 fix1) and later `56baff72` (W26 acceptance) while this task
ran — other tasks' commits land on the shared worktree between runs.

## Deliverables

- `internal/execution/restore_policy.go` / `restore_policy_test.go` —
  `MayDispatchAfterRestore(reconciled bool, unknown int) bool`: backup
  restore keeps NEW dispatch closed until every binding is reconciled and no
  dispatch outcome stayed unknown; a stale backup `queued` state is never
  read as "never executed".
- `internal/application/repository/agent_run_events.go` (+ tests appended to
  `agent_run_events_trim_test.go`) — `ApplyEventRetention`: 30-day
  configurable, snapshot-then-trim time-based retention pass with W33
  tombstone coordination and protected event families.
- `scripts/mobile-workbench/fault-scenarios.mjs` — guarded fault drill
  runner: five scenarios, per-drill JSON contract, harness mode.
- `deploy/mobile-workbench/README.md` — W35 runbook sections (restore
  admission, retention, drills) in the W34 wired-entrypoints style.

## RED (task behaviour, not environment failures)

Restore policy (`/tmp/w35-red-restore.txt`):

```text
go test ./internal/execution -run TestRestore -count=1
internal/execution/restore_policy_test.go:11:5: undefined: MayDispatchAfterRestore
internal/execution/restore_policy_test.go:14:6: undefined: MayDispatchAfterRestore
FAIL github.com/Tencent/WeKnora/internal/execution [build failed]
```

Retention (`/tmp/w35-red-retention.txt`):

```text
go test ./internal/application/repository -run 'TestAgentRunEventTimeRetention' -count=1
agent_run_events_trim_test.go: store.ApplyEventRetention undefined
agent_run_events_trim_test.go: undefined: EventRetentionOptions
FAIL github.com/Tencent/WeKnora/internal/application/repository [build failed]
```

(One intermediate test-authoring error — shadowing the builtin `append` —
was fixed before the implementation existed; it was a test compile error,
not a passing RED claim.)

## GREEN and regression

```text
go test ./internal/execution -run TestRestore -count=1          # PASS
go test ./internal/execution -count=1                            # ok
go test ./internal/application/repository \
  -run 'TestAgentRunEvent|TestAgentRunFinalize|TestExecutionCleanup|TestCleanup' -count=1
  # PASS: TestAgentRunEventsSequenceAndReplay, TestAgentRunFinalizeIdempotent,
  # TestAgentRunEventRetentionTrim, TestAgentRunEventTimeRetentionSnapshotsThenTrims,
  # TestAgentRunEventTimeRetentionSkipsActiveAndTombstoned,
  # TestExecutionCleanupTombstoneClaimAndSettlement,
  # TestExecutionCleanupFactsAreMonotonicAndRevisionFenced — ok
go vet ./internal/execution ./internal/application/repository     # exit 0
node --check scripts/mobile-workbench/fault-scenarios.mjs         # exit 0
```

Full `go test ./internal/application/repository -count=1` result:
`FAIL` — exactly two pre-existing failures, reproduced on a CLEAN HEAD
(`git worktree add --detach` at `56baff72`, i.e. without any W35 file):

- `TestCraftVersionsMigrationDownDropsVersionTables` — "cannot start a
  transaction within a transaction": the W33 sqlite NoTxWrap migration vs
  the migrator's transaction wrap, the same pre-existing BASE defect
  documented in `W34-deployment.md`.
- `TestExecutionDispatchSQLiteMigrationHead` — expects 20 sqlite migration
  files, 67 exist: stale hardcoded count, unrelated to W35.

Transcript: `/tmp/w35-final-verify.txt`.

## Restore policy semantics

`MayDispatchAfterRestore(reconciled, unknown) = reconciled && unknown == 0`.
Pinned by `TestRestoreCannotReplayUnreconciledCommands`. The fail-closed
reading is documented on the function: a backup that still shows a command
as queued proves nothing about whether the external process executed or
billed after the snapshot; admission reopens per binding only after
observation of live processes, usage-vs-receipt matching and projection
rebuild.

## Event retention semantics (harness/unit proven)

`ApplyEventRetention(ctx, opts)` (default window `DefaultEventRetention` =
30d, `EventRetentionOptions.Retention` overrides per pass, negative →
`ErrConflict`, pass limit default 128):

1. Candidates: terminal runs (`succeeded|failed|canceled`) with
   `updated_at` before the cutoff that still have trimmable (old,
   unprotected) events. Live runs are never candidates; fully-protected
   runs drop out of the candidate set so they cannot starve the pass limit.
2. Tombstone coordination (W33): candidates under a live (non-`purged`)
   `execution_cleanup` tombstone are counted
   (`RunsSkippedTombstoned`) and left untouched — their deletion belongs to
   the W33 evidence-fenced purge path (`RunCleanupPurgeOnce`), whose
   `ObserveUsage` counts the very `usage.*` rows this pass protects.
3. Protected families, never deleted by the global pass: `usage.*`
   (commercial records), `approval.*` (live approvals), `run_completed`
   (finalize idempotency receipt — deleting it would let a late finalize
   retry append a duplicate completion event), `retention.trimmed` (the
   snapshots themselves).
4. Snapshot-then-trim per run, one transaction: conditional no-op row update
   re-fences the run (finalize/reclaim racing the pass is skipped, not
   failed), the `retention.trimmed` summary event (trimmed count,
   first/last seq, per-type histogram, cutoff) is appended, and only then
   is the stale prefix deleted. Old cursors over the trimmed prefix then
   surface `ErrCursorExpired` (the explicit reload error), proven in
   `TestAgentRunEventTimeRetentionSnapshotsThenTrims`.
5. A second pass over the same state considers 0 runs and writes nothing
   (idempotence, same test).

Wiring status (documented in the README): the pass is a store-level API
invoked explicitly; no background scheduler calls it yet. The pre-existing
finalize-time watermark trim (`TrimEventsBefore`, last 1000 events) is
unchanged.

## Fault drills — script structure, validation, JSON contract

`node scripts/mobile-workbench/fault-scenarios.mjs --harness [--repeat N]`
executes the real validation gates and scenario bodies against an
in-process fake deployment implementing the driver contract (marker,
submit, inject, recover, snapshot/restore, credential re-verification,
counters). Guards verified with real exit codes:

| Check | Command (abridged) | Result |
| --- | --- | --- |
| No explicit target | `fault-scenarios.mjs` (no args) | exit 2, "no explicit target … refusing to touch anything", nothing contacted |
| Non-allowlisted origin | `--target https://prod.example.com --allowlist https://wb-test-1.internal` | exit 2, origin not in allowlist |
| Empty allowlist | guard unit (`guardAllowlist(x, '')`) | refused (harness asserts) |
| Not a test deployment | marker endpoint answers `test_deployment:false` (fake) | refused, zero injections attempted (harness asserts) |
| Version mismatch | `checkDeployment(..., expectVersion: 'other-9')` | refused (harness asserts) |
| Unreachable control endpoint | `--target https://wb-test-1.internal --allowlist …` | exit 3, "control endpoint unreachable" |
| `WB_TEST_DEPLOYMENT` fallback | `WB_TEST_DEPLOYMENT=… fault-scenarios.mjs --allowlist …` | target resolved from env, reaches the marker gate (exit 3 unreachable here) |

Harness, each scenario × 3 repeats on one shared fake deployment instance:

```text
node scripts/mobile-workbench/fault-scenarios.mjs --harness --repeat 3
harness: 15 records, 0 failures        # exit 0
```

Per-drill JSON contract (fields `scenario`, `baseline_sha`, `run_id`,
`actual_process_count`, `usage_count`, `result` + `checks`), one line per
drill, e.g.:

```json
{"scenario":"restore_snapshot","baseline_sha":"56baff72cce50f1ffcf3ca04dbfad2390b9244a1","run_id":"run-restore_snapshot-1","actual_process_count":0,"usage_count":1,"result":"pass","checks":{"no_duplicate_starts":true,"no_cross_tenant_leak":true,"no_unconfirmed_usage_lost":true,"db_run_count_grew_by_one":true,"processes_settled":true,"admission_follows_restore_policy":true,"run_present":true}}
```

The checks compare database run counts, external process counts and
commercial usage records — never an HTTP status alone. The restore policy
mirror `mayDispatchAfterRestore(reconciled, unknown)` keeps the JS drill
aligned with the Go predicate; `restore_snapshot` additionally asserts
dispatch stays closed immediately after the restore and only reopens after
reconciliation AND node-credential re-verification (credential epoch bump).

## Acceptance checklist mapping (brief step 6)

- Repeated 3x per scenario: duplicate starts 0 / cross-tenant leakage 0 /
  unconfirmed usage lost 0 — HARNESS-PROVEN (15/15 records pass, counters
  accumulate across repeats on one deployment instance).
- Unknown outcomes keep dispatch blocked, no automatic restart —
  CODE-PROVEN in Go (`MayDispatchAfterRestore(true, 1) == false`) and
  mirrored in the drill's admission check.
- Object metadata recovery matches summaries, delete tombstones preserved,
  `cursor_expired` recovery loses no content — the retention side is
  CODE-PROVEN: the `retention.trimmed` summary is written in the same
  transaction as the trim, tombstoned rows are preserved for the W33 fence,
  and trimmed-prefix replays surface `ErrCursorExpired`
  (`TestAgentRunEventTimeRetentionSnapshotsThenTrims`,
  `TestAgentRunEventTimeRetentionSkipsActiveAndTombstoned`).
- Control-plane recovery time and data loss measured per drill — the JSON
  record carries the raw counters per run; no fixed RTO/RPO is promised
  (no business SLA).

## blocked-env

- REAL-deployment injection for all five scenarios: no W20–W24 control
  endpoints / test deployment exists in this environment (bridge process
  entrypoint still a placeholder). The script's real mode, its HTTP driver
  and its guard chain are complete and fail closed; only the live
  deployment to point them at is missing.
- Live RTO/RPO measurement (restore a real database volume, watch recovery
  wall-clock): requires the same missing deployment.
- The in-process fake models external processes/usage rollback semantics
  (restore rolls durable state back; provider-side usage is re-matched
  during re-verification) — it proves the drill's contract, not the
  provider's behaviour.

No verification above was recorded as passing without its command and exit
status; every blocked item is labelled blocked-env rather than passed.
