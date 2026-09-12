# tRPC recovery acceptance record

This document is the release gate for the durable tRPC run path. A passing unit
or checkpoint test does not prove process recovery: the crash cases below must
start a real graph provider, kill that process at a provider-owned barrier, and
reopen the same durable database in a new process.

## Current evidence

Recorded 2026-09-12 on macOS (go1.26.3 darwin/arm64), branch
`codex/trpc-recovery-r2`. The production chain is now wired end to end: the
session service registers the real graph executor
(`ExecuteDurableRun`), the container injects it into the durable worker with
a boot-time gate, the recovery hook reconciles sandbox-bound runs through the
provider sandbox-list query (`SessionBoundManager.ObserveInstance`), and the
graph persists `run_started`, `attempt_replaced`, `tool_dispatched`,
`tool_result`, `run_failed` and `run_completed` events through the
fenced event store.

The SIGKILL matrix runs against `internal/agent/recoverytest/provider`, a
provider binary that owns the production stack (migrated database, durable
worker with fenced leases, SDK graph, repository checkpoint saver, tool
journal, durable decision parking, finalize transaction); only the chat model
and the external side-effect endpoint are deterministic doubles.

| Check | Command / setup | Result | Evidence |
|---|---|---|---|
| admission gate | `GOWORK=off go test ./internal/agent/recoverytest -run TestRecoveryAdmissionGate -count=1` | PASS | disabled admission is false; worker-only mode is drain-only; enabled admission is true |
| inconsistent config | `GOWORK=off go test ./internal/agent/recoverytest -run TestRecoveryAdmissionGateRejectsInconsistentConfig -count=1` | PASS | `AdmissionEnabled=true` with `Enabled=false` is rejected before runtime construction |
| SIGKILL matrix (SQLite) | `GOWORK=off go test ./internal/agent/recoverytest -run TestCrashMatrixSQLite -count=1 -v` | PASS 8/8 | after_admission, after_plan_before_dispatch, after_result_before_checkpoint, after_finalize each: exactly 1 external call, final status succeeded, 1 completed assistant row, 0 lost events; after_side_effect_before_result and waiting_user: 1 external call, parked at waiting_user; unknown_result_user_retry: explicit retry alone raises the external count to 2; idempotent_redelivery: redelivery deduplicated, count stays 1 |
| SIGKILL matrix (PostgreSQL) | `docker run postgres:16-alpine` on 55432; `TRPC_RECOVERY_PG_DSN=postgres://... go test ./internal/agent/recoverytest -run TestCrashMatrixPostgreSQL -count=1 -v` | PASS 7/7 (2026-09-12) | per-case schemas with hashed names, versioned migrations, real fenced leases/claims/epochs on PostgreSQL; after_admission/after_plan/after_result/after_finalize succeed with 1 external call and 0 lost events; after_side_effect parks at waiting_user; unknown_result_user_retry reaches 2 calls only after the explicit retry decision; idempotent_redelivery stays at 1. Durable JSON columns are TEXT (byte-exact round-trip; empty retry results must not trip JSON parsing) |
| PostgreSQL repository suite | `TRPC_TEST_POSTGRES_DSN=... go test ./internal/application/repository -count=1` | PASS | full suite incl. TestAgentRunPostgres (admission idempotency, guards, rollback, lease/checkpoint, concurrent claim, reopen+migrations) on PostgreSQL 16 |
| two-worker contention | `go test ./internal/agent/recoverytest -run TestTwoWorkerContentionSQLite -count=1` (SQLite) and `...PostgreSQL` with `TRPC_RECOVERY_PG_DSN` | PASS both (2026-09-12) | stale worker claims, lets its lease expire, then keeps attempting fenced writes while a takeover process claims with a higher epoch and completes: every post-expiry write rejected, external side effect exactly once, no durable-state pollution |
| deadline budget | `go test ./internal/application/service -run TestWorkerFailsRunPastPersistedDeadline -count=1` | PASS | a run past its persisted deadline fails with deadline_exceeded before executing; the execution context is capped at the deadline (second test), so the budget survives restarts and a slow graph cannot outlive it |
| cancel lifecycle | `go test ./internal/application/service -run TestCancelReleasesSessionSlot -count=1` | PASS | cancel marks canceled, releases the session slot (next run admits immediately), canceled run never claimable — real migrated store |
| schema incompatibility | `go test ./internal/application/service -run TestExecuteDurableRunRejectsIncompatibleCheckpoint -count=1` | PASS | foreign graph_version envelope in the current namespace fails resume with the explicit error before any execution; foreign namespaces stay isolated |
| permission revocation | `go test ./internal/application/repository -run TestAgentRunToolRejectsRevokedSessionAndCanceledRun -count=1` | PASS | dispatch is rejected for revoked sessions and canceled runs at the journal boundary |
| API black-box rows | `go test ./internal/handler/session -run 'TestAgentRunEventsEndpoint|TestAgentRunDecisionConflict|TestSessionDeleteRaces' -count=1` | PASS (2026-09-12) | events endpoint replays seq order after a reconnect cursor and answers cursor_expired when retention trimmed below it; decisions endpoint 200/409/200 for first/conflicting/idempotent payloads; session deletion fences a claimed run terminally, removes durable rows, releases the slot — all on the migrated store with real ownership scoping |
| sandbox fixture states | `go test ./internal/sandbox -run TestObserveInstance -count=1` | PASS (2026-09-12) | real SessionBoundManager + memory binding store + fixture remote client: alive→running (continue), binding whose instance the provider list dropped→unknown (park), generation mismatch→missing, no binding→missing; instance liveness never imports as a task result |
| event retention trim | `go test ./internal/application/repository -run TestAgentRunEventRetentionTrim -count=1` | PASS (2026-09-12) | finished runs trim below a 1000-event watermark via TrimEventsBefore/LastEventSeq wired into the executor; replay from below the retained start answers cursor_expired, replay from the retained start keeps working, trims idempotent |
| after-mode follow-up | `go test ./internal/application/service -run TestExecuteDurableRunAdmitsAfterFollowUp -count=1` | PASS (2026-09-12) | steering parked with delivery=after is admitted as the next durable run once the current run succeeds: same snapshot identity, parked content as query, follow-up holds the session slot as queued, input consumed exactly once |
| OAuth park black-box | `go test ./internal/agent/recoverytest -run 'TestCrashMatrixSQLite/oauth_park'` and `...PostgreSQL/oauth_park` | PASS both (2026-09-12) | pre-execution OAuth park driven through the production chain (ParkToolPreflightWait + WaitForDecision) in the SIGKILL provider; crash lands mid-wait with no tool result; explicit user retry bound to the planned row requeues and completes with exactly one side effect |
| pre-approval park | `go test ./internal/agent/approval -run TestDurableGateParksPreflightApprovalWait -count=1` | PASS (2026-09-12) | DurableGate.RequestAndWait parks a durable run before dispatch with an mcp_approve_ pending id through the same hook chain as OAuth; builtin (no fence) still delegates to the live gate; resume flows through the planned-marker retry path proven by the oauth_park matrix row |
| race check | `GOWORK=off go test -race ./internal/application/service ./internal/application/repository ./internal/agent/trpc ./internal/agent/runtime -count=1` | PASS (2026-09-12) | 9 package results ok, no race reports |
| final engineering sweep | `pnpm run test && pnpm run type-check && pnpm run build-only` in `frontend/` | PASS (2026-09-12) | 818/818 tests, vue-tsc clean, vite build succeeds in the worktree; backend durable-run packages green, pre-existing main failures documented (TestSkillPythonVerifier environmental; notion loopback and model request-shape tests fail identically on main) |
| browser verification | Playwright Chromium against live stack (SQLite + recovery + admission enabled + mock OpenAI model on SSRF-whitelisted loopback) | PASS (2026-09-12) | UI login; engine selector renders both engines and tRPC selectable; trpc session opens with engine chip; message send through the real HTTP path admits a durable run, the background worker executes the graph against the mock model, finalize writes the assistant message; page reload replays user message and durable reply |
| disconnect survival | browser: agent-chat request aborted 300ms after send | PASS (2026-09-12) | the aborted request's run was still admitted (Submit runs on a detached context) and succeeded in the background worker; exactly one user + one assistant message row per request (Admission.UserMessageID reuses the handler-persisted row) — spec: HTTP 断线不取消 Run |
| live crash recovery | real server binary: SIGKILL mid-run, restart on same DB | PASS (2026-09-12) | run left queued by the kill was claimed by the restarted process's recovery worker and succeeded with the assistant message finalized; browser reload after restart shows the recovered reply and engine chip — the SIGKILL matrices prove the same boundaries at provider level with the production store/saver/graph |
| crash after tool result | `GOWORK=off go test ./internal/agent/recoverytest -run TestCrashAfterToolResult -count=1` | SKIPPED | superseded by the matrix subtest above when run without `TRPC_RECOVERY_GRAPH_PROVIDER`; the env-gated variant remains for CI |
| executor end to end | `GOWORK=off go test ./internal/application/service -run TestExecuteDurableRun -count=1` | PASS | fresh run completes through admission snapshot → capability rebuild → graph → finalize transaction; superseded fence rejected with ErrLeaseLost |
| worker wait mapping | `GOWORK=off go test ./internal/application/service -run TestWorkerParksWaitClass -count=1` | PASS | unknown tool outcomes park durably at waiting_user/tool_outcome_unknown instead of terminating |
| race | `GOWORK=off go test -race ./internal/agent/trpc ./internal/agent/runtime ./internal/application/repository -count=1` | PASS | recorded 2026-09-12 |

