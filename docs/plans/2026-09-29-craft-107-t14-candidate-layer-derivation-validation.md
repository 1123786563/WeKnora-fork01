# T14 Candidate Layer Derivation — Independent Validation

## Result

**DONE_WITH_CONCERNS — static contract validation passes; runtime derivation is not validated.** No source or test files were changed. This report is the only file written by this validator.

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Revision inspected: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Frozen helper SHA-256: `de3cb6d8e06e388c43cb9e58d6f39593298e36d339131de81df9cb78298483f7`
- Frozen test SHA-256: `d2eeaa03455c9afa68a87242d122551fac3af3fbaa3d621c44be76a004ef3ba0`
- Implementation report SHA-256 at validation: `2b42acf8d041cbfc3666e55acf238cc0a3857c7e71d0fd1c57e2502787060022`
- Predecessor source-candidate report SHA-256: `181139de9decadb75542841419cc75bfa86db7aafd09cb999bd402b5d4160353` — exact match to plan/helper pin.

## Scope and acceptance

Reviewed the task plan and implementation report; predecessor source-candidate report; `docs/plans/2026-09-29-craft-107-t14-conntrack-vs-sockdiag-architecture.md`; approved Craft web-artifact Spec; Craft domain terms in `CONTEXT.md`; and ADR 0004. The relevant approved Spec requires no external egress and verified preview/build behavior. This helper task is narrower: produce a provenance-bound candidate with a single probe-file delta. Authentication, authorization, backend API contracts, schema migrations, and application cancellation are not applicable to this shell/Docker artifact helper.

The script and tests encode the assigned checks: fixed source and predecessor IDs; exact predecessor report digest; all six overlay digests; predecessor file checks before replacement; no start operation; created-state checks; exact `/opt/probe.py` diff; one added rootfs layer with predecessor layer prefix; preserved image config/platform, no tags or declared volumes; final embedded hashes and root:root mode 0644 for the probe; bounded Docker calls; and exit-trap cleanup using `docker rm -v`, with candidate deletion gated on full image-ID inspection.

The actual workspace overlay hashes match all six expected values. The helper is mode 0755 and the test is 0644. `git status` also showed numerous pre-existing unrelated T14 changes/untracked artifacts; none were modified by this validator.

## Commands and evidence

Executed at revision above:

```text
shasum -a 256 docs/plans/2026-09-29-craft-107-t14-current-source-candidate-live-attempt.md deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md
```

Result: predecessor report and helper/test hashes match the pins and implementation report. Implementation report itself currently hashes to `2b42acf8…`.

```text
python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_derive_volume_free_diagnostics_candidate.py' -v
```

Result: **PASS, 6/6 tests**.

```text
PYTHONPYCACHEPREFIX=/tmp/craft107-t14-validation-pycache python3 -m py_compile deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py
sh -n deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh
git diff --check -- deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md
```

Result: **PASS** (all three commands exit 0). An explicit trailing-whitespace scan on those three assigned paths found none.

```text
shasum -a 256 deploy/craft/render-boundary/probe.py deploy/craft/render-boundary/preview/index.html deploy/craft/render-boundary/preview/app.js deploy/craft/render-boundary/preview/style.css deploy/craft/render-boundary/preview/manifest.json deploy/craft/render-boundary/preview/asset.svg
```

Result: all six hashes equal the helper's pinned values.

No Docker, browser, network, or live image derivation command was run, as instructed.

## Findings, gaps, and risks

1. **Runtime semantics remain unproven.** Static validation cannot prove Docker's `create`, `cp`, `diff`, `commit`, image inspect, tar ownership/mode, layer ancestry, or cleanup behavior in the target daemon. Parent acceptance still requires a bounded authorized helper invocation and retained output. In particular, the single-file delta and exact one-layer ancestry are code assertions, not observed runtime evidence.
2. **Implementation report wording is inconsistent with the frozen code.** It describes an earlier “per-command cleanup timeout”; the final helper and sixth regression test enforce one shared 45-second total cleanup deadline. The code is consistent and the static check passes, but the report wording should be corrected and its resulting digest recorded before relying on it as final implementation evidence.
3. **No API/auth/data migration checks apply** to this helper. Runtime external-egress, browser behavior, and T14 acceptance remain outside this static validation and are not established by this result.

