# Craft #107 T19 E0 Recorder Restart Fix1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close RR-1 from `2026-09-24-craft-107-t19-e0-recorder-restart-task1-review.md` so restart host reference IDs cannot be reused by later recorder operations.

**Architecture:** One recorder-owned uniqueness registry covers accepted restart references and all later operation IDs. Validate before append/ack under the existing command serialization lock; duplicate IDs fail without changing the sealed timeline. Retain strict verifier checks as a separate final gate.

**Tech Stack:** Python private-socket recorder and focused tests.

**Spec:** Approved Craft #107 T19/E0; recorder restart-mode Task1 plan and independent review RR-1.

## Global Constraints

- Preserve the private recorder socket, source-owned provenance, begin/barrier/seal phase sequence and exact patch checkpoint. No runner, verifier, manifest, Docker or production code edits.
- No synthetic successful E0 claim; full pinned matrix remains gated. No commit/push.

## Review Focus

- Reusing any accepted restart reference as a later barrier/operation ID is rejected before append and private-socket ack. Same-name phase IDs and legal distinct IDs still behave as before.

### Task 1: Shared restart-reference uniqueness

**Depends on:** Recorder restart-mode Task1 review RR-1. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** Recorder source and focused recorder test under `deploy/craft/model-egress/` only.

- [ ] Save preimage and RED private-socket regression using a restart reference ID as a later barrier operation ID; assert no append and rejection ack.
- [ ] Add minimal shared uniqueness check before state append; preserve legal distinct-ID path.
- [ ] Run focused recorder and synthetic verifier checks, exact checkpoint and report; independent re-review of RR-1 and quality.

**Acceptance / failure handling:** Any reused restart reference returns an explicit error and leaves the timeline unchanged; otherwise report failing exact case without broadening scope.
