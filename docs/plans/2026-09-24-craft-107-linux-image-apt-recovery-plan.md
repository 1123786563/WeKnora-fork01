# T01 Linux image build: Debian fetch recovery

> **For Codex:** Execute this narrow SDD environment repair, verify the final image, and obtain independent review. The Docker build is a release prerequisite, not a proof of isolation.

**Source:** `2026-09-24-craft-107-t01-linux-image-build-probe.md`: two bounded builds, pinned OpenCode ARM64 fetch/version/SHA passed, runtime Debian package fetch failed on repeated 502. Original execution BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; record current HEAD and Dockerfile hash. No commits.

## Global Constraints

Keep Debian bookworm-slim base, exact package set, OpenCode v1.18.4/version/architecture binary SHA checks and runtime hardening. Do not replace with an unreviewed third-party image, disable TLS verification, or change the pinned binary. A successful build only permits recording the resulting image digest; it does not itself prove network/mount confinement. Other agents own Go source.

## Review Focus

HTTPS Debian mirror configuration and finite apt retries in both stages, no package/version omission, cached fetch-stage pin still checked, bounded build log, actual image digest and in-image version/SHA.

## Task 1 — resilient official Debian fetch

**Depends on:** two recorded 502 build failures. **Owner:** mechanical_worker. **Validator:** backend_validator. **Owned files:** `docker/craft/Dockerfile` and its task report/checkpoint only. **Consumes:** current Dockerfile and official Debian HTTPS endpoints. **Produces:** finite apt retry configuration at both stages before their first apt update, with HTTPS for official Debian URIs.

1. Verify the current bookworm-slim source format and RED evidence in the build probe. Edit only mirror transport/retry directives, retaining the same package lists and install steps. Keep retries bounded and fail loudly after exhaustion.
2. `docker build --platform linux/arm64 --progress=plain` under a hard 1200-second cap, one attempt after the source change. Capture full log in integration task directory. If successful, inspect image ID/digest, run exact binary `--version` and SHA-256 inside the image, and record command/output. If build fails, record exact step/error; do not repeatedly retry.
3. `git diff --check`, Dockerfile diff/source hashes and exact checkpoint; independent review checks only this narrow edit and evidence.

**Acceptance:** complete pinned image exists and binary SHA/version match its architecture, or an exact remaining build blocker is documented without weakening security. **Failure handling:** retain production default-off if digest/runtime validation is absent.

## Fix 1 — HTTPS certificate bootstrap ordering

The first Task1 build failed before any package install: `bookworm-slim` could not verify HTTPS Debian sources because `ca-certificates` was itself still missing. This is a deterministic bootstrap error, distinct from the earlier transient 502s. Keep the original official HTTP Debian source only for the minimal initial `ca-certificates` bootstrap in each stage, with finite `Acquire::Retries`; switch to official HTTPS sources **after** certificates are installed and before the large runtime package steps. Preserve all package lists, binary pins and hardening. RED is the exact Task1 build log at `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-linux-image-apt-recovery/build.log`. GREEN is one bounded native arm64 build plus in-image version/SHA and digest if it succeeds. Save a new fix1 report/checkpoint and obtain independent review after the build outcome; no TLS verification bypass or alternative unverified mirror.

## Fix 2 — verified Debian-listed mirror routing

Fix1 failed before bootstrap because `deb.debian.org` HTTP itself returned 502. The controller then checked Debian's current worldwide mirror list and independently ran disposable Docker `apt-get update` probes against the Debian-listed `ftp.cn.debian.org`: both trixie and **bookworm** InRelease plus arm64 package indexes completed with exit 0 inside the same Docker host. This is new positive evidence for a different official-list mirror, not a blind repeat. Change only the Debian apt URI host to `ftp.cn.debian.org` in both stages, preserving the Debian-signed repository suites, package lists and finite retries. APT's signed InRelease and package hashes remain mandatory; do not set Trusted=yes, skip signature checks, or bypass TLS for the OpenCode release download. Use the already-tested HTTP apt URI throughout if HTTPS cannot be bootstrapped; APT signatures supply package integrity. Run one bounded native arm64 build, record digest and in-image version/SHA if successful, otherwise exact failure. Save fix2 checkpoint/report and seek independent review; no further mirror or silent retry if it fails.

## Fix 3 — bounded whole-install recovery using retained archives

Fix2 reached both signed bookworm indexes and fetched about 150 MB in the large runtime install, then failed on six 502 archive URLs despite per-request retries. Independent `2026-09-24-craft-107-linux-image-apt-review.md` found no security regression and identified a bounded whole-install retry with retained successful archives as a plausible targeted recovery. This is a new build strategy for the precise remaining failure, not another unchanged mirror attempt.

In the two large runtime package `RUN` steps only, retain successfully downloaded `.deb` archives across at most **three** whole-install attempts in the same layer: neutralize only the base image's automatic APT archive cleanup for that layer if present, then remove cached archives after successful install so the final image stays bounded. Preserve each exact package list, signed source, per-request retry limit, OpenCode/version/SHA checks, and all later smoke checks. Do not mask the final apt exit code or accept partially installed dependencies. Run one native arm64 build with a hard 1500-second limit, capture full log. If it succeeds, verify image digest and binary SHA/version inside the final image; if it fails, record exact step and stop image-build recovery. Save a new fix3 checkpoint/report and independent review. No further mirror change or unbounded retry.
