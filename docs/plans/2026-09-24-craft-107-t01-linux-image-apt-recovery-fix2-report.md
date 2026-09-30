# T01 Linux image apt recovery — Fix2 report

Date: 2026-09-24 (Asia/Shanghai)

## Change

Both Debian bookworm-slim stages replace the default official Debian source host with Debian-listed `ftp.cn.debian.org`, preserving the configured finite apt retries, signed suite metadata, package lists, and package pins. The source remains HTTP as used by the successful disposable mirror probe; APT signature verification remains enabled. OpenCode release transport and digest/version checks are unchanged.

## One bounded native ARM64 build

Command: `timeout 1200 docker build --platform linux/arm64 --progress=plain -t weknora-craft-runtime:1.18.4 -f docker/craft/Dockerfile docker/craft`

The sole Fix2 build confirmed bookworm InRelease and arm64 package indexes fetched from `ftp.cn.debian.org` in both stages. The `opencode-fetch` stage also downloaded the pinned ARM64 release, verified SHA-256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`, and printed version `1.18.4`.

The final runtime image was not produced. Runtime `apt-get install` at Dockerfile line 112 fetched about 150 MB, then failed with exit 100 after archive 502 responses for `liblapack3`, `fonts-dejavu-core`, `libpoppler126`, `libreoffice-common`, `python3-jdcal`, and `python3-openpyxl`. APT reported `Unable to fetch some archives`; BuildKit failed. No further mirror or build attempt was made. Since there is no final image, no image digest or in-image binary inspection is available. No signature or TLS verification was disabled.

Full build log: `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery-fix2/build.log` (SHA-256 `df7779b173fa604060918ce04dad9206505d8eabd161cc328fc6b7f270ac5f07`).

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Dockerfile SHA-256 before (HEAD source): `1dee856c10a4616ea7c40ec65d51b2f54745c9ffe947898cbb76988ea516bce7`
- Dockerfile SHA-256 after Fix2: `841b5bbda51139b8c20f5f3f740c5e4a2e31116fa413a417608feb01ce2a4a67`
- `git diff --check`: passed
- Structured checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery-fix2/checkpoint.json`
