# T19 E0 volume-free image Task 2 report

**Checkpoint:** `t19-e0-volume-free-image-task2-20260924-01`  
**Status:** implementation and scoped smoke verified; independent review pending.  
**Scope:** `run_v2.py` and its focused runner tests only. No commit created.

## Implementation

- Pinned the runner to derivative image `sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615`. Before participant launch, it checks exact image identity/platform/config (`Volumes` empty, UID, XDG environment, entrypoint, command, working directory, and exposed port), OpenCode 1.18.4 and binary SHA-256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`, and local plus baked recorder SHA-256 `d1e7b870b35ede69f6c4e95435ffc11e6ce689a5a556b047e679204400962463`. The recorder help command is run as UID 10001 with network mode `none`.
- C launches as `10001:10001` with exactly its two named XDG volumes. D/A launch as UID 10001 from the baked recorder path with separate named volumes at `/ledger`; the script bind was removed. The helper launches at UID 10001 with no mounts.
- Each source ledger volume is prepared by a separate exited provisioner with network mode `none`, `CapDrop=ALL`, only `CAP_CHOWN`, and one mount at `/ledger`; each provisioner is removed and absence-inspected before the source participant starts.
- The bounded smoke remains explicitly `BLOCKED` as a full E0 verdict. It validates the no-egress internal-network source boundary only.

## Verification

- `python3 -m unittest discover -s docs/testing/craft/egress-probe -p 'test*.py' -v` — PASS, 48 tests in 16.167 seconds, including focused runner tests, recorder tests, image/topology tests, and verifier mutation tests.
- `python3 -m py_compile docs/testing/craft/egress-probe/run_v2.py` — PASS.
- `python3 docs/testing/craft/egress-probe/run_v2.py --source-boundary-smoke` — `source_boundary_smoke=PASS`, `cleanup_absent=true`, `verdict=BLOCKED`. Run `craft-e0-20260924T040250Z-70089` ran from 2026-09-24T04:02:50Z to 2026-09-24T04:03:52Z on the Docker-internal network. Raw inspect shows all four participants use the derivative ID and UID 10001; C has exactly two XDG named volumes, D/A have separate `/ledger` volume sources, and helper has zero mounts.
- Both provisioning helpers exited 0 with `NetworkMode=none`, `CapAdd=[CAP_CHOWN]`, `CapDrop=[ALL]`; each helper container cleanup and not-found inspect passed. All 9 run-owned resource removals and absence inspections passed. Independent post-cleanup `docker ps -a`, `docker network ls`, and `docker volume ls` returned no names with this run ID. Docker slot was released to T14 after cleanup.

## Evidence

Raw artifacts: `docs/testing/craft/egress-probe/e0-fix2/boundary-smoke-craft-e0-20260924T040250Z-70089`. `file-hashes.json` SHA-256: `48ce18b7cb28b4a9772bc0ea09c29bf87edd8fa0ffb1eb3d9b9100e70d5ed667`. Raw participant inspect SHA-256: `ddcabe32fb16f306351f08a21fde680996410b28d261d6e2b5c6eff1fc809245`. Cleanup evidence SHA-256: `c757d1605e4ca96e1c49a5d075c52f8bab7b5710a6190b027b446d5af7b74b2d`. Source stream hashes: direct `92901567a776f6840367aa9c91ceda0b4930426fbadf5ae72e6d522234d9dc0d`, adapter `6d19667807e51eb8e78405cf870970c3617d1d990764f9d111f74c62b245b89c`. Full hashes and exact cleanup rows are in the checkpoint JSON.

Owned code hashes: `run_v2.py` `773c80f80fec877a210a1878e1b7eaf44af0682ae1729cb5069c191e85596cf1`; `test_runner_source_integration_v2.py` `39301920954a34cf0268af0bd7b5d5d1cd78ba64b25b0479680caf3384b9d37d`.

## Limits

No full E0 matrix or paid egress was run, and this report does not claim full E0 PASS. `assert_v2.py` was not changed and still hardcodes the previous base image ID; a separate verifier-owned alignment is needed before the ordinary full runner can produce a passing result with the derivative image.
