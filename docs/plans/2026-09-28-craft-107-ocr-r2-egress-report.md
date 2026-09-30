# Craft-107 OCR R2 egress report

Date: 2026-09-28
Task: F07 and F28 in `internal/modules/craftegress/adapter.go`
Worktree: `/Users/wuyongjun/.codex/worktrees/ocr-egress/WeKnora-fork01`
Base / HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159` (unchanged; no commit)

## Findings and policy

- F07 (current ruling, Round 3/5): every 5xx response remains parked because response JSON shape cannot authenticate which hop produced it. This includes full `appFail`-shaped 502 `UPSTREAM_ERROR`; `ACTIVITY_UNRESOLVED` also remains parked. This avoids a new attempt ID and potential double billing, with known terminal 5xx failures requiring reconciliation.
- F28: a torn response cannot be classified from its body. Apply a status fallback: non-5xx other than 409/502 is terminal; 409/502 and all 5xx remain parked. A torn 2xx therefore resolves the previous physical attempt, allowing a subsequent retry to receive a new ID.
- Existing regression contract: a complete bare 409 with no `ACTIVITY_UNRESOLVED` envelope is a definitive upstream response; an explicit `ACTIVITY_UNRESOLVED` envelope parks regardless of status. This preserves `TestAdapterParksOnlyOnActivityUnresolvedCode` while the torn-body fallback conservatively parks 409/502 by status.

## Changes

- `internal/modules/craftegress/adapter.go`: added separate complete-response and torn-body outcome classifiers; response forwarding uses `bytes.NewReader` to avoid copying the request body through a string.
- `internal/modules/craftegress/adapter_test.go`: added a table for definitive and uncertain complete responses plus an integration matrix asserting ID reuse/reminting after a genuinely truncated response. Updated lost-body fixtures to explicitly represent the gateway's unresolved 502 response.

No authentication, authorization, migration, cancellation, or API contract changes. Credential forwarding remains unchanged. No migration is required.

## Verification

Commands and output (run from the worktree):

```text
go test ./internal/modules/craftegress -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craftegress 0.792s

go test ./internal/modules/craftegress -run 'TestGatewayOutcomeDefinitivePolicy|TestAdapterTornBodyResolutionIsStatusSensitive' -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craftegress 0.688s
```

The full package run includes `TestAdapterParksOnlyOnActivityUnresolvedCode`, which confirms the complete-response 409 compatibility behavior.

## Snapshot evidence

- `internal/modules/craftegress/adapter.go`: `bf44e17029e64ab630fc7288787d98f3395676e9785acc3004c358dbed4f3432`
- `internal/modules/craftegress/adapter_test.go`: `f9de516e1c5a64b40995b8acbecf487d8978c5d540514a63a2f62f78b1077564`
- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; working changes are uncommitted.

## Assumptions and limits

The adapter treats complete non-5xx responses as terminal unless an explicit `ACTIVITY_UNRESOLVED` envelope is present. No 5xx JSON shape proves origin, so all complete and torn 5xx remain parked. Torn-body fallback also parks 409/502. Known terminal upstream 5xx failures may consequently require manual reconciliation; this is the selected tradeoff to avoid minting a new physical identity while the prior send may still be active.

## Scoped review fix: Round 1/5 (F07)

The independent review found that arbitrary parseable JSON on 5xx responses was still accepted as terminal. The initial scoped change accepted only a known gateway terminal envelope. That behavior was superseded by the Round 3 ruling below because the envelope itself does not authenticate its source.

Verification from the worktree:

```text
go test ./internal/modules/craftegress -run 'TestGatewayOutcomeDefinitivePolicy|TestAdapterProxy5xxJSONKeepsAttemptParked|TestAdapterTornBodyResolutionIsStatusSensitive' -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craftegress 0.676s

