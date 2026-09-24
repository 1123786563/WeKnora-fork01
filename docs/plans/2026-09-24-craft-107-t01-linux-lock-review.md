# T01 Linux ARM64 OpenCode lock correction — independent review

Date: 2026-09-24 (Asia/Shanghai)

## Verdict

**Spec compliance: PASS. Code quality: PASS.** No critical, high, or medium finding in this lock-only correction. The lock describes the verified local Linux/arm64 image and binary without treating the local image ID as a published registry pin or enabling Craft runtime.

## Evidence

- Reviewed the approved Craft web artifact spec, `CONTEXT.md`, relevant ADR inventory (none governs this image lock), T01 Linux lock plan/report/checkpoint/task-local patch, Fix3 image report/review, Linux protocol smoke, `docker/craft/runtime-config.json`, current Docker lock, and Darwin protocol fixture.
- `python3 -m json.tool docker/craft/opencode.lock.json` and `git diff --check -- docker/craft/opencode.lock.json` pass. The task-local patch exactly equals the current Git diff for the owned lock file. Its SHA-256 is `f898c5999863ce048cd2937acf38fa81abac92da42d73de07bcfff2156feef6e`; the lock postimage SHA-256 is `056b340e57d2aa83ca0fbc3fa3892783455a6999ebb47f8a12f3c2e45d0d5fd6`, matching the checkpoint.
- Fresh `docker image inspect weknora-craft-runtime:1.18.4` returned image ID `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11` and `linux/arm64`. A fresh in-image probe returned OpenCode version `1.18.4` and `/usr/local/bin/opencode` SHA-256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`. These equal the lock's local image ID and top-level binary SHA; `runtime-config.json` independently carries the same Linux/arm64 binary SHA.
- `container_digest` remains `null`, and its status explicitly says registry publication and verification are pending. Docker's local `RepoDigests` display includes `weknora-craft-runtime@sha256:9678…`, but that daemon metadata is not evidence of a published registry digest. The lock makes no such publication claim.
- The nested Darwin local probe remains `darwin/arm64` with SHA-256 `9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98`. The separate protocol fixture SHA-256 is `3834e06533173c55dcbb1546229e0898a0540bb327b558ba505e7d34d268b37d`, unchanged from HEAD and the checkpoint. The lock's protocol, health-check, fixture list, version, and source commit are unchanged. No runtime-enable field or source file is in the task-local patch.

## Review limit

This review verifies metadata attribution and the local image contents. It does not establish registry availability or the approved spec's sandbox egress and mount isolation, which require deployment checks. The separate Fix3 review records a low-severity APT cleanup-hook issue in the Dockerfile; this lock change neither introduces nor resolves it.
