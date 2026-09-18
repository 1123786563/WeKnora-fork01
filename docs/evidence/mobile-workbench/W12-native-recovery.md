# W12 Foreground/background recovery controller and the two real chains

Date: 2026-09-17
Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`
Dispatched BASE: `9c13113c` (worktree HEAD at task start was `16ecf817`,
a parallel docs commit `docs(parity): R446 ledger entry…`; this task's commit
lands on top of it, so `9c13113c..HEAD` also contains that foreign commit).

## Deliverables

- `apps/mobile/sources/weknora/executions/recovery.ts` — `recoverRun(ports)`
  with the brief-mandated interface, plus the production controller
  `createExecutionRecovery` (AppState-driven, single-flighted, scope-fenced,
  404-safe) and `subscribeFromLastCommittedCursor` (resume from the W09
  durable maximum seq).
- `apps/mobile/sources/weknora/executions/recovery.test.ts` — node:test suite
  (brief case + controller semantics + two harness chains + reconciliation).
- `apps/mobile/sources/weknora/conversations/ConversationScreen.tsx` — new
  optional `recovery?: ExecutionRecovery` prop: subscribes to
  `AppState.addEventListener('change', …)`, forwards `active`/`background`,
  renders the `recovery-failure` alert with a `重试恢复` retry button, and on
  unmount removes the AppState subscription and disposes the per-mount
  handle. The W25 optional `attachments` prop and its control flow are
  untouched (verified by re-running `[id].test.tsx`).
- `apps/mobile/sources/weknora/conversations/ConversationScreen.test.tsx` —
  react-native mock extended with an `AppState` test seam; two new cases
  (AppState forwarding + unmount cleanup/dispose; failed-state retry button
  that clears on successful recovery).
- `internal/application/service/workbench/admission.go` +
  `admission_test.go` — W11 follow-up accepted by the W12 brief: `StartInput`
  gains `SpaceID string \`json:"space_id"\`` and the immutable run snapshot
  now persists `space_id` (read side already landed in W11's
  `workbench_list.go:workbenchSummaryFromRow`). `space_id` is deliberately
  NOT part of `requestHash`, so a retry that omits the navigation hint still
  reconciles onto the original request (asserted by the new Go test).

## Interface produced

```ts
recoverRun(ports: {
  status(): Promise<{ terminal: boolean }>;
  subscribe(): Promise<void>;
  refreshHistory(): Promise<void>;
}): Promise<void>
```

Order is fixed: `status → refreshHistory → subscribe only when non-terminal`.
W06 lookup/snapshot results map to the terminal verdict
(`succeeded|failed|canceled`); every other status (including `unknown`)
stays visible in the view model exactly as W10 established
(`连接状态未知，正在等待服务端确认` notice is untouched).

## Production semantics and where each is enforced

| Requirement | Enforcement point | Evidence |
| --- | --- | --- |
| subscribe resumes from W09's last committed cursor | `subscribeFromLastCommittedCursor` reduces `eventStorage.read(runID)` to the durable max seq and passes `String(cursor)` (or `undefined` when nothing is committed) as `lastEventID` | `subscription resumes from the last committed W09 cursor, not from zero` |
| concurrent active events single-flighted | `createExecutionRecovery.recover()` returns the same in-flight promise | `concurrent active events are single-flighted…` |
| scope switch cancels the current recovery | pass captures `scope.capture()`; each step re-checks `scope.accept(generation)` and races the captured signal; `scope.subscribe` listener aborts active passes | `scope switch cancels the in-flight recovery…` |
| HTTP 404 never auto-creates a session | controller structurally has no start/create port; 404 family (`execution stream HTTP 404`, `status === 404`, `NOT_FOUND`, gorm `record not found`) maps to `failed` state surfaced to the retry affordance | `HTTP 404 marks the recovery failed…`, `isRecoveryNotFound recognizes…` |
| background only closes the subscription, never cancels | `appStateChange('background')` only aborts the in-flight pass's background controller; there is no cancel command in the controller or its ports | `background only closes the subscription…` |
| unmount cleans up | ConversationScreen effect removes the AppState subscription, unsubscribes state listeners and calls `recovery.dispose()` | `forwards AppState transitions… and cleans up on unmount` |

`recover()` never rejects: failures surface through `getState()` /
`onStatus` so an unawaited AppState trigger cannot produce an unhandled
rejection; the retry button is state-driven.

## TDD record

RED (before `recovery.ts` existed):

```text
$ pnpm exec tsx --test apps/mobile/sources/weknora/executions/recovery.test.ts
✖ apps/mobile/sources/weknora/executions/recovery.test.ts
Error: Cannot find module './recovery.ts'   (recoverRun undefined)
ℹ tests 1  pass 0  fail 1
```

GREEN (after the minimal implementation and the full suite):

```text
$ pnpm exec tsx --test apps/mobile/sources/weknora/executions/recovery.test.ts
✔ completed run restores history without opening stream
✔ active run restores history and then opens the stream
✔ concurrent active events are single-flighted onto one recovery pass
✔ scope switch cancels the in-flight recovery and never resumes its steps
✔ HTTP 404 marks the recovery failed and never auto-creates a session
✔ background only closes the subscription; it never cancels the run
✔ isRecoveryNotFound recognizes handler-shaped 404 errors
✔ subscription resumes from the last committed W09 cursor, not from zero
✔ harness: knowledge-resource run recovers without duplicating history
✔ harness: controlled-tool run recovers its pending approval
✔ harness: dropped start response recovers via lookup onto the same run
ℹ tests 11  pass 11  fail 0
```

