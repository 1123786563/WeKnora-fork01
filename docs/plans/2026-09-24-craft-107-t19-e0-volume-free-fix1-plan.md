# Craft #107 T19 E0 Volume-Free Fixture Fix1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close VF-1/VF-2 in `2026-09-24-craft-107-t19-e0-volume-free-task1-review.md` before runner adoption.

**Architecture:** Retain exact provisioner command, exit and post-removal absence from a new bounded disposable probe. Strengthen topology tests to invoke the unchanged Fix6 verifier predicate for D/A source equality and ancestor isolation, alongside existing image/UID/mount checks. Do not infer these facts from volume names or summary text.

**Tech Stack:** Python fixture topology tests, Docker inspect, local Linux/arm64 derivative image.

**Spec:** Craft #107 T19 E0 RI-1 volume-free Task1 and independent review VF-1/VF-2.

## Global Constraints

- Preserve derivative Dockerfile/base image/binary/recorder identity and unchanged verifier. No runner, provider, production or paid egress edits. Docker tests run in exclusive resource window with finite timeout and exact cleanup. No commit.

## Review Focus

- Source-owned raw command/exit plus Docker absence prove provisioner completed and was removed. D/A source paths satisfy exact Fix6 verifier equality and ancestor isolation, with a negative alias/ancestor control.

### Task 1: Provisioner and source-isolation evidence

**Depends on:** volume-free Task1 independent review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** focused volume-free topology test/evidence under `docs/testing/craft/egress-probe/`; Dockerfile only if evidence reveals actual config defect. No `run_v2.py`/verifier/recorder edits.

- [ ] RED test: bare volume/container names lack required provisioner argv/exit/removal proof; source alias or ancestor overlap rejected by unchanged Fix6 predicate.
- [ ] Capture exact provisioning operation and exit, inspect source mounts and removal/absence; add direct unchanged verifier assertions and negative controls.
- [ ] Run bounded exclusive Docker topology suite, hash raw evidence, exact incremental checkpoint/report and independent re-review. If provisioning removal cannot be proven, keep image Task1 unverified.

**Acceptance / failure handling:** VF-1/VF-2 both close on raw evidence and test enforcement; E0 full matrix remains separate.
