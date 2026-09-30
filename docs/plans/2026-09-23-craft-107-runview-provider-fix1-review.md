# T01 RunView provider fix 1 — independent scoped re-review

**Scope:** The Task 2 provider repair in `internal/container/craft_runview_container_provider.go` and `_test.go`, against the three findings in `runview-provider-task2-review.md`, the approved Craft #107 spec, `CONTEXT.md`, ADR-0003, provider design/plan, and the fix plan/report/checkpoint. Source and tests were read only.

## Verdict

- **Scoped Spec compliance: PASS.** The three reported defects are closed in this checkpoint. The provider checks the prospective resolved sandbox root against forbidden host paths before creating or changing it; a unique complete-list session is confirmed through typed exact GET-by-ID metadata before its inventory is marked authoritative.
- **Scoped code quality: PASS.** Mismatch tests now establish a real marker-backed generation before mutating inspected facts and assert the inspection or probe failure stage. Focused provider tests pass independently.
- **Production T01/RunView acceptance: NOT VERIFIED.** This checkpoint still has no concrete container engine adapter, production assembly, Linux image digest, live network/mount/restart proof, or Run A→B exclusion proof. `docker/craft/opencode.lock.json` retains `container_digest: null`; the provider remains default-off.

## Findings

No remaining critical, high, medium, or low finding within the three-finding repair scope.

## Evidence

1. **Forbidden-root symlink closure:** `prepareSandboxRoot` obtains a canonical prospective path without creating missing segments and checks the forbidden list before `MkdirAll` or `Chmod` (`craft_runview_container_provider.go:715-787`). It checks the final resolved path again and rejects a path changed through a symlink during creation. The regression test points an allowed-looking path into a forbidden home directory and verifies rejection without changing the target's mode (`craft_runview_container_provider_test.go:136-168`). This establishes the stable symlink case; a deployment still needs a trusted dedicated host volume as required by the design.
2. **Inspection tests reach their target:** Each foreign/misconfigured case creates a valid provider container, confirms the durable marker, mutates the fake engine's inspected state, then requires the specific immutable-config inspection error and no second create (`craft_runview_container_provider_test.go:170-224`). The probe case requires the probe-specific error. These assertions would fail if validation were removed or the test short-circuited at the marker.
3. **Exact session confirmation:** The production private API delegates `GetSession` to Task 1's typed inventory (`craft_runview_container_provider.go:161-181`). `FindSessions` performs complete list validation first, then exact-GETs a single candidate; GET error or any ID/project/directory disagreement returns an empty non-authoritative inventory (`:350-384`). Tests cover matching metadata, missing GET, changed ID, project and directory (`craft_runview_container_provider_test.go:304-369`). Zero/multiple list rows remain coordinator-level unresolved/ambiguous cases.
4. **Checkpoint integrity:** SHA-256 matched the fix manifest: provider `2872bb3360bd4f85c52185ae8ea24419089a4b8f4369fedac7ff88fa1d98ae6d`; test `edc72c3dda199405148a198885cbfc6cad091559c2b086f69b5d4e15d12497ae`; full-content patch `6cdc87240105c86e3f401011d41dd788d98850155c8c029e8fd1484ba4c27e3e`.
5. **Independent focused check:** `go test ./internal/container -run '^TestCraftRunViewContainerProvider' -count=1` passed (`ok .../internal/container 2.911s`). The linker emitted only `ignoring duplicate libraries: '-lc++'`. The implementer report records a separate race pass; this review did not repeat it.
