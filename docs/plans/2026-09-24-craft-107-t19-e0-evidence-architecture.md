# T19 E0 evidence ownership and bounded live gate

Date: 2026-09-24. Status: fresh-context read-only architecture audit after two evidence-trust review failures; implementation recommendation, not E0 acceptance. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Observed HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

Scope: only this report was added to the repository. No production, fixture or test source edits, Docker run, commit, remote mutation or child agent. Temporary synthetic artifacts were used to reproduce verifier false accepts. Existing work belongs to other owners and was preserved. Skills: systematic-debugging for tracing/reproduction; codebase-design for the proposed recorder/verifier interface.

## Decision

E0 remains BLOCKED, model egress default-off, E1 gated. The next correction must replace case-attribution authority and verifier reconstruction together. Removing one client mount or adding another manifest boolean cannot fix the observed failure class.

Keep the disposable two-network experiment. Introduce a small evidence module with source-owned event streams and host-only phase commands. Treat every client header, body, stdout fragment and filesystem value as an observation to correlate, never as authority to omit a sink event. A sink event outside an explicitly proved control window is a failure, including events with no case, forged case, unexpected method or only a TCP accept.

The claim is bounded: independently reconstructable observations under the pinned image and inspected Docker environment. The fixture cannot cryptographically establish honesty of the host/controller/operator, prove all future egress behavior, or establish T19 physical-attempt accounting.

## Sources and current version

Read the original E0/Fix1 reviews, `2026-09-24-craft-107-t19-e0-fix2-task1-review.md`, `2026-09-24-craft-107-t19-e0-fix1-review.md`, E0 next-task/model-egress designs, root-cause plan, Fix1 plan/report and Fix2 Task 1 report. The approved design makes WeKnora authoritative for spending and keeps unknown retry identity unresolved. No fact from different live attempts is combined.

| File under `docs/testing/craft/egress-probe/` | SHA256 audited |
| --- | --- |
| `run_v2.py` | `ae7350469ef22c316bd78b86f9094aaed6de23a18a9d22540a596723fc03915c` |
| `server_v2.py` | `a18d7ece2cfdda8bb7f9df70a57e90f117a806aed445d38b522b723ad10bd6dc` |
| `assert_v2.py` | `bcf412f3421d3e399eea27369ecd5b21d49bd715557accd10b19ec8e60158d69` |
| `test_assert_v2.py` | `f16da2015643d2c6c16d731b4e69f6c655d1893a0f8f3dcdb7d0fb4b84f1483b` |

Live evidence is still limited to the earlier 566.736-second BLOCKED matrix and the later approximately 75-second setup failure. Neither exercised the final Fix1 source end to end. Known 503/disconnect automatic retries remain a protocol gate independent of evidence infrastructure quality.

## Root cause and reproducible evidence

Single hypothesis: evidence ownership is assigned by file path, while the verdict still delegates authority to client-controlled attribution and runner-created summaries. Therefore a real sink event can disappear from the acceptance calculation without disappearing from the raw file. Source inspection and mutation reproductions confirm this.

