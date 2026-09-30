# T14 Candidate Layer Derivation Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the candidate derivation helper's unknown-outcome cleanup gaps and add behavioral tests for success, daemon-side creation followed by client timeout, and cleanup failure.

**Architecture:** Give containers a unique, known name before `docker create`, so cleanup can reconcile by name if the client times out before receiving the ID. Keep commits untagged to preserve the acceptance contract; if `docker commit` returns no ID after a timeout, perform bounded inventory of dangling images and identify a candidate by exact predecessor layer ancestry, runtime config, and final probe hash. Remove it only by verified full ID. If inventory cannot prove absence or ownership within the bounded cleanup window, emit an explicit `unknown_outcome` with recovery identity and exit failure; never claim cleanup or retain a candidate for use. Separate validated-candidate retention from cleanup success, and add a fake-Docker behavioral harness exercising complete success and failure branches.

**Tech Stack:** POSIX shell, Python 3, `unittest`, fake Docker CLI in temporary test directories.

**Spec:** T14/#129 acceptance in `docs/specs/2026-09-23-craft-web-artifact-spec.md`; prior implementation/review/validation in `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-{report,review,validation}.md`.

## Global Constraints

- Keep predecessor/source/report/overlay pins exact; never accept caller-provided image identities.
- Never start a derivation or verification container; preserve single-file `/opt/probe.py` diff and volume cleanup.
- Successful output requires complete image/hash/config/layer verification and successful cleanup of both containers.
- On uncertain daemon outcome, do not reuse the candidate and do not claim verified cleanup; return a distinct failure state with reconciliation instructions.
- Keep final successful candidate untagged, volume-free, linux/arm64, and config-compatible with the predecessor.
- No Docker/browser/network/issue/commit/stage action during implementation, validation, or Review.

## Review Focus

- Create timeout after daemon-side success: the unique container name permits bounded full-ID reconciliation and safe cleanup.
- Commit timeout after daemon-side success: untagged candidate inventory must match predecessor layer prefix plus exactly one layer, config, platform, and embedded probe hash; ambiguity remains unknown and fails closed.
- Cleanup failure: candidate retention is allowed only if the full verification succeeded and both temporary containers were removed; any cleanup failure must attempt candidate removal and return failure.
- Behavioral tests: a fake Docker executable must exercise one complete success, create timeout with object creation but no client ID, commit timeout with untagged image creation but no client ID, and cleanup failure. No network or real Docker is used by tests.
- Report integrity: state the shared 45-second total cleanup bound accurately and capture final helper/test/report hashes.

## Task 1: Reconcile daemon outcomes and behavior-test the derivation lifecycle

**Depends on:** initial candidate-layer derivation checkpoint and review report; all three findings `T14-LAYER-R1`, `T14-LAYER-R2`, `T14-LAYER-R3`.

**Owner role:** `backend_implementer`; **validator:** `backend_validator`; **reviewer:** parent-dispatched read-only `reviewer`.

**Files owned:**

- Modify: `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh`
- Modify: `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py`
- Modify: `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md`

**Consumes:** predecessor report SHA, final probe SHA, review report with two high/one medium/one low findings.

**Produces:** known-name and inventory reconciliation, success-only candidate retention, fake-Docker end-to-end tests, corrected report.

1. Add behavioral tests before changing the helper. Fake Docker must expose deterministic source/predecessor/candidate JSON, tar streams with pinned source files and tar metadata, layer ancestry, `docker diff`, commit, and cleanup. Capture RED failures for no successful end-to-end execution, no timeout reconciliation, and cleanup errors.
2. Add unique derivation and verification container names before `docker create`; on command timeout, inspect the name and require full ID/image/state match before removal.
3. Keep `docker commit` untagged. On timeout without ID, boundedly inventory dangling images and filter by exact predecessor rootfs layer prefix + one layer, config, platform, and probe digest. Remove only one uniquely matched, full-ID-verified image; ambiguous or absent-at-deadline outcomes remain explicitly `unknown_outcome` and fail.
4. Refactor lifecycle state so a candidate is retained only after all invariants pass and both containers clean successfully. On any validation or cleanup failure, attempt removal of the candidate by confirmed ID; keep exit failure if cleanup is unverified.
5. Correct the report's cleanup-bound sentence, update it to include behavioral test results and final hashes, and avoid asserting guaranteed cleanup for an unresolved daemon timeout.
6. Run fake-Docker suite, py_compile, `sh -n`, static invariant checks, `git diff --check`, and whitespace scan; do not call Docker.

**Verification:** `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_derive_volume_free_diagnostics_candidate.py' -v`; compile test; shell syntax; static checks; whitespace.

**Acceptance:** tests cover success, timeout-after-create, timeout-after-commit, and cleanup failure; unknown daemon outcomes fail closed and require operator reconciliation; no candidate is announced usable before cleanup success; all pins and behavior constraints remain exact.

**Failure handling:** preserve original failing outputs. If the fake harness cannot model a required Docker lifecycle behavior without running Docker, state the uncovered risk; do not weaken invariants or proceed to a live candidate.
