# T19 E0 Root Cause and Bounded Probe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** replace the inconclusive E0 fixture with one reproducible, fail-closed experiment proving the pinned OpenCode model route and measuring adapter-only reachability, restart persistence, and physical send behavior without paid traffic.

**Architecture:** one isolated internal bridge contains only the OpenCode container and the adapter ingress. A second bridge contains mock gateway/provider/proxy listeners and the adapter's outbound interface. The same live external listeners supply positive controls and denial evidence; logs and exit codes are retained per case. This is a proposed experiment, not a verified production topology.

**Tech Stack:** Docker Engine 29.4.0 / Linux arm64 on OrbStack (observed), pinned OpenCode 1.18.4 image, Bash, Python standard library mock HTTP/SSE servers and assertions.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md` stories 34–35; `docs/plans/2026-09-23-craft-107-dag.md` T19/#138; `2026-09-24-craft-107-t19-model-egress-design.md`; `2026-09-24-craft-107-t19-egress-next-task-design.md`. E0 is a prerequisite for E1, not the T19 acceptance test.

**Status / provenance:** read-only systematic debugging, 2026-09-24. Read both E0 reviews, Fix1 plan, corrected E0 report, fixture, all three retained logs, image configuration, current egress design, and relevant Spec/DAG passages. No fixture execution, container creation, paid call, production edit or test edit occurred during this investigation. Only this file was written. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; observed HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Other owners have extensive uncommitted work; preserve it.

## Global Constraints

- WeKnora remains authoritative for Task, Run, permission and budget state. All title, summarization and normal model calls count as model activity under T19.
- Local image ID must equal `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`, platform `linux/arm64`, binary version `1.18.4`, binary SHA256 `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`. This is an image ID, not a registry manifest digest.
- No actual provider/gateway calls, keys, production services, shared database, host firewall mutation, Docker socket mount or privileged OpenCode container. Use fake key `probe-only` only.
- Keep production model egress default-off and E1 gated until independent E0 review passes. An incomplete matrix is blocked, including an untested host route or unknown automatic retry behavior.
- Own only the probe fixture and new per-attempt evidence/report paths below. No local commit unless the controller supplies authorization; use a task-local before/after checkpoint and hashes.
- Bound next execution to 20 minutes wall time, each ordinary request to 3 seconds and each OpenCode turn to 45 seconds. Timeout records a failure; it never supplies evidence of zero retry or successful completion.

## Review Focus

1. A listener that is absent, unready, reset or not counting GET/CONNECT traffic cannot prove denial. Task 1 requires positive controls before and after each negative group and monotonic counters.
2. Host services remain reachable on an ordinary internal bridge gateway. Task 1 uses isolated gateway mode and tests host/published-port paths; unsupported enforcement stays blocked.
3. Config and session data follow fixed XDG locations and image-declared volumes. Task 1 inspects the actual mounts and loaded config, then restarts the same container.
4. Title requests can overlap task requests; an invalid SSE response or a second new session can distort counts. Task 1 serves valid SSE and labels request purpose from controlled evidence, without using purpose/body hashes as production identity.
5. Shell success and DNS failure can conceal a missing test. Task 1 requires completion tokens, raw process exit, actual target resolution/IP controls, complete listener ledgers and negative self-checks.

## Root cause: what the retained evidence actually supports

### Four POSTs are two title calls and two task calls

`docs/testing/craft/egress-probe/raw-output-20260924013431.txt` contains the following requests. Every path is `/v1/chat/completions` and every body specifies `stream:true`:

| Raw line | Session suffix | Body purpose and corroborating client log | Body SHA256 |
| --- | --- | --- | --- |
| 64 | `...3dWI0MEJGg1MQ` | system starts `You are a title generator`; no tools; client `small=true agent=title` at line 22 | `8a3888900235c6fff2716dcfb03629b76ecfb15c79394bf58df2ea1647d39424` |
| 65 | same first session | software engineering system prompt, nine tools, first user prompt; client `small=false agent=build` at line 29 | `c4fd36defdfefab6fcc3f4c1bfe80ee3bc283c1d54eea3052d967bf3e42c2389` |
| 66 | `...eVvfkaG4fc1al` | title system prompt, no tools, second user prompt; client `agent=title` at line 52 | `41397f1862f785f23aeefa2efb578ac27f8efc4a69e24b6efb189799a61319ce` |
| 67 | same second session | task system prompt, nine tools, second user prompt; client `agent=build` at line 59 | `87104bd0daef2d23a615b1371d1dda4b94da8832c82c5f75bfc1c1326e5605e2` |

The four bodies and two distinct session IDs are explained by two invocations of `opencode run`. There is no observed retransmission in these four records. This does **not** prove retries are disabled or establish retry policy under transport/503 failure. Do not preserve the corrected report's blanket “unclassified extra title/attempt traffic” when these particular four records can be classified. Do not generalize this diagnostic classification into trusted model-call identity: title/build and session headers remain caller-controlled observations at the adapter boundary.

The mock returned `application/json` and a non-streaming completion to all four streaming requests. Client logs show loop exit but no `probe-ok` content. Therefore provider-route arrival is measured; successful completion is not. `|| true` suppresses both OpenCode exits, so a successful fixture exit could never establish turn success.

### The network experiment tests the wrong boundary

Fix1 creates an unprotected bridge, obtains a direct-listener baseline, destroys that listener, then creates a different direct listener on the **same internal network as the adapter**. Raw lines 96–110 show both listeners attached. Same-network reachability is permitted; the fixture never calls that direct listener from the protected OpenCode container. The two negative probes instead target unrelated `1.1.1.1` and `api.openai.com`, with no receiving counter. A failed DNS lookup proves neither IP routing denial nor an adapter whitelist.

The network inspect occurs after both `--rm` OpenCode containers exited; it proves two listener attachments at that later instant. It does not prove the run's actual attachment. The “internal=true containers=0” report fragment is absent from this raw inspect and does not describe that measurement.

Docker documents that `--internal` permits same-network communication and access to appropriately configured services at the gateway/host address. Separating the listeners onto another bridge fixes one defect but does not itself close that host path. See [Docker internal network semantics](https://docs.docker.com/reference/cli/docker/network/create/#network-internal-mode---internal). The proposed isolated gateway mode removes the bridge's host address; its effectiveness on this daemon must still be measured. See [Docker gateway modes](https://docs.docker.com/engine/network/port-publishing/#gateway-modes).

### HOME persistence was not demonstrated; the reported HOME reset is not established

Read-only image inspection returned UID/GID `10001:10001`, `HOME=/home/craft`, `XDG_CONFIG_HOME=/home/craft/.config`, `XDG_DATA_HOME=/home/craft/.local/share`, and image-declared volumes at `/home/craft/.config/opencode` and `/home/craft/.local/share/opencode`. The fixture overrides only HOME and mounts only `/tmp/home`. Fixed XDG paths still select `/home/craft/...`; the declared volumes create separate state for each fresh `--rm` container. Loading `/home/craft/.config/...` does **not** prove the process HOME reverted. Login-shell behavior is an additional unmeasured variable, not the supported root cause. Persist the actual XDG paths and inspect them explicitly; avoid `sh -lc` in the next experiment.

### The report still conflates different attempts

- `13413.txt` has only image/version and `baseline_direct_http=000`; it has no retained evidence establishing why the listener was unavailable.
- `13431.txt` begins with `baseline_direct_http=200`, then has all four requests and the network/negative outputs.
- `13433.txt` is a written summary claiming two calls and baseline 000; it is not raw stdout, and its exit-code statement is not paired with a captured process exit in `13431`.

The current report corrected the total POST count but its Fix1 subsection still attributes baseline 000 to the rerun and describes 200 as an earlier independent control. It also claims the fixture asserts image identity although `run.sh` only prints it. The next report should reference each run separately, retaining original evidence and recording which previous prose it supersedes.

## Proposed minimal topology and fixture contract

```text
OpenCode C -- P (internal + isolated) -- adapter A -- X (bridge) -- direct sink D
                                                                 :8080 gateway
                                                                 :8081 provider
                                                                 :8082 proxy