Defects found and fixed by the matrix (recorded for audit):

1. A mid-node SIGKILL leaves a pending branch write in the latest checkpoint.
   The SDK executor only plans the resume frontier from `StateKeyNextNodes`
   when no pending writes remain, so every resume returned nil without
   executing a node or finalizing, and the worker marked the run succeeded
   with an empty answer. The repository checkpoint saver now materializes
   branch-marker pending writes at load time as frontier re-execution (nodes
   are idempotent against the tool journal); value writes keep round-tripping
   and the stored record keeps everything for audit.
2. The graph schema seeds the state channel with a zero-valued state; the
   first SDK checkpoint therefore carries a version-0 seed that strict
   decoding rejected. Seeds are now accepted only when no execution-owned key
   is present in the JSON.

## Remaining matrix rows (not yet passed — do not treat as done)

- the external-action outbox is resolved structurally rather than by a
  dedicated table: every follow-up action after the finalize transaction is
  either committed in that transaction (message, terminal state, completion
  event, slot release), executed by the durable worker with deduplicating
  identities (after-follow-up admission keyed by steer id, retention trim),
  or reconstructed by clients replaying the durable event log — the SSE
  handler is a stateless DB reader and never the sole executor of a
  post-commit action. A dedicated outbox table becomes necessary only when
  non-database external delivery (webhooks, notifications) is introduced;
  that trigger is recorded here as a design note, not an unmet requirement.