| Failure seam | Evidence | Consequence |
| --- | --- | --- |
| Writable control state | `run_v2.py:390-393,433` creates mode-0777 artifact directories and mounts `/state` writable into C. `server_v2.py:78-91` reads the shared mode file for both direct and adapter requests. | C can change current case/mode, remove state, or race attribution. Removing `/ledger` closed only one write path. |
| Request-selected attribution | `server_v2.py:90` lets `X-E0-Case` override state. `assert_v2.py:214-260` relies on these labels and permits `control:*` events. | A bypass carrying a control label or no label escapes required-case zero checks. |
| Shared writers | `run_v2.py:416-417` gives adapter and direct sink the same writable ledger and state roots. | Adapter failure can corrupt direct evidence; the direct sink has no exclusive ownership of its own observations. |
| Summary-as-proof | `assert_v2.py:66-83,129-170,261-264` accepts status, completion, host controls, configuration trace and cleanup assertions without reconstructing required command results. | Successful-looking summaries can coexist with timeout, missing content, missing process trace or failed controls. |
| Counter/sequence confusion | `server_v2.py:25-37` uses one sequence across four direct role files, while runner `counts()` at `:188-192` excludes TCP accepts. Verifier compares range lengths with tagged counts, not actual raw slices. | Gaps are indistinguishable from other-role records or removal; equal-length shifted windows can be accepted. Redirect code at `run_v2.py:565` compares raw `seq` with an HTTP-only counter. |
| Incomplete quiescence | `run_v2.py:67-99` searches argv for `opencode` only; stable counters omit TCP accepts. | An unnamed child or an accepted partial HTTP connection can survive the check, write later, or contaminate control windows. |
| Request start/completion loss | `server_v2.py:57-62,141-146` reads the body before HTTP recording; `:179-180` records complete before sending SSE; `send_error()` swallows append failure. | An accepted stalled body is invisible to HTTP counts; an unsuccessful response can be labeled complete; write failures can silently lose observations. |
| Circular test oracle | The compact synthetic branch uses a different validator. Full-schema positive fixture contains almost no real controls or output and mostly zero windows. The missing-trace mutation actually changes URL consistency. | Passing tests validate fabricated summaries and selected checks, not completeness of the evidence contract. |

Reproduction executed with `python3 -B` and `runpy.run_path('docs/testing/craft/egress-probe/test_assert_v2.py')`. The existing suite passed. For each mutation, copy its returned `full_fixture_root` into a fresh temporary directory, apply exactly one mutation, and call its imported `assert_v2.verify(root)`. All results below were `(True, [])`:

1. Append provider event `{event:http, seq:3, boot_id:synthetic-boot, role:provider, method:GET, path:/actual-bypass, status:200, complete:true, case:null}`, retaining the fixture run ID.
2. Append that same event with `case:control:forged:provider:dns`.
3. Set final `manifest.verdict` to `BLOCKED`.
4. Replace normal-route stdout with `fatal: no model completion` while leaving claimed `task_content` untouched.
5. Add raw `snapshots/pre-syn-client-inspect.json` with a writable `/state` mount.
6. Set the normal-route case to `exit_code:null, timed_out:true` while retaining `status:pass`.

These are verifier reproductions, not an actual network bypass demonstration. The false attribution path is nevertheless concrete: a writable mode file or request header supplies precisely the ignored/misclassified labels used by the reproductions.

## Minimal ownership model

Trusted for this experiment: host controller, Docker daemon observations and pinned recorder code. Untrusted: protected client C including arbitrary children, project/HOME data, headers/body/case names, arbitrary process output. Adapter A is controlled fixture code but must have no write access to direct sink D evidence. A compromise of host/daemon is outside this experiment.

| Data / capability | Sole writer | Reader / exposure |
| --- | --- | --- |
| `direct/events.jsonl` | D recorder process | Host verifier; unavailable to C and A |
| `adapter/events.jsonl` | A recorder process | Host verifier; unavailable to C and D |
| Phase/mode command channel | Host controller | Corresponding recorder via a Unix socket in its private mount, never via model HTTP ingress |
| Command records, inspect snapshots, raw stdout/stderr | Host controller | Verifier; never mounted into C |
| Project config and XDG config/data | C, plus explicit host setup via `docker cp` | Retained host copies and in-container hashes for correlation; no sink-control authority |
| Manifest/report | Host controller | Index and claimed verdict only; verifier checks it against raw records |

Separate recorder-owned directories instead of shared `/state` and `/ledger`; grant only the recorder UID write access. Do not describe ordinary append-mode files as filesystem-enforced immutable: append-only is a process/ownership contract within this threat model. Prevent C/A writes by mount inventory, distinct directories and no shared parent bind. Hashes detect artifact drift after capture, not a malicious host rewriting all evidence.

Remove C's `/state` and the broad `/probe` fixture-directory bind. The latter is read-only today but includes every nested live artifact and would expose future control secrets/state. Copy the precise OpenCode config into C; mount only the specific read-only server file into A/D. No Docker socket, host PID/network namespace, privileged flags or extra capabilities. Verify *all* mount sources and destinations, including ancestor paths and aliases, instead of testing only whether `/ledger` exists. A failed `touch /ledger/...` proves little when a parent path is absent.

