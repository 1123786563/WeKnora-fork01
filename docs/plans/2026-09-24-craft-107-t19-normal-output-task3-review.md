# Craft #107 T19 normal-output Task 3 — independent review

Date: 2026-09-24. Read-only source review against the approved Craft web Artifact Spec, ADR-0004, `CONTEXT.md`, the normal-output plan and Task2b Task 3 brief. The upstream Task2b Task 2 and enabled-empty provider reviews both report scoped PASS. No OCR or Docker run was repeated.

## Verdict

- **Scoped Spec compliance: FAIL (one Medium finding).** The intended normal path orders immutable preparation/stage, inert create, full bind, claim, output open, one-use permission consumption and one provider attach correctly. Claimed replay is observation-only, and no production route was changed. However, a returned unknown result can simultaneously advertise complete output, which violates the explicit partial/unknown contract.
- **Code quality: FAIL pending the same correction.** The service has narrow interfaces and meaningful saved SQLite, isolated PostgreSQL, race and physical Docker evidence. One returned-result invariant is wrong; the tests do not exercise it.
- **T19 overall remains gated.** The original plan's caller-routing and joined budget/build proof, plus T19 egress gates, are separate downstream work.

## Finding

### T3-1 — Medium — Provider error can coexist with `OutputComplete=true`

**Evidence / affected symbol:** `CraftDockerNormalExecService.Execute`, `internal/application/service/craft_docker_normal_exec.go:189-207`, computes `OutputComplete` and clears `Output.Partial` before examining `startErr`. If a provider returns positive start, terminal process, complete transport and a sealed sink together with a nonnil error, the returned result has `OutputComplete=true` and `Output.Partial=false`, while the function changes `Transport` to partial and returns `RemoteOperationUnknown`. The current concrete Docker provider normally downgrades transport for its known stream/inspect failures; the service's typed provider interface does not require that invariant, and this branch explicitly handles a complete transport accompanied by an error. The fake provider can represent the case, but the existing response-loss test supplies unavailable transport only (`craft_docker_normal_exec_test.go:284-311`).

**Impact:** A caller that examines the typed result or persists it separately from the error can treat output as complete after an ambiguous claimed start. That contradicts Task 3's requirement that postclaim failures remain unknown/partial and cannot support build promotion. It also returns internally contradictory transport and output fields.

**Smallest defensible correction:** Include `startErr == nil` in the completeness predicate, or downgrade the transport and mark output partial before computing completeness. Add a focused fake-provider case with complete-looking outcome plus nonnil start error; require unknown error, exact receipt, one attach, `OutputComplete=false`, `Output.Partial=true`, and partial transport.

## Coverage and evidence limits

The focused suite covers exact bound resume, claimed replay without reattach, request mismatch, enabled-empty stdin, cancellation after output open, sink-open/seal failures, quota, cursor reads and one physical no-egress marker. The concrete provider suite separately covers clean EOF while still running. Task 3's requested service-level concurrent claim race and exit-zero/early-EOF composition are not explicit tests in this file; add them if the controller needs direct service-layer regression coverage. This is a coverage note, not a second observed defect.

Saved evidence reports PASS for focused service tests, SQLite and isolated PostgreSQL projection, and focused race. The physical Docker log shows an initial test-only named-type assertion failure followed by PASS: a pinned local image, `NetworkMode=none`, one create/start, claimed replay with one observation and no second start, marker `x`, and verified container removal. PostgreSQL used the isolated schema; it does not prove the full production migration chain. No charge settlement, build promotion or ordinary routing is implemented in this Task.

## Exact uncommitted review range

Both owned files are untracked and absent from HEAD. The Task 3 checkpoint records them as absent at task start, so the entire current files are the Task 3 addition; there is no test preimage to reconstruct for this task. Current SHA-256 hashes match the checkpoint: source `de0236971e91f92da17fdc8eb44d87333706f2c7f2590b80e5d208213dc5e430`, test `24718949d5c56a96b20362614cea4f1414579e2c8eb11b1de2a84e2f2c64c31d`. Report, focused output and Docker log hashes also match the checkpoint (`1dc4b24d...`, `0edc495c...`, `462b2c41...`). This review covers the full delivered two-file addition, not an empty `BASE..HEAD` diff. Upstream Task2b Task 2's missing task-start test preimage remains documented in its own review and does not affect the provenance of these new files.
