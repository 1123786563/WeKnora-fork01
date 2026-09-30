# Craft #107 T19 E0 Evidence Fix 6 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close two independently reproduced synthetic false-PASS paths while documenting the source-stream helper-start boundary required for any measured E0 result.

**Architecture:** Require an exact helper mount inventory disjoint from both recorder volumes, and bind every fixed case to its verifier-owned phase and bounded begin/case/barrier ordinal. A helper-start boundary must be source-owned, not inferred from controller log order; until the runner/recorder produces it, live proof remains BLOCKED.

**Tech Stack:** Python strict verifier, adversarial synthetic fixture.

**Spec:** `docs/plans/2026-09-24-craft-107-t19-e0-fix5-task1-review.md`; approved Craft #107 E0/E1 trust boundary.

## Global Constraints

- No runner, Docker or production edits in this narrow task. No synthetic PASS may be presented as measured E0 PASS.
- Reject missing/extra/aliased mounts and null, misplaced, duplicate or cross-phase case rows even if hashes and manifest agree.
- Do not manufacture helper-start/source-event order from independent logs. Define required source-owned cursor/ack contract and return BLOCKED when absent.
- No commit/push; exact uncommitted checkpoint and independent review.

## Review Focus

- Helper mounts D's exact writable volume under a different destination: reject.
- Direct-IP fixed curl with phase_id null and ordinal before control: reject.
- Source `tcp_accept` before helper-start boundary, despite valid phase begin/barrier: reject or explicit BLOCKED; no false synthetic/measured PASS.

---

### Task 1: Repair mount and phase binding; specify source boundary

**Depends on:** Fix5 Task1 independent review FAIL. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/assert_v2.py`, `test_assert_v2.py`, `test_source_owned_v2.py` only.

**Consumes / produces:** Fix5 raw inspect/host timeline; produces strict helper-volume/phase checks and a fail-closed helper-start source boundary contract.

- [ ] Capture three-file preimage. RED reproduce reviewer F1 and F2 using self-consistent copies with all raw hashes recomputed. Add F3 pre-helper traffic case; it must not PASS when source-owned helper-start cursor/ack is absent.
- [ ] Require exact approved helper mount inventory and compare source/device/name/ancestor/alias against D and A recorder volume identities. Require each scheduled fixed case's exact verifier-owned phase and begin < case < barrier; define preflight/fault/cleanup positions explicitly.
- [ ] Require a source-owned helper-start cursor/ack tied to controller operation ID and source event sequence, or return BLOCKED when evidence lacks it. Document runner/recorder schema change; do not infer from timestamps or manifest.
- [ ] Run verifier mutation and source-owned suites, py_compile, whitespace, exact patch reconstruction; save report/checkpoint for independent review.

**Acceptance / failure handling:** F1/F2 false-PASS examples reject. F3 either rejects with demonstrated source boundary or remains a precise producer block. E0 remains BLOCKED until runner, raw runtime/route facts and independent pinned OpenCode provenance all exist.
