# Craft #107 F07/F28 provenance audit

Date: 2026-09-29  
Scope: read-only audit of the egress adapter, Craft model gateway, durable attempt journal, and prior R2/R3 rulings.  
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`

## Conclusion

No safe implementation task exists at the current seam for distinguishing a terminal upstream 5xx from a proxy-generated or torn 5xx. All ambiguous 5xx responses must remain unresolved (parked) until an authenticated provenance contract is added between the gateway and adapter. The current F07/F28 ruling is therefore supported by the source.

## Verified facts

1. The egress adapter is the attempt authority. `CraftEgressAdapter.ServeHTTP` allocates/reuses a durable opaque attempt ID, sends it as `X-Craft-Activity-ID`, and forwards only the configured gateway URL with an adapter credential ([`internal/modules/craftegress/adapter.go:64-75`](../../internal/modules/craftegress/adapter.go:64), [`internal/modules/craftegress/adapter.go:183-217`](../../internal/modules/craftegress/adapter.go:183)).
2. On a complete response, `gatewayOutcomeIsDefinitive` parks an explicit `ACTIVITY_UNRESOLVED` and otherwise resolves only status `< 500`; its comment explicitly says JSON shape cannot authenticate origin, so every 5xx remains parked ([`internal/modules/craftegress/adapter.go:304-337`](../../internal/modules/craftegress/adapter.go:304)). The only parsed body field is `error.code`, and only to detect `ACTIVITY_UNRESOLVED` for status 409/502 ([`internal/modules/craftegress/adapter.go:307-325`](../../internal/modules/craftegress/adapter.go:307)).
3. On an incomplete/torn body, the adapter discards body-based classification and parks all 5xx plus 409/502; only other non-5xx statuses are treated as definitive ([`internal/modules/craftegress/adapter.go:220-232`](../../internal/modules/craftegress/adapter.go:339-344)). This is the F28 fallback.
4. The journal persists only attempt identity, request digest, state, gateway status, and timestamps. It stores no response headers, body hash, hop identity, signature, or transport provenance ([`internal/modules/craftegress/journal.go:35-45`](../../internal/modules/craftegress/journal.go:201-240)). A non-definitive resolve leaves the request fingerprint mapped to the same parked attempt; a definitive resolve removes that mapping.
5. The gateway's signed/authenticated material authenticates the *request*, not the response. `cmg1` is an HMAC-signed execution credential; `Forward` verifies it and requires the activity header, then resolves/records charge-start state before forwarding ([`internal/handler/craft_model_gateway.go:27-45`](../../internal/handler/craft_model_gateway.go:191-218), [`internal/handler/craft_model_gateway.go:268-296`](../../internal/handler/craft_model_gateway.go:438-506)).
6. Gateway error bodies are ordinary `appFail` JSON. For transport/unknown cases it emits `ACTIVITY_UNRESOLVED`; for known no-start or post-start body failures it emits `UPSTREAM_ERROR`, all commonly with HTTP 502 ([`internal/handler/craft_model_gateway.go:524-613`](../../internal/handler/craft_model_gateway.go:524)). There is no response signature, authenticated hop header, nonce, or gateway-issued response attestation in this path. The usage-record failure headers are supplemental diagnostics and are emitted only on usage persistence failure, not a general response provenance proof ([`internal/handler/craft_model_gateway.go:721-752`](../../internal/handler/craft_model_gateway.go:721)).
7. The adapter copies only `Content-Type` and `X-Craft-Activity-ID` back to its caller; it does not preserve or validate a provenance header ([`internal/modules/craftegress/adapter.go:252-259`](../../internal/modules/craftegress/adapter.go:252)). The configured HTTP client refuses redirects, but that establishes endpoint confinement, not origin authentication ([`internal/modules/craftegress/adapter.go:140-151`](../../internal/modules/craftegress/adapter.go:140)).

## Existing ruling and tests

The R2 report records the selected policy: every complete 5xx remains parked, including a full gateway-shaped 502 `UPSTREAM_ERROR`; torn responses use the status fallback; known terminal 5xx may require reconciliation ([`docs/plans/2026-09-28-craft-107-ocr-r2-egress-report.md:8-12`](2026-09-28-craft-107-ocr-r2-egress-report.md:8), [`...:90-118`](2026-09-28-craft-107-ocr-r2-egress-report.md:90)). It explicitly records the trust limit: an exact envelope is only a protocol convention because a proxy can synthesize it, and no signed assertion or authenticated hop identity exists ([`docs/plans/2026-09-28-craft-107-ocr-r2-egress-report.md:65-69`](2026-09-28-craft-107-ocr-r2-egress-report.md:65)).

The adapter regression matrix encodes this ruling: full or partial `UPSTREAM_ERROR` 502, proxy JSON 502/504, and torn 5xx reuse the same activity ID; non-5xx terminal statuses remint after resolution ([`internal/modules/craftegress/adapter_test.go:301-439`](../../internal/modules/craftegress/adapter_test.go:301)).

## Inference

Because the adapter sees only HTTP status, ordinary response headers/body, and no authenticated response metadata, a terminal upstream 5xx and a proxy-generated/torn 5xx are observationally equivalent at this boundary. Treating any one as definitive would permit a retry to mint a new physical identity while the original call may still be active, creating the double-billing risk documented by the R2 ruling. The request-side HMAC credential and `X-Craft-Activity-ID` do not solve this: they prove who may request the gateway operation, not which hop produced the returned 5xx.

## Safe next task / missing contract

There is no adapter-only implementation to schedule. A future task must first define and own a gateway↔adapter response provenance contract, for example a gateway-issued response attestation bound to activity ID, status, and response digest, authenticated with a key the adapter can verify; or a mutually authenticated hop whose identity is exposed to the adapter. The contract must specify replay binding, behavior on missing/invalid attestations, and whether the attestation survives the trusted proxy. Only after that interface is approved can `gatewayOutcomeIsDefinitive` safely resolve selected 5xx responses and the journal persist the proof metadata. Until then, keep all 5xx unresolved and rely on reconciliation.

## Recommendation

Keep the current F07/F28 policy. Record any future provenance protocol as a separate Spec/ADR/interface change owned jointly by `internal/handler/craft_model_gateway.go` (attestation emission) and `internal/modules/craftegress/adapter.go` (verification/classification), with `internal/modules/craftegress/journal.go` owning durable proof metadata. Do not infer provenance from status, body shape, `Content-Type`, `X-Craft-Activity-ID`, or the existing usage-record diagnostic headers.