## Recorder interface and raw schema

One stream for D across gateway/provider/proxy/host ports, one stream for A. Per-role reports are derived views; they are never a competing source of truth. Each stream has exclusive writer, fixed run ID, boot UUID, contiguous `seq` starting at 1, source name, wall time for diagnosis and monotonic time for local ordering. Event order is authoritative; wall clocks are not compared across processes for causal attribution.

Expose only `begin_phase(phase_id, mode)`, `barrier(phase_id)` and `seal()` on the host-only control socket. Commands carry monotonic controller ordinals and reject reuse/out-of-order transitions. Each acknowledgment is appended and fsynced under the same recorder lock as network events before returning its stream cursor. A manifest cursor alone cannot create a phase. Fault mode and one-shot counters live in the A process, changed atomically by this interface; no handler writes a shared JSON file.

Required events:

- `stream_start` includes source identity, listened ports and initial mode.
- `phase_begin` / `phase_barrier` include exact host-selected phase ID, control/negative classification and active connection/request counts. The same phase ID appears in both streams and the controller command log.
- `tcp_accept` allocates a source-owned connection ID before handing the socket to a handler. Record peer/local address and port and the current phase at accept time.
- `request_start` records request ordinal/connection ID, method, path and raw caller case header as `untrusted_case_header`. Record it before reading a possibly incomplete body. Caller values never overwrite source phase or source identity.
- `request_body` records parsed model/purpose as observations, length/hash and whether the body completed. Apply bounded size/read deadlines; no silent truncation of oversized body evidence.
- `response_sent` only after the write/flush succeeds; `request_error`, `parse_error` and `connection_close` cover failure and EOF paths. Log malformed/non-HTTP connections too. Do not silently ignore append errors: stop accepting traffic, retain a fatal error and fail the run.
- `stream_end` states final sequence, total events and zero active connections. Its fsynced bytes are copied/hash-bound after writer termination. Missing end, restarted recorder, gaps, duplicates, invalid JSON, leftover bytes or an inconsistent connection state machine invalidate the artifact.

Keep connection-origin phase on late request/body/close events even if a new controller phase was attempted. Cross-phase active connections prevent transition. No request is attributed by a header, prompt token, session ID, model purpose or a manifest counter. Hash chaining is optional defense against accidental interior edits; contiguous records, ownership and a final sealed length/hash are the required minimum. Neither supplies authentication against the host itself.

## Case scheduling, controls and quiescence

Use one sequential schedule. Controls run from one explicitly named inspected helper on external network X, registered in the resource inventory. Do not use untracked `docker run --rm` helpers that may outlive a timed-out CLI. Keep the helper stopped outside control phases.

For each negative group: C stopped and verified; D/A drained; open a dedicated pre-control phase; start helper and execute the exact same-sink controls; stop/wait helper; drain; close that phase; begin negative phase on both recorders; start the same C and inspect it; execute one bounded case; preserve command exit/output; stop/wait the entire C even when the command returns normally; drain; close negative phase; run the matching post-control phase with C still stopped. This excludes late C children from control attribution. Extra controlled starts/stops must be recorded; preserve the explicit required restart checkpoint and same container/volume/config identity rather than silently claiming an uninterrupted session.

If preserving a continuously running C becomes an essential protocol requirement, design and validate a process/cgroup supervisor separately. Do not substitute name-matching `/proc` checks or stable samples for process ownership.

Stopping C uses bounded `docker stop --timeout 2`, then a bounded kill/wait path if necessary; retain exact Docker results and inspect the same immutable container ID with `Running=false`, `Restarting=false`, `Pid=0`. Failure to prove this aborts the matrix. Docker documents stop escalation to SIGKILL and the wait command's termination observation: [stop](https://docs.docker.com/reference/cli/docker/container/stop/), [wait](https://docs.docker.com/reference/cli/docker/container/wait/).

