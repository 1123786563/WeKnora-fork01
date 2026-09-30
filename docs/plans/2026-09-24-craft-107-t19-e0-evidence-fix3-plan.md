# Craft #107 T19 E0 Evidence Fix 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the five independent Task 1 review findings before adapting the E0 runner or attempting another Docker matrix.

**Architecture:** First make recorder sealing and per-connection request accounting complete. Then make the verifier's control schedule and host facts derive from fixed expectations plus raw process/inspect evidence, rather than self-consistent phase/manifest claims.

**Tech Stack:** Python 3, HTTP/1.1 recorder, Unix phase socket, strict JSONL verifier and focused synthetic mutations.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-e0-fix2-task1-review.md`; `docs/plans/2026-09-24-craft-107-t19-e0-evidence-architecture.md`; approved Craft Spec #107/T19.

## Global Constraints

- E0 remains BLOCKED, E1 and model egress gated; no real keys, paid calls, production edits, Docker run or false PASS from synthetic source.
- Direct D and adapter A each own an exclusive contiguous stream. Client headers/body/mounts and manifest summaries carry no authority to omit an event or declare a control.
- Raw accepted sockets, every HTTP/1.1 request, body and terminal outcome, and all closures must be accounted for through a final seal after listener shutdown.
- Integration Worktree; no runner edits until both tasks independently pass. Save exact pre/post hashes, patch, command outputs and review.

## Review Focus

- Two keep-alive requests on one D connection, first bypass and second expected control, must fail.
- A seal racing a new accept must close/join the listener first or poison the artifact.
- Missing or duplicate request terminal events and wrong ordinal/ID must fail.
- Missing pre/post control, forged stopped flag, wrong helper peer or C still running must fail despite matching phase labels.
- Self-consistent false config, topology, restart or cleanup summaries must fail against raw commands/inspects.

---

### Task 1: Complete recorder lifecycle and every-request accounting

**Depends on:** E0 Fix2 Task1 reviewed FAIL. **Owner:** `backend_implementer`. **Validator:** `backend_validator` plus independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/server_v2.py`, `assert_v2.py`, `test_assert_v2.py`, `test_source_owned_v2.py`.

**Consumes / produces:** Existing source streams and phase control; produces exhaustive ordered request state per connection and a stream_end that bounds all accepts.

- [ ] Capture preimage. RED real recorder and raw-artifact tests for two HTTP/1.1 requests on one connection (first unapproved GET/CONNECT), duplicate/wrong request IDs or ordinal, missing/double terminal, response-then-error, and concurrent accept during seal.
- [ ] Track every request as a separate ordered observation under its accept connection; verify all requests/errors against fixed expectations. Enforce exactly one terminal and one close per accepted connection. Stop/join listeners and drain before fsync stream_end; poison on shutdown/append/late accept error.
- [ ] Run focused tests, Python compile, diff check, save exact checkpoint/report/patch; independent review must close F1/F4/F5 before Task 2.

**Acceptance / failure handling:** No raw event is overwritten or dropped; no post-seal unrecorded accept; malformed/partial HTTP remains visible and fail-closed. Still no E0 PASS or runner compatibility.

### Task 2: Fixed control schedule and independently proved host facts

**Depends on:** Task 1 independently reviewed PASS. **Owner:** `backend_implementer`. **Validator:** `backend_validator` plus independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/assert_v2.py`, `test_assert_v2.py`, `test_source_owned_v2.py` only; runner remains read-only.

**Consumes / produces:** Task 1 complete streams and host command/snapshot artifact schema; produces verifier-owned full case/control schedule, actual C-stop/helper proof, fixed argv/config/topology/restart/cleanup reconstruction.

- [ ] RED mutations on a full chronological artifact: omit each required pre/post control; forge `client_stopped` or helper ID/peer; change a required argv/target/env; alter raw client/A/D/helper/network inspect, mount ancestor alias, route/sysctl, restart identity, config-load/hash/attempt trace, successful removal or exact not-found result while summaries remain PASS.
- [ ] Encode fixed per-case and paired-control sequence in verifier code, not manifest. Require host stop/wait/inspect for the exact C ID before controls, helper start/identity/peer evidence, and zero active connections/barrier at transitions.
- [ ] Reconstruct command/config success, selected URL and attempted network failure from retained raw rows. Validate all network membership, mounts, privilege/caps, sysctls/routes, image/UID and restart identity from raw Docker inspect plus successful commands. Derive cleanup inventory from creation records, require seals before removal, exact removals and specific not-found observations.
- [ ] Run focused tests, Python compile and diff check; checkpoint and independent review. If OpenCode does not expose selected URL/load provenance, report exact missing seam as BLOCKED rather than accept a free-text trace.

**Acceptance / failure handling:** F2/F3 closed and one full synthetic fixture passes the same strict verifier used for live artifacts. Only then can the original Fix2 Task 2 runner adapt to this schema and attempt one bounded full Docker run.
