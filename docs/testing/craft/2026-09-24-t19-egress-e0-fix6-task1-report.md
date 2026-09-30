# Craft #107 T19 E0 Evidence Fix6 Task 1 Report

Date: 2026-09-24. Scope is limited to `assert_v2.py`, `test_assert_v2.py`, and `test_source_owned_v2.py`. No runner, production code, Docker, or measured E0 artifact was changed or run. The source-owned recorder test file is unchanged. E0 remains **BLOCKED**.

## RED evidence and corrections

The two independent review false-PASS paths were reproduced against the Fix6 preimage using full synthetic fixture copies with raw evidence hashes refreshed:

- A helper was given D's writable recorder volume mounted at `/stolen-evidence`; the old verifier still returned synthetic PASS.
- `direct_ip_pre` was changed to `phase_id: null` and its ordinal swapped with `baseline_ip` to keep the global ordinal set unique/contiguous; the old verifier still returned synthetic PASS.
- The missing source helper-start acknowledgment control also returned synthetic PASS before the boundary requirement was added.

The verifier now requires the helper's complete raw mount inventory to be exactly empty in both inspect snapshots. Any helper mount, including D's exact writable Docker volume under a different destination, fails.

Every fixed case is assigned a verifier-owned position. Scheduled control/negative/normal/restart cases must carry their exact expected phase ID and lie strictly between that phase's begin and barrier ordinals. The fixed preflight set must be unphased and before the first begin. Fault cases must be unphased after the final scheduled barrier and before the seal. Cleanup must be unphased and after all removal and absence-inspection rows. Null, wrong, early, late, and cross-phase case rows fail.

## Source-owned helper-start contract

The controller operation log alone cannot establish when a listener observed helper start. The verifier therefore requires one `helper_start` event in each D and A source stream for every control phase, and no such event outside the fixed control schedule. Required event fields are:

```json
{
  "event": "helper_start",
  "phase_id": "control:<case>:before",
  "boundary": "helper_start",
  "operation_id": "<the referenced successful docker_start operation ID>",
  "controller_ordinal": 123,
  "active_connections": 0,
  "active_requests": 0
}
```

The ordinary source identity, boot ID, contiguous `seq`, wall time, and monotonic time fields also apply. Each recorder must append and acknowledge this event after the successful fixed helper-start operation and before control traffic is enabled. For each source stream, the verifier requires `phase_begin.seq < helper_start.seq < phase_barrier.seq`, ties the event's operation ID and controller ordinal to that phase's raw helper-start command, requires a drained boundary, and rejects any control-phase `tcp_accept` whose source sequence precedes the boundary. Event file order/timestamps or the controller log cannot substitute for this acknowledgment.

The current producer does not emit this event or expose a source command/ack interface that can append it after the helper starts. This is an exact producer gap; the verifier returns BLOCKED for missing/duplicate/mismatched boundary evidence. The interface change belongs to a later runner/recorder task, outside this assignment.

## Verification

- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed. Synthetic parser fixture is labelled synthetic; 66 evidence mutations were rejected, including both Fix6 reproductions, absent helper boundary, and direct control traffic before the helper acknowledgment. This is not measured E0 evidence.
- `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` — 15 tests passed.
- `python3 -m py_compile docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py docs/testing/craft/egress-probe/test_source_owned_v2.py` — passed.
- Owned-file trailing-whitespace scan — passed.
- Exact patch reconstruction against the captured preimage — passed; `patch -p1` exited 0 and recreated all three postimage hashes. No commit was made.

## Remaining blockers

1. `run_v2.py`/`server_v2.py` do not produce the source-owned helper-start record above. Current evidence remains blocked until the producer interface writes and acknowledges it in both D and A streams.
2. The runner schema is still incompatible with the Fix5/Fix6 controller ordinal, operation ID, isolated recorder-volume and inspect requirements.
3. Independent pinned OpenCode selected-provider/actual-network-attempt provenance is still unavailable; live evidence without it is unconditionally BLOCKED.
4. Fixed raw runtime command/result reconstruction for effective identity, binary version/hash and route inventory, and full route/interface/gateway reconstruction remain incomplete as listed in the Fix5 report.

This checkpoint only repairs the named synthetic false-PASS paths and defines the missing source boundary contract. It does not release Task2 or establish E0 PASS.