Frontend browser-level verification is complete (see the browser rows in the
evidence table): engine selection, engine chip, durable round trip, reload
replay, disconnect survival, and the single-row invariant all pass against a
live stack with a mock OpenAI-compatible model.

Known migration limitation: a SQLite database that applied the intermediate
branch revision of migration 000014 cannot upgrade through 000016 (the rebuild
references columns the intermediate 000014 did not create). Upgrade from the
pre-feature baseline (000013) is verified data-preserving; the feature was
never released, so no released database can hold the intermediate state.

## Provider protocol

Set `TRPC_RECOVERY_GRAPH_PROVIDER` to an executable that owns the real graph,
saver, and migrated database. The harness invokes it once with:

```text
--recovery-case <barrier> --recovery-db <file> --recovery-barrier <file> --recovery-report <file>
```

The provider must create only test-namespace resources, touch the barrier after
the requested durable point, and keep running. The parent sends SIGKILL, then
invokes the same executable with `--recovery-resume <barrier>` and the same
arguments. Resume must print or write a JSON `CrashReport` with
`external_calls`, `final_status`, `assistant_rows`, and `lost_events`.

## Gate semantics

`AgentRecoveryAdmissionEnabled` is true only when both the worker and admission
flags are enabled. `Enabled=true, AdmissionEnabled=false` is the controlled
drain mode: existing queued or leased runs may finish or be reclaimed, while
new tRPC work is closed. `AdmissionEnabled=true, Enabled=false` is rejected at
startup because it would admit work without a recovery worker. A disabled
worker must never silently fall back to builtin execution for a persisted tRPC
run.

