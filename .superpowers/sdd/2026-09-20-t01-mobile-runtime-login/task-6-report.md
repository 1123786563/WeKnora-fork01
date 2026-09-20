# Ticket #31 / Task 6 — real HTTP Runtime login evidence

## Scope

Implemented only the Task 6 HTTP integration harness:

- `packages/api-client/src/mobile/runtime.integration.test.ts`
- `apps/mobile/src/runtime-integration-smoke.ts`

The existing mobile workflow already runs the mobile test and type-check gates.
This integration test is intentionally opt-in because it needs a dedicated,
short-lived deployment account. No workflow change was required.

## Command for an authorized test deployment

```sh
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=https://deployment.example \
WEKNORA_MOBILE_TEST_EMAIL=mobile-test@example.test \
WEKNORA_MOBILE_TEST_PASSWORD=short-lived-secret \
pnpm exec tsx --test packages/api-client/src/mobile/runtime.integration.test.ts
```

The runner accepts only a credential-free HTTPS origin. It uses Node's real
`fetch` through `createJsonTransport`, `createWeKnoraClient`,
`createMobileRuntimeRemote`, and `createMobileRuntime`. The credential store is
in-memory for the process and is never inspected or serialized by the test.

The test prints one JSON evidence record only after an authorized result. Its
permitted fields are:

```json
{
  "deploymentOrigin": "https://deployment.example",
  "clientProtocol": 3,
  "capabilityMode": "compatible",
  "identity": "present",
  "outcome": "authorized",
  "commandTimestamp": "2026-09-21T00:00:00.000Z"
}
```

The record deliberately excludes access tokens, refresh tokens, passwords,
email addresses, request bodies, and raw server responses.

## TDD evidence

### RED

The integration assertion was created before the harness. Its first focused
execution failed because the runner module did not exist:

```text
Error [ERR_MODULE_NOT_FOUND]: Cannot find module
'.../apps/mobile/src/runtime-integration-smoke.ts'
```

### GREEN / absent environment behavior

After adding the runner, the same command in this workspace produced an
explicit skipped test and no authorized result:

```text
MOBILE_RUNTIME_HTTP_SKIPPED: missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL,
WEKNORA_MOBILE_TEST_EMAIL, WEKNORA_MOBILE_TEST_PASSWORD
tests 1 / pass 0 / fail 0 / skipped 1
```

## Verification run

All commands below were run on 2026-09-21 without printing environment values.

```text
pnpm --filter @weknora/mobile typecheck
# exit 0

pnpm exec tsx --test packages/api-client/src/mobile/runtime.integration.test.ts \
  packages/api-client/src/mobile/runtime.test.ts \
  packages/mobile-core/src/runtime/mobile-runtime.test.ts
# tests 21 / pass 20 / fail 0 / skipped 1
# the sole skip is the explicit missing-environment integration test above

go test ./internal/handler -run 'Test(MobileOIDCExchange|OIDCMobile)' -count=1
# ok github.com/Tencent/WeKnora/internal/handler

go test ./internal/router -run '^TestRouterAppliesConfiguredProtocolWindowToCapabilities$' -count=1
# ok github.com/Tencent/WeKnora/internal/router

git diff --check
# exit 0
```

## Local fixture / protected environment investigation

The task environment had no configured `WEKNORA_MOBILE_TEST_*` variables, no
listener at `127.0.0.1:8080`, and no running Compose service reported by
`docker compose ps`. Consequently, there was no local HTTPS WeKnora fixture or
authorized staging credential available to exercise the live path. This report
does not claim live authorization. Running the documented command with an
authorized test deployment remains required to produce `outcome: "authorized"`.

## Review fix round 1

Addressed the review findings without changing the pre-existing production
composition protocol literal, which remains deferred for the final branch
review. The Task 6 runner now imports `CLIENT_PROTOCOL_VERSION` from the
domain package, so its evidence and Runtime use the domain protocol generation.
The mobile app declares that direct workspace dependency explicitly.

`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL` now rejects any non-root path before
normalization; `https://deployment.example/api/v1` cannot be shortened into an
accepted origin. The test emits its already-redacted JSON evidence immediately
after the runner returns and before it asserts authorization. Therefore a live
`not-authorized`, incompatible, or missing-identity outcome is available in
test diagnostics without exposing credentials.

### RED

```text
pnpm exec tsx --test packages/api-client/src/mobile/runtime.integration.test.ts
# SyntaxError: ... does not provide an export named
# 'emitMobileRuntimeIntegrationEvidence'
```

The new tests were written first: one rejected a path-bearing deployment URL;
the other required a deterministic `not-authorized` JSON record that omitted
credential-shaped fields.

### GREEN

```text
pnpm exec tsx --test packages/api-client/src/mobile/runtime.integration.test.ts \
  packages/api-client/src/mobile/runtime.test.ts \
  packages/mobile-core/src/runtime/mobile-runtime.test.ts
# tests 23 / pass 22 / fail 0 / skipped 1
# the single skip remains MOBILE_RUNTIME_HTTP_SKIPPED because no test
# deployment credentials are configured

pnpm --filter @weknora/mobile typecheck
# exit 0

git diff --check
# exit 0
```

The local fixture and protected-environment limitation above is unchanged; no
live authorization was performed or claimed in this fix round.
