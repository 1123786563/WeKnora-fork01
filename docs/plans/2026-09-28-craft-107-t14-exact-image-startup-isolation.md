# T14 exact-image startup isolation and policy-helper diagnostic

Date: 2026-09-28 UTC. Scope: diagnose the earlier T14 live run failure without weakening any network policy or claiming acceptance.

## Evidence

- The immutable renderer image `sha256:a2dc460f05d310ad0de3f82658a83676d71db6628832ad0d9958075d3893b590` starts Chrome 153.0.8010.52 under the renderer's restrictive container settings. A closer smoke using Chrome's NetLog and performance logging options also starts and yields 197 performance entries. Raw compact output and exact constraints are in `docs/testing/craft/t14/2026-09-28-startup-isolation/chrome-smoke.txt`.
- The existing policy integration test `test_policy_allows_preview_and_counts_post_policy_loopback_drop` was run twice against pre-existing helper image ID `sha256:dc52d91d9b644c9b06b1a932e6522ad7c106d3dc8ffd559630b34147e2cddd54` without rebuilding it. First run hit the test helper's 45-second controller timeout. Second run allowed 120 seconds and returned explicit failure: `policy-helper: kernel conntrack table is unavailable; refusing WebDriver exception`; controller cleanup status was `verified-clean`.
- Post-run inspection confirmed helper image ID unchanged and no diagnostic renderer/helper remains. Four unrelated `t14_*` renderer containers had been running for hours before the test and remained untouched.

## Ruling

Chrome binary startup by itself succeeds, narrowing the earlier `Chrome instance exited` failure to the complete probe/barrier environment or another integration step. The policy helper refuses to install a WebDriver exception without host-readable kernel conntrack evidence. Do not weaken this fail-closed check. The diagnostic did not establish the old-connection-allowed / new-same-listener-denied behavior and is not T14 acceptance evidence.

## Next required evidence

Provide a host/runtime where the bounded host barrier can read authoritative conntrack state and run the full exact-image T14 matrix, including established-flow proof, fresh same-listener denial and cleanup verification. Then review and persist complete raw evidence.