Recorders enforce finite socket/header/body deadlines and track all active connections. After senders stop, require zero active connections plus a bounded settle interval on **all-event** cursors. Record samples and barriers, never HTTP-only counts. Any late event remains evidence, and any unexpected D event fails globally even outside a case. If drain times out, the current and remaining cases are blocked; no next control starts. A finite quiet interval proves the specified observation window, not absence of arbitrary future packets; global event accounting through final seals covers later arrivals in this run.

A command timeout always remains BLOCKED, even after successful cleanup/quiescence. Stop and finalize that run rather than continuing to mix later controls into it. Transport-denial rows may accept a completed nonzero process only when raw error evidence establishes a network attempt to the intended target. CLI/config parse errors and missing binaries never prove denial.

## Independent verifier contract

`verify(root)` uses one full schema for both real and synthetic artifacts. Synthetic is provenance that makes a result ineligible for live acceptance; it must not select a simpler validation implementation. Fixed verifier-owned case specifications define every expected command category and observation. The manifest cannot define which required cases or event types to ignore.

1. Validate exact schema, required case set and unique command/phase IDs; include `proxy_opencode_pre` and `proxy_opencode_post`, which the current required set omits. Reject missing/unknown verdict, manifest BLOCKED, any required timed-out command, missing evidence or contradictory summary. Restrict artifact paths to the root and reject symlink/path traversal. Verify stored hashes against bytes, including fixture hashes frozen before launch.
2. Reconstruct topology from mandatory Docker raw inspect/network snapshots both before and after restart. Read UID, network mode/attachments, complete mounts, privileges/capabilities, ports, sysctls, bridge settings, image ID/platform and restart identity from those records, not manifest booleans. Require runtime command success for binary hash/version and route facts.
3. Parse both source streams, check contiguous identity/order and connection state transitions; reconstruct windows from acknowledged phase markers. Every raw event is classified exactly once. No ignored unknown labels, unknown event kinds, unexplained TCP accepts, events outside phases, changed boots or missing seals. Validate any reported deltas against exact raw slices, including events of every role and type.
4. A D network event is allowed only inside an explicit successful control phase while C is proved stopped and the registered helper is active. Match listener role/port, source/helper evidence, exact method/path, response status and expected event cardinality. Check every DNS/IP/HTTP/CONNECT/host and redirect positive-control instance separately, before and after its paired group. A `control:*` request string cannot whitelist anything. No generic one-control-per-run rule.
5. Negative windows require zero D TCP accepts as well as zero HTTP/parse traffic. Any unmatched D event anywhere fails globally. Adapter-as-proxy requires both actual rejected absolute-form and CONNECT requests, plus no D traffic; a matching stderr substring alone is insufficient.
6. Normal pre/post/restore turns require recorded actual command exit 0, no timeout, task output `probe-ok` in the retained process stream, and exactly one title plus one task POST with expected path/model/stream/session observations in the corresponding A raw window. Check response-send completion as well; current intent-to-send flags are insufficient.
7. Hostile project/global cases require retained exact config bytes, independently recomputed hash matching the actual successful in-container hash command, correct selected provider/model, process config-load path and actual attempted URL/network failure in raw logs or a separately justified process trace. No copied `effective_config_trace` string is authoritative. If pinned OpenCode does not expose selected URL, record that missing seam as BLOCKED; do not assume argv or a file alone proves use. Match before/after restart cases and restore hashes independently.
8. Redirect phases require actual A POST and 307/308 response plus the exact helper POST-follow control reaching D in its own control phase. Fault phases derive attempts and ordered outcomes directly from A events; do not use body equality to infer identity or label a retry-safe outcome.
9. Reconstruct cleanup from named registered resources and successful removals followed by specific not-found observations. Daemon unavailable/permission errors are not evidence of absence. Require recorder final seals before removal and a final complete inventory.

The verifier computes `evidence_integrity`, `network_observation` and `retry_protocol` results separately for diagnosis, with one conservative overall verdict. Do not quietly turn measured automatic retries into permission for E1. Existing root-cause plan and independent review keep uncontrolled retries as an architecture/protocol gate; a change in that gate needs an explicit recorded ruling, not a fixture condition change.

## Focused RED tests before another live run

