# T19 E0 pinned provider route / bypass probe

Date: 2026-09-24 (Asia/Shanghai)  
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
Image: `weknora-craft-runtime:1.18.4`, ID/digest `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`, `linux/arm64`; `opencode --version` printed `1.18.4`.

## Verified facts

The disposable fixture is [egress-probe](./egress-probe/). Its config uses the documented custom OpenAI-compatible provider shape (`npm: @ai-sdk/openai-compatible`, `options.baseURL`, `options.apiKey`) and points to `http://mock-adapter:8080/v1`. The fixture command was:

```text
./docs/testing/craft/egress-probe/run.sh
```

The exact run loaded `/workspace/opencode.json`, selected `mock/mock-model`, and the retained raw log shows four POSTs to the adapter across the two requested turns:

```text
POST /v1/chat/completions (4 total across two turns)
```

Observed headers included `User-Agent: opencode/1.18.4 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14`, `Authorization: Bearer probe-only`, `x-session-affinity`, and `x-session-id`. OpenCode generated extra title/attempt traffic; the exact reason and retry classification are unverified. No upstream credential or real provider key was mounted. The effective process HOME remained `/home/craft`; the attempted `/tmp/home` override was not authoritative. The direct listener and adapter were both attached to the same internal network, so that listener setup does not prove a same-network direct route is denied.

The unprotected control succeeded:

```text
baseline_direct_http=200
```

This came from a separate non-internal disposable bridge network and proves the image can reach a directly attached listener absent policy.

Under the internal network policy, direct and DNS bypass attempts failed:

```text
direct_ip_rc=7 http=000
provider_dns_rc=6 http=000
internal=true containers=0
```

The image had no proxy or API-key environment variables in the inspected environment. Disposable resources were removed by the fixture trap and explicit cleanup.

## Inference and limits

The probe demonstrates that OpenCode 1.18.4 can route model traffic to a private OpenAI-compatible adapter using `provider.*.options.baseURL`, and that Docker's `--internal` network blocked the tested external IP and provider-DNS attempts. It does not prove a production gateway-only policy: the direct listener shared the internal network, effective HOME was `/home/craft`, and the writable custom baseURL, redirect, proxy, direct gateway/provider IP/DNS, restart, and per-listener zero-count matrix remain untested. The four POSTs and extra title/attempt traffic remain unclassified; they do not establish physical-attempt identity or retry semantics.

## Recommendation / gate status

**E0: partial evidence; keep model egress default-off.** The adapter route and basic internal-network deny control are reproducible, but the strict no-bypass acceptance requires the remaining restart, redirect, custom-baseURL/proxy override, and listener-zero checks under the final proposed runtime policy. Do not treat this report as T19 completion or as authorization for paid provider calls.

## Primary source pointer

OpenCode's provider documentation describes custom OpenAI-compatible providers and `options.baseURL`, `options.apiKey`, and `options.headers`: <https://github.com/anomalyco/opencode/blob/dev/packages/web/src/content/docs/providers.mdx>. The pinned image/version and binary digest are recorded in `docs/plans/2026-09-24-craft-107-t01-linux-image-apt-recovery-fix3-report.md`.

## Fix1 rerun (2026-09-24)

The fixture pins/asserts image ID, architecture, and in-image version; retains raw output; attempts to persist HOME across two OpenCode turns; attaches an explicit direct-listener alongside the adapter; and records effective internal network membership plus direct IP/DNS attempts. Independent review corrects the adapter count to four `/v1/chat/completions` POSTs across the two turns; OpenCode's extra title/attempt behavior is unclassified.

The rerun direct baseline returned `baseline_direct_http=000` because the disposable listener was unavailable in that attempt. The earlier independent control remains the verified unprotected baseline (`baseline_direct_http=200`). Internal deny evidence remained `direct_ip_rc=7 http=000` and `provider_dns_rc=6 http=000`; both direct-listener and adapter were attached to the `Internal=true` network. The script exited `6` because the DNS curl failure propagated through tee; cleanup ran.

**Fix1 status: blocked/partial.** Strict E0 remains gated: this rerun lacks a successful direct baseline, effective HOME was `/home/craft`, and the same-internal-network listener cannot establish no-bypass. The redirect, writable custom baseURL, proxy, direct gateway/provider IP/DNS, restart, and per-listener zero-count matrix remains outstanding. Keep model egress default-off.
