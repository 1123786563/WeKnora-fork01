# Craft #107 T19 Docker S3 Fix 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the S3 Fix1 Task1 review's terminal-after-cancellation race and document the first-inspect liveness limit without weakening fail-closed behavior.

**Architecture:** Recheck the bounded Wait context after each inspection and before accepting any terminal observation. Keep the durable claim/receipt and unknown result when cancellation or deadline wins. Preserve positive-running provenance: a first `false/zero` remains unknown.

**Tech Stack:** Go, existing restricted Docker service fake and targeted race tests.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-docker-s3-fix1-task1-review.md`; `docs/plans/2026-09-24-craft-107-t19-docker-s3-provider-design.md`.

## Global Constraints

- No provider call after a post-claim cancellation; no retry of an ambiguous chargeable Start.
- Wait cancellation/deadline/inspect failure is unknown with exact receipt and retained durable claim; stdout/stderr and unmeasured duration remain explicitly unavailable.
- No production routing, unrelated edits, commit or push. Preserve all concurrent Worktree changes; exact pre/post checkpoint and independent review required.

## Review Focus

- Inspect returns a terminal success after cancellation during the call.
- Inspect returns a terminal failure after Wait deadline during the call.
- First inspection sees false/zero without positive running provenance.
- Context cancellation races with a previously accepted positive running observation.
- Claimed receipt and durable hold remain unchanged on all unknown results.

---

### Task 1: Bound the terminal decision after inspection

**Depends on:** S3 Fix1 Task1 checkpoint/review. **Owner:** `backend_implementer`. **Validator:** `backend_validator` plus independent `reviewer`.

**Owned files:** `internal/application/service/craft_docker_restricted_exec.go`, `_test.go` only.

**Consumes / produces:** Existing `CraftDockerRestrictedExec.Wait`/`Observe` and exact receipt; produces a terminal decision only while Wait context is live.

- [ ] Capture preimage hashes. Add deterministic RED tests with a blocked inspect that ignores cancellation and then returns terminal success/failure after the context is canceled or deadline expires; assert unknown, exact receipt and retained claim. Add first-inspect false/zero test asserting conservative unknown/liveness limit.
- [ ] Run RED and save output. Add the smallest post-Observe context check before accepting terminal state; preserve unknown handling and no hold release.
- [ ] Run focused normal and race tests, compile check, gofmt and diff check. Save exact checkpoint/patch/report; independent reviewer gives separate Spec and quality verdicts on F1/F3.

**Acceptance / failure handling:** F1 passes with deterministic ordering. F3 is accurately tested/documented. S3-R1 remains open until an authoritative command-duration source is reviewed; S3-R2 coordinator proof still follows the earlier Fix1 Task2 plan. A passing F1 review does not assert full S3 or T19 acceptance.
