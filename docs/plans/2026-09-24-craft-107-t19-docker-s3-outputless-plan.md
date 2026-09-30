# Craft #107 T19 Docker S3 restricted outputless initiation plan

> **For Codex:** Execute through SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint, backend validation and independent Spec/quality review. Do not advertise `RemoteExecInitiator` or enable production normal commands.

**Goal:** Prove one safe physical Docker `ExecStart` path after S2's durable hold, receipt and exclusive claim. This is an intentionally outputless restricted capability; complete stdin/stdout/stderr/live-output semantics and full T19 remain gated.

**Sources:** Approved Craft Spec Task Budget/recovery; `2026-09-24-craft-107-t19-docker-s3-provider-design.md`; S1/S2 plans and S2 Fix1 independent PASS; pinned Docker client; `remote_operation.go` and `docker_remote_client.go`.

## Global constraints

- Integration Worktree, no commit/push, production enablement, paid calls or external credentials. A claimed operation is never resent; same Exec ID 404/false-zero is unknown and retains hold/fence.
- `ExecCreate` is inert but `ExecStart` is chargeable. Commit hold before create, exact receipt before claim, claim before one physical detached Start. No SQL transaction across Docker I/O.
- Use a distinct API/type requiring explicit output-discard opt-in. Reject stdin, `OnOutput` and output-requiring callers before create. Never call `ExecAttach`, legacy `Exec`, or advertise the full `RemoteExecInitiator` on this restricted path.

## Review focus

Physical `ExecStart` count, pre-send CAS, lost-response/cancel/crash ambiguity, same-ID recovery, no false empty-output claim, bounded RPC/wait, real Docker marker once and cleanup.

## Task 1 — Narrow provider transport and coordinator composition

**Depends on:** reviewed S2 Fix1. **Role:** backend_implementer; validator backend_validator. **Owned files:** new `internal/modules/execution/sandbox/docker_restricted_exec.go/_test.go` plus narrow Docker client-private adapter methods in `docker_remote_client.go` only if necessary, and new `internal/application/service/craft_docker_restricted_exec.go/_test.go` for explicit coordinator composition. Do not edit Run admission, generic remote_operation.go or central container assembly. **Consumes:** S2 opaque `DockerSendOperation`, exact Docker handle/request and engine. **Produces:** outputless bounded initiation/observation result with durable receipt and no more than one start call.

1. RED fake-engine and coordinator tests: reject unsupported stdin/callback/output contract before any hold/create; prepare commits hold, lost create response leaves only inert orphan; bind failure makes zero starts; two claimants only one physical Start; crash after bind resumes same ID; crash/timeout/cancel after claim, 5xx and lost response result in unknown with unchanged hold/fence and zero re-send; false/zero, 404 and mismatched inspect stay unknown; positive running→terminal same ID resolves exit without pretending output is empty. Test token consumption plus actual callback count and side-effect marker, not token counter alone.
2. GREEN: define separate outputless request/result (stdout/stderr explicitly unavailable). Use existing command validation and timeout wrapper where safe, ensure running handle and stable container identity before inert `ContainerExecCreate`; bind S1 receipt, claim S2, consume permission exactly once before `ContainerExecStart(Detach=true,TTY=false)` with short context deadline. Observation and bounded wait inspect only the same ID and require positive-start provenance before terminal result. Preserve unknown and held fence on ambiguous outcomes. Keep legacy `RemoteExecInitiator` unsupported for Docker.
3. Run targeted normal/race tests and a disposable pinned real Docker probe: unique marker written exactly once, detached Start/Inspect, read-only proof that attach is never used, false-zero/404 classification, cleanup inventory. Pair with S2 SQLite/PG claim tests where feasible. Save exact file hashes, task-local patch, image/client/daemon versions, command exits and independent review. If the pinned SDK cannot support a bounded detached call or outputless result without changing generic API, stop and report exact seam instead of widening implicitly.

**Acceptance:** restricted outputless path satisfies one-start safety and real Docker behavior; S2-R3 physical callback gap closes for this path only. Full T19 ordinary command routing, output/streaming, E2B/Cube/create and egress remain open. **Failure handling:** keep provider capability undiscoverable and production runtime off until review.
