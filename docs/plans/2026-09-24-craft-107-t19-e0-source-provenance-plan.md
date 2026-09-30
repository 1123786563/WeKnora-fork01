# Craft #107 T19 E0 Source Provenance Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace synthetic OpenCode `process_start`/`provider_selection`/`network_attempt` claims with three independently sourced, exactly joined observations, then make the full E0 verifier capable of accepting real evidence only when the measured fixed matrix is complete.

**Architecture:** Host launch records process/container/config identity. A pinned local OpenCode 1.18.4 plugin emits actual `chat.params` provider/model selection from the runtime hook. A separately controlled process-aware observer captures actual connection attempts and outcomes from the OpenCode process. The verifier joins by one invocation ID, PID/container/netns/config hash and bounded phase, retaining every retry/redirect. It never treats plugin input, a client URL, human log, manifest field or sink silence as network-attempt proof. The current unconditional `evidence_type != synthetic` BLOCKED gate is revised only after all source boundaries are reviewed and adversarial tests pass.

**Tech Stack:** OpenCode 1.18.4 plugin API, Python E0 runner/recorder/verifier, Docker/OrbStack Linux/arm64, process/network observation. Official pinned source: OpenCode v1.18.4 commit `49c69c5ed3ccf706b61b3febb43c8aaff7f8325e`, `packages/plugin/src/index.ts` `chat.params` hook and `packages/opencode/src/plugin/index.ts` loader.

**Spec:** Approved Craft #107 T19 E0; Fix6 source-owned verifier; `2026-09-24-craft-107-t19-e0-volume-free-image-task2-fix1-plan.md`; read-only E0 provenance architecture 2026-09-24. Full E0 remains BLOCKED until Task5.

## Global Constraints

- No paid external provider call. Fixed local mock, direct/listener and isolated no-uplink controls only; no customer data. Preserve pinned OpenCode executable hash/version and reviewed derivative unless a necessary instrumentation build explicitly creates a new pin/review gate.
- A plugin proves selection only. It cannot author `network_attempt`, because `chat.params` runs before network send and plugin load may fail/skip. Host-generated `process_start` must include observed process identity, not copied config fields alone. A denied attempt may have no sink log; absence of sink traffic is not denial proof.
- The physical observer must be process-aware and source-owned: exact PID/container/netns, destination, syscall/packet outcome, monotonic window and raw log hash. If this runtime cannot provide such observation, stop this branch with a verified blocker; do not fabricate rows or weaken verifier.
- Preserve D/A private recorder, helper-start acknowledgment, fixed config bytes, full control matrix and cleanup. No release, production route, commit/push/deploy until E0 and OCR pass.

## Review Focus

- Host launch, runtime selection and physical attempt derive from three separate authorities and join to one invocation; wrong PID/config/attempt/proxy/phase rejects. Retries/redirects remain visible rather than collapsed into exactly one invented row.
- Full verifier cannot pass a synthetic-only fixture as measured E0, and cannot accept live evidence solely because `evidence_type` changed. Every fixed listener/case/phase and cleanup has raw source receipts.

### Task 1: Process-aware attempt observation feasibility

**Depends on:** volume-free image/runner Fix1 reviews PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** new bounded observer/test files under `docs/testing/craft/egress-probe/`, evidence/report; no `run_v2.py`, `assert_v2.py`, recorder or production edits.

- [ ] Inspect available observer (`strace` parent-child, host kernel/packet observer or equivalent) inside the pinned derivative with no mutation to OpenCode binary. RED controlled fixed connect without observer has no process-attributed attempt.
- [ ] In one disposable `--network none`/internal-only fixture, capture host-issued invocation/PID/container/netns, raw source-owned connect/sendto attempts to fixed local and denied destinations, outcome/ordering and cleanup. Prove wrong PID, no packet, unrelated process and response loss do not receive attempt credit.
- [ ] Save exact commands, raw syscall/packet logs, image/observer hashes, cleanup and checkpoint; independent review. If kernel/runtime cannot expose PID attribution without unacceptable scope, report exact verified blocker and stop downstream provenance Tasks.

**Acceptance / failure handling:** One real process-bound network attempt and one negative control are independently reviewable; no E0 PASS.

### Task 2: Runtime provider-selection hook

**Depends on:** Task1 reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** new local plugin under `docs/testing/craft/egress-probe/`, focused plugin/test and image wiring only; no runner/verifier edits.

- [ ] RED no plugin hook/failed load yields no selection. Install a reviewed fixed local plugin using the pinned 1.18.4 API and private source-owned output; prove `chat.params` runs with actual runtime model/provider and configuration bytes/ID before a local mock request.
- [ ] Retain plugin source hash, loader success/failure, hook invocation, PID/invocation/config binding, and negative wrong-provider/failed-hook controls. No self-declared network attempt.
- [ ] Exact checkpoint and independent review; if hook cannot safely bind to invocation, report blocker.

### Task 3: Runner joins host launch, plugin and observer

**Depends on:** Tasks1–2 reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/run_v2.py` and focused tests only. No verifier/recorder edits.

- [ ] RED human-log-only and adapter-counter-only records are insufficient. Launch one OpenCode invocation under host capture, collect plugin and observer private streams, join exact PID/container/netns/config hash/phase and all attempt rows; fail closed on missing/different/extra origin.
- [ ] Bounded no-egress local mock plus denied-target proof, raw files/hashes/cleanup, exact checkpoint/review. Do not synthesize missing trace rows.

### Task 4: Verifier accepts measured evidence only from joined raw sources

**Depends on:** Task3 reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/assert_v2.py` and focused adversarial tests only. No runner/plugin/observer edits.

- [ ] RED current unconditional live BLOCKED and exactly-three-row synthetic schema; adversarial fixtures for forged PID/config/provider/target, missing attempt, retries/redirects, unrelated process, phase/cursor overlap and cleanup.
- [ ] Validate raw host/plugin/observer authority and joins, every fixed control and full matrix; replace the unconditional live BLOCKED gate only with measured-evidence predicate. Keep synthetic fixtures labeled synthetic and never a release result.
- [ ] Focused mutation suite/checkpoint/independent review.

### Task 5: One full pinned no-egress E0 run

**Depends on:** Tasks1–4 independently reviewed PASS and Docker resource slot. **Owner:** `backend_validator`; **validator:** independent `reviewer`.

**Owned files:** evidence/report only.

- [ ] Run one finite fixed pre/post matrix with raw source streams, host commands, inspect/config hashes, provider/attempt joins and exact cleanup.
- [ ] Run unchanged-after-review verifier; retain complete report and list exact red cases. Only observed complete PASS can unblock E1/production routing.
