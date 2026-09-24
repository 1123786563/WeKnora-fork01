# Craft #107 T19 E0 runner source integration Task 1 — independent review

Date: 2026-09-24. Scope: the exact two-file runner checkpoint and retained one-phase smoke artifact. Reviewed the approved Craft web-artifact Spec, `CONTEXT.md`, E0 evidence architecture, Fix6 verifier, source-helper-boundary and recorder phase reviews, Task 1 plan/report/checkpoint. No Docker, OCR, production edit, or live E0 matrix was run.

## Findings

### RI-1 — High: runtime mounts and UID cannot satisfy the existing Fix6 verifier

**Evidence:** `run_v2.py:200-216, 405-420, 851-871` runs D and A as UID `0:0`, mounts their recorder volumes at `/evidence`, and creates the helper from the pinned OpenCode image without explicit mounts. The retained `boundary-source-mount-inspects.json` shows D and A each have four mounts: one read-only script bind, one private `/evidence` volume and two anonymous writable XDG volumes inherited from the image. The helper has two anonymous writable XDG volumes; C has two anonymous writable XDG volumes in the smoke. These are real Docker `Mounts` entries despite the helper argv having no `-v`/`--mount`. The unchanged Fix6 verifier requires UID `10001:10001` for all participants (`assert_v2.py:550-555`), an exactly empty helper mount set (`:577-579`), and exactly one D/A private recorder volume at `/ledger` (`:585-589`). The smoke's local check at `run_v2.py:230-235` only searches for the two named recorder volumes on helper/C and therefore misses the inherited mounts.

**Impact:** The smoke demonstrates a source boundary but its topology cannot pass the review-approved verifier contract. Extending this producer to the full E0 matrix without resolving the image/mount/UID design would leave E0 blocked, and the helper does not meet the plan's zero-mount condition.

**Smallest defensible correction:** Resolve the fixture/verifier interface explicitly: use a reviewed source/helper runtime without inherited volumes, prepare private recorder volume ownership so D/A can remain non-root, and align the recorder mount path and fixed inspect schema. Preserve the pinned OpenCode C identity and keep the verifier fail-closed; any changed evidence contract needs independent review before a measured run.

### RI-2 — Medium: smoke cleanup can report PASS after failed removal

**Evidence:** `run_v2.py:282-303` records each removal's `exit_code` but computes `absent` solely as `inspect.exit_code != 0`. It does not require the removal to succeed or require the inspect failure to mean “not found.” A Docker daemon/permission error can make both calls fail while the resource remains, yet `source_boundary_smoke` becomes `PASS` when the phase itself succeeded. The four focused runner tests do not exercise cleanup failure. Retained smoke removal rows all show exit 0, so the observed run is unaffected.

**Impact:** A cleanup failure can be mislabeled as a successful source-boundary smoke and leave owned containers, networks or volumes behind. This violates the plan's fail-closed cleanup check.

**Smallest correction:** Require successful removal (or a verified already-absent response) and a specific not-found inspect result for every owned resource; classify any daemon/permission/timeout response as BLOCKED. Add a focused failed-removal/failed-inspect test.

## Positive evidence and checkpoint

The preimage index hash matches. Applying the 43,248-byte patch (SHA-256 `221c7ce7de1854241fba5de6f57545e8c9b6918005ff4a7fd7820ce8996eb1c5`) to the captured preimage reproduces both current files byte for byte; their hashes and the report hash match the checkpoint. All eight selected raw smoke artifact hashes in the checkpoint match current bytes. Independently ran four focused runner tests; all passed.

The raw smoke host log has one successful `docker start <owned helper>` operation with ID `…b8937a2557f74ef8b2a2bb23b0a18c27` and controller ordinal 4. D and A each fsync `helper_start` with that ID/ordinal at source cursor 3; D's first `tcp_accept` is seq 4. Begin/barrier/seal cursors are retained in `phase-commands.jsonl`; both source streams seal. `start_helper_and_ack` at `run_v2.py:578-627` persists the host operation before sending D/A private commands and cannot dispatch control traffic until both echoed identities/cursors validate. Focused tests reject failed/wrong start and one-sided/forged ack. The smoke uses an internal Docker network, and its manifest retains `verdict: BLOCKED`. The current ordinary `main()` still uses the older ephemeral helper path for other control phases, as the implementer report states.

## Verdict

**Spec compliance: FAIL for Task 1's private fixed topology and fail-closed cleanup requirements. Code quality: FAIL** for the runtime inspect mismatch and smoke cleanup predicate. The one-phase ordering mechanism itself is supported by raw host/source events and focused tests. No complete pinned OpenCode provider/attempt provenance, fixed E0 matrix, restart or full cleanup reconstruction was measured; **E0 remains BLOCKED**, and the smoke PASS must not be used as release evidence.
