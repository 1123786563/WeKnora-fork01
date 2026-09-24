# Craft #107 T19 Normal Output Task 2b Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Compose the reviewed normal Docker provider, durable one-send claim and bounded output store into a replay-safe application operation, while keeping production routing default-off.

**Architecture:** Admission and budget hold precede an immutable input/policy stage and inert `ExecCreate`. Bind a complete receipt, including stdin identity and timeout, before the durable send claim. Create a single durable writer only for the claimed receipt. Consume the process-local permission immediately before one `ExecAttach`. Recovery observes the exact stored receipt and committed output through a cursor; it never reattaches. Process success and output completeness remain separate.

**Tech Stack:** Go, Gorm, SQLite/PostgreSQL paired migrations, pinned Moby client, disposable Docker.

**Spec:** Approved Craft Spec #107 T19; `2026-09-24-craft-107-t19-normal-output-plan.md`; reviewed provider Fix3 and output-store Fix1 reports; S2/S3 coordinator reviews. The read-only architecture check on 2026-09-24 identified the three-ID receipt and unbound Prepare replay as the principal gaps.

## Global Constraints

- `Prepare` commits Run fence, intent and hold before Docker create. Any changed command, stdin, timeout, output quota or policy for the same activity conflicts before physical work.
- A complete, immutable provider receipt includes container/exec ID, stdin-enabled flag, byte count, SHA-256 and timeout; stage bounded stdin bytes durably without logging raw content. Bind and claim must compare the exact staged identity.
- Claim is the sole send authority. Any outcome after claim that lacks positive start/terminal/transport/sealed-output evidence is unknown or partial, holds budget/fence and cannot promote a build. `ExecAttach` is the sole physical start and cannot retry after ambiguity.
- Direct output callbacks remain rejected. Live readers use only durable `ReadAfter`; a slow reader cannot block provider append or seal. Do not change ordinary production routing in Tasks 1–3.
- No commit/push/stash. Exact pre/post checkpoints and ignored migration bytes are required. PostgreSQL full chain is currently blocked by missing base `vector` extension; use isolated target migration schema and separately record that limit.

## Review Focus

- No changed activity request can reuse an unbound intent or bound receipt.
- Crash at stage/create/bind/claim/open-sink/start/stream/seal/inspect never triggers a second start or turns missing output into success.
- Current Run, tenant/Task/Run/activity and exact Docker receipt remain checked under durable authority. A new writer cannot be reopened after uncertain seal.
- Complete requires positive start evidence, exact-ID terminal inspect, complete transport and durable sealed, nonpartial, nontruncated output; quota loss remains partial even if process exit is zero.

### Task 1: Immutable normal input stage and full receipt

**Depends on:** Provider Fix3 and output-store Fix1 independent PASS; migration 203 isolated PG PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** new `internal/application/repository/craft_docker_normal_input.go` and tests; narrow `internal/application/repository/craft_docker_send_claim.go` and tests; paired next migration numbers assigned by controller after SQLite124/PG203 (reserve SQLite125/PG204), no coordinator or routing files.

**Consumes / produces:** `(tenant, Task, Run, activity)` and canonical command/env/user/working-dir/timeout/stdin/output policy; produces create-only staged bounded input and immutable full receipt with exact replay, scoped recovery read and no raw byte logging.

- [ ] Capture preimage. RED SQLite and isolated PG tests: exact replay, changed command/env/stdin/timeout/quota conflict; cross-tenant/Run mismatch; concurrent same activity; bind after claim; altered full receipt; zero-byte stdin identity; corrupt staged bytes.
- [ ] Implement schema and narrow repository transaction/CAS. Stage before `ExecCreate`; persist complete provider metadata before claim. Define bounded blob protection compatible with repository deployment (do not invent encryption key material); if safe at-rest staging cannot be established, leave Task blocked rather than persist sensitive stdin contrary to project policy.
- [ ] Verify migration up/down/up, focused/race tests, exact checkpoint/report and independent review.

**Acceptance / failure handling:** The repository can reconstruct only the original create request and receipt; no changed input can inherit a held intent.

### Task 2: Coordinator binds staged identity

**Depends on:** Task 1 reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** narrow `internal/application/service/craft_docker_send_coordinator.go` and tests, service adapter to Task1 stage. No provider, output store or production route edits.

**Consumes / produces:** Existing S2 Prepare/ResumeBound/Bind/Claim and Task1 staged identity; returns exact full receipt only after validating Task binding, current Run/revision and immutable stage. `ResumeBound` remains preclaim only; claimed replay is observation-only.

- [ ] RED Prepare replay with changed request, stale Run, changed grant binding, same exact stage, crash after create/bind, concurrent claim.
- [ ] Add explicit typed normal operation method rather than weakening restricted outputless protocol. Keep external Docker I/O outside DB transactions. Preserve journal unknown/hold semantics.
- [ ] Focused SQLite/PG/race checks, checkpoint/report, independent review.

**Acceptance / failure handling:** No activity request or receipt identity drift across Prepare→Claim or recovery.

### Task 3: Normal operation service and durable output projection

**Depends on:** Task 2 reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** new `internal/application/service/craft_docker_normal_exec.go` and tests, narrow output-store transaction guard only if review proves a Run/claim race; no central DI/routing.

**Consumes / produces:** S2 normal claim, provider `CreateAttachedExec`/`StartAttachedExecOnce`/`ObserveAttachedExec`, output `OpenSink`/`ReadAfter`; returns typed process, transport and output state, exact receipt and cursor.

- [ ] RED fake/restart tests for bound replay, claim race, after-claim cancellation, sink open/seal failure, response loss, stdin mismatch, quota, stdout/stderr interleave, exit-zero/early EOF, cursor reconnect and no second attach.
- [ ] Compose `Prepare → stage → inert create → Bind → Claim → OpenSink → permission Consume → one ExecAttach`; on postclaim failures return exact unknown receipt and observation-only recovery. Never call provider direct callbacks. Do not settle charge or promote build from partial evidence.
- [ ] Focused SQLite/PG/race and one disposable Docker end-to-end no-egress marker-once test; checkpoint and independent review.

**Acceptance / failure handling:** Exact one-start physical path with durable output and explicit unknown/partial classification. Production routing remains default-off until Task 3 review and the original plan's routing Task 3 plus T19 egress gates.