go test ./internal/modules/craftegress -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craftegress 0.759s
```

Updated hashes:

- `internal/modules/craftegress/adapter.go`: `21d17bcf08a1100cacf67baf30d7ed5720e799c03162e757bebfe799915dffb1`
- `internal/modules/craftegress/adapter_test.go`: `3274b93608a21d902e5233cb591a5351fbfdce9d4ee2efe3db241979a68692cd`
- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit.

## Scoped review fix: Round 2/5 (F07 envelope shape; superseded by Round 3)

The next independent review correctly noted that the short `error.code` fragment does not match the gateway's actual `appFail` response shape. At that stage, the classifier required the full envelope. Round 3 later superseded this policy: all 5xx stay parked because a matching body cannot authenticate its source.

Trust limit: this validates the gateway's documented/source-defined envelope shape; it does not cryptographically authenticate response provenance. A proxy that can synthesize the complete exact shape remains indistinguishable at this adapter boundary. The adapter has no independent signed assertion or authenticated hop identity to verify here. The report therefore treats the exact envelope as a protocol convention, not proof against a malicious or deliberately mimicking proxy.

Verification from the worktree:

```text
go test ./internal/modules/craftegress -run 'TestGatewayOutcomeDefinitivePolicy|TestAdapter502EnvelopeResolutionRequiresFullAppFailShape|TestAdapterProxy5xxJSONKeepsAttemptParked|TestAdapterTornBodyResolutionIsStatusSensitive' -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craftegress 1.034s

go test ./internal/modules/craftegress -count=1
FAIL: TestAdapterParksOnlyOnActivityUnresolvedCode (internal/modules/craftegress/ocr_regression_test.go:125)
expected resolved, got unresolved for the incomplete body {"error":{"code":"UPSTREAM_ERROR"}}
```

The existing `ocr_regression_test.go` assertion conflicted with Round 2's required rejection of incomplete/lookalike envelopes. The parent subsequently authorized a narrow expansion to that test file. It is updated in Round 3 below.

Updated hashes:

- `internal/modules/craftegress/adapter.go`: `2c980d378f41ef3b8230d479e7d11639ffd61fa10dbb422a5173a29a178cb3af`
- `internal/modules/craftegress/adapter_test.go`: `97d3b178c9826a0d6311a143d84221a3905118f69ce4d22a4192c169c02d8b0f`
- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit.

## Scoped review fix: Round 3/5 (F07 provenance ruling / Fix3)

Parent ruling: JSON shape alone cannot establish trusted origin; absent an already-available authenticated provenance marker, every 5xx must remain unresolved. No authentication protocol was added. The adapter classifier now resolves only non-5xx responses without an explicit `ACTIVITY_UNRESOLVED` signal; the existing torn-body fallback continues to resolve only non-5xx other than 409/502. This means known terminal upstream 5xx errors, including the real gateway `UPSTREAM_ERROR` response, retain the attempt ID and can require manual reconciliation. The tradeoff is intentional to prioritize avoiding double billing.

Updated tests so complete appFail-shaped 502, incomplete 502 proxy envelope, proxy JSON 502/504, and torn 5xx retries all reuse the same activity ID. Updated `TestAdapterParksOnlyOnActivityUnresolvedCode` in the newly authorized regression-test file: an incomplete `UPSTREAM_ERROR` 502 now remains unresolved. The integration retry-ID assertion is in `TestAdapterProxy5xxJSONKeepsAttemptParked` and `TestAdapterComplete502EnvelopeKeepsAttemptParked`.

Verification from the worktree:

```text
go test ./internal/modules/craftegress -run 'TestGatewayOutcomeDefinitivePolicy|TestAdapterComplete502EnvelopeKeepsAttemptParked|TestAdapterProxy5xxJSONKeepsAttemptParked|TestAdapterTornBodyResolutionIsStatusSensitive|TestAdapterParksOnlyOnActivityUnresolvedCode' -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craftegress 1.094s

go test ./internal/modules/craftegress -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craftegress 1.600s

git diff --check
<no output; exit 0>
```

Updated hashes:

- `internal/modules/craftegress/adapter.go`: `7b1869acb7982556ca38f4c81c60429165b1728247a272cec16c3a56c97ba7cc`
- `internal/modules/craftegress/adapter_test.go`: `ef69162e0093a40ce894e009a19d40b09880f4457b53676a8f680256e1c93247`
- `internal/modules/craftegress/ocr_regression_test.go`: `7d657ed205f8f9d72c304bcab1e0eef9eb1a51f679a05ac33a54adbf0d717485`
- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit.

## Scoped review follow-up: stale ServeHTTP comment

Updated the ServeHTTP resolution comment to state that all 5xx responses remain parked because their origin cannot be authenticated from status or body shape. Bare non-5xx statuses such as an upstream 409 remain definitive, while explicit `ACTIVITY_UNRESOLVED` also remains parked.

Verification:

```text
go test ./internal/modules/craftegress -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craftegress 1.184s

git diff --check
<no output; exit 0>
```

Updated adapter hash: `9f082b6f51d7a12579618f9026fa4bb720428e5688b6a5d53910c5579e9bc4d4`. Tests and regression test hashes are unchanged from Fix3. HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit.
