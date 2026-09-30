# T14 verification-container ownership repair Fix9

> Narrow review repair plan. Do not stage or commit.

**Goal:** Ensure a candidate verification container is removed only after ownership is proved, including ambiguous create outcomes.

**Source:** Fix8 High/Medium findings in `docs/plans/2026-09-29-craft-107-t14-offline-dockerfile-fix8-review.md`.

## Task 1

- Owned files: `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh`, its fake-Docker test, this Fix9 report/checkpoint package.
- Preserve generated verification name before create. Assign the EXIT-cleanup ID only after inspect proves exact full ID, candidate image, name, and `created` state.
- If create fails, returns malformed output, or direct inspect does not prove exact identity, reconcile by unique name within cleanup bounds. Only an inspected full-ID record with expected image/name/state authorizes assigning that ID for removal. Otherwise emit unresolved name/returned identity and leave the target untouched.
- Add fake cases for timeout after daemon create, malformed create output after create, and inspect mismatch. Strengthen failed-rm evidence to exact nonzero statuses, candidate remains referenced, no retained-ID message, and ordering.
- Run targeted/full fake suite, `sh -n`, `py_compile`, `git diff --check`, and static checks. No Docker/network.
- Save before/after hashes and incremental patch in `.superpowers/sdd/2026-09-29-craft-107-t14-offline-dockerfile-fix9/`; report at `docs/plans/2026-09-29-craft-107-t14-offline-dockerfile-fix9-report.md`.

**Acceptance:** Proven verification containers are removed before candidate cleanup. Unproven containers are never passed to `docker rm`, and their unresolved name is reported. All specified fake cases pass.
