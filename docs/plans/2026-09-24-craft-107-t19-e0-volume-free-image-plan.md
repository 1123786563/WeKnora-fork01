# Craft #107 T19 E0 Volume-Free Pinned Fixture Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the reviewed E0 RI-1 topology contradiction: the current pinned OpenCode image declares two XDG `VOLUME`s, so D/A/helper inherit writable mounts that the existing verifier forbids even when runner argv names none.

**Architecture:** Build and review a new immutable Linux/arm64 test image from the exact filesystem of base `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11` (OpenCode 1.18.4, binary SHA-256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`). `FROM scratch COPY --from=<base> / /` drops inherited Docker image `VOLUME` metadata while retaining bytes; bake the reviewed recorder script into the derivative so D/A need no script bind. Reapply exact PATH/HOME/XDG/WORKDIR/USER/entrypoint config. All four participants use the inspected derivative ID; C receives exactly its named config/data volumes explicitly, D/A exactly one private `/ledger` volume each, helper none. The verifier remains unchanged and checks the new manifest-pinned ID.

**Tech Stack:** Dockerfile, OrbStack Linux/arm64, source-owned recorder, Docker inspect, disposable mock fixture.

**Spec:** Approved Craft #107 T19; `2026-09-24-craft-107-t19-e0-runner-source-integration-task1-review.md` RI-1; Fix6 `assert_v2.py` fixed topology. This is test fixture provenance, not production image publishing or E0 acceptance.

## Global Constraints

- Do not change or republish the original base image, pinned OpenCode binary, production runtime, verifier topology checks or paid egress. Preserve same binary hash/version/config bytes and record source→derivative image identity/hash. New image must have `Config.Volumes == nil/empty` and `User=10001:10001`.
- C may mount only explicit named XDG config/data volumes. D/A each have one separate writer-only `/ledger` volume and no bind; helper has zero mounts. Pre-create/chown those recorder volumes using a separate bounded provisioning operation, then prove it was removed and is not a measured participant. No UID 0 D/A/Helper.
- No E0 PASS until full pinned-image matrix, source event joins and OpenCode physical attempt provenance all pass. No commit/push/deploy; exact checkpoint/raw inspect.

## Review Focus

- Base FS and binary/config identity retained, derivative image ID and platform pinned; no inherited VOLUME metadata.
- Raw D/A/helper/C inspect matches unchanged Fix6 verifier UID/mount expectations including source/destination aliases, and no client/helper access to recorder volumes.
- Volume provisioning does not leak a live privileged helper or grant broad capabilities; failure cleans up or remains explicit BLOCKED.

### Task 1: Deterministic derivative image and isolated topology proof

**Depends on:** RI-1 independent FAIL review and recorder phase/restart scoped PASS; runner cleanup RI-2 may proceed separately. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** new `docs/testing/craft/egress-probe/Dockerfile.volume-free` and focused build/topology tests/report. No `run_v2.py` in Task1 (owned by RI-2 fix until release), no verifier/recorder edits.

- [ ] Capture exact base image inspect, OpenCode binary/version and recorder source hash. RED inspect test shows base inherited two anonymous XDG mounts and fails fixed topology.
- [ ] Build one bounded derivative from pinned base FS, copy reviewed recorder source; set exact config and no VOLUME. Inspect/verify image ID, volume metadata, UID/env/entrypoint, binary SHA/version, and source bytes.
- [ ] Run disposable D/A/helper/C topology probe with explicit named C XDG volumes and separate D/A `/ledger` volumes; retain all raw inspect and cleanup. Test that unchanged Fix6 topology predicates accept mount/UID shape, while malformed source/mount fails.
- [ ] Save exact Dockerfile/test checkpoint, build log and raw evidence hashes; independent review before runner switches image ID.

**Acceptance / failure handling:** A reviewed immutable derivative can satisfy the fixed topology without changing the verifier. If filesystem copy or binary identity differs, keep RI-1 blocked.

### Task 2: Runner adopts reviewed derivative and fixed mounts

**Depends on:** Task1 independent PASS and RI-2 cleanup fix independent PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `docs/testing/craft/egress-probe/run_v2.py` and focused tests only.

- [ ] Pin Task1 derivative image ID and verify binary/config source before launch. Use UID10001 for C/D/A/helper, baked recorder path and exactly fixed named mounts; remove script bind and inherited-volume assumptions.
- [ ] Run one bounded no-egress source-boundary smoke and raw inspect replay; require exact verifier fixed topology and cleanup. Save checkpoint/review. The ordinary full runner remains BLOCKED until all controls use paired source boundary and OpenCode provenance is proven.

**Acceptance / failure handling:** RI-1 closed for the scoped source smoke; full E0 gate remains separate.