```

Only C and A are attached to P during model turns. C is never attached to X, has no published ports, host network, privileged mode, extra capabilities or Docker socket. D is never attached to P. A has IPv4 forwarding disabled and is a fixed-purpose application endpoint, not a TCP/HTTP proxy. Only A has the outward interface. Baseline commands use a short-lived copy of the **same pinned runtime image** on X to contact the same D addresses and ports. No container is allowed to resolve real provider hostnames for HTTP traffic.

Use one new Python fixture server with two roles: adapter (SSE plus controlled fault/redirect modes), direct sink (three ports with separate role counters). All methods, including GET, POST, CONNECT and malformed requests, increment an append-only ledger; also log TCP accepts so successful connections that send no HTTP cannot hide. Listener logs are stored outside OpenCode's volumes. Read readiness/metrics by `docker exec` or local file inspection, not by uncounted HTTP health endpoints. Readiness polling has a deadline and verifies bind/listen, process liveness and expected configuration.

Each log record must include `run_id`, server `boot_id`, monotonic `seq`, time, role/port, method/path, peer, session header, body digest, controlled-purpose label, selected fault mode and response status/completion. Only synthetic prompts and fake credentials may be retained. Each case records its before/after sequence ranges, command argv, process exit, HTTP status and expected assertion. Never truncate or reset logs to obtain zero counts.

### Exact topology command recipe for the implementer

The following recipe is to be embedded in the new fixture after implementing the server contract above; it was not executed during this investigation. Use unique names from one run ID and record them before each creation so cleanup covers partial startup. Use only the pinned runtime image for all Python helpers to avoid an unpinned `python:latest` dependency.

```bash
set -euo pipefail
IMAGE=sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11
RUN_ID="craft-e0-$(date -u +%Y%m%dT%H%M%SZ)-$$"
P="$RUN_ID-private"; X="$RUN_ID-external"
A="$RUN_ID-adapter"; D="$RUN_ID-direct"; C="$RUN_ID-client"
CFG="$RUN_ID-config"; DATA="$RUN_ID-data"
FIXTURE="$PWD/docs/testing/craft/egress-probe"
test "$(docker image inspect "$IMAGE" --format '{{.Id}} {{.Os}}/{{.Architecture}}')" = "$IMAGE linux/arm64"
docker network create --internal \
  -o com.docker.network.bridge.gateway_mode_ipv4=isolated "$P"
