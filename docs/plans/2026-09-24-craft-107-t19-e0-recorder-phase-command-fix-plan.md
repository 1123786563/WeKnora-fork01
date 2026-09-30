# Craft #107 T19 E0 Recorder Phase Command Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align source recorder phase/seal events with the existing Fix6 verifier's host operation ID and controller ordinal contract.

**Architecture:** Extend private `begin_phase`, `barrier` and `seal` control commands with host-supplied `operation_id` and `controller_ordinal`. Recorder validates strict increasing controller ordinals and unique operation IDs separately from source-local command ordinal, fsyncs both fields in its event, returns exact cursor and rejects mismatched/duplicate/reordered commands. `helper_start` retains its separate successful Docker-start operation ID and must lie between matching begin/barrier source events.

**Tech Stack:** Python Unix control socket recorder and focused source-owned tests.

**Spec:** Approved Craft #107 T19; Fix6 `assert_v2.py` event schemas/host command reconstruction; `2026-09-24-craft-107-t19-e0-source-helper-boundary-task1-review.md` PASS; runner source integration plan.

## Global Constraints

- Only `server_v2.py` and its focused tests; a separate worker owns `run_v2.py`. Agree on the command schema above before either edit. No verifier weakening or fabricated host evidence.
- The recorder owns seq/source/boot and fsync. Host command fields are observations matched by verifier to retained raw host operation records; client HTTP cannot write them.
- No E0 PASS until runner, pinned OpenCode provider/attempt provenance and full live raw matrix pass. No commit/push/stash; exact checkpoint.

## Review Focus

- `phase_begin`, `phase_barrier`, `stream_end` exact fields and ack cursor match verifier; controller ordinal is monotonic and operation ID unique across this source.
- Wrong/duplicate/out-of-order commands, phase mismatch, active connection, early traffic and append failure fail closed without a usable ack.

### Task 1: Recorder phase/seal identity

**Depends on:** Source helper boundary independent PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/server_v2.py`, `test_source_owned_v2.py` only. No runner/verifier/production edits.

**Consumes / produces:** Host-issued phase/seal command identity; produces source-owned exact event and cursor for independent verifier reconstruction.

- [ ] Capture preimage. RED socket tests for complete valid sequence and duplicate/missing/wrong operation ID/controller ordinal, source ordinal drift, helper-start ordering, failed append.
- [ ] Implement narrow schema/validation/events; retain preexisting helper-start and TCP poisoning behavior.
- [ ] Run source-owned focused and verifier synthetic tests, py_compile, exact patch reconstruction/checkpoint/report, independent review.

**Acceptance / failure handling:** Recorder phase boundary events can be joined to actual runner host commands once that separate producer is reviewed; this scoped Task alone cannot credit E0.
