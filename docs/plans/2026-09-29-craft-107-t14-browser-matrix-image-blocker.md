# T14 Browser Matrix Image Selection Blocker

Date: 2026-09-29

## Decision

The 37-attempt Chromium matrix is not run from a renderer image that fails the current runner's exact embedded-source integrity gate. T14 remains running/unverified.

## Facts

- Current source tree at T14 Worktree `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01` has `probe.py` SHA-256 `6537b05abdcdd917a95b5ae65248b66e87ed717aa61ccab2788f805a6376157b` and `preview/app.js` SHA-256 `83173b30c49d075d8d9d89a04fca66cc7bf9d45de077b28b2cc45f06166738ea`.
- Previously reviewed immutable renderer `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27` embeds `probe.py` SHA-256 `95211d16a8085017a1d6ecfdb476f59e31e533622a8363a8675d7cbe991bfd7b`. Its older input evidence proves the five preview resources were current at the earlier checkpoint, but does not make the current probe hash match.
- Previously derived no-volume image `sha256:b74c73b06c45e128546180cb668e892e4cf7d2348b27c99e6940876460da9bd8` copies that same old runtime filesystem and therefore does not solve embedded-source mismatch.
- Current `run-probe.sh` extracts `/opt/probe.py` and `/opt/craft-preview/`, compares the complete file manifest with the current source tree, and exits before diagnostic execution on any missing, extra, or differing file. The immutable old image failed this factual preflight (`/opt/probe.py` hash mismatch).
- Existing Fix1/Fix2 image-derived work records an exact-image BuildKit source-resolution failure: digest-qualified local image-ID and named docker-image contexts were resolved via a configured registry mirror and returned HTTP 403; the required exact-source GREEN image was not produced.

## Disposition

No browser matrix was executed with an image whose embedded probe differs from the current source. No claim of browser denial, 37-cell coverage, or T14 acceptance is made. Retain the existing image source restrictions and runner integrity check. T14 remains gated; T15 stays pending.

## Safe continuation

An authorized/reviewed image production method must create a volume-free derivative from the exact reviewed renderer ID while replacing only current probe/preview files and preserving all runtime binaries/configuration. Then independently verify source ID, resulting immutable image ID, platform, runtime identity/hash, no unexpected volumes, full embedded manifest equality, and disposable-container cleanup before running the browser matrix once. The parent may also consider a reviewable SDD repair to support a provenance-preserving local image-copy method, but must not silently relax exact-source identity or allowlist arbitrary source tags.

The independent live policy test passed before this image check and is recorded at `docs/testing/craft/t14/2026-09-29-sockdiag-live-acceptance/`; it is a separate acceptance slice and does not resolve this browser-matrix blocker.
