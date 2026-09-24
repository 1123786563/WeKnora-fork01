# Craft #107 T19 E0 Evidence Fix5 Task 1 Report

Date: 2026-09-24. Scope: verifier and synthetic evidence tests only. No runner, production source, Docker, or measured E0 artifacts were changed or run. E0 remains **BLOCKED**.

## Changes

- `assert_v2.py` now treats verifier-owned `controller_ordinal` and unique `operation_id` as the cross-file event order for phase acknowledgments, host operations, case commands, config capture records, and cleanup. It rejects missing/duplicate/gapped ordinals, duplicate IDs, cross-phase operations, out-of-order control boundaries, case commands outside their acknowledged phase, cleanup before the seal, and not-found inspection before removal.
- The exact curl control argv is reconstructed from verifier constants and the inspected direct listener address. Shell commands and any changed executable, option, environment, proxy, or target are rejected; expected URLs hidden in comments or extra shell commands have no effect.
- Hostile URL expectations are now fixed in verifier code: the direct-IP cases use the inspected direct container address and DNS cases use `mock-hostile-provider`. Provider/model IDs are fixed by case, and the attempted endpoint must be exactly `<baseURL>/chat/completions` with no query. Manifest summaries and matching retained config/trace bytes cannot redefine these values.
- Pre and post-restart Docker evidence must cover all four participants and both networks. Raw participant inspection is checked for immutable pinned image ID, UID/GID 10001, privilege/capability policy, fixed network attachments and network mode, and published-port policy. The client must have only the two fixed XDG volumes; D and A must have distinct recorder-owned `/ledger` volumes; the helper cannot have a writable host bind. Private/external network membership, isolated/private status, helper peer identity, adapter forwarding sysctl, and client restart identity/mount continuity are re-derived from raw inspect.
- Added self-consistent forgeries for wrong hostile origin/provider, wrong model endpoint, shell-inert expected target, unsafe adapter inspect, shared D/A evidence volume, extra helper network, and missing/duplicate/cross-phase controller ordinals. Added a non-synthetic input control proving that lack of an independent pinned OpenCode provider/attempt source yields BLOCKED.

## Verification

Commands run from the integration worktree:

- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed. The complete synthetic schema fixture remains explicitly labelled synthetic; 62 evidence mutations reject, including all Fix5 cases. The non-synthetic no-provenance control returns BLOCKED as intended.
- `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` — 15 tests passed.
- `python3 -m py_compile docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py docs/testing/craft/egress-probe/test_source_owned_v2.py` — passed.

These checks are parser/recorder tests only. No Docker matrix ran and no synthetic result is measured E0 evidence.

## Exact checkpoint hashes

The patch reconstructs the three-file postimage from the saved preimage; `test_source_owned_v2.py` is unchanged.

| File | Preimage SHA256 | Postimage SHA256 |
|---|---|---|
| `assert_v2.py` | `7c701f6c89616f5f4617e98e456bb85292eedb81c4e46f78ac269dfb680cb996` | `92ac76ad8d54d5c9b9f41c8c2ceb081f32c463ec9badf51de67b460c83254b17` |
| `test_assert_v2.py` | `ead80e5455c1b9b72cd43bce35643911d7c93a3ab388ebe63bbe2d4dbcc05881` | `4a4c2691421618f09f07f399b1a806d03781ab39276cd96a4d04ad9daefa0a57` |
| `test_source_owned_v2.py` | `799bf51ad8b3824cbfe681173969f9c55f0c464892ec5b131e9fd19903ddc118` | `799bf51ad8b3824cbfe681173969f9c55f0c464892ec5b131e9fd19903ddc118` |

Patch: `docs/plans/2026-09-24-craft-107-t19-e0-fix5-task1.patch`, SHA256 `5f7d266355e45750b49683b826c694e56db325470ca3114056b8526a9e594f44`.

## Remaining exact blockers / incomplete invariants

1. **Producer schema mismatch.** `run_v2.py` does not emit a single controller-owned ordinal and operation ID across raw D/A phase markers, every host operation, case/config capture, both source seals, removal, and not-found inspect. The new schema therefore cannot accept a current runner artifact. File order and manifest sequence are intentionally not fallback authorities.
2. **Recorder mount ownership mismatch.** The current runner bind-mounts one shared host ledger directory into both direct sink D and adapter A. The verifier now requires separate recorder-owned Docker volumes at `/ledger`; raw inspection cannot prove the old shared bind is safe. The helper is also currently started as an unregistered disposable `docker run --rm` control process rather than a fixed inspected resource with the required identity and capability policy.
3. **Pinned OpenCode provenance is unavailable.** A self-authored JSON trace plus matching config bytes/hash is not independent evidence that pinned OpenCode selected the provider/model or attempted that URL. No pinned process/runtime source was found that binds PID, actual selected provider/model/baseURL and actual network attempt. Live input without this source is unconditionally BLOCKED; schema-shaped synthetic trace is diagnostic only.
4. **Fixed runtime-command reconstruction is still incomplete.** The verifier does not yet require controller-ordinal raw host command/result records for effective UID/GID (`id -u/-g`), binary version/hash, and IPv4/IPv6 route inventory. The current runtime-facts output and route summary are not independently re-derived from a fixed exact argv/result schema. This is a remaining Task1 invariant and blocks any claim of full topology proof.
5. **Network interface details remain partial.** Participant attachment sets and network membership are checked, but raw inspect is not yet compared against verifier-fixed participant IP/gateway facts or a complete interface inventory. The Docker daemon/platform route behavior still requires a compatible producer and measured validation.

The checkpoint is a verifier/test improvement only. It does not release Fix5 Task2 or establish E0 PASS.
