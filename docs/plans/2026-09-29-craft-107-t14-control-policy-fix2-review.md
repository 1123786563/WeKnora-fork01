# T14 Control Policy Fix 2 — Independent Review

Date: 2026-09-29 (Asia/Shanghai)  
Reviewer: `/root/t14_fix2_reviewer`  
Scope: frozen `fix2.patch`, Fix 2 report and live source at the recorded hashes. This is a read-only source review; no Docker or OCR run was made.

## Verdict

- **Spec compliance: FAIL / T14 acceptance unverified.** The two targeted source changes preserve the exact established-flow exception, loopback drop rules and fail-closed canary predicate. The required live socket reuse, fresh same-listener denial/counter, conntrack behavior and full T14 attempt matrix have not run successfully. Fix 1's Docker Buildx timeout occurred before any integration test or renderer started.
- **Code quality: FAIL.** The generated Python `SyntaxError` is closed and the immediate marker-read race is replaced with polling. One medium finding remains: the purported 2-second wait deadline does not bound its Docker subprocess callbacks and can accept a marker after the deadline.

## Frozen evidence and verification

- Fix 1 package `after/` source hashes match the report's before hashes: controller `eb2540a84873bb4a6a98764a4fb6459e6f9b887f46ffd04fabc3ac6e04487add`; integration test `d3a0b7deb5f9e2a76b4c429ea73ea55ef8ccb9097cc6acd99a387522444a6d77`.
- Live source hashes match the Fix 2 report and `source-hashes.sha256`: controller `9e6840357d22e4f2da1d4f79d52a01f5953991766fe9a30b9433973902f0a439`; integration test `bc9affe43234b5da09654e3df9f9b7a5867be4662ea4d663736f31988fd36ab0`.
- Regenerating the unified diff from those exact before and after bodies matched `fix2.patch` byte for byte; its SHA-256 is `61a07909877134a3edc2720db1fcf1def02e2427d6297c819b1707c459e5b388`.
- Independently compiled the generated IPv4 and IPv6 programs. Focused non-Docker unittest run: **16 tests, 1 Docker-gated skip, pass**. `py_compile` and scoped `git diff --check` passed.

## Finding

### Medium — reuse marker deadline does not bound Docker reads

**Evidence:** `deploy/craft/render-boundary/policy-helper/controller.py:173`, `wait_for_reuse_marker`, calculates a monotonic deadline but invokes `renderer_running()` and `read_marker()` before checking it. The callbacks in `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py:510` invoke the local `run()` helper with its default **30-second timeout each**, even though the requested marker deadline is two seconds. A callback that slept 50 ms and returned `b"reuse-ok"` made `wait_for_reuse_marker(timeout=.001)` return success after about 56 ms, past its 1 ms deadline.

**Impact:** A stalled Docker inspect or exec can hold the disposable proof for tens of seconds and may accept late marker evidence. This weakens the short, explicit synchronization deadline required to distinguish a successful established-socket exchange from a hung renderer/probe.

**Smallest correction:** Pass the remaining monotonic budget into both callbacks and each subprocess timeout, and check expiration before accepting the returned marker. Add a test with a slow callback that returns the expected bytes after the deadline and assert timeout. Keep the exact marker bytes, renderer liveness check and the same established socket.

## Targeted finding disposition

1. **High, generated canary `SyntaxError`: closed in source.** A newline now separates the assignment from the compound `if`; generated IPv4 and IPv6 scripts compile. The canary still requires a distinct source port, an actual failed connect, a timeout-class failure and positive family-specific loopback counter delta. The helper retains nft OUTPUT priority `-150`, two exact tuple `ct state established` rules, preview-only allows and loopback drops. Runtime canary behavior remains unverified.
2. **Medium, immediate USR1 marker read race: partially closed.** The integration test now polls exact marker bytes while checking renderer liveness, eliminating the immediate one-shot `cat` race when Docker calls return promptly. The unbounded callbacks and acceptance after deadline remain open as the finding above.

The original T14 control policy findings cannot be considered fully closed from static and unit evidence. The disposable namespace test and immutable T14 probe remain unrun for this revision; no successful live egress or WebDriver-control proof is claimed.
