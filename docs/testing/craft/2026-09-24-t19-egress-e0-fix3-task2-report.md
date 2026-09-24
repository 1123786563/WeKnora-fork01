# Craft #107 T19 E0 Evidence Fix3 Task2 Report

Date: 2026-09-24. Scope: verifier and synthetic regression suite only. The assigned files are `assert_v2.py`, `test_assert_v2.py`, and `test_source_owned_v2.py`. `run_v2.py` and production files were not edited. No Docker matrix or commit was run.

## Implemented seam

- The verifier owns an exact chronological schedule: initial direct-sink controls; paired pre controls around each fixed pre negative case; normal route; same-client restart checkpoint; paired post controls around each fixed post negative case; normal post-restart route and restore route. Both source streams must carry that exact sequence.
- Every direct control uses the fixed four tuples (gateway GET, provider GET, proxy absolute-form GET, proxy CONNECT) and the helper IPv4 address derived from raw external-network inspect. Host `expected_direct`, `client_stopped`, `helper_active`, and helper ID flags no longer authorize traffic.
- Control phases reference raw Docker stop, client inspect state, helper start/stop, and helper identity/peer records. Restart requires raw restart and post-restart inspect records bound to the same client identity. The aggregate raw inspect checks fixed private/external membership and network modes, exact client config/data volume pair, adapter forwarding sysctl, helper peer presence, and pre/post client ID/mount continuity.
- Fixed verifier-owned target and proxy environment constraints cover the pre/post direct-IP, DNS, forced-IP, custom-URL, and proxy curl cases. These checks still require the raw host command and summary argv to agree.
- Config evidence is no longer accepted from a free-text trace. It requires a strict JSONL process-start/config-hash, provider-selection, and network-attempt trace; retained config bytes must define the selected provider/model/baseURL and the attempt must target that origin with a connection failure.
- Cleanup inventory derives from successful typed resource-creation records and must cover exactly the client, adapter, direct/helper, private/external networks, and XDG config/data volumes. Removal argv and resource-specific Docker not-found diagnostics are checked.
- Added full synthetic mutations for missing paired control, forged stopped-client/helper identity, altered config selection/attempt, changed curl target/proxy env with summaries changed in lockstep, mismatched helper peer, mount escape, dropped inventory creation, and non-absence cleanup errors.

## Verification

- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — exit 0; the complete synthetic artifact passed; 49 adversarial mutations were rejected, including the new Task2 cases. This is synthetic-only evidence.
- `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` — exit 0; 15 tests passed.
- `python3 -m py_compile docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py docs/testing/craft/egress-probe/test_source_owned_v2.py` — exit 0.
- Task-local patch added-line whitespace scan — exit 0; 0 added lines contain trailing whitespace.
- Task-local patch applied to captured preimage with `patch -p1` in a temporary directory — exit 0; reconstructed owned postimage hashes match the checkpoint.
- `run_v2.py` remained read-only; SHA256 observed: `ae7350469ef22c316bd78b86f9094aaed6de23a18a9d22540a596723fc03915c`.

## Remaining blockers and limits

**E0 remains BLOCKED.** The unchanged runner does not emit the new source phase schedule, operation records, helper registration/identity evidence, aggregate pre/post inspect schema, or structured config trace. It must be adapted and independently reviewed before a live run. Specifically, the pinned OpenCode logs currently exposed by the runner are filtered free text and do not provide machine-verifiable process-to-config-load, selected provider/baseURL, or actual attempted target/outcome provenance. The verifier intentionally rejects that evidence; a schema-shaped synthetic trace does not establish that the real binary exposes the fields.

The operation artifacts do not yet provide a single shared ordinal/timeline proving each helper stop happened after its control barrier and each cleanup happened after both recorder seals. Complete raw participant/container security inspection (including all capabilities, binds/ancestor aliases, UID/image/network options) and fixed argv reconstruction for every non-curl case also remain incomplete. These gaps require the runner/recorder interface and evidence schema to be completed and reviewed; this report does not claim F2/F3 closure or E0 PASS.

## Checkpoint artifacts

- Checkpoint: `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task2-checkpoint.json`
- Task-local patch: `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task2.patch`
- Preimage: `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task2-preimage/`
