# Craft #107 T19 Normal Output Provider Fix 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining unbounded caller callback lifetime finding in normal-output Fix 2.

**Architecture:** Keep the provider's accepted operation limited to a separately verified durable `Append/Seal` sink. Reject every direct caller output callback, including the exported typed interface, before `ExecCreate`. Later live projection reads the durable output store under its own cancellation and cursor contract; it is not invoked synchronously inside the Docker transport.

**Tech Stack:** Go, pinned Moby client, focused/race tests.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-normal-output-fix2-task1-review.md`; approved Craft #107 T19 output/abort/recovery requirements; normal-output architecture plan.

## Global Constraints

- One `ExecAttach` after durable claim; no second start, reattach or fallback. Do not alter frame/transport behavior unless required by this narrow fix.
- No accepted arbitrary caller callback may block bounded cancellation or produce work after provider return. Do not silently ignore a requested callback; reject before any Docker side effect.
- A conforming durable output sink remains required; its real implementation and live projection belong to separate application Task2 review. Unsupported callback rejection must be explicit and typed.
- No production routing, commit or push. Record exact checkpoint and independent review.

## Review Focus

- A typed callback whose `Seal` blocks forever and one whose `Seal` lies are both rejected before `ExecCreate`, so they never enter the provider lifecycle.
- Legacy callback rejection remains; no callback work after return for accepted requests.
- Existing EOF/unknown, cancellation, sink freeze, stdin/demux and one-send behavior remain intact.

---

### Task 1: Remove accepted direct callback capability

**Depends on:** Fix2 independent review FAIL High. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/modules/execution/sandbox/docker_normal_exec.go`, `internal/modules/execution/sandbox/docker_normal_exec_test.go` only.

**Consumes / produces:** Fix2 provider API; produces explicit pre-create rejection of all direct output callback forms and an output-only durable sink contract.

- [ ] Capture two-file preimage. RED tests pass a typed callback with blocking `Seal` and one with ineffective `Seal`, then prove provider returns an unsupported/invalid-input error before `ExecCreate`/`ExecAttach`; no unsafe callback is invoked. Retain legacy rejection test.
- [ ] Remove synchronous typed callback invocation and its lifecycle machinery from the accepted path. If an exported request field must remain for source compatibility, reject non-nil values explicitly. Document that application live projection must consume durable output after store append; do not claim it implemented here.
- [ ] Run focused normal tests and `-race`, compile/package check, formatting/diff. Preserve prior real Docker transport proof unless transport changes. Save exact report, checkpoint and incremental patch for independent review.

**Acceptance / failure handling:** Callback F1 High closed without compromising output persistence. Full T19 normal route remains gated on durable sink and application coordinator Tasks 2–3.
