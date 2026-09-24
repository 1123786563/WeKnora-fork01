# Craft #107 T19 E0 Recorder Restart Mode Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the reviewed source recorder express the verifier's fixed same-client restart checkpoint without weakening phase or helper-start rules.

**Architecture:** A private `begin_phase(mode=restart)` command accepts exact host `restart_command_id` and `post_restart_inspect_id` only after both raw host operations have completed. The recorder appends a source-owned `phase_begin` with `phase_class=restart` and those refs, then a drained `phase_barrier`; no control helper_start is permitted in restart mode. The independent verifier checks the referenced host commands and their order/identity.

**Tech Stack:** Python source recorder and socket tests; existing Fix6 synthetic verifier.

**Spec:** Approved Craft #107 T19; `assert_v2.py` fixed schedule and restart contract; `2026-09-24-craft-107-t19-e0-recorder-phase-task1-review.md` scoped PASS with identified integration gap.

## Global Constraints

- Only `server_v2.py` and focused `test_source_owned_v2.py`; runner is separately owned. No verifier relaxation or fabricated raw Docker operation. No E0 release claim/commit/push.
- Host operation refs are nonempty unique strings and controller ordinal remains strictly ordered; restart mode cannot be used as a control or negative phase to launder network accepts.

## Review Focus

- Exact restart begin command/event schema matches verifier (`restart_command_id`, `post_restart_inspect_id`, operation ID/controller ordinal, phase class). Wrong/missing/duplicate refs reject before state mutation.
- Restart has no helper_start or accepted network traffic; barrier sees zero active connection/request. Existing control/negative phase tests still pass.

### Task 1: Restart phase source contract

**Depends on:** Recorder phase/seal identity independent PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/server_v2.py`, `test_source_owned_v2.py` only.

- [ ] Capture exact reviewed preimage. RED socket tests for valid restart and malformed/missing host refs, helper-start in restart, early TCP, duplicate/reordered operation identity.
- [ ] Implement narrow restart phase mode with exact fsynced event/ack. Keep prior phase/helper-start contracts.
- [ ] Run focused recorder and verifier synthetic tests, py_compile, exact pre/post incremental checkpoint/report/patch. Independent review before runner claims a full fixed schedule.

**Acceptance / failure handling:** Recorder can express the required restart boundary, but real Docker operation provenance and full E0 remain separately gated.
