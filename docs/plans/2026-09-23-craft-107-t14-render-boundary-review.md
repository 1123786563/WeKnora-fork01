# T14 / #129 render-boundary proof independent review

Date: 2026-09-23. Read-only review of external commit `ef3a0a8678b64728fb39e4ed0fbc7e18811e8f0f..e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` plus the uncommitted `deploy/craft/render-boundary/probe.py` and untracked `docs/plans/2026-09-23-craft-107-t14-render-boundary-{report,recovery-report}.md`. The nine render-boundary source files' path-sorted SHA-256 aggregate is `3fd1fd1928225005cab06861d200e2e4d25a511aaa2733e8fbe521c39364b6ae`, independently recomputed on the current worktree; `probe.py` is `c81c95b92a323cd453c1fe67cd5a93cdde8b8d432308460f0effda94a4d7ce02`. Current `git status --short` shows exactly the uncommitted probe and two untracked reports. The full commit range also contains earlier T14 fix-1 Go/probe/docs work; this review focuses on the new render-boundary proof files while retaining the earlier fail-closed review as context. Sources: approved Craft web-artifact Spec, `CONTEXT.md`, ADR-0004, #129/T14 plan, prior fix-1 review, navigation architecture, render-boundary plan and both reports. No OCR or browser probe was run by this reviewer.

## Verdict

- **Spec compliance: BLOCKED / not verified for T14.** The checked-in fixture and runner are a plausible offline-render design, but no Chromium container was built or launched in this environment. BuildKit exited 102 after memory exhaustion while installing Chromium, and the separate official ARM64 image pull did not materialize a local image. There is no same-renderer evidence of local asset load, interaction, browser egress denial, screenshot, or effective runtime policy. The approved Spec and T14 plan require actual browser and infrastructure-level evidence. The existing direct preview remains closed; its fail-closed state is appropriate but does not satisfy working-preview acceptance.
- **Code/design quality: CONDITIONAL FAIL for proof readiness.** The manifest loader is bounded and digest-checked, the static server serves only manifest keys, and the Docker runner requests `--network none`, read-only root, non-root user, dropped capabilities, no binds or published ports. I independently ran `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_*.py' -v` (5/5 pass) and both committed/uncommitted `git diff --check` (pass). The runner's missing attempt-coverage assertions below permit a false GREEN report after an eventual successful image run.

## Findings

### 1. Medium — probe can pass without exercising the promised browser egress attempts

**Evidence / affected symbols:** `deploy/craft/render-boundary/probe.py:225-265` invokes `window.startEgressAttempts()` and collects browser events and page attempt results. `run-probe.sh:78-86` asserts only that `browser_network_events` is nonempty, all process probes are denied, and no external `Network.responseReceived` entry exists. The event list is already nonempty after loading local HTML/CSS/JS/SVG, and the runner never checks required public/private/metadata/Docker-gateway/loopback/DNS navigation, fetch, WebSocket, popup, anchor or form attempts, their failure events, or the `page.attempts` values. The five unit tests cover manifest/path behavior only.

**Impact:** A fixture regression, early script return, or browser behavior that suppresses the attack attempts could produce a zero-external-response result and pass the runner while proving only local rendering. That would leave the highest-risk T14 navigation/egress acceptance untested.

**Smallest defensible correction:** Define required target-class IDs and assert each was attempted from the running Chromium page, appears in browser request/failure telemetry where applicable, and has no successful external response or open WebSocket. Include a negative test that removes an attempt class and makes evidence validation fail. Keep the network-mode/process assertions as independent checks.

### 2. High, integration gate — no authenticated render broker or same-renderer runtime attestation

**Evidence / affected symbols:** The Dockerfile copies a fixed fixture (`deploy/craft/render-boundary/Dockerfile:19-23`); `probe.py:18,186` loads `/opt/craft-preview`, and `run-probe.sh:20-36` launches only that image. No server-selected Task/Version manifest, fresh T08 permission check, bounded screenshot/input broker, or binding from `CraftPreviewService` to this particular container is present. The prior fix-1 review found that `BrowserNavigationProtected` is an unbound boolean; the current production assembly leaves it false. The render-boundary report explicitly identifies broker and attestation as later work.

**Impact:** Even a successful fixed-fixture `--network none` probe would establish a deployment primitive, not that a user preview runs in the same isolated renderer or that revoked viewers cannot reopen it. #129 cannot be accepted or the preview gate enabled from this checkpoint.

**Smallest defensible correction:** Build the authenticated server broker that selects a fixed Version's validated manifest after current Task access, stages only those files into a fresh renderer, transports bounded pixels/input, and binds live network/container attestation to issuance and reconnect. Run the local-asset and denied-egress probe against that selected deployed configuration before enabling preview.

## Evidence boundary

The primary report's disposable `--network none` process probe confirms no route or successful TCP/DNS egress for a non-browser image. It does not establish Chromium navigation behavior. The BuildKit OOM and stalled image pull are environment blockers, not review findings against source logic. This review therefore does not treat the reported manifest tests, Dockerfile, `--network none` command line, or absent external responses as a passing browser proof. `BrowserNavigationProtected` must stay false and T14/T15 remain gated until the live browser and bound deployment checks complete.