docker network create "$X"
docker volume create "$CFG"
docker volume create "$DATA"
docker run -d --name "$D" --network "$X" \
  --network-alias mock-gateway --network-alias mock-provider --network-alias mock-proxy \
  --cap-drop ALL --security-opt no-new-privileges \
  -v "$FIXTURE:/probe:ro" --entrypoint python3 "$IMAGE" \
  /probe/server_v2.py --role direct --run-id "$RUN_ID"
docker run -d --name "$A" --network "$P" --network-alias mock-adapter \
  --cap-drop ALL --security-opt no-new-privileges \
  --sysctl net.ipv4.ip_forward=0 \
  -v "$FIXTURE:/probe:ro" --entrypoint python3 "$IMAGE" \
  /probe/server_v2.py --role adapter --run-id "$RUN_ID"
docker network connect "$X" "$A"
DIRECT_IP=$(docker inspect "$D" --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
docker run -d --name "$C" --network "$P" \
  --cap-drop ALL --security-opt no-new-privileges \
  -e HOME=/home/craft -e XDG_CONFIG_HOME=/home/craft/.config \
  -e XDG_DATA_HOME=/home/craft/.local/share \
  --mount "type=volume,source=$CFG,target=/home/craft/.config/opencode" \
  --mount "type=volume,source=$DATA,target=/home/craft/.local/share/opencode" \
  -v "$FIXTURE:/probe:ro" --entrypoint sleep "$IMAGE" infinity
```

Initialize the writable synthetic project at `/workspace/output/e0` by `docker exec` as UID 10001, copying the fixture's config. Keep the original global policy populated from the image volume; record its hash. Explicitly verify version and binary hash inside C. Record `docker inspect C A D`, both network inspections, `id`, HOME/XDG values, `/etc/resolv.conf`, `/etc/hosts`, `/proc/net/route`, `/proc/net/ipv6_route`, mountinfo, effective capabilities and the config file hashes. Require exactly one attachment for C; P membership exactly C+A; no IPv6 route outside loopback and `EnableIPv6=false`. Record the image-declared anonymous helper volumes and remove them with `docker rm -v`; remove only the two named volumes owned by this run.

If isolated gateway mode is rejected, silently ignored, or the host path remains reachable, stop with that exact blocker. Do not weaken to plain `--internal`. Do not install host firewall rules as an improvised fix; a separate runtime policy decision would be required.

### Valid SSE and physical-send evidence

For `stream:true`, send `Content-Type: text/event-stream` and valid `chat.completion.chunk` frames: assistant role, a content delta (`probe-ok` for task or `Probe title` for title), finish_reason `stop`, a usage-only frame with `choices:[]`, and `data: [DONE]`. Each frame ends with two newlines; terminate the HTTP response cleanly. Use unique completion IDs and a constant model `mock-model`, numeric creation timestamp, index 0 and total usage 2. For non-streaming requests return a valid JSON completion. Normal responses contain no tool calls.

Run each OpenCode invocation with Python `subprocess.run(..., timeout=45)` around `docker exec`; capture stdout, stderr and exit without a pipeline. Kill the active exec/turn on timeout before reading the final counters; a still-running process invalidates zero-count snapshots. The invocation is:

```bash
docker exec --workdir /workspace/output/e0 "$C" \
  opencode --print-logs --model mock/mock-model run 'E0 normal: reply with one word.'
```

Assert exit 0 **and** task output containing the exact `probe-ok` assistant content. For a fresh session, expect one title and one build request under the observed behavior; require exactly two POSTs, the expected path/model/stream flags, one session ID and distinct controlled-purpose records. An unexpected number stops for diagnosis; do not rewrite expected counts to match. The adapter's records identify physical inbound HTTP requests, not a production-trusted attempt ID.

After normal SSE succeeds, use an independent single-turn fault case: title always gets a normal SSE response; the first task POST receives a complete 503 response; subsequent task POSTs receive normal SSE. Record whether and when OpenCode sends another task POST, its session/body digest, and process retry/error events. A second request in this controlled, single user-turn fault context is evidence of automatic retry; matching hashes alone are not. A separate fault case accepts a task POST then closes the connection before response headers. Stop at the deadline; a timeout is “retry behavior not fully observed”. Record exact counts, never assume a global `maxRetries` option exists. If disabling retries is required, verify an actual pinned OpenCode configuration/source contract in a follow-on bounded protocol task; generic provider options are not sufficient evidence. The E0 fixture must not purport to implement durable ambiguity handling; that remains E1/E2/E3.

## Task 1 — E0 Fix2: topology and protocol evidence fixture

**Depends on:** this investigation and existing image evidence. **Owner role:** `backend_implementer` (bounded Python/Bash test infrastructure); main controller dispatches, no child agents. **Validator:** `backend_validator` followed by independent `reviewer`. **Status:** ready for dispatch, not executed. **Commit strategy:** uncommitted checkpoint unless controller authorizes a commit.

**Owned files:** create `docs/testing/craft/egress-probe/run_v2.py`, `server_v2.py`, `assert_v2.py`, and `docs/testing/craft/egress-probe/e0-fix2/<run-id>/` artifacts; create `docs/testing/craft/2026-09-24-t19-egress-e0-fix2-report.md`. Preserve previous fixtures/logs as historical evidence. No other writes.

**Consumes:** exact image ID, Docker daemon, current fixture config format, this topology and assertion contract. **Produces:** `manifest.json` (commands, exits, case status, image/version/platform, file hashes), immutable raw per-case logs, network/config/mount snapshots, all-method listener ledgers, cleanup inventory, and an explicit E0 pass/blocked verdict. `assert_v2.py <artifact-dir>` must recompute verdict from evidence and return nonzero for missing cases, missing logs, nonzero bypass counts, unsuccessful controls or unexpected route/completion behavior. An evidence parser never treats absent fields as zero.

- [ ] **Step 1: create RED assertion fixtures before Docker execution.** Feed the assertion program synthetic artifacts with (a) missing baseline, (b) baseline HTTP 000, (c) wrong image ID, (d) zero adapter requests, (e) exit 0 but missing task content, (f) one bypass GET or CONNECT, (g) stale boot ID/reset counters, (h) missing restart snapshot. Each must return nonzero. Include a complete synthetic pass fixture to check parser structure, explicitly labelled synthetic and excluded from measured acceptance. Run `python3 -m py_compile` on all three scripts.
- [ ] **Step 2: implement server and bounded command recorder.** Use thread-safe monotonic ledger writes and a threaded HTTP server because title and task calls can overlap. Record each TCP accept before request parsing. Pre-register cleanup resource names, preserve the initial exit status, and make cleanup failure fail the overall run. Readiness polls replace fixed `sleep 1`. Implement the exact topology and inspection recipe above.
- [ ] **Step 3: establish controls on the same D.** From the same image on X, curl gateway:8080, provider:8081 and proxy:8082 by DNS and the inspected literal IP; require HTTP 200 and exactly one matching counted request for each. Keep D running. A through-proxy control uses `curl --proxy http://mock-proxy:8082 --noproxy '' http://probe.invalid/`; the sink logs absolute-form requests and returns 200 without forwarding. Test CONNECT with a synthetic HTTPS target; successful receipt of CONNECT is enough for the positive proxy control, not a model completion. Repeat liveness controls after every negative group and exclude only these explicitly tagged control records from bypass deltas.
- [ ] **Step 4: normal route and full denial matrix.** Obtain successful SSE completion through A. Then execute every row below from C; all external listener deltas must be zero, and each command must demonstrably attempt its intended route. Use one case token per request/turn and retain stderr and exit. A local command/config parse failure is not a denied network attempt.
- [ ] **Step 5: restart and tamper.** Save C's container ID, CFG/DATA mount source names, config hashes and session data marker. `docker restart "$C"`; inspect the same fields, require identical container/mount identities and persisted marker. Repeat normal route and every denial row. Next write a hostile project provider baseURL and separately a hostile XDG global provider baseURL, verify effective config loading, and require failure to reach D. Restore known config and assert a successful adapter turn. A mutable URL need not be physically immutable if the host-owned network enforcement makes every alternate address fail; production E4 must still inspect/reapply the intended config and network on recovery.
- [ ] **Step 6: measure retry behavior using the two controlled fault cases above.** Title requests are logged/accounted separately from task attempts. Record any concurrent arrivals for the future adapter serial-admission design; do not add a body-derived identity rule. If behavior remains ambiguous, record the bounded protocol follow-up and leave E0 blocked.
- [ ] **Step 7: finish evidence and review package.** Stop active turns before snapshotting counters; verify all listeners stayed healthy and logs are complete. Remove only run-owned resources, assert exact names absent and preserve cleanup results. Run `python3 docs/testing/craft/egress-probe/assert_v2.py <artifact-dir>` with the actual generated path; preserve its exit. Run `git diff --check`, hash owned files/raw artifacts, create before/after task checkpoint. Independent review checks Spec compliance and evidence quality before any E1 dispatch.

### Mandatory negative matrix, both before and after restart

| Case | Exact attempt and required observation |
| --- | --- |
| Direct gateway/provider IP | `curl --noproxy '*' --connect-timeout 1 --max-time 3 http://$DIRECT_IP:8080/<case>` and port 8081; nonzero connect/timeout result, HTTP 000, zero TCP/HTTP sink delta. |
| DNS name | Try `mock-gateway:8080` and `mock-provider:8081`; record resolution error separately. Then force the **same** targets with `curl --resolve mock-provider:8081:$DIRECT_IP ...`; this rules out DNS failure masquerading as egress enforcement. |
| Custom baseURL | Write writable project config selecting `http://$DIRECT_IP:8081/v1`, then the mock DNS name plus host mapping; run OpenCode. Require the intended config was loaded and no sink TCP/request; a successful model answer is forbidden. DNS-only failure is insufficient. |
| Proxy environment | On a fresh exec inside C set HTTP_PROXY, HTTPS_PROXY, ALL_PROXY and lower-case equivalents to `http://$DIRECT_IP:8082`, set NO_PROXY/no_proxy empty; run OpenCode and an explicit curl proxy attempt. If OpenCode ignores env and still uses A, report that observation; explicit proxy transport must fail and D receives no CONNECT/absolute-form request. Repeat using mock proxy name. |
| Redirect | Adapter's **POST** `/v1/chat/completions` response becomes 307, then a separate 308 case, Location `http://$DIRECT_IP:8081/v1/chat/completions`; body consumed, bounded response. Assert A received the original POST and D receives zero traffic. Use `curl -L --max-redirs 2` POST control to prove follow behavior and run OpenCode to record actual SDK follow/reject behavior. A GET-only `/redirect` mock does not exercise this case. |
| Adapter as proxy | Send CONNECT and an absolute-form external URL to A; A rejects both without contacting D. Hostile upstream URL/header fields cannot convert A into an unrestricted relay. This tests the mock route contract; production adapter validation remains E2. |
| Host/published port | Create a disposable mock host-facing listener with an explicit owned published port; prove it reachable from a baseline container by the observed Docker/OrbStack host address. From C try that address, `host.docker.internal`, `gateway.docker.internal`, and any bridge gateway recorded in routes/inspect, using a forced IP control where possible. Zero sink TCP/request plus connection denial required. No live host services may be targeted. If the local engine cannot provide this positive control, this row is blocked, not skipped/pass. |
| IPv6 | Require IPv6 disabled on P and no usable non-loopback route; record IPv6 resolution/route facts. If IPv6 is enabled or a route appears, add a controlled IPv6 listener and dual-stack denial checks before passing. |

For the host row, publishing only on host loopback is not an adequate positive control when the baseline container cannot reach it. Choose and record an address that the baseline actually reaches, bind only this disposable fixture, and promptly remove it. Do not reuse a pre-existing port/service or declare the row passed because no service happened to listen.

## Decisions and remaining uncertainty

The experiment's root failures are an incorrect boundary, non-streaming responses to streaming requests, persistence of the wrong filesystem locations, and non-asserting/mixed evidence. They are not evidence that OpenCode retries caused four charges. The four observed calls are explained and each would need admission in production.

The local daemon is 29.4.0, Linux arm64 on OrbStack. The isolated bridge proposal is based on current Docker documentation but is not yet empirically verified here, and a pass here does not cover another daemon/network backend or amd64 deployment. Host-route controls and the final attachment/mount/config checks are deliberate gates.

Next bounded work is **Task 1 only**. Do not start another broad adapter implementation or modify gateway/RunView owners' code to make the probe pass. If a host-route gap or a mandatory uncontrolled retry remains after this run, preserve evidence and return the exact gap for an architecture/protocol decision. E1/E2/E3/E4/E5 retain their existing scopes; successful E0 still cannot prove journal durability, per-attempt billing, budget denial provider-zero, or unknown-result recovery.
