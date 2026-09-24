# Craft #107 T19 E0 Volume-Free Image Fix1 Task 1 Report

Date: 2026-09-24. Scope: VI-1 only, full verifier exact image pin and its focused synthetic verifier suite. No Docker or paid egress, runner/recorder/Dockerfile changes, or commit.

## Change

Changed the verifier's single immutable image ID from the old base digest to the independently reviewed derivative `sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615`. The full verifier still checks the manifest against that exact pin and checks raw participant inspect identities against the manifest. Platform/version, binary/source, participant, UID, mount, source, and topology checks are unchanged.

Extended the synthetic full-schema fixture in `test_assert_v2.py` to exercise the derivative image with the manifest, image case evidence, and raw participant inspect all aligned. It must pass the full verifier. The same derivative baseline is mutated to the old base and an arbitrary SHA-256 ID; both must fail with the fixed-pin error.

## Evidence

- RED: `python3 test_assert_v2.py` failed on the complete derivative fixture before the verifier edit. The reported error was exactly `wrong or missing pinned image ID`.
- GREEN: `python3 test_assert_v2.py` — exit 0. Output confirms the complete derivative fixture passed as synthetic, old base and arbitrary image IDs were rejected, all existing verifier mutation cases were rejected, and the two quiescence checks completed.
- `python3 -m py_compile assert_v2.py test_assert_v2.py` — exit 0.
- Incremental patch replayed against captured preimages; resulting source and test SHA-256 values match the recorded postimages.

## Checkpoint

Preimages, postimages, replayable patch, and hashes are in `2026-09-24-craft-107-t19-e0-volume-free-image-task2-fix1-task1-checkpoint/`. Both owned files were untracked in the integration worktree at task start; captured preimages preserve their exact baseline content.

## Review / Limits

This implementation report is not an independent review. E0 remains BLOCKED pending independent review and the physical attempt/provider provenance and network matrix. No live or paid run was performed.
