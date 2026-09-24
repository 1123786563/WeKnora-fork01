# Craft #107 R5 Effect Task 3b Physical Provider Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Connect the reviewed admitted RunView coordinator to real Docker/OpenCode operations without any external mutation hidden inside an observation or sent without a durable, generation-bound claim.

**Architecture:** Extend effect authority with separate `docker_network_create` and `docker_probe` intents. A new split adapter exposes read-only network/container/state/session observations and one-send mutations. The coordinator first claims network creation, then container creation, start, and runtime probe as separate effects; each succeeds only with an exact independently observed receipt. Ambiguous responses remain unknown and are reconciled by read-only inspection without resending. Central DI selects the admitted path only after physical evidence and registry pin are verified.

**Tech Stack:** Go Craft domain/application repository/container, Moby Docker client, SQLite/PostgreSQL migrations, isolated Docker tests.

**Spec:** Approved Craft #107 T01/R5 and H2; `2026-09-24-craft-107-runview-r5-effect-task3a-fix1-task1-review.md` scoped PASS; read-only R5 Task3b architecture 2026-09-24; R4 Task3 and T05 H3 central DI remain downstream.

## Global Constraints

- `InspectOrCreateContainer`, `verifyCurrent`, `EnsurePrivateNetwork` and `ProbeRuntime` are physically mutating combined APIs and cannot implement admitted read methods. Keep legacy callers separate until admitted DI is proven.
- Every network create, container create, container start, Docker ExecCreate/ExecAttach runtime probe, and OpenCode session POST requires its own durable claim. A claim precedes exactly one send; an unknown send never repeats. A read-only inspection may resolve a uniquely attributable outcome; absence after unknown is unresolved, not permission to resend.
- Original admitted `craft.Task` fence/digest and generation bind all claims. No key-only production fallback, no publication, no live default-on without registry digest and full independent review. No commit/push/stash; exact uncommitted checkpoints.
- Reserve SQLite `000126` and PostgreSQL `000205` for this plan's effect-kind schema extension; verify the numbers remain unused immediately before edits. SQLite table rebuild must preserve existing rows/indexes/FKs, PG constraint migration must preserve existing data and reversible down behavior. Both migration trees are ignored in Git and must be force-tracked in final OCR workspace.

## Review Focus

- Observation call graph contains no network create, container create/start, Docker Exec or filesystem mutation. On ambiguous response, replay is inspect-only and never resends.
- Exact network ID/name/internal driver/labels/scope/attachments and container ID/generation/image/mounts/security/runtime hash are sourced from Docker, never request echo. Foreign resources or identity drift fail closed.
- Runtime probe effect does not become a repeatable Docker Exec on replay. OpenCode session creation is one claimed POST with exact project/directory/session receipt.

### Task 1: Persist network and runtime-probe effect kinds

**Depends on:** R5 Task3a Fix1 independent PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/modules/craft/run_view_effect.go`, `internal/application/repository/craft_run_view_effect.go`, focused effect repository tests, migrations `migrations/sqlite/000126_*` and `migrations/versioned/000205_*`; no container adapter/DI files.

**Consumes:** existing admitted Task/generation/fence/digest and a canonical request/spec digest. **Produces:** `docker_network_create` and `docker_probe` claim/finish/unknown rows with the same transition guard. Add an `BeginEffectWithDigest` seam for new kinds; legacy `BeginEffect` must reject those kinds without a request digest.

- [ ] Capture exact preimage and check migration number vacancy. RED tests for new kinds rejected before migration and accepted after it, one-shot claim, exact receipt, unknown replay, transition fence, old rows retained, down/up.
- [ ] Extend kind validation, persist a request/spec digest and paired schema constraints without weakening existing kinds or allocation prerequisite. First claim and exact replay bind the digest; changed request must fail before send. Enforce nonempty succeeded receipts.
- [ ] Run focused SQLite/isolated PG behavior, race, migration up/down/up, exact checkpoint/review. Failure leaves kinds disabled.

**Acceptance / failure handling:** Both new kinds use the existing durable one-send authority with no duplicate send permission or lost old intent.

**Task 1 request identity contract:** `docker_network_create` and `docker_probe` claims require `RunViewEffectRequestAuthority.BeginEffectWithDigest` with a lowercase 64-character SHA-256 digest of the complete canonical request, including a domain separator chosen by the caller. Network input covers at least name, driver, internal flag, labels, scope/options; probe input covers the exact target container and probe operation/expected runtime identity. The repository stores and compares the digest but does not canonicalize provider input. Exact replay must pass the same digest; drift conflicts before a second send. The legacy `BeginEffect` remains valid for existing kinds and refuses these two new kinds without a digest. Down migration refuses while either new kind has durable rows rather than losing or relabeling their authorization history.

### Task 2: Split real Docker provider into pure reads and bounded sends

**Depends on:** Task1 independent PASS for effect contract; legacy Docker engine review PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/craft_runview_container_provider.go` or new admitted provider file, `internal/container/craft_runview_docker_engine.go`, their focused tests. No runtime coordinator or central DI edits.

