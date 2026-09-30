# Craft #107 T19 E0 Runner Source Integration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Have the host runner produce the independently reviewed source-owned `helper_start` boundary and isolated D/A evidence topology for each E0 control phase, without claiming the full live E0 gate.

**Architecture:** The host controller records one successful exact Docker helper-start operation with an operation ID and monotonic ordinal, then sends a private `helper_start` command to D and A recorders and retains both fsynced cursors before any control request. D and A use separate writable recorder volumes; C and the helper receive none of those mounts. A failed start/ack or early source traffic aborts the phase and marks the artifact blocked. The verifier reconstructs from raw operation log, inspect snapshots and streams; runner summaries cannot supply missing events.

**Tech Stack:** Python E0 Docker fixture runner, private Unix recorder sockets, existing source/verifier tests, disposable OrbStack Docker.

**Spec:** Approved Craft #107 T19; `2026-09-24-craft-107-t19-e0-evidence-architecture.md`; Fix6 verifier; `2026-09-24-craft-107-t19-e0-source-helper-boundary-task1-review.md` scoped PASS.

## Global Constraints

- Only synthetic fixture/mocked or disposable Docker; no paid model egress, production routing, keys, privileged C/A/D or shared writer mounts. C cannot write D/A recorder evidence or command sockets; A cannot write D evidence. Helper has no recorder mounts.
- Host operation ID/ordinal comes from actual Docker command result, not source/client header. Both source acknowledgments precede same-sink control traffic; no phase proceeds on one-sided or failed ack.
- Preserve exact original pinned OpenCode image/config and process-result evidence. This Task does not invent selected-provider or physical-attempt provenance; missing proof keeps E0 BLOCKED. No commit/push/stash; exact checkpoint and raw logs.

## Review Focus

- Every control phase has exactly one successful helper-start operation, then D/A source `helper_start` events matching its ID/ordinal, then first `tcp_accept`.
- All mount source/destination aliases and ancestor paths show D/A private ownership, zero helper mounts and no client access. No mode-0777 shared evidence directory.
- A missing/duplicate/wrong op, failed command, timeout, early TCP, after-barrier event or cleanup failure yields BLOCKED with retained raw evidence.

### Task 1: Paired source acknowledgments and private recorder topology

**Depends on:** Recorder source helper boundary independent PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/run_v2.py` and focused runner tests (new test file if needed). Do not edit `server_v2.py`, `assert_v2.py`, production gateway or T19 normal Docker source.

**Consumes / produces:** Actual helper Docker start receipt/host operation log and private D/A socket commands; produces paired source event cursors and complete volume/inspect inventory for the existing Fix6 verifier.

- [ ] Capture preimage. RED synthetic runner/recorder integration tests: command success before both ack, source event before control accept, wrong/duplicate ID/ordinal, D/A private volume inventory including alternate destinations, abort and cleanup when any ack fails.
- [ ] Implement exact operation log and `helper_start` command dispatch immediately after each successful host helper start. Split recorder volumes and host command sockets by source, remove broad shared `/state` and `/ledger` client/helper exposure. Retain raw inspect/config/command output and hash-bound artifact bytes.
- [ ] Run focused source/runner/verifier tests and one bounded disposable no-egress control-phase probe, not a full E0 matrix. Save raw artifact, checkpoint/patch/report and independent review.

**Acceptance / failure handling:** Real producer can satisfy Fix6 helper boundaries on one control phase with private evidence ownership. Full E0 stays blocked until all fixed phases, OpenCode selected-provider/attempt provenance, route and runtime reconstruction pass in one pinned-image run.
