# T14 candidate layer derivation Fix 2 — independent review

## Scope and checkpoint

Reviewed the Fix 2 plan, original task plan, Fix 1 review, approved Craft web artifact Spec, `CONTEXT.md`, ADR inventory (no Craft-specific ADR), helper, tests, and task report. This was a source review; no Docker, browser, network, OCR, or live candidate derivation was run. The reviewed checkpoint initially matched the assigned SHA-256 values:

| File | SHA-256 |
| --- | --- |
| `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | `8070571364b8d14906f3f226c6cf46f94568c39ddc4bbdba7a89edb76d48b833` |
| `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | `7a7bd83bc253daafb1a57728beadb3f13e7862584ef353004860be1757b51764` |
| `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md` | `3af713b09fd390d90241cd7f0cb5b10e7ccad1f74d11dbb49042cc1ddd2a2d34` |

## Finding

### Medium — A successful `docker create` with an untrusted ID bypasses owned-container reconciliation

**Evidence:** The derivation create failure branch at `derive-volume-free-diagnostics-candidate.sh:202-220` reconciles by unique name only when the command exits nonzero. If Docker exits zero with empty, truncated, or otherwise non-full output, lines 221-223 exit immediately. The verification create has the same split at lines 327-348. Neither branch records the container ID for the EXIT cleanup trap. The fake Docker behavior tests cover true subprocess timeout, but not a daemon-created container followed by zero exit and untrusted output (`test_derive_volume_free_diagnostics_candidate.py:267-308`).

**Impact:** A real daemon-side container can remain after a failed derivation, including its anonymous volumes. In the verification-create case it also keeps a reference to the newly committed candidate; the later image removal then conflicts, leaving both objects. This violates the plan's cleanup and exact unresolved-identity reporting contract. It does fail closed with no candidate-retained marker.

**Smallest correction:** Route every create result lacking a valid full ID through the existing name-based inspection, exact image/name/state check, and bounded removal path. Record the exact unresolved ID or name if removal cannot be proven. Add a fake-Docker case for zero exit with non-full output for both create positions and assert final container/image state.

## Fix 1 finding disposition

- **Bounded reserve:** Addressed for mutating `create`, `commit`, and `cp`: `docker_bounded` leaves a 45-second reserve; `docker_recovery` caps calls by both remaining derivation time and that reserve. The behavioral fake waits long enough to trigger Python `subprocess.TimeoutExpired` rather than merely returning exit 124.
- **Ambiguous commit timeout:** Addressed. Lines 287-303 treat inventory as diagnostic only and set ownership only from a full ID directly returned by the commit command. The adversarial test preserves the pre-existing and concurrent lookalikes and asserts no image removal.
- **Image-reference conflict:** Addressed for the tested normal-removal failure. The fake rejects `image rm` while a container references the image, and the helper re-inspects the exact owned verification container before force removal and candidate removal. An unresolved removal remains a failing run with the identity printed.
- **Temporary-directory cleanup gate:** Addressed. The local directory removal precedes candidate-retention decision; a failure triggers removal of the owned candidate and nonzero exit. The fake injects that failure.

## Verdict

**Spec compliance: blocked for this helper's approved lifecycle plan by the medium finding.** The helper preserves pinned source/predecessor/report evidence, six overlay hashes, never-started containers, one-file diff, one added layer, config/platform/tag/volume checks, and fail-closed commit-timeout policy. The approved product Spec's live build, preview, network denial, and version-promotion acceptance are outside this source-only review.

**Code quality: one correction required.** The stateful tests now model real timeout behavior and Docker image-reference conflicts, and the four prior Fix 1 findings are materially resolved. The zero-exit/untrusted-ID path is an uncovered cleanup regression. The report's 11 passing tests and syntax/compile checks are implementation evidence, not independent live Docker validation.
