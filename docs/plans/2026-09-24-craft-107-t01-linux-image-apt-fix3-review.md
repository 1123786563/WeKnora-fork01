# T01 Linux image APT Fix3 — independent review

Date: 2026-09-24 (Asia/Shanghai)

## Verdict

**Spec compliance: PASS with one low-severity scope finding. Code quality: PASS.** No critical, high, or medium finding in the Fix3 change. The native `linux/arm64` image exists and the pinned OpenCode binary was verified inside it. This build establishes image contents; it does not establish sandbox network or mount confinement.

## Evidence and scope

- Reviewed the approved Craft web artifact spec, `CONTEXT.md`, relevant ADRs, the Linux image APT recovery plan (including Fix3), the Fix3 task report, `docker/craft/Dockerfile`, the structured checkpoint, and selected full build-log passages. No source or test file was changed during review.
- Checkpoint HEAD is `a5e9195acd6500c085c85d60c852148e7bbbbf34`, matching `git rev-parse HEAD`. Current Dockerfile SHA-256 `fa28b930178459512b856f31e67477fe032df6efc1e58c0a17ff8513dced732c` and build-log SHA-256 `d2598906e2e352f4b076c519778105eee3aca91e1a74a48c00bd0c24da8182e0` match the checkpoint. `git diff --check -- docker/craft/Dockerfile` passes.
- The D02 and D03 package lists are unchanged by the retry wrapping. `Acquire::Retries "3"` remains in both stages; each large install runs at most three whole-install attempts and exits nonzero after exhaustion. The build log records D02 attempt 1 failing on six 502 archive URLs after fetching 122 MB, followed by attempt 2 completing in the same layer. D03 completes on attempt 1. Successful `.deb` archives are removed; an in-image check found zero `.deb` files in `/var/cache/apt/archives`.
- The image retains Debian bookworm, bookworm-updates and bookworm-security sources with `Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg`; no trusted-source or signature bypass appears in the Dockerfile. OpenCode release download remains HTTPS with the pinned SHA/version checks. The Fix3 log's fetch layer was cached, but the preceding Fix2 log records `/tmp/unpack/opencode: OK` and version `1.18.4`; the final-image probe independently returned SHA-256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e` and version `1.18.4`.
- `docker image inspect weknora-craft-runtime:1.18.4` returned `OS=linux`, `Arch=arm64`, and image ID/repository digest `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`, agreeing with the build log's exported manifest list and the checkpoint.

## Finding

**LOW — APT's automatic archive cleanup remains disabled in the final image.** `docker/craft/Dockerfile` D02/D03 `RUN` blocks remove `/etc/apt/apt.conf.d/docker-clean` but do not restore it after the successful install. An in-image check confirms the hook is absent. Fix3 calls for neutralizing that hook only for the install layer and cleaning archives afterward. The immediate image has no cached `.deb` files, so this does not invalidate the build or pin evidence; later privileged APT activity in a derivative image could retain downloads unexpectedly. Smallest correction: save the existing hook before the retry loop and restore it after successful cleanup within the same `RUN` step, or use a layer-local APT override that does not persist. Rebuild and reverify the resulting digest if this is changed.

## Review limits

The successful build reused the prior fetch-stage layer; the prior log and final-image probe provide the pin evidence. No fresh uncached OpenCode download was needed for Fix3. The approved spec's sandbox egress and mount checks require separate runtime deployment verification.
