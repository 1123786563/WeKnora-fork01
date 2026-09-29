# T14 disposable exact established-flow diagnostic

Helper image ID before and after: `sha256:dc52d91d9b644c9b06b1a932e6522ad7c106d3dc8ffd559630b34147e2cddd54 linux/arm64`.

The existing `PolicyControllerIntegrationTests.test_policy_allows_preview_and_counts_post_policy_loopback_drop` was invoked through a temporary test subclass which bypassed only its unconditional `docker build` in `setUpClass`, checked the helper image ID above, and allowed 120 seconds for the controller. The test creates a disposable network-none renderer container and its `tearDown` removes it. It attempts the controller's full install path; controller recorded fail-closed refusal because the host kernel conntrack table was unavailable, then `cleanup status=verified-clean`.

Result: FAILED / blocked before policy installation and before preview/counter assertions. This is not proof that an established socket survives while a fresh socket is denied. Existing unrelated four-hour `t14_*` renderer containers were present before and after; none was stopped or modified. No `t14ph-*` helper or diagnostic renderer remained after either run.
