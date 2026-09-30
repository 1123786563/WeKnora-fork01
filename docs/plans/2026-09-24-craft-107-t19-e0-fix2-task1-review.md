# Craft #107 T19 E0 Fix2 Task 1 — independent review

Date: 2026-09-24. Scope: the Task 1 uncommitted recorder/verifier checkpoint, the Fix2 plan and E0 architecture audit, prior E0 reviews, approved Craft Spec, T19 design decisions, CONTEXT.md, Task 1 report, checkpoint and patch. Read only as to requirements, probe source/tests, measured evidence and remote state. No OCR or Docker rerun.

## Checkpoint and checks

The captured preimage plus saved patch reconstructs server_v2.py, assert_v2.py, test_assert_v2.py and the new test_source_owned_v2.py with `patch -p1` exit 0. Their reconstructed and current SHA256 values equal the checkpoint: `a286d1c332e23004ce110e3b9762749d039b8998036555f93dff15498d0ef04a`, `fd64ef774c7cc643accfde1f0852b367bf9af0c7ffc659ec410602fd4630dcef`, `a69112a5493835b7a7ee22a29542fd1cb4ffcc1fc709f001aa3a4a3c6d28b16b`, `e451a4ee5e58affe7abf44a83c9cfb0c15e7bf85673bc277106a0531fa33e3ba`. Patch SHA256 is `397b3ac5fca3488923be040137d265ccabee80618db90ac840de5d08d7dabab8`; unchanged run_v2.py is `ae7350469ef22c316bd78b86f9094aaed6de23a18a9d22540a596723fc03915c`.

Independent runs of `python3 docs/testing/craft/egress-probe/test_assert_v2.py` and `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` exited 0; the latter ran 10 tests. The full synthetic fixture passed. Mutations reject the six prior false-accept classes (unowned/forged direct event, BLOCKED verdict, absent actual normal output, writable client state and timeout), plus untagged GET, CONNECT, malformed/TCP-only arrivals, sequence/seal/cursor drift, command timeout, config drift, missing inspect participant and failed removal. This is regression evidence for those mutations, not proof of the full fixed schedule or live Docker provenance.

## Findings

### F1 — High — extra HTTP requests on one direct connection disappear from control comparison

Evidence: server_v2.py:277-278 uses HTTP/1.1, and :194-201 accepts multiple requests per open connection. In assert_v2.py:218-225 each request_start replaces `conn["request"]`; `_verify_direct_phase` at :400-419 compares only the final request on each connection with the host-supplied expected list. A completed unapproved first GET or CONNECT followed by the expected request on that connection can therefore match the expected control cardinality. Impact: a recorded bypass in a control window can be accepted, violating whole-stream accounting. Smallest correction: retain and validate every ordered request/error per connection, compare all direct observations to verifier-owned control expectations, and test two keep-alive requests under one TCP accept.

### F2 — High — required control windows and client stop proof are self-declared

Evidence: assert_v2.py:171-179 permits any acknowledged phase sequence. `_verify_direct_phase` at :390-419 accepts `expected_direct`, `client_stopped`, `helper_active` and `helper_id` from the same host begin row that claims the control. No fixed mapping requires each negative case and its pre/post same-sink control phases, nor does the verifier bind a control to successful stop/wait and same-container inspect with Running=false, Restarting=false and Pid=0. It does not bind observed peer to the inspected helper. Impact: missing control pairs, a wrong helper, or C running during a control can still produce internally matching records. Smallest correction: encode the exact phase/case schedule in verifier code; bind each control to raw stop/wait/inspect and helper identity; mutate absent pre/post, forged stopped flag and wrong peer while summaries remain PASS.

### F3 — High — host/config/topology/cleanup checks cannot establish the required facts

Evidence: assert_v2.py:285-384 checks case-row existence and file/hash agreement but not fixed per-case argv, target or environment; config bytes are compared with a host-supplied hash and free-text URL trace, without parsing config semantics, verifying a successful in-container hash/load command, or matching actual attempted URL and network failure. New inspect validation checks participant presence and a few client flags at :342-367, but not full network membership, all mounts and ancestor aliases, A-to-D write isolation, restart identity, route/sysctl facts or Docker command success. Cleanup resources are sourced from the manifest, while arbitrary nonzero inspect output containing “not found” suffices at :369-381. Legacy manifest topology/restart booleans remain at :503-565. Impact: a false PASS can mask a wrong route/config, shared writable recorder root, incomplete cleanup or wrong restart identity. Smallest correction: make fixed case specifications drive command/target/environment validation; parse raw Docker inspect/network and successful command results for all topology facts; bind config bytes to successful in-container hash/load and actual target attempt; derive cleanup inventory from successful creation records and verify exact removal plus specific not-found results after source seals. Add raw-fact mutations with summaries unchanged.

### F4 — Medium — final seal precedes listener shutdown

Evidence: server_v2.py:171-181 appends stream_end and sets sealed before on_seal; :472 sets on_seal to listener shutdown. get_request at :257-265 can accept a socket during this interval, then RECORDER.accept raises because the stream is sealed. This post-seal exception does not set fatal_error. Impact: the final seal need not bound all actual accepts. Smallest correction: stop/join all accept loops, drain active connections, then fsync stream_end; poison on any shutdown/accept failure. Add concurrent connect-versus-seal coverage.

### F5 — Medium — request terminal state is under-validated

Evidence: assert_v2.py:218-259 does not validate generated request ID/ordinal progression, exactly one terminal response/error, or an error after response; closing a connection with a request object is allowed whenever its ID is no longer in the active set. The recorder generates IDs and terminal events, but the verifier does not enforce them on raw artifacts. Impact: duplicated or contradictory request records may count as complete observations. Smallest correction: enforce per-connection request ordinal and ID sequence, exactly one terminal event, terminal order and one close per accept; test duplicate and missing terminal mutations.

## Verdict and gate

**Spec compliance: FAIL for Task 1.** The separate D/A streams, private control socket, fsynced contiguous records and targeted RED closures are useful. F1-F3 retain false-PASS paths, and F4 leaves an unsealed accept interval. The admitted fixed-case verifier gaps are Task 1 obligations under the architecture audit and brief, not only Task 2 orchestration work.

**Code quality: changes required.** Focused tests pass but omit reused connections, missing control pairs, raw topology/config/cleanup forgeries and seal concurrency. The unchanged runner emits the legacy layout and cannot produce this schema. The earlier 566.736-second measured artifact remains BLOCKED; no E0 PASS, E1 release or model-egress enablement follows. Correct and independently review Task 1 before Task 2's live matrix. Observed 503/disconnect retries remain a separate unresolved protocol gate.
