# T19 normal input Task2b Fix1 Task1 independent re-review

## Scope and exact package

Reviewed the previous Task1 review findings T19-NI-1/2, the Fix1 plan/report/checkpoint, and the eight-file incremental patch. This review covers only the repository stage and receipt seam; coordinator, physical Docker execution, output store, and production routing are outside it.

The Fix1 patch SHA-256 is `0d70b80126990eed05a0cb4b0b1e2351298fef0edb45d9e4b6a4794ca83d3dd2`. I reconstructed the S2 Fix1 snapshot, applied the prior Task2b Task1 patch, verified all eight Fix1 preimage hashes, applied this incremental patch, then verified every resulting file byte-for-byte against the current integration worktree and all Fix1 postimage hashes. This includes ignored SQLite 125 and versioned PostgreSQL 204 up/down migrations. The send-claim test file and both down migrations remain byte-identical to their preimages. Saved RED, focused, and race output hashes match the checkpoint; focused and race runs report PASS on SQLite and isolated PostgreSQL, including actual migration 204 up/down/up. I did not rerun the controller's database tests.

## Review of prior findings

**T19-NI-1 — closed.** `craft_docker_send_claim.go:57-69,112-125` now takes `lockDockerSendRunFence` inside the transaction before asking `normalInputExists(tx, ...)`, and rejects the legacy API before either journal UPDATE. `Stage` inserts under the same Run lock (`craft_docker_normal_input.go:129-174`); the former check-then-lock gap is gone. The new controlled Stage-vs-legacy Bind and Claim tests in `craft_docker_normal_input_test.go:148-312` exercise both SQLite and PostgreSQL. Their saved RED result demonstrates the old Bind succeeding and old Claim missing the staged-input error; the fixed focused/race results show the typed conflict and unchanged journal claim. The Claim test deliberately binds a three-field journal row in the held Stage transaction to make its otherwise unreachable race branch observable.

**T19-NI-2 — closed.** `craft_docker_normal_input.go:96-125,273-347` calculates the compact JSON size before marshaling/encryption, enforces a 2 MiB canonical request cap, verifies the actual marshaled size, and rejects oversized ciphertext before decryption. The reader also rejects decrypted plaintext above 2 MiB and checks canonical length. The count handles JSON control/HTML/U+2028/U+2029/invalid-UTF-8 escaping and byte-array base64 size; the exact-boundary and escaping tests exercise those cases. AES-GCM adds 12 nonce and 16 tag bytes: raw URL-base64 of 2,097,180 bytes is 2,796,240 bytes, plus seven bytes of `enc:v1:` = **2,796,247**. Both SQLite 125 and PostgreSQL 204 constrain stored ciphertext to that number. Oversized command/environment requests fail before key lookup or insert; exact 2 MiB request round-trips.

## Verdicts and limits

- **Scoped Spec compliance: PASS.** Both prior findings are closed in the exact patch. The stage and legacy authority exclusion is serialized, and the canonical request, ciphertext, recovery read and DB constraints share a compatible bound. Existing fail-closed key, scope and full-receipt rules remain in the reviewed postimage.
- **Scoped code quality: PASS.** No new blocking finding. The interleaving test is deterministic at the shared Run lock, and SQLite/PostgreSQL focused and race outputs are bound to the same checkpoint hashes.

The isolated PostgreSQL tests use minimal parent tables, so they do not prove the full migration chain. The normal bind still writes the journal and full stage receipt in two steps; crash-boundary replay and physical one-start behavior remain for coordinator integration. This review does not release T19 normal execution or enable production routing.
