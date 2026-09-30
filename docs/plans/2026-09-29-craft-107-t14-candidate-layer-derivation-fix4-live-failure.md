# T14 candidate derivation Fix4 live attempt — verified failure

Date: 2026-09-29. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`. Helper/test hashes matched the Fix4 reviewed checkpoint (`c8e6c503...` / `65976640...`) before invocation.

## Attempt

Ran the reviewed helper once from the repository root:

```text
sh deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh
```

Full output: `docs/testing/craft/t14/2026-09-29-candidate-layer-fix4/derivation.log`.

The helper passed source ID, predecessor ID, and predecessor report SHA checks, then created derivation container `a25081ee2ffa838ad20bf743fd4926cf2de7dfebcfd8fef1a36bcc75aaaa939f`. It stopped before copying the new probe or committing an image with `predecessor probe owner/mode mismatch`. Cleanup evidence in the log shows the exact container was removed (`exit=0`) and temporary directory cleanup succeeded (`exit=0`). Subsequent `docker ps -a --no-trunc --filter name=t14` showed no matching T14 containers; the helper created no candidate image.

## Root cause evidence

1. Created diagnostic container `t14-probe-owner-diagnostic-20260929` from the exact immutable predecessor `sha256:c115e66adc210e483f72525f62c544f17d8b5e9808acb1ad3e21da9b85650894`; it was never started.
2. `docker cp <container>:/opt/probe.py -` archive metadata showed mode 0644, UID 501, GID 20.
3. `docker export` of that stopped container was inspected with Python `tarfile`; `opt/probe.py` also showed mode 0644, UID 501, GID 20. Thus this is the actual predecessor rootfs metadata, not only the cp-to-host archive representation.
4. Diagnostic container was removed with `docker rm -v`; no container remained. The exact predecessor image ID and image list were inspected; no image was created or deleted by the failed helper.
5. The helper's pre-mutation check hard-codes predecessor UID/GID 0:0, while its post-copy and candidate checks also require UID/GID 0:0. The predecessor mismatch is the first failure. The candidate root requirement remains a distinct intended output invariant.

Conclusion: the failure is caused by an incorrect predecessor metadata expectation for the pinned immutable predecessor. Fix5 must bind expected predecessor `(501,20,0644)` to that exact image/report/content identity, and retain `(0,0,0644)` for the newly copied probe. No retry is permitted until that change has its own SDD review and independent validation.

No browser, network request, source image mutation, or candidate image creation occurred in this attempt.
