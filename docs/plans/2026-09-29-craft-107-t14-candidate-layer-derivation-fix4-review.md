# T14 candidate-layer derivation Fix4 — independent review

Reviewed 2026-09-29 against the approved Craft web-artifact Spec, `CONTEXT.md`, the ADR inventory, the Fix4 plan and task brief, the Fix4 task report, and the Fix3 live failure log. The relevant Spec requires isolated preview and evidence before artifact promotion; no ADR adds a different root-resolution rule. This review is limited to the owned helper and focused fake-Docker fixture/test. No Docker live run is part of this narrow path correction.

| Owned file | SHA-256 in task report | SHA-256 verified before review |
| --- | --- | --- |
| `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | `c8e6c5037dac9f281826438dbca8ae075ad1cb7162c93abd6cd8ecb78e943c21` | `c8e6c5037dac9f281826438dbca8ae075ad1cb7162c93abd6cd8ecb78e943c21` |
| `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | `659766409957036be74fe5e6eb481b736abef3c8cc94a15c83878c6282b4a06a` | `659766409957036be74fe5e6eb481b736abef3c8cc94a15c83878c6282b4a06a` |

## Evidence and findings

No new finding in the assigned Fix4 scope.

- The Fix3 live log records `shasum` looking for the pinned predecessor report one directory above the checkout, followed by an empty actual digest and exit code 1. The helper now sets `SCRIPT_DIR` from its own path and resolves `REPO_ROOT` with `../../../` (`derive-volume-free-diagnostics-candidate.sh:33-37`). For the real `<repo>/deploy/craft/render-boundary` location, this resolves to `<repo>`, so the pinned report lookup occurs within the checkout before the first Docker command at line 198.
- The fixture creates exactly `<repo>/deploy/craft/render-boundary` and `<repo>/docs/plans` (`test_derive_volume_free_diagnostics_candidate.py:187-193`), writes a matching pinned report, and runs the helper from a different cwd (`:227-243,267`). The new root-preflight case rejects the old digest-mismatch failure and confirms execution proceeds to fake Docker (`:274-278`). The report records all 15 focused tests passing, plus shell syntax, Python compilation, and whitespace checks. I did not rerun them.
- The path change is confined to root discovery. The visible preflight still checks the pinned report and six overlay hashes before Docker work (`derive-volume-free-diagnostics-candidate.sh:36-58`). The existing lifecycle tests remain in the fixture. The earlier low finding about an absent verification-create removal-failure case remains a separate test-coverage item from the Fix3 review; it is not introduced by this path correction.

## Verdicts

**Spec compliance: PASS for the assigned Fix4 root-resolution correction.** The real checkout layout now reaches its pinned report; the prior live failure path is addressed without relaxing the visible provenance preflight. This verdict does not establish product-level T14 preview or artifact acceptance.

**Code quality: PASS for the assigned Fix4 scope.** The correction is one relative-path adjustment paired with an exact-layout behavioral fixture and a different-cwd invocation. The recorded fake-Docker suite passed at matching hashes. No Docker daemon, browser, or live candidate derivation was run as part of this review.
