# T19 E0 Fix2 Task 1 report

Date: 2026-09-24 (Asia/Shanghai)  
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
Plan: `docs/plans/2026-09-24-craft-107-t19-e0-rootcause-plan.md` Task 1  
Checkpoint: `docs/plans/2026-09-24-craft-107-t19-e0-fix2-task1-checkpoint.json`

## Verdict

**DONE_WITH_CONCERNS — E0 remains BLOCKED; keep model egress default-off and do not dispatch E1.** The probe implementation and one 566.736-second disposable run captured positive route controls, successful SSE turns, restart identity, host-route controls, and the retry behavior of the exact pinned OpenCode build. OpenCode’s proxy and hostile-config turns timed out, and its redirected POST turns produced repeated adapter requests before timeout. The final verifier exits 1. This is not a pass or proof of no automatic retries.

The recorded run used the fixture version before the final verifier/runner amendments that add the post-restart host-mapped custom DNS URL attempt, repeat hostile project/global URL cases after restart, bracket the host-route row with host-facing controls, directly assert the adapter forwarding sysctl, and compare the global config hash across restart. Those omitted checks remain unmeasured; the final assertion reports missing post-restart cases instead of treating them as zero. No raw run artifact was rewritten to make it match the later script, and no second Docker run was used to claim coverage for the amended runner.

## Changes

- Added `run_v2.py`, `server_v2.py`, `assert_v2.py`, and `test_assert_v2.py` under `docs/testing/craft/egress-probe/`.
- Added retained per-attempt artifacts under `docs/testing/craft/egress-probe/e0-fix2/`. Earlier startup attempts remain separate and preserved. The full measured run is `craft-e0-20260923T191914Z-43540`.
- Added this report and the task checkpoint. The existing Fix2 report and all historical fixtures/logs remain unchanged.
- No production code, paid traffic, real credential, external service, commit, host firewall rule, or privileged container was used.

## Environment and measured evidence

