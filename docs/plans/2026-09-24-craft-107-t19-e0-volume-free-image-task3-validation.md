# Craft #107 T19 E0 Volume-Free Image Fix1 — Task 3 Validation

Date: 2026-09-24. Worktree revision: `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Validation scope: synthetic full-verifier integration only. Prerequisites: Task 1 derivative pin review PASS and Task 2 C-name review PASS, as recorded in `2026-09-24-craft-107-t19-e0-volume-free-image-task2-fix1-task1-review.md` and `...task2-review.md`.

## Commands and results

- `python3 test_assert_v2.py` (cwd `docs/testing/craft/egress-probe`) — exit 0, 20.8 s. The aligned full verifier accepted the complete synthetic derivative-image fixture (`reviewed-derivative-image: rc=0`, explicitly labelled synthetic), rejected old-base and arbitrary image IDs (`rc=1` each), and rejected `fix3-aggregate-inspect-mount-escape` (`rc=1`). All remaining mutation checks also rejected; timeout quiescence controls produced their expected output.
- `python3 -m unittest -v test_volume_free_image.py` (same cwd) — exit 0; 6 tests passed, including retained derivative image/raw topology pin checks and malformed topology rejection.

Verifier/test SHA-256 at validation: `assert_v2.py` `8e410d4021e06a6e024d51b66124fb2482d017e836d83f7ffd07a1d8f2650869`; `test_assert_v2.py` `d3d7fa30a683474ab82f557abf86827cd65e69ae753582d190e0ca5af33293a7`; `test_volume_free_image.py` `3ab9e0fcf60a9474c49533a8dca94c3a80c2ba429c6479a1aabea6709221c896`.

Retained raw topology evidence includes C/D/A/helper inspect data in `docs/testing/craft/egress-probe/e0-fix2/boundary-smoke-craft-e0-20260924T040250Z-70089/snapshots/boundary-source-mount-inspects.json` (SHA-256 `ddcabe32fb16f306351f08a21fde680996410b28d261d6e2b5c6eff1fc809245`), plus derivative D/A inspect records `volume-free/topology-fix1-direct-inspect.json` (`9a40e738ce0febb90a78186777b6fdd623a5ebe2f0b14ac1bb382a1fbf2068c4`) and `topology-fix1-adapter-inspect.json` (`dab97af14493293323dea8df4ebbe88d24fc8b452c67bd7050b3148e0574158a`). The topology test independently verifies these retained facts and the fixed mount constraints.

## Scoped verdict and limitation

**DONE_WITH_CONCERNS for synthetic identity/mutation coverage; Task 3's literal raw-inspect replay acceptance is not fully demonstrated.** The full-verifier suite creates a complete synthetic inspect fixture, then mutates it to the reviewed derivative ID; its full-verifier C mount negative is synthetic. The retained raw C/D/A/helper inspect was checked by the separate volume-free topology tests, but was not substituted into a complete full-verifier artifact and replayed as one integrated fixture. No test source was altered to manufacture that replay. Accordingly, the verifier acceptance with the actual retained raw inspect artifact remains an acceptance gap.

No source/test edits, Docker use, paid egress, remote issue operations, commits, or E0 PASS claim. Full E0 remains BLOCKED pending physical provider provenance and network matrix evidence.
