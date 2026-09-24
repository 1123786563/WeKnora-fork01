# Craft #107 T19 E0 Fix6 Task 1 — independent review

Date: 2026-09-24. Scope: the exact three-file Fix6 checkpoint, Fix6 plan/report, prior Fix5 independent review, approved Craft web artifact Spec, `CONTEXT.md`, relevant ADRs and E0 evidence architecture. This review did not change requirements, production source, tests, remote issues or commits, and did not run OCR or Docker. Other workers share the worktree; this verdict binds to the hashes below.

## Checkpoint and verification

All three preimage and postimage byte counts/SHA256 values, patch hash and report hash match `2026-09-24-craft-107-t19-e0-fix6-task1-checkpoint.json`. Applying the patch with `patch -p1` to temporary copies of the captured preimages exited 0 and reproduced all three postimage hashes. Reviewed `assert_v2.py` SHA256: `1ea01079abc4c5b32760caf8436dca8dc8e969a14c65deeec0a9dd3781372f6d`; patch SHA256: `c25082906fe3d7ea4f0523182f67c2083e704faac09a51e54d8a7299d4c0e5f8`.

Independent focused checks passed: `test_assert_v2.py` (complete labelled synthetic fixture and 66 `reject_mutation` cases), `test_source_owned_v2.py -q` (15 tests), and `py_compile` of all three owned files. These are local parser/recorder checks, not a measured E0 run.

## Independent adversarial challenges

I copied the passing full synthetic fixture into separate temporary directories for each challenge and recomputed the affected raw-evidence SHA256 in its manifest. The unmodified fixture returned `verify=True`.

| Challenge | Verifier result | Relevant check |
|---|---|---|
| Helper mounts D's writable evidence volume under `/alias` in both raw inspect snapshots | `False` | `assert_v2.py:577-578` requires the helper's complete mount inventory to be empty. |
| `direct_ip_pre` fixed curl row has `phase_id: null`; its ordinal is swapped with an earlier preflight row | `False` | `assert_v2.py:856-883` binds each scheduled case to its exact phase and begin/case/barrier interval. |
| A direct `tcp_accept` is placed before the `helper_start` source event while ordinals, hashes and stream sequence remain self-consistent | `False` | `assert_v2.py:821-846` requires the source-owned boundary before control traffic. |
| The direct `helper_start` event refers to an existing but wrong operation ID | `False` | The boundary must match the phase's referenced successful helper-start operation ID and controller ordinal. |

The new source event schema also rejects a missing or duplicate helper-start boundary. The exact helper mount rule closes both direct-volume and volume-alias cases for the retained Docker `Mounts` list. I found no new false-PASS path within the Fix6 Task 1 repair scope.

## Remaining evidence boundary

The parser accepts a synthetic fixture containing correctly shaped `helper_start` events, but the current `run_v2.py`/`server_v2.py` cannot issue and acknowledge those events from D and A after the helper actually starts. Matching operation ID/ordinal values inside a synthetic source row do not themselves prove the producer's ordering. This is an explicit **runner/recorder interface blocker**, not measured control evidence. The old runner also lacks the unified controller ordinal/operation ID schema and isolated recorder volumes.

`verify()` still blocks every artifact whose manifest `evidence_type` is not `synthetic` because independent pinned OpenCode selected-provider and actual network-attempt provenance is unavailable. A labelled synthetic fixture can return parser `PASS`; the output marks `synthetic: true`, and that result must not be accepted as measured E0. Fixed raw runtime identity/version/hash and IPv4/IPv6 route command/result reconstruction, plus complete interface/gateway comparison, also remain incomplete. No Docker matrix or measured E0 PASS was produced.

## Verdict

**Scoped Spec compliance: PASS for Fix6 Task 1's mount, phase and fail-closed helper-boundary requirements.** The four independent challenges above reject, and the missing producer boundary remains explicitly BLOCKED. **Overall E0 acceptance: BLOCKED.** The approved offline-egress requirement is not established; Fix6 Task 2, E1 and model egress remain gated by runner, provenance and route evidence.

**Scoped code quality: PASS.** The checkpoint is reproducible, focused suites pass, and the corrections are narrow. The parser's synthetic PASS has only fixture-test meaning; independent review of the future producer and measured artifacts is still required.
