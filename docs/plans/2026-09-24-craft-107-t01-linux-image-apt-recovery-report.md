# T01 Linux image apt recovery report

Date: 2026-09-24 (Asia/Shanghai)

## Change

In both Debian bookworm-slim stages, changed official `deb.debian.org` source URIs from HTTP to HTTPS and wrote `Acquire::Retries "3";` before the first apt update. Package lists, install commands, binary pins, and hardening are unchanged.

## One bounded build

Command: `timeout 1200 docker build --platform linux/arm64 --progress=plain -t weknora-craft-runtime:1.18.4 -f docker/craft/Dockerfile docker/craft`

The single permitted build failed at the first `apt-get update` in both stages. The HTTPS connections to `198.18.0.119:443` reported `Certificate verification failed: The certificate is NOT trusted. The certificate issuer is unknown.` APT then reported no system certificates, failed to fetch the three Debian InRelease files, and could not locate `ca-certificates` / `curl` candidates; Dockerfile line 45 exited 100 and BuildKit failed. No TLS verification bypass was attempted. No image was produced (`docker image inspect` reports no matching image); image digest and in-image OpenCode version/SHA are therefore unavailable. No retry was run.

Full build output: `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery/build.log` (SHA-256 `0a2f951b89a263e861156303d34ff3135468a395ed22a94cc14bbc93b0134cc2`).

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Dockerfile SHA-256 before (HEAD source): `1dee856c10a4616ea7c40ec65d51b2f54745c9ffe947898cbb76988ea516bce7`
- Dockerfile SHA-256 after: `5511b8c3bdc7f65bea9663804d9adbd9a4e7e7dec7f95a6c20af9d7831b64393`
- `git diff --check -- docker/craft/Dockerfile`: passed
- Structured checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery/checkpoint.json`