**Consumes:** exact generation network/container specs from admitted coordinator. **Produces:** `ObserveNetwork`, `CreateGenerationNetwork`, `ObserveContainer`, `CreateGenerationContainer`, `ObserveContainerState`, `StartGenerationContainer`, `ObserveRuntimeProbe`, `SendRuntimeProbe`, `ObserveSessions`, `CreateOpenCodeSession` as typed operations. `ObserveRuntimeProbe` may report unknown if Docker has no genuinely read-only proof; it must never run Exec.

- [ ] Capture preimage and RED strict call-count tests asserting every Observe uses inspection/read-only filesystem only; validate exact labels, network attachment, image digest/security, marker and provenance.
- [ ] Split `EnsurePrivateNetwork` into inspect/create and `ProbeRuntime` into claimed send/read-only evidence. Bound each physical method to one Docker API send; re-inspect exact ID after a response. Preserve legacy interface behavior for existing callers.
- [ ] Run focused fake and one disposable real Docker fault/replay suite serially, exact checkpoint/review. On unavailable OpenCode observation or probe receipt, report interface blocker and fail closed.

**Acceptance / failure handling:** Adapter cannot mutate from Observe, and ambiguous sends cannot be reissued by adapter or coordinator.

### Task 3: Sequence new effects in admitted coordinator

**Depends on:** Tasks1–2 independently reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/craft_runview_runtime.go` and admitted runtime focused tests only. No adapter/repository/DI edits.

**Consumes:** Task2 typed split provider and Task1 effect kinds. **Produces:** exact network→container create→start→probe→OpenCode ordering and bound handle.

- [ ] RED fake tests for network response loss, duplicate name/foreign attachment, network unknown barrier, start response loss, probe Exec response loss, replay inspect-only and no second send.
- [ ] Claim each effect immediately before corresponding send; observe before claim where a read is needed; persist exact independent receipt or unknown. Never advance downstream on pending/unknown.
- [ ] Focused/race tests and exact checkpoint/review. If runtime probe cannot prove completed effects, leave admitted path unresolved.

**Acceptance / failure handling:** No Docker/OpenCode mutation occurs without one effect claim for the same admitted Task/generation; all unknown stages stay parked.

### Task 4: Production assembly and bounded physical admission

**Depends on:** Tasks1–3 independently reviewed PASS, reviewed registry digest available. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/container.go`, assembly tests and narrow configuration files. Coordinate with R4/T05 owners before touching shared DI.

- [ ] RED assembly test that key-only/legacy provider cannot be selected for production Craft.
- [ ] Wire admitted provider and authority under default-off gate; verify exact image digest, one physical Docker/OpenCode admission and replay/unknown fail-closed in a disposable isolated environment.
- [ ] Save full evidence/checkpoint/review; hand bound handle to R4 Task3 and H3. No default-on until complete branch OCR.

**Acceptance / failure handling:** Production path uses only the reviewed admitted coordinator and pinned physical provider; any missing digest/provider observation keeps gate off.