Replace the permissive full-schema fixture with one explicit chronological artifact containing every required control, process result, inspect snapshot, config observation, phase marker and socket close. Generate it through the same recorder interface where feasible. Real and synthetic paths call the same verifier. Then mutate one fact at a time and assert rejection for that specific reason:

- All six reproduced false accepts above; direct GET, CONNECT, malformed bytes and TCP-only accepts with absent, unknown or forged control headers.
- Delete one pre/post control, change helper peer/role/method/status, or reuse another control's evidence.
- Shift windows without changing lengths; delete interior or final event; duplicate sequence; change boot/source; truncate final JSON; remove seal.
- Change actual raw client mount/network inspect while summaries say safe; expose an ancestor bind; grant A write access to D evidence.
- Remove only actual hostile log trace, with URL values still consistent; remove config bytes/hash command; replace network failure with CLI parse failure.
- Delete normal output/content or alter raw exit/timeout; remove post-restart POSTs; omit proxy environment cases.
- Exercise real recorder sockets with a partial HTTP body and delayed connection close; prove barrier refusal until drained. Exercise an actual separately named child sender in a disposable client for the whole-container stop test; a mocked alive predicate is not that evidence.
- Crash a recorder or force append failure and prove no PASS/healthy barrier can be emitted.

Infrastructure acceptance requires independent review of ownership and these tests before spending a full live matrix. Tests cannot establish the live E0 verdict.

## Bounded live gate and dispatch seam

One next full disposable run only after the changed fixture checkpoint is reviewed; exact script hashes frozen before launch and rechecked before verdict. Existing image/architecture/version pins remain unchanged. No paid providers, real keys, production services, shared DB, host firewall changes or privileged containers. Use only run-owned mock endpoints and resources.

Use a monotonic total budget of 20 minutes **including cleanup**: reserve 120 seconds for stop/drain/export/inventory; execution stops by 18 minutes. Cap ordinary curl at 3 seconds, OpenCode turns at 45 seconds and drain at a documented small limit (for example 8 seconds). Compute worst-case planned operations before launch; refuse an over-budget schedule. A watchdog outside the runner enforces the final bound and preserves failed cleanup as BLOCKED. Current `remaining()` floors at 0.1 and permits work/cleanup after deadline, so it is not a hard total bound.

Run cheap gates first: exact image; raw topology/mount ownership; recorder control/append health; same-sink controls; one normal completion; one hostile-config selection observability check. Abort on the first missing prerequisite. Then complete the entire specified pre/post restart matrix, actual proxy-env OpenCode cases, host/IPv6 checks and bounded 503/disconnect observations. No reruns concealed in the same artifact, no unmeasured source amendments after capture and no joining earlier artifacts to fill missing cases.

The network design still needs both inspection and behavioral checks. Docker states containers on one bridge can communicate, and `isolated` gateway mode requires `--internal`; it avoids assigning an address to that bridge. This supports the current two-network proposal but does not prove all OrbStack host aliases or daemon settings behave as intended: [bridge driver](https://docs.docker.com/engine/network/drivers/bridge/), [gateway modes](https://docs.docker.com/engine/network/port-publishing/). Require C only on P; A on P and X with forwarding disabled; D/helper only on X; no management socket reachable from C; no host networking or shared network namespace; IPv6 disabled/no non-loopback route. Retain same-sink host-published controls and actual target resolution. Unknown platform behavior stays BLOCKED.

After final seals and cleanup, run the independent verifier on that one artifact and retain its output/exit. Obtain independent Spec and quality review bound to identical source/evidence hashes. Infrastructure PASS with live BLOCKED is a legitimate outcome; it must not release E1. A further failure should name the concrete unsupported protocol/environment fact, avoiding another broad evidence-patching loop.

Suggested implementation scope is one serial backend infrastructure task for recorder/runner/verifier plus focused tests under `docs/testing/craft/egress-probe/`, followed by read-only backend validation and independent reviewer. Production gateway, RunView and Dockerfile owners remain outside scope. The controller should write a numbered repair plan and checkpoint from this report; this audit does not implement that task or authorize its own live run.
