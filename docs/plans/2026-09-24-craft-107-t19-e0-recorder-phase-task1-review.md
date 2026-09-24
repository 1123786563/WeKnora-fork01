# Craft #107 T19 E0 recorder phase Task 1 — independent review

Date: 2026-09-24. Scope: the exact two-file recorder phase-command increment. Reviewed the approved Craft web-artifact Spec, `CONTEXT.md`, relevant ADR/trust constraints, the E0 phase fix plan, Fix6 verifier contract, prior source-helper-boundary review, checkpoint and report. No production/source/test edit, OCR, Docker run, or live E0 measurement was performed.

## Findings

No blocking finding in this two-file Task 1 increment.

- `server_v2.py:149-187` requires exact private schemas for `begin_phase`, `barrier`, `seal`, and the existing `helper_start`; each phase/seal command now carries `operation_id` and `controller_ordinal`. `_controller_identity` at `:110-123` rejects a missing, reused, or nonincreasing host identity independently of the contiguous source-local command ordinal. `helper_start` shares that identity set and ordinal cursor.
- `server_v2.py:84-108, 210-256` appends and fsyncs source-owned `phase_begin`, `phase_barrier`, and `stream_end` rows before committing phase/identity state or returning an acknowledgment. Successful acks echo source, phase, operation ID, controller ordinal and the exact event sequence as `stream_cursor`. An append/fsync error poisons the source and returns no usable ack. The control barrier still requires a prior helper-start event and drained connections; `accept` at `:258-269` still records and poisons early control TCP traffic.
- Focused tests reject missing/reused/stale identities without an event, exercise fsync failure and seal cursor echo, and retain prior helper-start, early-TCP, phase mismatch, active-connection and source-ordinal tests.

### Pre-existing E0 integration gap — Medium, outside this increment

`server_v2.py:198-200` does not admit `restart` as a phase mode, while the unchanged Fix6 verifier requires a `restart` phase in its fixed schedule (`assert_v2.py:58-67, 766-775`). This mismatch predates this checkpoint and is not caused by the identity change, but a complete measured E0 sequence cannot currently pass through this recorder. The smallest correction in a separate scoped task is to define the recorder's restart phase semantics, test begin/barrier around the actual same-container restart, and preserve the verifier's fixed schedule. Do not weaken the verifier or treat a synthetic event as measured evidence.

## Checkpoint and verification

The earlier reviewed source-helper-boundary patch SHA-256 matches its checkpoint. I replayed that patch onto its archived two-file preimages; the result matched both Task 1 archived preimage files byte for byte and their recorded hashes (`87750f01…` and `d0d13d87…`). The current incremental patch is 31,022 bytes with SHA-256 `255749f695b4b1b8a8af9fd8eb708f12155d8bd9ef8b86dbe3b4d4ff5b07d40b`. Applying it to the reconstructed preimages produced byte-for-byte current postimages matching SHA-256 `045849d16e6516995a73005df5936ea2c3273e71c42ae964bdb0e9a61c9a7207` and `07578377c8e49128fb419cb667d094a5b2ff66e33c5af105466a3fb34e81f83c`. `git diff --check` passed.

Independently ran `python3 -m unittest test_source_owned_v2 -q`: 23 tests passed. The `test_assert_v2` script accepted its labelled complete synthetic fixture only as synthetic and rejected its mutation cases; `unittest` reports zero test methods because that script runs checks at import time. These results check local producer and parser behavior, not real Docker commands or OpenCode provenance.

## Verdict

**Scoped Spec compliance: PASS. Scoped code quality: PASS** for the requested phase/seal identity and source-owned acknowledgment increment. **Measured E0: BLOCKED** by runner integration, pinned OpenCode provider/attempt provenance, full raw matrix and the pre-existing restart-mode mismatch above. The recorder still treats host identity fields as claims; the verifier must join them to retained raw host operations before any live credit or release decision.
