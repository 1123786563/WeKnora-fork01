# T01 Linux ARM64 OpenCode local image lock correction

> **For Codex:** Apply a narrow lock metadata correction with exact checkpoint and independent review. This does not publish or enable an image.

**Sources:** reviewed native ARM64 image Fix3 report/review; `2026-09-24-craft-107-t01-linux-protocol-smoke.md`; `docker/craft/runtime-config.json`; current `docker/craft/opencode.lock.json`; Darwin fixture `internal/modules/agentruntime/agent/opencode/testdata/protocol-lock.json`. Local image ID `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`, in-image OpenCode 1.18.4 SHA `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`. Do not conflate local image ID with registry RepoDigest or Darwin binary hash.

## Global Constraints

Own only `docker/craft/opencode.lock.json` and task report/checkpoint. Keep `testdata/protocol-lock.json` Darwin fixture and its live host tests unchanged. Set Docker lock target to Linux ARM64 and top-level binary digest to verified Linux value; retain nested Darwin local probe with its own value. Record local image ID separately, leave `container_digest` null until a registry digest is actually published/verified. Never claim a deployable registry pin or enable production runtime based only on local image. Preserve protocol fields and version.

## Review Focus

Arch-specific SHA attribution, image ID vs registry digest, host test fixture compatibility, no production enablement, exact JSON validity and checkpoint.

## Task 1

**Depends on:** Fix3 image independent PASS and Linux protocol smoke. **Owner:** mechanical_worker. **Validator:** reviewer. **Owned file:** `docker/craft/opencode.lock.json`, plus new report/checkpoint. **Consumes:** exact verified image/binary hashes. **Produces:** truthful local Linux lock metadata.

1. RED: show current Docker lock top-level binary digest is Darwin while Linux image binary differs; capture preimage/hash.
2. Set `binary_sha256` to Linux ARM64 digest, `target.arch` to `arm64` and target note, add `local_container_image_id` exact ID, revise `container_digest_status` to state local verification complete/registry pin pending, revise local_probe note; do not set `container_digest` to local image ID. Keep Darwin fixture unchanged.
3. Parse JSON, compare exact hash/version/image with live inspect and report, run `git diff --check`; save exact task-local patch/hashes/report and independent review.

**Acceptance:** Docker lock states what was actually verified without implying registry release. **Failure handling:** if actual local image ID or in-image hash differs on fresh inspection, stop and report; no metadata guess.