## Two real chains — code-level wiring + harness evidence

Both chains are wired through the accepted W05–W11 stack (knowledge resource
selection → `resolveProductSessionResources` → product session route →
`createProductConversationViewModel` → `projectExecutionSnapshot/Event` →
`ProductConversationMessages`; custom agent → `pendingInteractions` →
approve/reject `commands.approve/reject` → `executionEvents` stream), with
the W12 controller closing the lifecycle gap. Harness cases replay each
chain's event shapes through the real projection modules:

1. Knowledge resource → cited answer → history
   (`harness: knowledge-resource run recovers without duplicating history`):
   a `run-kb` snapshot with a cited assistant answer (`[1] 引用来源 knowledge-7`)
   is consumed live, then "relaunched": the controller sees a terminal run,
   replays the durable snapshot, never opens a second stream, and the
   restored message list is identical to the live one (no duplicates, same
   run identity).
2. No-KB custom agent → controlled tool → approve/reject → artifact
   (`harness: controlled-tool run recovers its pending approval`): a
   `run-agent` snapshot paused on `tool.approval.requested`
   (`pi-1`, revision 4). Recovery replays history, opens the stream because
   the run is non-terminal (`waiting_user`), the pending interaction stays
   addressable, the approval decision lands on `run-agent`, and the artifact
   message (`产物：检索结果已生成`) is projected onto the same run.
3. Reconciliation semantics for a dropped start response
   (`harness: dropped start response recovers via lookup onto the same run`):
   recovery resolves through the W06 lookup path onto the original
   `request-1 → run-9` mapping and never re-issues `start`, which is the
   client-side mirror of "the database holds one execution / one charge".

## Acceptance matrix (layered honestly)

| Brief acceptance step | Layer achieved | Notes |
| --- | --- | --- |
| Generating run → background → foreground | unit/harness | `background only closes the subscription…` + `appStateChange('active')` replays the full sequence; real-device transition blocked-env |
| Kill app → relaunch → history has no duplicates, same Run | unit/harness | chain-1 harness case + W10's remount test remain green; device kill/relaunch blocked-env |
| Network switch mid-generation | unit (structure) | scope/abort races are the same seam a network loss surfaces through; the transport-level retry is W10's lookup path, re-asserted here in the dropped-start harness |
| Server finishes while app dead → restart shows final state | unit/harness | terminal verdict short-circuits the stream and replays the durable snapshot |
| Dropped start response → recover original request → one execution/charge | harness + Go admission idempotency | client: lookup-only recovery; server: `TestAdmissionTwentyConcurrentIdenticalRequestsCreateOneRun` (pre-existing, still green) plus the new space_id retry-reconciliation test |
| Record server RunID, model version, dual-platform client versions, no tokens | blocked-env | no reachable backend instance or signed device builds in this environment; harness RunIDs are simulated (`run-kb`, `run-agent`). No credential material is persisted anywhere in the controller or tests |

## blocked-env classification

- Physical iOS/Android devices: unavailable in this environment. Simulator
  process survival explicitly does NOT count as interactive acceptance, so
  no simulator-based claim is made either.
- Live server RunID / model version / client build metadata: requires the
  deployed workbench topology (W34 compose is in-flight and BASE had build
  breakage there); recorded as blocked-env rather than fabricated.
- Pre-existing shared-worktree failures unrelated to this task (verified
  identical with the task's Go edits stashed):
  `internal/handler/session` suite fails on
  `cannot start a transaction within a transaction` (SQLite migrator
  NoTxWrap configuration), and `internal/application/repository` fails
  `TestCraftVersionsMigrationDownDropsVersionTables` +
  `TestExecutionDispatchSQLiteMigrationHead`. Both fail at baseline.

## Regression matrix

| Command | Result |
| --- | --- |
| `pnpm exec tsx --test …/executions/recovery.test.ts` | 11 pass / 0 fail |
| `pnpm exec tsx --test …/executions/recovery.test.ts …/conversations/{execution-projection,view-model,resources}.test.ts` | 30 pass / 0 fail |
| `pnpm --filter @weknora/mobile exec vitest run …/ConversationScreen.test.tsx` | 6 pass / 0 fail (4 pre-existing W25 cases + 2 new) |
| `pnpm --filter @weknora/mobile exec vitest run …/session/\(app\)/[id].test.tsx` (W25 attachments pipeline re-check) | 3 pass / 0 fail |
| `pnpm --filter @weknora/mobile run typecheck` (`tsc --noEmit`) | exit 0 |
| `go test ./internal/application/service/workbench/` | ok (includes `TestAdmissionSnapshotPersistsSpaceIDWithoutHashImpact`) |

## Notes / handoff

- `recovery.ts` / `recovery.test.ts` are matched by the repo-wide
  `.gitignore` `WeKnora` rule (line 31), same as every earlier
  `apps/mobile/sources/weknora/**` deliverable; they were added with
  `git add -f` and are disclosed here.
- Production route assembly (`session/[id].tsx` passing a `recovery` prop
  built from `createExecutionRecovery` + the view model's execution API) is
  intentionally NOT done: the route file is concurrently owned by the W25
  attachment work and the brief forbids touching it. The screen-level
  contract (`recovery?: ExecutionRecovery`, per-mount instance, disposed on
  unmount) is final here; the one-line route wiring belongs to the next task
  that owns that file (W31/W36 chain work).
- `space_id` stays out of `requestHash` on purpose: it is navigation
  metadata, and hashing it would turn a retried request that omits the hint
  into a 409 instead of a reconciliation onto the original run.
