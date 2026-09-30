# T01 Linux image apt recovery — Fix3 report

Date: 2026-09-24 (Asia/Shanghai)

## Change

The two large runtime apt install steps now remove Debian's `docker-clean` apt hook only when present, keep successful `.deb` archives across at most three full `apt-get install` attempts in the same RUN layer, fail after the third failure, and remove cached archives and apt lists after successful installation. Both original package lists, Debian signed sources, retry configuration, OpenCode release verification, and subsequent smoke checks are preserved.

## One bounded native ARM64 build

Command: `timeout 1500 docker build --platform linux/arm64 --progress=plain -t weknora-craft-runtime:1.18.4 -f docker/craft/Dockerfile docker/craft`

Build succeeded. D02's first install attempt encountered archive 502 responses and failed after fetching 122 MB; the second install attempt reused the retained archive cache and completed. D03's large apt install completed on its first attempt. All later wheel SHA checks, python-pptx/LibreOffice/poppler probes, DOCX round-trip, and workspace symlink assertion completed.

- Final local image ID / manifest digest: `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`
- Image architecture: `linux/arm64`
- In-image command: `docker run --rm --entrypoint /bin/sh weknora-craft-runtime:1.18.4 -c 'sha256sum /usr/local/bin/opencode; mkdir -p /tmp/oc-probe; HOME=/tmp/oc-probe /usr/local/bin/opencode --version'`
- In-image OpenCode SHA-256: `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e` (matches pinned arm64 value)
- In-image OpenCode version: `1.18.4`

Full build output: `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery-fix3/build.log` (SHA-256 `d2598906e2e352f4b076c519778105eee3aca91e1a74a48c00bd0c24da8182e0`).

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Dockerfile SHA-256 before Fix3: `841b5bbda51139b8c20f5f3f740c5e4a2e31116fa413a417608feb01ce2a4a67`
- Dockerfile SHA-256 after Fix3: `fa28b930178459512b856f31e67477fe032df6efc1e58c0a17ff8513dced732c`
- `git diff --check`: passed
- Structured checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery-fix3/checkpoint.json`
