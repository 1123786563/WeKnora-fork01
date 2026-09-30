# T19 E0 probe evidence correction

> **For Codex:** Execute the bounded E0 evidence task with exact fixture/report checkpoint and independent review. It does not enable production egress.

**Sources:** `2026-09-24-craft-107-t19-egress-next-task-design.md`; `2026-09-24-craft-107-t19-egress-e0-review.md`; approved Spec #107/#138. Pinned local Linux/arm64 image ID `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`. No real provider credentials/calls, no production files or shared DB.

## Global Constraints

Use disposable Docker containers/networks and mock endpoints. Distinguish an image ID from a registry digest. Keep model egress default-off. A successful shell exit with missing route/deny evidence is failure. Bound the probe to 15 minutes and record each untestable bypass as an explicit release blocker.

## Review Focus

Retained exact commands/raw redacted output, asserted adapter POST path/count and model completion, unprotected baseline, direct listener zero-count, Docker effective network/config/HOME/proxy at creation and restart, redirect/custom baseURL/proxy/IP/DNS bypasses, cleanup, no paid calls.

## Task 1 — reproducible route/bypass fixture

**Depends on:** E0 first report/review. **Role:** researcher. **Owned files:** `docs/testing/craft/egress-probe/*`, new timestamped raw output under that directory, updated `docs/testing/craft/2026-09-24-t19-egress-e0-report.md` or a new fix1 report. **Consumes:** local image and disposable mock adapter/listeners. **Produces:** one rerunnable fail-closed probe and exact evidence, or a verified partial result/blocker.

1. Pin/assert image ID, architecture and in-image version before probe. Capture a direct-listener HTTP 200 baseline outside the internal policy.
2. Under the proposed internal network, assert model turn status and exact adapter `/v1/chat/completions` count/path with retained redacted server/client logs; check direct IP/DNS denial and zero direct-listener traffic. Persist and restart the same HOME, inspect effective config/mounts/network/env before both turns.
3. Exercise redirect, writable custom provider/baseURL, proxy env and direct gateway/provider IP/DNS paths. Require a denied connection and zero listener count for each; do not infer no-bypass from `curl` exit alone. Capture post-run inventory and cleanup even on failure.
4. Record commands, exit codes, actual image ID, output hashes, limitations and exact pass/fail matrix. Save a task-local patch/checkpoint of owned files and `git diff --check`; obtain independent review.

**Acceptance:** E0 only passes if every required route and bypass check is evidenced. Otherwise report the concrete blocker and leave E1 gated; no assertion of full T19 completion.
