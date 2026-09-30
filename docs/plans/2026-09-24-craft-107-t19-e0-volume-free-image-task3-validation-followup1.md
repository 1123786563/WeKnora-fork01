# Craft #107 T19 E0 Volume-Free Image Fix1 — Task 3 Validation Follow-up 1

Date: 2026-09-24. Worktree revision: `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Validation only; no business/test source edits, Docker, paid egress, or remote issue operation.

## Hybrid fixture construction

Commands: `python3 /tmp/craft_capture_fixture.py` (SHA-256 `9f41762530ce1fa33db324706548d1086cee28d252fda6df02acbc9eef8a7cff`), then `python3 /tmp/craft_build_hybrid.py` (SHA-256 `7b84136a8c0d0a721aa09e2cda2970e926fd6e459cf7b720c79aa51b908b6b35`). The former ran the existing `test_assert_v2.py` suite and intercepted its calls to the aligned verifier to preserve the complete synthetic derivative fixture. The first command captured `/tmp/craft-e0-scaffold1` (the derivative positive fixture). A temporary Python heredoc copied that scaffold to `/tmp/craft-e0-hybrid`, replaced the `client`, `direct`, `adapter`, and `helper` container inspect blocks in both synthetic stages with the corresponding retained raw blocks from `boundary-source-mount-inspects.json`, set manifest participant/network and `config`/`data` names to that smoke run's exact names, updated synthetic host/phase references and raw hashes, and retained synthetic case/stream/cleanup evidence. The four retained participant objects were copied intact; wrapper stage fields and synthetic network/control evidence remain synthetic.

- Aligned verifier: `python3 docs/testing/craft/egress-probe/assert_v2.py /tmp/craft-e0-hybrid` — exit 1, `verdict: BLOCKED`, 252 errors. Relevant derivative identity and C mount checks did not report an error: all four raw participant IDs match the reviewed derivative, and both C XDG volume names match the manifest. Full positive verification is blocked because this raw one-phase boundary smoke is not the complete full-matrix inspect/control/cleanup provenance expected by the synthetic full fixture; e.g. synthetic control operations do not identify the retained raw client/helper lifecycle, and the raw creation/cleanup inventory differs from the full fixture's synthetic inventory. The hybrid therefore does not produce a full verifier PASS.
- Old-image negative: copied hybrid; changed only manifest `image.id` to the old base; reran the same verifier — exit 1, `wrong or missing pinned image ID` reported.
- Wrong-C-name negative: copied hybrid; changed only manifest `names.config` to `wrong-config-volume`; reran the same verifier — exit 1, `raw pre client mount set is not the fixed XDG config/data volume pair` and the same post-restart error reported.

## Provenance and hashes

Verifier commands: `python3 docs/testing/craft/egress-probe/assert_v2.py /tmp/craft-e0-hybrid`; old-image and wrong-name controls each used a copied `/tmp/craft-e0-hybrid` and reran that exact verifier command. Positive verifier output was `{"verdict":"BLOCKED","synthetic":true,"errors":[...]}` (252 errors); the two negative controls reported the exact target diagnostics listed above.

- Revision: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Verifier `assert_v2.py`: `8e410d4021e06a6e024d51b66124fb2482d017e836d83f7ffd07a1d8f2650869`.
- Retained raw smoke source `snapshots/boundary-source-mount-inspects.json`: `ddcabe32fb16f306351f08a21fde680996410b28d261d6e2b5c6eff1fc809245`.
- Retained derivative image inspect pin `volume-free/derivative-image-inspect.json`: `c1cfff156b3c6e65ad77f07b8cd0d56f9afdf2f971e6d444553001e2a5c35941`.
- Captured synthetic scaffold manifest: `e6f7a522614f4c20cfda423e2a2d748272efbf5c10229df0d35da7cf7841684d`.
- Hybrid manifest: `d4e4731cd0a7dd68a00a90a02ff93155d66eebfd975e5fba0551885ab2105c1f`; hybrid inspect wrapper: `ace82d545e9dd21ecc6fc52a469acfbc3d3dde340f9dcd200279b8c20e182d35`.

## Result

**DONE_WITH_CONCERNS.** The retained raw records are now replayed through `assert_v2.py` in a hybrid fixture, and the aligned verifier specifically accepts their exact derivative/C-volume identity while rejecting old-image and wrong-C-name controls. However, the hybrid full-verifier artifact is BLOCKED with unrelated lifecycle/evidence-schema mismatches, so a complete full-verifier PASS using retained raw inspect has not been demonstrated. This is synthetic integration evidence only and is not measured E0; full E0 remains BLOCKED pending physical provenance and network matrix evidence.
