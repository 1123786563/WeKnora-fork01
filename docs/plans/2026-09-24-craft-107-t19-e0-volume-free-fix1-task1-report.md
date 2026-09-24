# Craft #107 T19 E0 volume-free Fix1 Task1 report

**Status:** DONE for VF-1/VF-2 scope. The derivative remains a local topology fixture; full E0 and runner adoption remain separate gates.

## Scope and changes

Closed the two findings in `2026-09-24-craft-107-t19-e0-volume-free-task1-review.md` without changing the Dockerfile, runner, recorder, or verifier. The focused topology oracle now requires D/A raw mount `Source` strings to exist and applies the unchanged Fix6 equality and parent/child overlap predicate. Added distinct-name shared-source, both ancestor directions, and missing-source controls. A direct call to the unchanged `assert_v2.validate_host_evidence` rejects equal and nested D/A sources with its `raw pre A can write the direct recorder evidence mount` diagnostic.

Added a bounded disposable Docker run record with exact provisioner argv/results, post-start container inspect, removal and not-found inspect, volume inspect, participant inspect, UID 10001 writer check, and cleanup/absence observations. Both ledgers were separately chowned by isolated root provisioners using only CHOWN with all other capabilities dropped; the provisioners used `--network none` and exited successfully. Direct and adapter mounts point to distinct volume sources, and the participant source matches the provisioned volume mountpoint.

## Docker evidence

Pinned derivative: `sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615` (`linux/arm64`). The exact command records and raw inspect JSON are in `docs/testing/craft/egress-probe/volume-free/topology-provisioners.json` and the two `topology-fix1-*-inspect.json` files. SHA-256 values:

- `topology-provisioners.json`: `3b65b613ef52ad810c1e06bb4a6f0e74d24ef5509e3c31fffeaff3e7333096ff`
- `topology-fix1-direct-inspect.json`: `9a40e738ce0febb90a78186777b6fdd623a5ebe2f0b14ac1bb382a1fbf2068c4`
- `topology-fix1-adapter-inspect.json`: `dab97af14493293323dea8df4ebbe88d24fc8b452c67bd7050b3148e0574158a`
- `topology-fix1-cleanup.json`: `37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570`
- `topology-fix1-run-bounds.json`: `26bd1c7c28aeaca818fa3fb43efbc7aa73e4f23a3006091e90fcc76dc1448589`
- Machine-readable test/compile/whitespace output: `docs/plans/2026-09-24-craft-107-t19-e0-volume-free-fix1-task1-verification.json`.

For each role, `docker create` recorded the complete argv with root UID, `--network none`, `--cap-drop ALL`, `--cap-add CHOWN`, one target `/ledger` volume, and `/usr/bin/chown 10001:10001 /ledger`; exit 0. `docker start --attach <provisioner>` exited 0. A subsequent `docker inspect` showed `State.Status=exited`, `State.ExitCode=0`, `Config.User=0:0`, `NetworkMode=none`, `CapDrop=[ALL]`, and `CapAdd=[CAP_CHOWN]`. `docker rm <provisioner>` exited 0; `docker inspect <provisioner>` exited 1 with `no such object` for both named provisioners. D/A participants each wrote `/ledger/uid-check` as `10001:10001`. Their raw inspect records show separate sources `/var/lib/docker/volumes/t19vf-fix1-direct-ledger/_data` and `/var/lib/docker/volumes/t19vf-fix1-adapter-ledger/_data`. Both participants and volumes were also removed and received explicit absence inspections. The probe created no networks; every launched container used network mode `none`. No paid egress was used.

## Commands and results

- `python3 -m unittest docs.testing.craft.egress-probe.test_volume_free_image.VolumeFreeImageTests.test_fixed_disposable_mount_topology_accepts_and_rejects_malformed_inputs` — RED before the source predicate: equal D/A `Source` values under distinct names returned no topology error. GREEN after the predicate and controls: passed.
- `python3 -m unittest docs.testing.craft.egress-probe.test_volume_free_image.VolumeFreeImageTests.test_unmodified_fix6_predicate_rejects_source_alias_and_ancestor_overlap` — passed; directly invoked the unchanged Fix6 `validate_host_evidence` predicate for equal and nested source paths.
- Bounded Docker topology provisioner/participant run — passed; command argv, exit codes, inspect output, writer checks, removal and not-found diagnostics are recorded in `topology-provisioners.json`; aggregate bounds are 15 seconds per Docker CLI call and 100 seconds overall.
- `python3 -m unittest docs.testing.craft.egress-probe.test_volume_free_image -v` — 6 tests passed.
- `python3 -m py_compile docs/testing/craft/egress-probe/test_volume_free_image.py` — passed.
- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed; synthetic live-schema accepted and the existing verifier forgery/mutation controls were rejected.
- `git diff --check` equivalent for this untracked checkpoint: a whitespace/newline scan of the test, report, and JSON evidence found no trailing whitespace or missing final newline.
- Reconstructed the original Task1 preimage from its saved checkpoint patch, then applied this Fix1 task-local patch in a temporary directory — both `patch -p1 --batch --forward` operations exited 0; all nine recorded Fix1 postimage hashes matched exactly.

RED/GREEN results and final test output are recorded here and in the verification JSON. Exact baseline, postimage hashes, artifact hashes, and the incremental patch are recorded in `2026-09-24-craft-107-t19-e0-volume-free-fix1-task1-checkpoint.json`.

## Limits

This probe proves only the D/A volume provisioning lifecycle, UID 10001 write access, and scoped mount-source separation. It does not satisfy the full fixed E0 network topology and does not test paid or external egress. The original image Task1 review remains historical; this report and checkpoint are its narrow Fix1 addendum. No production, runner, verifier, Dockerfile, or remote issue changes were made; no commit was created.
