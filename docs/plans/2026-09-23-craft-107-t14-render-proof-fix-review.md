# T14 / #129 render proof validator fix independent review

Date: 2026-09-23. Read-only review of T14 worktree HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` plus three uncommitted proof files and four untracked plans/reports. Current full-content SHA-256 matches the fix report: `deploy/craft/render-boundary/probe.py` `0881037432b62eac20e667c56070747e54de22ff5768de2011684a3a5de545ca`, `run-probe.sh` `1087faed20ccf99b2e7e775261c9f2e590c939802cd3c723bc3d21d57eaf4f81`, `test_probe.py` `9622c86d0b584a4b05277f9fba4260b886fc133bf3174a5e73710c0d94db83ab`; fix report hash `07a22f09e9492ea6e8edda0ea45b8ffeb991b9a6c27f42fbf49329fd21e84aeb`. Sources: approved Craft Spec/#129, ADR-0004 and `CONTEXT.md`, earlier render-boundary independent review, render-proof fix plan/report, and the unchanged fixture. No OCR or live browser probe was run.

## Verdict

- **Scoped validator Spec compliance: FAIL pending CDP correlation correction.** It now requires all 37 declared page attempts, explicit denial, and no external response/handshake, so the earlier “nonempty local network events” false pass is substantially closed. However, a CDP event or failure with the right `?attempt=` ID can count for the wrong target URL and mechanism; the claimed browser coverage is not strictly mapped to the required matrix.
- **Scoped code quality: FAIL on the Medium finding below.** The validator is bounded and fails closed on absent page IDs, duplicate/unknown IDs, malformed page target mapping, missing denial, unmapped external activity and successful external responses. Its synthetic test suite passes (11/11 independently rerun), as do `py_compile`, `bash -n` and `git diff --check`. The tests use canonical event URLs only and miss event mapping substitution.
- **Full T14/#129 Spec compliance: BLOCKED / unverified.** No actual Chromium container, browser event stream, screenshot or effective browser egress denial exists for this checkpoint. The BuildKit OOM and stalled ARM64 image pull remain environment blockers. The authenticated render broker and same-renderer attestation remain absent; `BrowserNavigationProtected` must remain false.

## Findings

### Medium — attempt ID alone can credit a mismatched browser request

**Evidence / affected symbol:** `validate_browser_evidence` maps page telemetry to a target and mechanism (`deploy/craft/render-boundary/probe.py:74-93`), but its CDP `browser_network_events` loop groups an external URL solely by the `attempt` query ID (`:104-113`), and `browser_failed_requests` similarly adds IDs without verifying target URL or event type (`:119-131`). The final coverage loop requires only that `external_events[attempt_id]` exists (`:148-176`). A `Network.requestWillBeSent` entry for `http://10.0.0.1/craft-probe?attempt=fetch-public` and matching failure can therefore credit the required public fetch even though its actual destination is private; a `Document` event can credit a WebSocket attempt. The synthetic tests (`test_probe.py:112-207`) do not substitute wrong-target/wrong-type events.

**Impact:** An eventual green evidence summary could claim a target/mechanism boundary was exercised when Chromium actually sent a different request. This weakens the proof of public/private/metadata/gateway/loopback/DNS coverage that the fix plan requires.

**Smallest defensible correction:** For each external CDP request and failure, require the parsed URL to match the declared attempt's expected target class and scheme/path, and match the mechanism to the appropriate CDP request type (at least WebSocket versus HTTP/document). Prefer exact URL equality with the page attempt's declared URL after canonical parsing. Reject mismatched activity rather than crediting its ID. Add synthetic wrong-target and wrong-mechanism cases, including a failed request with a valid ID but altered destination.

### High, proof-readiness gate — unchanged fixture cannot satisfy the new attempt schema

**Evidence / affected files:** The validator requires `attempt_id`, `mechanism`, `target_class`, `url`, `attempted` and `denied=true` for all 37 IDs (`probe.py:74-97`). The unchanged `deploy/craft/render-boundary/preview/app.js:34-97` emits legacy `{target,attempted}` records for a smaller set of requests; several navigation entries lack denial and it does not issue the full matrix. `run-probe.sh:73` calls the validator on that fixture's telemetry before writing evidence.

**Impact:** Even if the Chromium image becomes available, the current runner cannot produce a passing browser proof from its checked-in fixture. This is correctly fail closed and explicitly disclosed in the fix report, but the proof package is not ready for T14 acceptance.

**Smallest defensible correction:** Update the fixture under explicit ownership to emit the required stable IDs and complete target/mechanism matrix with real asynchronous outcomes, then run the runner in the actual isolated Chromium image. Keep the validator strict; do not relax the matrix to accommodate old telemetry.

## Evidence boundary

The new runner calls `validate_browser_evidence` and saves its machine-readable summary only after it returns (`run-probe.sh:60-81`); process namespace assertions remain separate (`:38-47,83-89`). The previous nonempty-network-event assertion is removed. The local test command `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_*.py' -v` passed all 11 tests; syntax and diff checks passed. These are synthetic/static results only. A fixed fixture, actual container network policy, same-renderer broker and authenticated Version selection still need independent integrated proof before enabling preview.
