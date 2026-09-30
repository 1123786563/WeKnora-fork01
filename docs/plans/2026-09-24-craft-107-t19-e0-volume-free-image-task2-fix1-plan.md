# Craft #107 T19 E0 Volume-Free Runner Fix1 Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close VI-1/VI-2 from `2026-09-24-craft-107-t19-e0-volume-free-image-task2-review.md` so the reviewed derivative runner and fixed full verifier agree on one exact image and C volume names.

**Architecture:** The verifier pins the single independently reviewed derivative image ID, continuing to reject the old base and arbitrary manifest values. The runner writes its exact run-owned `config`/`data` names into the manifest before C launch, and its focused test exercises the unchanged verifier topology predicate using representative raw C inspect. Two tasks own disjoint files and can execute concurrently; final integration test starts after both checkpoints are reviewed.

**Tech Stack:** Python E0 fixture runner/verifier and synthetic mutation suite.

**Spec:** Approved Craft #107 T19 E0, volume-free Task1+VF Fix1 independent PASS, runner Task2 independent VI-1/VI-2 review.

## Global Constraints

- Keep exact Linux/arm64 derivative ID `sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615`, OpenCode binary SHA, source recorder SHA, UID/mount/source-isolation and raw participant identity checks. No arbitrary manifest trust or acceptance of old base ID.
- Preserve full E0 BLOCKED until physical attempt/provider provenance and network matrix pass. No paid egress, commit, push or production edit. Docker not needed for Tasks1–2; evidence already contains representative raw inspect.

## Review Focus

- Full verifier rejects old and arbitrary image IDs, accepts only reviewed derivative when all other evidence is valid.
- Runner manifest `names.config/data` exactly equals created/mounted C volume names; unchanged source/destination topology predicate passes representative inspect and rejects tampered names.

### Task 1: Verifier derivative pin

**Depends on:** runner Task2 review VI-1. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/assert_v2.py` and focused verifier tests only. No runner/recorder/Dockerfile edits.

- [ ] RED mutation: reviewed derivative manifest rejected under old hardcoded base; old base and arbitrary ID must fail after fix.
- [ ] Change exact image pin to reviewed derivative, retaining participant identity/platform/binary/source/topology checks.
- [ ] Run focused/synthetic mutation suite, checkpoint and independent review.

**Acceptance / failure handling:** One exact derivative image is accepted; no broadening to manifest-supplied IDs.

### Task 2: Runner full manifest C volume names

**Depends on:** runner Task2 review VI-2. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/run_v2.py` and focused runner tests only. No verifier/recorder/Dockerfile edits.

- [ ] RED representative raw C inspect through existing verifier topology predicate with missing manifest names.
- [ ] Populate exact run-owned `config`/`data` names before creation/manifest save, and enforce the two-volume predicate in focused tests.
- [ ] Run focused runner tests, checkpoint and independent review.

**Acceptance / failure handling:** Manifest and real C inspect join by exact source-owned names; no generic aliases.

### Task 3: Synthetic full-verifier integration

**Depends on:** Tasks1–2 independent PASS. **Owner:** `backend_validator`; **validator:** independent `reviewer`.

**Owned files:** evidence/report only.

- [ ] Replay retained raw derivative C/D/A/helper inspect through the aligned verifier with representative manifest; mutate old/wrong image and C names as negative controls.
- [ ] Report only scoped integration PASS; retain E0 BLOCKED pending full physical matrix.
