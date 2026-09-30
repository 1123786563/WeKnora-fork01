# T01 Linux ARM64 OpenCode lock correction report

Date: 2026-09-24 (Asia/Shanghai)

## Scope

Updated only `docker/craft/opencode.lock.json`; preserved the Darwin protocol fixture. No runtime enablement, image publication, commit, or push was performed.

## Fresh local verification

```text
docker image inspect weknora-craft-runtime:1.18.4 --format '{{.Id}} {{.Os}}/{{.Architecture}}'
sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11 linux/arm64

docker run --rm --entrypoint /bin/sh weknora-craft-runtime:1.18.4 -c 'sha256sum /usr/local/bin/opencode; /usr/local/bin/opencode --version'
3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e  /usr/local/bin/opencode
1.18.4
```

The lock now targets Linux/arm64 and uses that in-image binary SHA-256 at top level. `local_container_image_id` records the local Docker image ID separately. `container_digest` remains null because no registry RepoDigest has been published or verified. Darwin local probe OS, architecture, and SHA remain explicitly intact.

## Checks

- `python3 -m json.tool docker/craft/opencode.lock.json`: PASS.
- Assertions for Linux target/hash, local image ID, null registry digest, and preserved Darwin probe: PASS.
- `git diff --check -- docker/craft/opencode.lock.json`: PASS.
- Darwin fixture SHA-256 before and after: `3834e06533173c55dcbb1546229e0898a0540bb327b558ba505e7d34d268b37d` (unchanged).

## Checkpoint evidence

- Preimage Git blob: `d7b9ec56540cf206f7309c967a88a053917f5e16`.
- Preimage SHA-256: `3834e06533173c55dcbb1546229e0898a0540bb327b558ba505e7d34d268b37d`.
- Postimage SHA-256: `056b340e57d2aa83ca0fbc3fa3892783455a6999ebb47f8a12f3c2e45d0d5fd6`.
- Exact task-local patch: `docs/plans/2026-09-24-craft-107-t01-linux-lock-task-local.patch`.
- Patch SHA-256: `f898c5999863ce048cd2937acf38fa81abac92da42d73de07bcfff2156feef6e`.