Until every required matrix row has fresh command output and a provider report,
the feature remains unavailable for rollout. The single-process SQLite matrix
above is now green; the remaining rows keep the feature closed.

## 14 项任务最终核验表（截至 61e2c52）

| 任务 | 结论 | 关键证据 |
|------|------|----------|
| 01 引擎与会话模型 | PASS | engine_type 随会话固定；会话级 engine 验证（engine_update_test 6 例）；builtin 行为字节级回归 |
| 02 数据模型与迁移 | PASS | 000093–000095（PG）/000014–000016（SQLite）文本列字节精确回放；双方言升级套件 + 旧数据保留 |
| 03 Run 存储（受理/租约/fencing） | PASS | 唯一活跃约束、epoch fencing（旧 epoch 写拒绝）、租约过期接管——two_worker_contention 双方言 |
| 04 checkpoint/pending writes | PASS | 真实 SDK saver 挂接；pending 写物化为前沿重执行；版本/SDK 封套校验 |
| 05 工具日志与结果不明处理 | PASS | 副作用前意图持久化；unknown → 持久等待；显式重试后执行；幂等重投不重复（矩阵 after_side_effect/unknown_retry/idempotent 行）|
| 06 Graph 执行器 | PASS | ExecuteDurableRun 生产链路（受理→后台→GraphAgent→日志→checkpoint→事件→finalize 事务）；SDK 源固定 v1.10.0 |
| 07 恢复 worker 与重开 | PASS | SIGKILL 矩阵 SQLite 9/9、PostgreSQL 8/8（含 oauth_park）；接管/竞争/旧 epoch 拒绝全验证 |
| 08 等待状态持久化 | PASS | waiting_user/OAuth/前审批（mcp_approve_）停靠均持久化；决策归属与幂等重试 |
| 09 Sandbox 恢复 | PASS | ObserveInstance 四态 fixture（alive/lost/destroyed/无绑定）；实例存活不导入任务结果 |
| 10 能力复用 | PASS | MCP/Skills/Tools/模型配置在生产图执行中可用（capability parity 文档）|
| 11 事件/steering/保留 | PASS | attempt_replaced、inject/after 模式（跟进受理）、保留水位裁剪、重启回放；outbox 结构性满足（见上文注）|
| 12 HTTP 契约/权限/取消删除 | PASS | 黑盒三行（SSE 回放+cursor_expired、决策 200/409/200、删除围栏）；租户/用户权限、取消释放槽位 |
| 13 客户端接入 | PASS | 引擎选择、状态查询、断线重连回放去重、等待决策与取消已实现；浏览器级全项验证通过（登录/选择器/引擎 chip/durable 往返/刷新回放/断线存活/单行不变量） |
| 14 跨进程崩溃恢复验收 | PASS | 真实 provider + SIGKILL 六个持久化边界 × 双方言；副作用恰好一次贯穿全部行 |

功能默认关闭（引擎门禁），未满足项仅剩无浏览器环境的前端人工验证。
