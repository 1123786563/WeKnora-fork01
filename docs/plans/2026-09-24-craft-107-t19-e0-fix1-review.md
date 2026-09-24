# T19 E0 probe Fix1 — independent Spec and quality review

Date: 2026-09-24. Scope: the four Fix1 fixture scripts, the Fix1 plan/report/checkpoint, and the prior F1–F4 review. Read-only review against approved Craft Spec stories 34–35, `CONTEXT.md`, the T19 model egress design and E0 root-cause brief. No OCR or Docker rerun. All four reviewed script SHA256 values match the Fix1 checkpoint. The synthetic mutation suite passed locally; that is code-path evidence only.

## Findings

### F1 — High — Client can influence supposedly independent case attribution

**Evidence:** `run_v2.py:433` still bind-mounts the artifact `state` directory writable at `/state` into the protected client. `server_v2.py:78–91` reads `/state/adapter-mode.json` for *both* the adapter and direct sink, and chooses `X-E0-Case` from the request over that host state. The runner creates the artifact `state` directory mode 0777 (`run_v2.py:390–393`). The Fix1 topology check proves only that `/ledger` is absent; it does not check `/state`. The comment in `server_v2.py:83` that state is not mounted into the client is false. A client process can change or remove the case marker before a bypass, or supply a misleading case header. The verifier attributes zero/bypass decisions to these labels (`assert_v2.py:214–260`).

**Impact:** A direct sink request can be mislabeled as a positive control or another case, undermining the independent no-bypass observation. The original F1 ledger-file isolation is improved, but its evidence boundary remains compromised through shared mutable metadata.

**Smallest correction:** remove `/state` from the client; keep sink mode/case data host-owned and inaccessible from it. Treat request headers as untrusted evidence, and reconcile *all* direct sink HTTP/parse events and sequence windows without relying solely on client-supplied labels. Inspect the client mount inventory in the verifier.

### F2 — High — Verifier still accepts incomplete or forged positive evidence

**Evidence:** `assert_v2.py:66–83` accepts manifest `status: pass` and artifact existence for most cases; `:77–80` trusts manifest `task_content`; `:149–155` trusts the manifest `effective_config_trace` string and never reads the case stdout/stderr or config file/hash; `:164–170` trusts redirect control counts; `:220–225` accepts one `control:*` GET per role and one CONNECT anywhere in the run, without matching each required baseline or pre/post bracket. `:226–252` compares declared range *length* to tagged event count, but not the actual events between start/end, and has no global rule rejecting unattributed/unknown direct sink events. `:261–264` never checks `manifest.verdict` or the recorded command/process result of each positive case. The full-schema synthetic pass (`test_assert_v2.py:40–84`) uses zero length ranges for almost every case, no raw baseline events, no raw host controls or redirect follow events, and no actual hostile process trace, yet passes.

**Impact:** A complete-looking artifact can pass while positive controls, actual OpenCode completion, hostile selection, or raw window attribution were not proved. Prior F2 and F4 are only partially resolved.

**Smallest correction:** verify every required positive control by exact case/tag, role, method, status, and its before/after bracket; bind process success/content and config-load text to the actual retained stdout/stderr and in-container hash command records; compare each range with the corresponding raw sequence slice and reject unmatched sink events. Require final manifest PASS and reject blocked/timed-out command records for required rows. Make the full-schema positive test contain real per-case events and artifacts, then mutate each one separately.

### F3 — Medium — Hostile-config mutation test does not test the missing-trace condition

**Evidence:** `test_assert_v2.py:120–125` removes `effective_config_trace_file`, a key absent from the passing fixture, while leaving `effective_config_trace` unchanged. It also changes `intended_base_url` and `selected_base_url` to another value, which is enough for `assert_v2.py:154` to reject the fixture. Thus the test's advertised “without effective load trace” rejection is caused by a URL mismatch and gives no coverage for an absent or forged process trace.

**Impact:** The test suite's passing output overstates coverage of the F4 correction.

**Smallest correction:** retain matching intended/selected URL and remove or falsify the actual trace, then assert the verifier rejects it because the retained process log/config evidence is missing or inconsistent.

### F4 — Medium — Timeout quiescence checks only an OpenCode process name

**Evidence:** `run_v2.py:67–99` sends SIGTERM and polls `/proc` only for argv elements equal to or ending in `opencode`; `wait_for_quiescence` then samples HTTP/parse counts. The dedicated client can run child request senders under a different process name, and the server records TCP accepts separately (`server_v2.py:40–47`). The synthetic late-child test (`test_assert_v2.py:161–174`) supplies a mocked alive predicate and counter; it does not establish the real container process tree or socket quiescence. Timed-out rows are blocked, but a late untracked sender can contaminate subsequent case attribution.

**Impact:** The original F3 is improved for OpenCode processes, but the runner cannot yet establish a stable boundary for every child network sender after timeout.

**Smallest correction:** on timeout stop the disposable client container or verify its complete process tree and live sockets are gone before the final listener window; include TCP accepts in the settle check and block the run when quiescence is unproved. Retain the original timeout logs.

## Verdict and required next evidence

**Infrastructure Spec/quality: FAIL, changes required.** Removing `/ledger` from the client, monotonic raw records, raw HTTP mutation checks, and timeout blocking are useful progress. F1 and F2 still prevent this fixture from serving as independent fail-closed no-bypass evidence. The synthetic suite passed, but the full-schema positive fixture is too permissive and one named mutation does not test its claimed condition.

**Live E0: BLOCKED/default-off; do not dispatch E1.** The only Fix1 Docker attempt stopped during setup with `ValueError: embedded null byte` after about 75 seconds. Its manifest reports `client_mounts_ledger=false`, write denial and cleanup, but no full matrix, actual timeout quiescence or hostile config trace. The escaped process probe and later verifier/runner hardening were not measured. The earlier 566.736-second Fix2 artifact is also BLOCKED and predates Fix1. No result may combine those runs or extrapolate a PASS from synthetic tests.

After correcting the findings, a **new bounded disposable live run on the exact reviewed script hashes** must exercise the complete pre/post restart matrix, same-sink HTTP/CONNECT and host controls, normal adapter title/task turns, denied direct IP/DNS/forced IP/custom URL/proxy/redirect/host routes, timeout quiescence, and actual project/global config load plus selected baseURL. Retain raw per-role ledgers, client mount/process snapshots, stdout/stderr, config hashes, cleanup inventory, manifest and file hashes; run the independent verifier on that same artifact and obtain exit 0 with no missing, timed-out or retried acceptance row. Observed OpenCode 503/disconnect retries remain an independent T19 identity/egress gate even if this fixture's network denial later passes.
