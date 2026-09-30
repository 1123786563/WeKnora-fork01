# T14 Candidate Layer Derivation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide a bounded, auditable way to derive the final trace candidate from the already verified volume-free candidate when full export/import exceeds its 900-second bound.

**Architecture:** Pin the only accepted prior candidate ID and its immutable source-derivation report digest. Create a never-started container from that prior candidate, verify all six predecessor overlay hashes, replace only `/opt/probe.py`, require `docker diff` to show exactly that path, and commit one new image layer. Verify layer ancestry, config, platform, tags, volumes, probe ownership/mode, and all six embedded hashes before retaining the new candidate. This creates an explicit provenance chain from the original source ID; it does not claim the new candidate metadata independently encodes that source.

**Tech Stack:** POSIX shell, Docker CLI, Python 3, `unittest`.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`, T14/#129 evidence acceptance. Failure context: `docs/plans/2026-09-29-craft-107-t14-diagnostic-candidate-build-fix1-failure.md`; prior candidate provenance: `docs/plans/2026-09-29-craft-107-t14-current-source-candidate-live-attempt.md`.

## Global Constraints

- Preserve exact source ID `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27` provenance through the hash-pinned predecessor report and candidate `sha256:c115e66adc210e483f72525f62c544f17d8b5e9808acb1ad3e21da9b85650894`.
- Require a stopped, never-started container and remove it with `docker rm -v` on every exit path.
- The only filesystem delta allowed in the new container is `/opt/probe.py`; verify with `docker diff` and one extra image layer.
- Preserve linux/arm64, exact config, no repo tags, no Volumes, all six overlay hashes, and expected root ownership/mode for `/opt/probe.py`.
- Any mismatch fails closed and removes the newly created candidate image; do not fall back to an unverified candidate or run a browser.
- No timeout, policy, receipt, browser, source identity, or acceptance semantics changes.

## Review Focus

- Predecessor chain: reject arbitrary or altered base candidates; bind to exact immutable evidence digest and source ID.
- Container lifecycle: never start the derivation container; prove `.State.Status == created`; always clean it with volumes.
- Single-file delta: require `docker diff` exact path set and rootfs layer ancestry to equal prior layers plus one.
- File identity and metadata: compare final source hash, expected root ownership and mode, and all five unchanged overlay hashes.
- Candidate config: preserve platform/config, no tags and no volumes; on any failed check remove the candidate.

## Task 1: Implement the bounded exact-predecessor derivation helper

**Depends on:** T14 trace Fix1 checkpoint; architecture audit in current task history; verified prior candidate report.

**Owner role:** `backend_implementer`; **validator:** `backend_validator`; **reviewer:** parent-dispatched `reviewer`.

**Files owned:**

- Create: `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh`
- Create: `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py`
- Create: `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md`

**Consumes:** prior candidate ID + exact-source builder report (SHA pinned); current six overlay hashes; final `probe.py` SHA from trace Fix1.

**Produces:** reviewed shell helper that commits a single-probe layer and complete static tests/report.

1. Write RED static topology tests for pinned predecessor/source IDs, pinned prior evidence report SHA, exact probe SHA, fixed five overlay hashes, no `docker start`, `docker diff` exact path, commit/layer-prefix checks, image config checks, and failure cleanup.
2. Run targeted unittest and capture missing-helper/source failure.
3. Implement shell helper with bounded `docker` subprocess timeout; never start the container; remove by verified full ID only; retain candidate only after every post-commit inspection passes.
4. The helper must inspect/copy the base probe before and after to verify unchanged mode; require Docker's default copy-to-container ownership root:root. Verify candidate file hashes via a never-started verification container, then clean verification and derivation containers.
5. Run all focused tests, py_compile for tests, `sh -n`, no-network/static invariant checks, and explicit whitespace checks including the new untracked paths.
6. Report exact before/after hashes, static test RED/GREEN, commands, shell failure cleanup reasoning, and state that no Docker/browser command ran.

**Verification:** `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_derive_volume_free_diagnostics_candidate.py' -v`; Python compile; `sh -n`; source lint/whitespace. Independent validation/review must inspect all hashes and failure branches before any parent Docker invocation.

**Acceptance:** every identity/config/layer/content/metadata invariant is checked; tests pass; no arbitrary base argument, no start, no success without complete verification, and cleanup is guaranteed. Parent then runs this helper once under a hard 900-second bound and records complete output before the separate browser diagnostic.

**Failure handling:** any identity, file, diff, layer, config, metadata, cleanup, or timeout discrepancy is a fail-closed blocker; preserve evidence and delete any newly created image only after verifying its full image ID.
