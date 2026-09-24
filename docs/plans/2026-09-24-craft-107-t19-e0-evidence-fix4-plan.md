# Craft #107 T19 E0 Evidence Fix 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject malformed or partial HTTP traffic recorded after an expected control request on the same keep-alive direct connection.

**Architecture:** Treat every direct-sink request, parse error, TCP accept and close as an observation that must match the verifier-owned control window. A valid expected request cannot whitelist a later malformed request on the same connection.

**Tech Stack:** Python 3, E0 source-owned JSONL streams and strict verifier tests.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task1-review.md`; `docs/plans/2026-09-24-craft-107-t19-e0-evidence-fix3-plan.md` Task1.

## Global Constraints

- E0 and model egress stay blocked/default-off; no runner, production or Docker edits.
- Every accepted direct connection/event is accounted for. A client header, phase label or successful earlier control cannot authorize malformed later traffic.
- Integration Worktree, no commit; exact pre/post file hashes, focused tests and independent review.

## Review Focus

- Expected GET followed by malformed bytes on one keep-alive connection fails.
- Expected CONNECT followed by partial request or parse error fails.
- Parse errors in negative or outside-control windows fail globally.
- A recorded error cannot be hidden behind a later valid request/close.
- All normal expected controls continue to pass with valid terminal/close sequence.

---

### Task 1: Count direct parse errors as unmatched sink traffic

**Depends on:** E0 Fix3 Task1 reviewed FAIL. **Owner:** `backend_implementer`; **validator:** `backend_validator`, then independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/assert_v2.py`, `test_assert_v2.py`, `test_source_owned_v2.py` only; `server_v2.py` read-only unless the reviewer mutation cannot be represented by its current schema.

**Consumes / produces:** Current per-connection ordered request/parse state; produces a strict window decision over every raw direct event.

- [ ] Capture preimage. RED reproduce the exact reviewer mutation on the full synthetic fixture, refreshing sequence/cursors/hashes: expected control request then same-connection parse_error; add CONNECT/partial, negative-window, outside-phase and later-valid request variants.
- [ ] Make `_verify_direct_phase` and global direct stream accounting reject any parse/error observation not in a fixed explicitly allowed control expectation (none for prescribed positive controls). Preserve request ordinal and terminal/close checks.
- [ ] Run focused tests, compile/diff checks; save exact checkpoint/report/patch. Independent reviewer must close this false PASS before Fix3 Task2 fixed host/control verification is released.

**Acceptance / failure handling:** No malformed/partial direct traffic can coexist with PASS, even on an otherwise expected connection. E0 remains BLOCKED until later control/host verifier and live matrix gates.