The run manifest records Docker Engine `29.4.0`, `linux/arm64`, exact image ID `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`, OpenCode `1.18.4`, and in-container binary SHA256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`. Runtime inspection shows an internal private network with IPv4 gateway mode `isolated`, IPv6 disabled, client attached only to that network, adapter attached to private plus external networks, direct sink attached only externally, client unprivileged without added capabilities or published ports, and adapter `net.ipv4.ip_forward=0`. The client used UID 10001 and the declared XDG paths.

| Probe area | Evidence | Result |
| --- | --- | --- |
| Same-sink positive controls | Direct gateway/provider HTTP 200 by DNS and literal IP; absolute-form proxy HTTP 200 and a counted CONNECT; controls bracketed negative groups | Pass |
| Normal adapter route | Exact `probe-ok` SSE content; two POSTs (`title`, `task`), one session, `/v1/chat/completions`, `mock-model`, `stream:true` | Pass before and after restart; restore route also passed |
| Direct/DNS/forced IP | Direct IP and forced IP curl exit 7 / HTTP 000; unresolved gateway DNS exit 6 / HTTP 000 | Zero gateway/provider/proxy/host sink delta |
| Explicit proxy | Direct-IP and controlled proxy-name attempts exit 7 / HTTP 000, before and after restart | Zero external sink delta |
| Adapter as proxy | Absolute-form and CONNECT requests rejected; curl exit 56; adapter ledger recorded them | Zero external sink delta |
| Host/published port | Disposable host-published sink returned HTTP 200 from the external-bridge baseline before and after restart; isolated client tried host aliases, host gateway and published address | Positive controls passed; isolated route matrix failed with HTTP 000 and host sink delta 0 |
| Restart | Same client container ID, same XDG volume sources, persistent data marker | Pass for container/mount/marker identity; global config hash was not compared after restart |
| Fault 503 | Task POST received 503 followed by a second task POST with 200; process exited 0 | Automatic retry observed (2 task attempts) |
| Connection closed before headers | First task POST had no response status, followed by second task POST with 200; process exited 0 | Automatic retry observed (2 task attempts) |
| 307/308 task POST redirect | Curl follow control reached provider sink; OpenCode generated repeated adapter POSTs (5 task POSTs per case), external provider sink delta 0 | OpenCode turns hit 45-second timeout; retry/follow behavior not fully observed; blocked |
| Proxy environment in OpenCode | OpenCode did not reach adapter and timed out while logging API connection errors/retries; no direct sink traffic | Blocked; explicit proxy curl transport remained denied |
| Hostile project/global baseURL | OpenCode invocations timed out; no adapter or direct sink request was observed | Blocked because config selection/turn completion was not established by a completed process result |
| Cleanup | All registered containers, networks and named volumes absent after cleanup | Pass |

The full raw records, per-role append-only ledgers, inspect/network snapshots, stdout/stderr, cleanup inventory, manifest and hashes are under `docs/testing/craft/egress-probe/e0-fix2/craft-e0-20260923T191914Z-43540/`. The adapter ledger records retry counts directly; equal body hashes are not used as retry identity.

## Verification commands and results

- `python3 -m py_compile docs/testing/craft/egress-probe/{run_v2.py,server_v2.py,assert_v2.py,test_assert_v2.py}` — passed.
- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed: all eight malformed synthetic evidence cases returned nonzero; the compact synthetic pass and a complete labelled synthetic live-schema/ledger pass returned zero. Synthetic evidence is temporary and excluded from measured acceptance.
- `python3 docs/testing/craft/egress-probe/run_v2.py` — full disposable run completed within the 20-minute limit; manifest verdict `BLOCKED`, elapsed `566.736s`; all run-owned resources were removed.
- `python3 docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/e0-fix2/craft-e0-20260923T191914Z-43540` — exit 1, `BLOCKED`. Reasons include four unmeasured post-restart DNS/project/global URL cases, three pre-restart hostile-configuration and two proxy-environment timeouts, four redirected OpenCode timeout cases, missing XDG global config hash comparison across restart, and the unmeasured post-attempt host positive control.
- `git diff --check -- docs/testing/craft/egress-probe/run_v2.py docs/testing/craft/egress-probe/server_v2.py docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py` — passed.

Several earlier startup attempts are preserved in their own artifact directories: one failed on a double JSON decode, one exposed an IPv6-loopback classifier bug, and two exposed duplicate control labels. Their resources were cleaned. The measured run was executed after those corrections; these attempts are not combined with its counters.

## Remaining blockers and limits

1. OpenCode 1.18.4 retries a task after both a completed 503 and a lost response. Therefore an adapter cannot assume one inbound POST equals one physical attempt; no retry-disable contract or durable attempt identity was proven here.
2. Redirect, hostile URL, and proxy-environment OpenCode turns did not all finish within the 45-second cap. Their timeout is not evidence of no retry or safe successful completion.
3. The measured run omitted the post-restart host-mapped custom DNS URL and hostile project/global URL cases and did not compare the restored global XDG config hash after restart. The final runner and verifier now require these checks, but this recorded run does not supply that evidence.
4. The no-bypass observation is specific to this Docker Engine/OrbStack Linux ARM64 environment and the pinned image. It does not establish behavior for another daemon, runtime or architecture.

No production implementation scope was started. The verifier’s nonzero exit and these concrete protocol/evidence gaps keep E0 blocked and T19 model egress default-off.

## Task 1 source-owned recorder/verifier checkpoint (2026-09-24)

This append records the Fix2 Task 1 checkpoint; it does not change the measured run or its verdict. E0 remains BLOCKED. No Docker run was performed. `run_v2.py` was not edited and remains SHA256 `ae7350469ef22c316bd78b86f9094aaed6de23a18a9d22540a596723fc03915c`.

### Preimage and resulting hashes

The task preimage is retained under `docs/plans/2026-09-24-craft-107-t19-e0-fix2-task1-preimage/` with an index. Initial SHA256 values: `server_v2.py` `a18d7ece2cfdda8bb7f9df70a57e90f117a806aed445d38b522b723ad10bd6dc`; `assert_v2.py` `bcf412f3421d3e399eea27369ecd5b21d49bd715557accd10b19ec8e60158d69`; `test_assert_v2.py` `f16da2015643d2c6c16d731b4e69f6c655d1893a0f8f3dcdb7d0fb4b84f1483b`.

Task checkpoint SHA256: `server_v2.py` `a286d1c332e23004ce110e3b9762749d039b8998036555f93dff15498d0ef04a`; `assert_v2.py` `fd64ef774c7cc643accfde1f0852b367bf9af0c7ffc659ec410602fd4630dcef`; `test_assert_v2.py` `a69112a5493835b7a7ee22a29542fd1cb4ffcc1fc709f001aa3a4a3c6d28b16b`; `run_v2.py` unchanged as above.

### Implementation and evidence

- `server_v2.py` owns separate contiguous `direct` and `adapter` streams, exclusive per-process evidence roots, fsynced stream events, private Unix-socket host phase commands/acks, active connection/request barriers, phase-scoped controller ordinals, fatal append poisoning, TCP accept before handling, request-start before body read, body completion/hash observations, response-after-send, and close/error events. Caller case headers are observations only.
- `assert_v2.py` routes synthetic and measured artifacts through the same full verifier. It checks source schemas/seals/sequences/cursors, exact D/A source identities, host control expectations and adapter POST pair; it requires the fixed exact required case set, raw host command rows correlated to manifest case summaries and raw stdout/stderr, retained config bytes with hashes and process-load traces, pre/post-restart inspections of all four participants plus both networks, and cleanup removal plus independent not-found inspection of every registered resource. Raw streams, phase commands, host commands, inspections and cleanup are hash-bound.
- Adversarial tests now reject all six recorded Fix2 false-accept classes (BLOCKED manifest, absent actual normal output, writable `/state`, timeout; plus malformed/untrusted sink accept patterns), and mutations for untagged GET, forged control GET, CONNECT, malformed and TCP-only direct traffic, missing normal raw POST, sequence/boot/seal/cursor drift, host command timeout mismatch, config byte drift, incomplete participant inspection, and failed cleanup.

Commands and results:

- `python3 -m py_compile docs/testing/craft/egress-probe/server_v2.py docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py docs/testing/craft/egress-probe/test_source_owned_v2.py` — passed.
- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed; complete same-path synthetic full-schema artifact accepted, all mutation cases rejected, and quiescence checks passed.
- `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -v` — 10 passed, including real Unix-socket recorder control, partial HTTP body drain barrier, caller label handling, append poisoning and seal behavior.
- `git diff --check -- docs/testing/craft/egress-probe/server_v2.py docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py` — passed.
- The previous measured artifact recheck remains BLOCKED and was captured in `docs/testing/craft/2026-09-24-t19-e0-fix2-task1-measured-verifier.txt`; it lacks the new owned evidence schema and fixed post cases. No measured evidence is upgraded by this checkpoint.

### Compatibility boundary and remaining review risks

The unchanged `run_v2.py` emits the legacy shared ledger/manifest layout; it does not start the new source recorders or produce the host command/config/inspection/cleanup evidence files required by this verifier. Running it unchanged against the new verifier is therefore expected to fail closed. Task 2 must migrate orchestration and artifact export before any new live run.

The verifier still retains legacy ledger consistency checks and some topology/case-summary cross-checks. Although source streams now independently account for sink activity and raw inspect evidence is required, the existing runner does not provide the new evidence contract, so the entire experiment is not yet end-to-end reconstructable. The synthetic host evidence is protocol-shaped test input, not proof that Docker emitted the same observations. A raw host log also cannot independently prove the truthfulness of the host controller/daemon; this limitation matches the architecture report. No E0 PASS claim is made.

### Fixed-case verifier invariants not implemented in this checkpoint

The fixed case *names* are exact and case summaries are tied to raw process output/result rows, but this checkpoint does not implement the full planned schedule contract. In particular:

1. Raw host case rows are not checked against per-case expected argv semantics, target, environment, or a host-authenticated execution record; raw command rows are schema-checked and cross-checked against summaries only. Setup/inspect/export commands also are not reconstructed from this case log.
2. The controller phase log is not bound one-to-one to the fixed case schedule. Negative case phases and each corresponding pre/post same-sink control phase are not required by case ID; the direct-control event list is supplied by the host log and compared for equality, but its expected control matrix is not itself fixed in verifier code. No proof establishes C is stopped around every control beyond each present control command’s booleans.
3. Config evidence requires bytes, their SHA256, selected URL and a trace containing that URL, but it does not validate the config’s full semantic contents, the exact in-container config path/hash command, or a process-level trace format that independently proves the application loaded the bytes. The trace is text and host supplied.
4. Inspect evidence requires participant/network entries and checks basic client privilege/writable-bind constraints, but it does not reconstruct network membership and all mount sources/destinations/ancestor aliases, prove A cannot write D evidence, validate D/A attachment sets from raw Docker JSON, compare container IDs/mount identity across restart, or reconstruct IPv6/routes/sysctls from the aggregate evidence. Several related assertions still read legacy manifest summaries or older client/adapter snapshots.
5. Cleanup requires a remove row and a not-found row for each manifest-registered resource, but does not derive the resource inventory independently from raw creation/registration commands, validate exact Docker argv/IDs, or require source stream seals before cleanup. Absence is based on the row’s nonzero exit and stderr text.
6. Legacy per-role ledgers, counters/ranges, and manifest topology/case summaries are still parsed by the verifier. They are cross-checks and are not a substitute for the new streams, but removing this legacy dependency and completing raw topology reconstruction remains necessary for the planned strict independent verifier boundary.

These are explicit remaining Task 1 gaps for independent review. Runner compatibility is also pending Task 2. Do not run a live E0 matrix or claim E0 PASS from this checkpoint.

### Exact review checkpoint packaging

The exact file hashes and remaining fixed-case invariants are recorded in `docs/plans/2026-09-24-craft-107-t19-e0-fix2-task1-checkpoint.json` under `task1_exact_checkpoint`. The saved patch SHA256 is `397b3ac5fca3488923be040137d265ccabee80618db90ac840de5d08d7dabab8`. It applies cleanly to the captured preimage plus the previously absent recorder test and reconstructs the final SHA256 values for `server_v2.py`, `assert_v2.py`, `test_assert_v2.py`, and `test_source_owned_v2.py` exactly. `run_v2.py` remains unchanged. Independent review should bind to these checkpoint hashes.
