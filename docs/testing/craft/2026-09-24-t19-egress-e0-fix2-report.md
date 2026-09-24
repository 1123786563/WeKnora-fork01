# T19 E0 Fix2 bounded topology probe

Run: `e0probe-20260923T180318Z-40990` (UTC), Docker Engine `29.4.0`, `linux/arm64`.

## Verdict

**BLOCKED / incomplete.** The isolated bridge topology was created and measured, but this is not an E0 pass: the required `run_v2.py`, `server_v2.py`, and `assert_v2.py` fixture did not exist at probe start, so no valid SSE adapter response, OpenCode completion assertion, listener ledger, restart matrix, fault/retry case, or complete denial matrix was executed.

## Verified facts

- The required image resolved to `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11 linux/arm64`.
- Docker accepted `--internal -o com.docker.network.bridge.gateway_mode_ipv4=isolated`; network inspect reported `Internal=true`, `EnableIPv6=false`, and the isolated option verbatim.
- The private network contained exactly the disposable adapter and client containers at inspection time. The client had one network attachment, no gateway, IPv4 `192.168.229.2`, and no IPv6 address.
- The client ran as `uid=10001(craft) gid=10001(craft)`, with `HOME=/home/craft`, `XDG_CONFIG_HOME=/home/craft/.config`, and `XDG_DATA_HOME=/home/craft/.local/share`. Named config/data volumes were mounted at the declared OpenCode paths.
- `/proc/net/route` showed only the private subnet route and no gateway; `/proc/net/ipv6_route` contained loopback routes only. Docker resolver was `127.0.0.11`.
- A baseline container on the external bridge reached the disposable direct listener by DNS with HTTP 200.
- The isolated client attempted the inspected direct-listener IP (`192.168.239.2:8080`) and received connect failure, HTTP 000 (`curl` exit 7). This is one topology observation only; no append-only sink counter existed.
- Cleanup removed the three disposable containers, two networks, and two named volumes through the run-owned cleanup trap.

## Not established

- No adapter process or valid SSE response was present; no physical model route or exact title/build classification was measured.
- No exit-code/content assertion, external listener baseline/zero delta, host/published-port positive control, DNS forced-IP matrix, proxy/CONNECT, redirect, hostile config, restart persistence, or retry behavior was run.
- The direct listener used Python's standard `http.server` on port 8080 only, so it cannot serve as the required three-port role-separated ledger.
- The direct IP test is evidence of this isolated bridge's observed path, not proof of the full E0 policy. It does not test an adapter ingress or host route.

## Evidence pointers

Raw command captures are in `/tmp/e0probe-20260923T180318Z-40990/` on the probe host (`manifest.txt`, `private-network.json`, `client-summary.txt`, `client-routes.txt`, `direct-curl.txt`, `baseline.txt`). The historical fixture remains under `docs/testing/craft/egress-probe/` and was not modified.

## Recommendation

Treat E0 as blocked pending implementation and execution of the exact Fix2 fixture contract. Do not dispatch E1 from this result. The next run must provide immutable per-case logs and a fail-closed assertion report before interpreting topology reachability.
