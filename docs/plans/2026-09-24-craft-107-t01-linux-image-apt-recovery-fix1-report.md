# T01 Linux image apt recovery — Fix1 report

Date: 2026-09-24 (Asia/Shanghai)

## Change

In both Debian bookworm-slim stages, finite apt retries are configured before the initial update. The minimal initial certificate bootstrap uses the official HTTP Debian sources to install `ca-certificates`; each stage switches `deb.debian.org` to HTTPS immediately after that install. The fetch stage then refreshes over HTTPS before installing its existing `curl` package. No package was added or removed; pins, install sets, and runtime hardening remain unchanged.

## One bounded native ARM64 build

Command: `timeout 1200 docker build --platform linux/arm64 --progress=plain -t weknora-craft-runtime:1.18.4 -f docker/craft/Dockerfile docker/craft`

The sole Fix1 build failed in `opencode-fetch` at the first HTTP `apt-get update`. Exact error: `http://deb.debian.org/debian bookworm InRelease` returned `502 Bad Gateway [IP: 198.18.0.119 80]`; APT then reported `E: Failed to fetch ... bookworm/InRelease 502 ...` and `E: The repository 'http://deb.debian.org/debian bookworm InRelease' is not signed.` The apt process exited 100 and BuildKit failed. The runtime bootstrap stage was canceled by BuildKit after the peer stage failed. There was no retry build. No final image/tag or digest exists, so in-image OpenCode version/SHA checks could not run. No TLS bypass was used.

Full build log: `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery-fix1/build.log` (SHA-256 `dfbff1431dcb06eb343680544b8fce951d6a68954c6cdaf04776efe3b5982da0`).

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Dockerfile SHA-256 before (HEAD source): `1dee856c10a4616ea7c40ec65d51b2f54745c9ffe947898cbb76988ea516bce7`
- Dockerfile SHA-256 after Fix1: `8c8d9fa3fc0acb2b3bcb692d39f3f45a00005e784bd7f6a130c64e3c7e464861`
- `git diff --check`: passed
- Structured checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery-fix1/checkpoint.json`
