# Craft #107 T19 E0 volume-free Fix1 Task 1 — independent review

Date: 2026-09-24. Scope: VF-1/VF-2 repair only. Read the Fix1 plan, original Task 1 review, approved Craft web-artifact Spec, `CONTEXT.md`, volume-free image plan, unchanged Fix6 verifier, Task 1 and Fix1 reports/checkpoints, focused test, and retained raw evidence. No Docker or OCR rerun, source/test edit, or remote issue change.

## Findings

No blocking finding in the Fix1 scope.

The retained `writer_check` values are a short observed result, without a separate `docker exec` argv/exit record. They are consistent with the participant inspect `Config.User=10001:10001` and successful UID check described in the original Task 1 evidence. Fix1's new proof concerns the provisioner argv/exit/removal and D/A source isolation, so this does not reopen VF-1 or VF-2. If the later full E0 runner treats the disposable UID check as measured run evidence, retain its exact exec operation there.

## VF-1 evidence — closed

`topology-provisioners.json` records distinct direct and adapter volume creation, then exact `docker create` argv using the pinned derivative ID, root `0:0`, `--network none`, `--cap-drop ALL`, `--cap-add CHOWN`, one named `/ledger` mount, and `/usr/bin/chown 10001:10001 /ledger`. Both creates and attached starts returned exit 0 without timeout. Their raw inspect records show exited status, exit code 0, the same create IDs, `NetworkMode=none`, `CapDrop=[ALL]`, `CapAdd=[CAP_CHOWN]`, `Privileged=false`, and the expected single writable volume. Each `docker rm` returned 0 and its subsequent `docker inspect` returned 1 with a specific `no such object` diagnostic. The participant and volume remove/absence pairs also returned the expected statuses. Every recorded Docker command elapsed below the 15-second limit; the retained run bounds report a 100-second overall limit and no created networks. The focused test asserts the provisioner command/result and absence fields.

The new direct and adapter participant inspects have `Config.User=10001:10001`, no added capabilities, `CapDrop=[ALL]`, one writable `/ledger` volume each, and `NetworkMode=none`. Their mount sources equal the corresponding provisioned volume mountpoints. Each role's retained writer result is `10001:10001 /ledger/uid-check`. `topology-fix1-cleanup.json` is an empty final resource inventory; the individual remove and not-found records provide the stronger resource-specific cleanup proof.

## VF-2 evidence — closed

`topology_errors` now requires D/A mount `Source` strings and rejects equality and either parent/child direction using the same string predicate as unchanged `assert_v2.validate_host_evidence`. The focused test accepts a distinct-source fixture and rejects distinct volume names with a shared source, both ancestor directions, and a missing source. A separate test imports the unchanged verifier and confirms that its raw pre-stage diagnostic rejects shared and nested D/A sources. The retained participant sources are `/var/lib/docker/volumes/t19vf-fix1-direct-ledger/_data` and `/var/lib/docker/volumes/t19vf-fix1-adapter-ledger/_data`; neither equals nor contains the other.

## Checkpoint and verification

The original Task 1 patch hash is `91684377c87cc218fb6e84548200d310c9077864a1230c299cb2e4a5d71608d7`; the Fix1 incremental patch hash is `1a2a7cc39fc50e5e5aef3ab0e1cbf1c7db88b54e71a9641ad72df7fbc4ae0864`. Both applied with `patch -p1 --batch --forward` and exit 0 in a fresh temporary directory. All original Task 1 postimage hashes for files untouched by Fix1 and all nine Fix1 postimage hashes matched the replayed bytes. All nine Fix1 hashes match current bytes. The original Task 1 test hash differs from the current test only because Fix1 changed that file, as expected. The retained verification JSON binds the current test and evidence hashes to six passing focused unit tests, successful Python compilation, and the unchanged verifier mutation suite.

**Spec compliance: PASS for the narrow VF-1/VF-2 repair. Code quality: PASS for the narrow repair.** The derivative remains a local fixture. This disposable `none`-network probe does not satisfy the full fixed E0 network/control topology; source event joins, physical OpenCode attempt provenance, runner adoption, and full E0 acceptance remain separate gates. No full E0 PASS is claimed.
