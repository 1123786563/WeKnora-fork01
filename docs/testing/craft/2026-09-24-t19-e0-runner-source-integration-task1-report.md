# T19 E0 Runner Source Integration Task1 Report

**Status:** source-helper boundary smoke PASS; E0 remains BLOCKED.

## Scope

Changed only `docs/testing/craft/egress-probe/run_v2.py` and added `docs/testing/craft/egress-probe/test_runner_source_integration_v2.py`. The runner now has a private D/A evidence-volume topology, no recorder mounts on the helper/client, exact source control commands with echoed controller identity/cursors, a raw successful helper `docker start` operation record, and a one-phase disposable smoke path on an internal Docker network.

The runner helper-start sequence records the successful host `docker start` plus inspected container ID and peer IP, fsyncs that operation row, obtains and validates the D and A `helper_start` acknowledgments, then dispatches probe traffic. A one-sided/mismatched acknowledgment raises before traffic dispatch. D and A recorder volumes are distinct; the probe helper and client do not mount either. The smoke recorder containers run as UID 0 with all Linux capabilities dropped and `no-new-privileges`: the pinned image's default UID 10001 could not chmod the fresh Docker volume during recorder initialization. The sources remain isolated by separate private volumes and the client/helper receive no source volume.

## TDD and Verification

- RED: `python3 docs/testing/craft/egress-probe/test_runner_source_integration_v2.py -q` failed before implementation with four expected missing-interface errors (`start_helper_and_ack` and source-mount helpers absent).
- GREEN: `python3 docs/testing/craft/egress-probe/test_runner_source_integration_v2.py -q` — PASS, 4 tests.
- `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` — PASS, 26 tests.
- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — PASS; synthetic full-schema control accepted and the listed adversarial mutations were rejected, including Fix6 early/missing helper-boundary controls.
- `python3 -m py_compile docs/testing/craft/egress-probe/run_v2.py docs/testing/craft/egress-probe/test_runner_source_integration_v2.py` — PASS.
- Whitespace check over the owned runner/test sources — PASS.
- `python3 docs/testing/craft/egress-probe/run_v2.py --source-boundary-smoke` — source boundary PASS, E0 BLOCKED, all owned Docker resources absent after cleanup.

Smoke artifact: `docs/testing/craft/egress-probe/e0-fix2/boundary-smoke-craft-e0-20260924T021935Z-45023/`.

- Raw Docker network inspect records `Internal: true`; D and helper share only this disposable internal network, while A and idle C use `network=none`.
- The source streams both record the same helper operation ID and controller ordinal 4 at cursor 3. D's `tcp_accept` is seq 4, after its `helper_start` seq 3; A contains the paired helper boundary but no TCP accept.
- Controller phase events are begin ordinal 3, helper start operation ordinal 4, barrier ordinal 5, helper stop ordinal 6, and seal ordinal 7. Source cursors are retained in the manifest.
- The raw helper-start operation is `docker start <owned helper name>` with exit 0, inspected container ID and peer IP. Cleanup absence inspection passed.
- Smoke manifest SHA-256: `df20c5e21046936e6f5ea4ca351f2a91bac5dcfb06237f82d1d85e880fda4f36`.
- Direct source stream SHA-256: `69d4b02a3676d625740f286062ea555d784d4ca9ea9f26ead10a0765b7972f3e`.
- Adapter source stream SHA-256: `83c8265a36f1d3e04ceb608bdcb13cddc02838d2bf43315fa2aa09dd9e6cf88f`.

Running `python3 docs/testing/craft/egress-probe/assert_v2.py <smoke artifact>` exits 1 with verdict BLOCKED, as expected: this is one boundary smoke and intentionally lacks the fixed matrix, complete OpenCode provenance, and full E0 host/cleanup evidence. The smoke's `source_boundary_smoke=PASS` is not an E0 PASS claim.

## Remaining Limits / Blockers

- Existing normal `main()` control helper calls still use the prior ephemeral `docker run --rm` path rather than routing every fixed control through `run_source_control_phase`; the full matrix producer remains incomplete and must not be treated as E0 evidence.
- The smoke is not a substitute for the complete pinned OpenCode selected-provider, physical-attempt, route, restart, and cleanup reconstruction requirements. No full E0 run was attempted.
- A separate recorder owner is aligning phase begin/barrier/seal identity fields; this runner sends the agreed `{op, ordinal, phase_id, operation_id, controller_ordinal}` fields and validates echoed operation/controller identity and cursor.

## Commands and Files

The exact owned file preimage/postimage hashes, patch replay evidence, smoke hashes and command results are bound in `docs/plans/2026-09-24-craft-107-t19-e0-runner-source-integration-task1-checkpoint.json`. The replayable patch is `docs/plans/2026-09-24-craft-107-t19-e0-runner-source-integration-task1.patch`.
