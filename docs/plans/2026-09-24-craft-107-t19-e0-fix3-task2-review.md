# Craft #107 T19 E0 Fix3 Task 2 — independent review

Date: 2026-09-24. Scope: the exact uncommitted Task 2 checkpoint, assigned Fix3 plan, prior Fix3 Task 1 and Fix4 Task 1 reviews, approved Craft #107 web artifact Spec, `CONTEXT.md`, applicable ADRs and the E0 evidence architecture. This review changed no source, test, requirement or remote issue, and did not run OCR or Docker. Other workers own the shared worktree; findings bind to the hashes below and must be reconsidered if those files change.

## Checkpoint and checks

All three preimage SHA256 values, three current postimage values, patch SHA256 and unchanged runner SHA256 match `2026-09-24-craft-107-t19-e0-fix3-task2-checkpoint.json`. Applying the patch with `patch -p1` to a temporary copy of the three captured preimages exited 0 and reproduced every postimage hash. The reviewed `assert_v2.py` hash is `7c701f6c89616f5f4617e98e456bb85292eedb81c4e46f78ac269dfb680cb996`; the patch hash is `6af0ac1c74c20edcaf15dc3eb8042272fff5dfb0acf458dadf6df5d6f80aaa25`.

Independent focused runs: `python3 docs/testing/craft/egress-probe/test_assert_v2.py` exited 0 with the clean full synthetic fixture and all 49 mutations rejected; `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` exited 0 with 15 tests. These are synthetic and local recorder checks, not measured E0 evidence. The Task 2 report explicitly records that the unchanged runner cannot produce this schema or independently sourced OpenCode selection/attempt traces.

## Findings

### F1 — High — control and cleanup operations have no proven order

**Evidence:** `assert_v2.py:619-692` checks an exact sequence of phase *labels*, then builds `operations` by ID from `host_rows`. A control phase can reference any matching stop, state-inspect, helper-start and helper-stop row. Neither those rows nor the phase commands have a shared time/ordinal that establishes stop and inspect before the control request, helper start before traffic, or helper stop after the barrier. `assert_v2.py:560-602` likewise checks cleanup row values but does not establish both recorder seals before removal. The full synthetic fixture in `test_assert_v2.py:137-190` builds control operations separately from source phases; its pass does not exercise causal order. The Task 2 report acknowledges this missing timeline.

**Impact:** A complete artifact can be structurally PASS while the client was running during a control, the helper was stopped before it, or cleanup began before final seals. That defeats the E0 trust boundary: a client request could be attributed to a control window, and removed resources could hide late observations. This is a verifier false-PASS path when host rows are recorded out of order or misassociated, even with an exact phase schedule.

**Smallest correction:** Give every host operation, phase acknowledgment and cleanup action one controller-owned ordered timeline, bind operation IDs to their unique phase, and require `stop → wait/inspect C → helper start → control traffic/barrier → helper stop` plus `both stream_end seals → removals → not-found inspections`. Add mutations that move each operation across its required boundary while keeping all row values and hashes valid.

### F2 — High — fixed curl targets are checked as substrings of arbitrary shell text

**Evidence:** `_fixed_invocation_errors` at `assert_v2.py:359-392` accepts any `sh -c` string containing `curl`, `--max-time 3`, `--noproxy '*'` and the expected target/proxy tokens anywhere. A shell command can execute a different curl target while carrying all expected tokens in an inert comment, echo or second command. The negative-case check at `assert_v2.py:870-883` requires a nonzero exit and `http_000_required` manifest flag or `HTTP=000` text; it does not bind the failed transport to the expected curl process. The tests at `test_assert_v2.py:570-572` remove a required token, but do not test an extra executable command or inert expected tokens.

**Impact:** A command that never attempts the intended direct/proxy route can satisfy the raw argv, failed-exit and zero-observation checks, giving a false network-denial PASS. The approved E0 contract requires a concrete attempt to the fixed target, not a matching string embedded in a shell program.

**Smallest correction:** Use verifier-owned exact argv or a constrained command template with no free-form shell, and bind exit/stderr to that single invocation. Add a mutation whose shell executes `curl` against a different target while quoting the required target and proxy strings only as inert text.

### F3 — High — hostile config target and trace remain self-declared

**Evidence:** `assert_v2.py:459-503` takes `wanted` from `manifest.cases[name].intended_base_url`, then checks retained config bytes and the three JSONL trace rows against that value. Apart from `custom_url_dns_pre` at `assert_v2.py:895-897`, the hostile project/global case origins are not fixed by the verifier. The JSONL process-start row validates a 64-character `argv_sha256`, but does not recompute it from a retained actual process argv; the config hash is recomputed from retained bytes, but no successful in-container hash/load command or independently sourced OpenCode event is bound to those rows. The unchanged runner cannot emit this provenance, as its Task 2 report states. Existing mutations change only one trace or summary field, not a self-consistent wrong origin.

**Impact:** Manifest, config and trace can agree on a different origin and still meet these checks, including a target that never probes the specified hostile route. A schema-shaped trace alone cannot prove that pinned OpenCode loaded this file, selected this provider/model or attempted that URL. This is the remaining E0 config provenance gate; it cannot be treated as an infrastructure PASS from synthetic evidence.

**Smallest correction:** Fix each case's required provider/model/origin in verifier code (deriving only the permitted direct IP from independently inspected topology). Bind process argv, actual successful config-hash/load observation and selected network attempt to host-captured process output or a separately justified pinned-binary trace. If the binary exposes no such seam, retain BLOCKED and name that limitation.

### F4 — High — raw topology reconstruction omits security-critical participant and route facts

**Evidence:** `assert_v2.py:504-558` verifies client writable bind/privilege/cap additions, selected network membership, two client volumes and adapter forwarding sysctl. It does not inspect A/D/helper mounts, privilege/capabilities, image/UID or their network modes and port bindings; nor does it establish the client image/UID, IPv6 routes or runtime binary result from successful fixed commands. The remaining checks at `assert_v2.py:805-862` use a second limited client snapshot plus manifest topology booleans and a free-form runtime `stdout` string. The synthetic inspected A/D/helper objects at `test_assert_v2.py:100-112` are intentionally sparse, so the suite cannot detect these omissions. The Task 2 report records the incomplete security inspection.

**Impact:** A participant may have a writable evidence mount, extra network access or elevated privileges while the manifest still claims safe topology. Client image/UID/route deviations can likewise be hidden by self-declared fields. This violates the E0 architecture's independent raw-inspect boundary and can make a network isolation result untrustworthy.

**Smallest correction:** Require exact raw container/network inspect schemas for every participant at pre and post restart, check all mounts including ancestors/aliases, effective UID, image, capabilities, modes, port bindings and routes, and bind binary/version checks to successful fixed commands. Mutate one raw A/D/helper mount or privilege fact while summaries remain safe.

## Verdict

**Spec compliance: FAIL for Fix3 Task 2 acceptance.** The fixed phase schedule, missing-control rejection, direct helper peer matching and same-client restart identity improve the verifier, and Task 1's direct-stream protections remain intact. F1–F4 leave the required causal control proof and independent host/config/topology reconstruction incomplete. The unchanged runner is incompatible, so this checkpoint cannot release the Docker matrix, measured E0 PASS, E1 or model egress.

**Code quality: changes required.** The patch is reproducible and focused tests pass, but they mutate isolated fields inside a synthetic artifact and miss self-consistent wrong evidence and operation reordering. The Task 2 report correctly marks the result BLOCKED. No approved Spec or ADR change was made by this review.
