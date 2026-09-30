# T14 Loopback WebDriver Control-Path Root Cause

## Finding

The exact-source candidate reaches the host-policy release, then blocks in Selenium's first post-policy synchronous operation, `browser.execute_script("return window.enableDocumentProbe()")`. The control request to ChromeDriver must use loopback TCP. The host nft OUTPUT policy currently permits only the preview listener on loopback and drops all other 127/8 and ::1 traffic. A separate network=none smoke using the same image, Chromium/ChromeDriver versions, fixture and script completed this call in 0.0104 seconds before policy installation.

## Evidence

- Candidate image: `sha256:c5a0f04006308ebb086f4a3249eea5fd7ed8bcaab1c48b8b601982d1da14e4f9` (linux/arm64).
- Repro directory: `/tmp/craft107-t14-stage-start-counter-proof`.
- Live console: `/tmp/craft107-t14-stage-start-counter-proof-console.log`.
- Live run emitted `T14_STAGE_START={"label":"renderer_enable_probe",...}` after diagnostic release and no matching `T14_STAGE_TIMING` completion, then host wait failed closed on `002-attempt_begin-receipt.json`.
- The initial policy install record had `loopback_v4_drop={packets:0,bytes:0}`. Mandatory 8081 loopback canary was the sole counted packet at installation (`before_packets=0`, `after_packets=1`, `delta_packets=1`).
- A separate read-only controller snapshot, taken while the same renderer remained alive after `renderer_enable_probe` START, reported `loopback_v4_drop={packets:13,bytes:684}` and `loopback_v6_drop={packets:24,bytes:5855}`. This is 12 additional IPv4 dropped packets after the canary while the Selenium stage was pending. Snapshot evidence: `/tmp/craft107-t14-stage-start-counter-proof/policy-observer/0001-snapshot.json` (whole-file SHA-256 `4a3a0e6651ecc433b00303db75d111e0e3d3ff1dfcf66a71b0658a0de6c9cf59`). The exact drop can be caused by the WebDriver request and its TCP retransmissions; no other renderer browser action is scheduled in that post-release first stage.
- Isolated same-image local fixture smoke without host nft policy: Chromium 153.0.8010.52, ChromeDriver 153.0.8010.52, Selenium 4.35.0; `enableDocumentProbe()` returned `True` in 0.010365 seconds.
- Candidate image stage-emitter smoke sent the fixed record to Docker stderr successfully; the live stage-start line was also read from `docker logs` before container cleanup.
- Host policy exact rule source is `policy-helper/helper.py`: output policy is `drop`, allows only the preview dport/sport, then drops 127/8 and ::1.
- Host controller cleanup recorded `cleanup_verified=true`; the exact temporary renderer ID was then removed and `docker inspect` confirmed `no such object`.

## Interpretation and boundary

The evidence confirms the control-path conflict: the post-policy WebDriver stage remains pending while loopback drop counters increase; the identical local fixture command succeeds before policy. It does not yet prove that Selenium keeps one tracked TCP flow across policy installation or that conntrack reports that old flow as ESTABLISHED.

A safe candidate repair may permit only the host-attested pre-existing Python-to-ChromeDriver four-tuple when `ct state established`, in both directions. A driver-port rule, general established-flow rule, tuple rule without state, or renderer-chosen exception would weaken the egress boundary and is out of scope. The dedicated plan `2026-09-28-craft-107-t14-webdriver-control-policy-plan.md` requires an integration proof that the old flow remains usable and a new connection to that same listener is dropped before the full T14 matrix is run.

## Status

T14 remains unverified. The no-egress matrix did not run; this diagnostic is not preview acceptance. Full T14 is blocked on completing the exact-flow policy repair and obtaining a green immutable-image browser/policy run. The external F08 production build-receipt seam is a separate T20 hard blocker.
