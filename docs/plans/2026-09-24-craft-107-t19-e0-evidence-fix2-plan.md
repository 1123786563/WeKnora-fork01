# Craft #107 T19 E0 Evidence Ownership Fix 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the disposable E0 egress experiment reject every unowned sink event and independently reconstruct a bounded full matrix from raw source-owned records.

**Architecture:** Direct sink D and adapter A each own one contiguous event stream and receive phase commands only from a host controller Unix socket. Client C cannot write recorder state or artifacts, and is fully stopped during positive controls. A single strict verifier reconstructs topology, phases, network events, commands, config and cleanup from raw records; manifest claims serve only as cross-checks.

**Tech Stack:** Python 3, Docker CLI/Engine, pinned local OpenCode image, unittest/pytest as available.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-e0-evidence-architecture.md`; approved Craft web Artifact Spec #107 T19; `docs/plans/2026-09-24-craft-107-t19-e0-fix1-review.md`.

## Global Constraints

- No real provider keys, paid egress, production routing, host firewall change, privileged container or shared database. Model egress remains default-off and E1 gated.
- C is on private P only; A bridges P/X with forwarding disabled; D and a named helper are on X only. Pin exact image and retain raw Docker inspection before/after restart.
- D and A must have exclusive write ownership of distinct event roots; C cannot write case/phase state or artifact roots and cannot read a broad `/probe` bind. Case headers and client output are untrusted observations.
- All accepted sockets, partial bodies, responses and errors are recorded with contiguous source-owned sequence; recorder append failures poison the run. No `manifest` summary may substitute for raw event/command/config/inspect evidence.
- Total monotonic budget is 20 minutes including 120 seconds reserved cleanup; no repeat hidden in one artifact. One full disposable run follows independent code/test review, then independent evidence review.
- Integration Worktree, no commit/push, no production source edits. Save task-local pre/post hashes, patch, actual commands and output.

## Review Focus

- Untagged, forged-control, malformed, CONNECT and TCP-only direct sink arrivals must fail globally.
- A client-writable state/mount or A write path into D records must fail raw topology verification.
- Missing controls, shifted/corrupt source sequence, missing seal or unclosed connection must fail even if all summaries say PASS.
- Timed-out or incomplete normal/hostile turns, absent `probe-ok`, missing config-load/network evidence and absent proxy pre/post cases must fail.
- An incomplete stop/drain or 20-minute watchdog expiration must leave E0 BLOCKED with intact raw evidence and cleanup record.

---

### Task 1: Source-owned recorder and strict verifier

**Depends on:** E0 architecture audit. **Owner / validator:** `backend_implementer` / `backend_validator`, followed by independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/server_v2.py`, `assert_v2.py`, `test_assert_v2.py`; may add a focused recorder test in the same directory. `run_v2.py` remains read-only until Task 2.

**Consumes / produces:** Audit §“Recorder interface and raw schema” and §“Independent verifier contract”; produces exclusive D/A streams and one verifier schema that rejects synthetic/live false accepts equally.

- [ ] Capture preimage and RED reproduce six audit false accepts plus untagged GET/CONNECT/TCP accept, removed/shifted event, forged control, missing normal output and timeout.
- [ ] Implement fixed source identity, contiguous event/connection IDs, `stream_start`, source-owned phase control/ack/barrier/seal via private Unix socket, fsynced records and fatal append-error handling. Record accept before handling and request start before body read; record response only after successful send/flush and every close/error.
- [ ] Implement strict parse, stream/connection state machine, host-acknowledged phase reconstruction, whole-stream event accounting, fixed required case set including proxy pre/post, and raw command/config/inspect/cleanup verification. Never let a request header or manifest field grant a control exception.
- [ ] Use one full chronological synthetic fixture under the same verifier; add targeted mutations listed in architecture audit. Run RED/GREEN tests and syntax checks, retain logs/checkpoint/patch/report. Stop if verifier cannot reconstruct a required fact; do not fabricate source evidence.

**Acceptance / failure handling:** All reproduced false accepts now fail for the intended raw-evidence reason; valid complete fixture passes; partial socket/append failure cannot seal PASS. Independent review must pass Spec and quality before runner work.

### Task 2: Host-only phase orchestration and isolated runner

**Depends on:** Task 1 independently reviewed. **Owner / validator:** `backend_implementer` / `backend_validator`, followed by independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/run_v2.py` and focused runner tests in the same directory; Task 1 source only if a reviewed interface correction is explicitly replanned.

**Consumes / produces:** Task 1 recorder/control API; produces an inspected, bounded, source-owned E0 artifact with exact positive and negative windows.

- [ ] RED tests for writable `/state`, broad `/probe`, helper/client overlap, incomplete stop, active socket at barrier, timer budget overrun and manifest/raw disagreement.
- [ ] Remove C write access to recorder control/artifact roots. Give D/A distinct private roots; copy exact config to C. Use host-only phase socket, named helper, sequential pre-control → stopped C → negative C → stopped C → post-control schedule; record all Docker commands/IDs and prove C `Running=false`, `Restarting=false`, `Pid=0` before controls.
- [ ] Enforce all-event zero-connection drain and monotonic 18-minute execution plus 120-second cleanup reserve with external watchdog. Retain raw stdout/stderr/exit/timeouts, config bytes/in-container hash/process trace, pre/post inspect, source seals, export hashes, and exact cleanup/not-found observations.
- [ ] Run focused tests, checkpoint and independent review. Then run **one** bounded disposable Docker matrix, cheap gates first and full prescribed cases only if gates pass. Preserve raw artifact on every nonzero result and run the strict verifier on that same artifact. Report measured retry behavior separately; do not grant E1 from infrastructure proof alone.

**Acceptance / failure handling:** Fixture checkpoint reviewed, one full live artifact independently reconstructable within deadline, no unmatched direct event, all prescribed controls/negative/normal/hostile/restart/proxy cases evidenced. Missing platform or OpenCode trace remains BLOCKED with precise cause. Cleanup failure remains BLOCKED, not PASS.
