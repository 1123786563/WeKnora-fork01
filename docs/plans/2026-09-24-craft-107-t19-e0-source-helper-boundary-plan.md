# Craft #107 T19 E0 Source Helper Boundary Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the recorder itself acknowledge the host helper start after the successful Docker operation and before any control traffic, closing the exact Fix6 producer gap without claiming live E0 PASS.

**Architecture:** Extend the existing private Unix control socket with a `helper_start` command. It carries the active control phase, successful host Docker-start operation ID and controller ordinal. The source recorder appends/fsyncs one event under its event lock and returns its cursor. Only a drained control phase accepts it; `tcp_accept` before the boundary poisons or blocks the control phase. Both D and A streams must independently acknowledge the same host operation. A later runner task issues the command after observing actual helper start; this Task does not forge the host operation.

**Tech Stack:** Python source recorder and socket tests.

**Spec:** Approved Craft #107 T19; `2026-09-24-craft-107-t19-e0-evidence-architecture.md`; Fix6 report/review and verifier-required `helper_start` schema.

## Global Constraints

- Source-owned events, contiguous sequence, boot/source/run identity, fsync and fatal append errors remain intact. Client HTTP/header/body cannot invoke the private control command.
- Require exact active control phase and fresh ordinal/operation identity; no event outside control phase, duplicate or after traffic. Keep existing begin/barrier/seal semantics.
- No runner, verifier or production gateway edit in this task; synthetic tests cannot release E0. No commit/push/shared stash; record exact checkpoint.

## Review Focus

- Before helper acknowledgment, control traffic cannot be accepted as a valid phase. A duplicate or wrong-phase command cannot create a second event.
- Source event fields match Fix6 verifier exactly: `event`, `phase_id`, `boundary`, `operation_id`, `controller_ordinal`, drained active counts, plus recorder-owned seq/source/boot/time.
- D and A are independent writers; host operation ID is an observation matched later by verifier, not caller-supplied HTTP authority.

### Task 1: Recorder helper-start command

**Depends on:** Fix6 verifier scoped PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/server_v2.py`, `test_source_owned_v2.py` only. No `run_v2.py`, `assert_v2.py` or production files.

**Consumes / produces:** Host private control command and current control phase; produces one acknowledged source-owned helper-start event per source/control phase.

- [ ] Capture preimage. RED socket tests for normal, missing, duplicate, wrong phase/ordinal/op ID, early TCP, active connection, append/fsync failure and seal; exercise both D/A recorder identities.
- [ ] Implement narrow command schema and state checks with one fsynced append under recorder lock. Preserve cross-thread safety and error poisoning. Refuse/mark invalid any control `tcp_accept` before boundary so a later acknowledgment cannot launder early traffic.
- [ ] Run focused recorder and existing verifier synthetic tests, Python compile and exact patch reconstruction. Save report/checkpoint; independent review with separate Spec/quality verdict.

**Acceptance / failure handling:** Recorder can emit the verifier's required source boundary only after a host command; E0 remains BLOCKED until runner produces a real operation log, paired commands, isolated volumes and OpenCode provenance.
