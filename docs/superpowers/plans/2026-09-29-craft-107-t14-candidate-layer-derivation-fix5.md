# T14 Candidate Derivation Fix 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reconcile the pinned predecessor's actual `/opt/probe.py` owner metadata with the derivation helper while preserving root ownership for the newly copied candidate probe.

**Architecture:** Keep source/predecessor IDs, report digest, content hashes, and all image invariants pinned. Record the exact immutable predecessor probe UID/GID/mode observed from a never-started container/rootfs export; validate that metadata before modification. After the helper copies the approved probe into the container, continue requiring root:root mode 0644 for the resulting candidate. This changes only the incorrect predecessor metadata assumption and its test fixture.

**Tech Stack:** POSIX shell, Docker CLI (for later parent acceptance only), Python 3 unittest fake-Docker harness.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`, T14/#129. Prior task plan and evidence: Fix4 plan/report/review/validation; live failed derivation at `docs/testing/craft/t14/2026-09-29-candidate-layer-fix4/derivation.log`.

## Global Constraints

- Preserve the exact source ID `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27`, predecessor image ID `sha256:c115e66adc210e483f72525f62c544f17d8b5e9808acb1ad3e21da9b85650894`, predecessor report SHA, and six overlay content hashes.
- Predecessor is read from a never-started container; no command in this task may start it or run a browser/network probe.
- Before mutation, verify the predecessor probe digest, actual owner UID/GID, and mode against constants specifically bound to that immutable predecessor ID. The live export observed `opt/probe.py` UID 501, GID 20, mode 0644.
- After copying the approved probe, require the candidate `/opt/probe.py` to be UID 0, GID 0, mode 0644; preserve all other pinned image, layer, config, tag, volume, and cleanup checks.
- No new image may be created during implementation/tests; no Docker calls, network, staging, or commits by the implementer.

## Review Focus

- A changed predecessor digest must not silently inherit UID/GID 501:20; the metadata check is meaningful only together with the immutable pinned image ID and source report digest.
- The fake Docker fixture must represent the exact predecessor archive UID/GID/mode and prove mismatch fails before `docker cp` mutation or commit.
- The copied candidate file must still be root:root 0644; relaxing predecessor validation must not relax output validation.

---

### Task 1: Pin predecessor metadata and preserve candidate ownership checks

**Files:**
- Modify: `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh`
- Test: `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py`
- Create: `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-fix5-report.md`

**Depends on:** Fix4 task-level Review and validation; live evidence in the Fix4 derivation log and parent diagnostic report.

**Interfaces:**
- Consumes: pinned predecessor ID/report SHA and observed `opt/probe.py` `(uid=501,gid=20,mode=0644)` from `docker export` of the never-started exact predecessor.
- Produces: pre-mutation validation pinned to that predecessor metadata; copied output remains `(uid=0,gid=0,mode=0644)`.

- [ ] Add failing fake-Docker tests where predecessor tar metadata is 501:20/0644 and accepted, mismatched predecessor metadata is rejected before the copy/commit mutation, and candidate tar metadata still must be 0:0/0644.
- [ ] Run focused tests and capture RED against the existing root:root predecessor assumption.
- [ ] Add predecessor UID/GID constants and validate the exact metadata together with the pinned image and content identities; retain the existing root-owned candidate checks unchanged.
- [ ] Run the full fake-Docker derivation test suite, shell syntax, Python compilation, static invariant and whitespace checks. No Docker command.
- [ ] Save report with exact before/after hashes, RED/GREEN output, test commands/results, and pointer to the prior live failure log.

**Expected:** Fake predecessor archive with the evidence-backed 501:20/0644 metadata passes preflight; any other predecessor owner/mode or digest fails before container mutation; candidate output still requires root:root 0644.

**Failure handling:** If Docker archive metadata cannot distinguish container UID/GID from host tar ownership, stop and report evidence; do not infer values or loosen validation.
