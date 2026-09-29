# T14 Candidate Layer Derivation — Task 1 Report

## Result

Implemented a pinned, bounded shell helper that derives a renderer candidate from the reviewed predecessor by replacing only `/opt/probe.py`. Added static contract tests. The helper verifies predecessor evidence, base and final file content and metadata, created container state, exact filesystem diff, candidate layer ancestry, image configuration and cleanup behavior.

No Docker, browser, network, issue, staging, or commit command was executed by the implementation worker. This report does not claim live candidate construction or T14 acceptance; the parent must run the helper after independent validation and review.

## Provenance and checks encoded

- Pins source image `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27`, predecessor `sha256:c115e66adc210e483f72525f62c544f17d8b5e9808acb1ad3e21da9b85650894`, and predecessor report SHA-256 `181139de9decadb75542841419cc75bfa86db7aafd09cb999bd402b5d4160353`.
- Pins and checks all six final overlay hashes. Before editing, checks all six predecessor files, including predecessor probe SHA-256 `6537b05abdcdd917a95b5ae65248b66e87ed717aa61ccab2788f805a6376157b`.
- The predecessor builder uses plain `docker cp` into its container. The helper verifies the predecessor probe tar header is `root:root`, mode `0644`; it independently verifies the copied and committed probe remains `root:root`, mode `0644`. The renderer `USER` setting is not used as a proxy for file ownership.
- Requires `State.Status == created` for derivation and verification containers, with no start operation. Requires `docker diff` to contain exactly `/opt/probe.py`.
- Requires candidate rootfs layers to equal the predecessor layer list plus one layer, and checks platform `linux/arm64`, no repository tags, no volumes, exact predecessor config, and source runtime config fields.
- Bounds derivation Docker calls to 840 seconds. Cleanup attempts run with a separate 45-second per-command bound. Container cleanup uses `docker rm -v`; candidate removal is attempted only after `docker image inspect` confirms its complete image ID.

## Verification evidence

RED was observed before implementation:

```text
python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_derive_volume_free_diagnostics_candidate.py' -v
```

Result: test setup failed because the assigned helper did not yet exist (`FileNotFoundError` for `derive-volume-free-diagnostics-candidate.sh`).

An additional cleanup-bound test caught the first implementation’s per-command cleanup timeout. The helper was changed to establish one 45-second cleanup deadline shared by all container/image cleanup calls; the added regression test now passes.

GREEN and static checks:

| Command | Result |
| --- | --- |
| `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_derive_volume_free_diagnostics_candidate.py' -v` | PASS, 6 tests |
| `PYTHONPYCACHEPREFIX=/tmp/craft107-t14-pycache python3 -m py_compile deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | PASS |
| `sh -n deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | PASS |
| Static Python assertions: no `docker start`/`docker run`; exact diff path; predecessor report digest; one added layer; root metadata validation; volume cleanup; shared cleanup deadline | PASS, 8 assertions |
| `git diff --check -- deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md` | PASS, no diagnostics |
| Explicit trailing whitespace scan across all three assigned files | PASS, none found |

The helper itself was not invoked. Docker lifecycle, `docker cp` tar-header ownership/mode, commit ancestry, image config and cleanup remain runtime checks for the parent’s authorized execution.

## File digests

At report creation:

| File | SHA-256 |
| --- | --- |
| `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | `de3cb6d8e06e388c43cb9e58d6f39593298e36d339131de81df9cb78298483f7` |
| `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | `d2eeaa03455c9afa68a87242d122551fac3af3fbaa3d621c44be76a004ef3ba0` |
